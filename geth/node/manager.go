package node

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/les"
	"github.com/ethereum/go-ethereum/node"
	whisper "github.com/ethereum/go-ethereum/whisper/whisperv5"
	"github.com/status-im/status-go/geth/log"
	"github.com/status-im/status-go/geth/mailservice"
	"github.com/status-im/status-go/geth/params"
	"github.com/status-im/status-go/geth/rpc"
)

// errors
var (
	ErrNodeExists                  = errors.New("node is already running")
	ErrNoRunningNode               = errors.New("there is no running node")
	ErrInvalidNodeManager          = errors.New("node manager is not properly initialized")
	ErrInvalidWhisperService       = errors.New("whisper service is unavailable")
	ErrInvalidLightEthereumService = errors.New("LES service is unavailable")
	ErrInvalidAccountManager       = errors.New("could not retrieve account manager")
	ErrAccountKeyStoreMissing      = errors.New("account key store is not set")
	ErrRPCClient                   = errors.New("failed to init RPC client")
)

// Manager : manages a StatusNode (which abstracts contained geth node)
type Manager interface {
	// StartNode initializes and starts a StatusNode based on the given
	// configuration parameter
	StartNode(config *params.NodeConfig) error

	// StopNode stops the current running StatusNode.
	StopNode() error

	// ResetChainData removes the chain data if node is not running
	ResetChainData() error

	// Node gets the underlying geth node related with the current manager.
	Node() (*node.Node, error)

	// PopulateStaticPeers connects manager related node with the default
	// boot enodes (LES/SHH/Swarm clusters) set up on the Status node config
	// "NodeConfig.BootClusterConfig.BootNodes"
	PopulateStaticPeers() error

	// AddPeer adds a new static peer to the manager related node
	AddPeer(url string) error

	// PeerCount returns the number of configured peers on the related StatusNode,
	// in case there is no available Node it will return "0"
	PeerCount() int

	// NodeConfig retrieves the NodeConfig provided during the StatusNode
	// initialization
	NodeConfig() (*params.NodeConfig, error)

	// LightEthereumService gets the assiciated node LES service.
	LightEthereumService() (*les.LightEthereum, error)

	// WhisperService gets the assiciated node whisper service.
	WhisperService() (*whisper.Whisper, error)

	// AccountManager gets the assiciated node account manager.
	AccountManager() (*accounts.Manager, error)

	// AccountKeyStore gets the assiciated node account keyStore.
	AccountKeyStore() (*keystore.KeyStore, error)

	// RPCClient exposes reference to RPC client connected to the running node.
	RPCClient() *rpc.Client

	// EnsureSync waits until blockchain synchronization is completed and returns.
	EnsureSync(ctx context.Context) error

	// IsNodeRunning confirm that node is running
	IsNodeRunning() bool
}

// manager manages Status node (which abstracts contained geth node)
type manager struct {
	mu    sync.RWMutex
	snode *statusNode
}

// NewManager makes new instance of manager
func NewManager() Manager {
	return &manager{snode: &statusNode{}}
}

// StartNode initializes and starts a StatusNode based on the given
// configuration parameter
// In addition it will setup global logger with the values provided on the
// config parameter.
// It will return an ErrNodeExists indicating the node has already been
// started
func (m *manager) StartNode(config *params.NodeConfig) error {
	_, err := m.availableNode()
	if err == nil {
		return ErrNodeExists
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.initLog(config)
	m.snode, err = newStatusNode(config, func(_ *node.ServiceContext) (node.Service, error) {
		return mailservice.New(m), nil
	})

	return err
}

// StopNode stops the current running StatusNode. It will return an
// error ErrNoRunningNode in case the node is not available
func (m *manager) StopNode() error {
	snode, err := m.availableNode()
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	return snode.stop()
}

// ResetChainData removes chain data if node is not running.
func (m *manager) ResetChainData() error {
	snode, err := m.availableNode()
	if err != nil {
		return ErrNoRunningNode
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	return snode.resetChainData()
}

// Node gets the underlying geth node related with the current manager.
// Additionally it will return an error ErrNoRunningNode in case the node
// is not available
func (m *manager) Node() (*node.Node, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snode, err := m.availableNode()
	if err != nil {
		return nil, err
	}
	return snode.getNode(), nil
}

// PopulateStaticPeers connects current node with status publicly available
// LES/SHH/Swarm cluster as defined on the StatusNode config
func (m *manager) PopulateStaticPeers() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.populateStaticPeers()
}

// PopulateStaticPeers connects manager related node with the default
// boot enodes (LES/SHH/Swarm clusters) set up on the Status node config
// "NodeConfig.BootClusterConfig.BootNodes"
// It will return an ErrNoRunningNode indicating the node hasn't been
// initialized.
func (m *manager) populateStaticPeers() error {
	snode, err := m.availableNode()
	if err != nil {
		return err
	}

	return snode.populateStaticPeers()
}

// AddPeer adds a new static peer to the manager related node
// It will return an ErrNoRunningNode indicating the node hasn't been
// initialized.
func (m *manager) AddPeer(url string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snode, err := m.availableNode()
	if err != nil {
		return err
	}

	return snode.addPeer(url)
}

// PeerCount returns the number of configured peers on the related StatusNode,
// in case there is no available Node it will return "0"
func (m *manager) PeerCount() int {
	snode, err := m.availableNode()
	if err != nil {
		return 0
	}

	return snode.peerCount()
}

// NodeConfig retrieves the NodeConfig provided during the StatusNode
// initialization, additionally will return an ErrNoRunningNode in case the
// related node is not yet initialized
func (m *manager) NodeConfig() (*params.NodeConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snode, err := m.availableNode()
	if err != nil {
		return nil, err
	}

	return snode.getConfig(), nil
}

// LightEthereumService gets the assiciated node LES service. Additionally it
// will return an error in case the ErrNoRunningNode indicating the node hasn't
// been initialized or an ErrInvalidLightEthereumService in case the LES
// service is not accessible.
func (m *manager) LightEthereumService() (*les.LightEthereum, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snode, err := m.availableNode()
	if err != nil {
		return nil, err
	}

	return snode.getLightEthereumService()
}

// WhisperService gets the assiciated node whisper service. Additionally it
// will return an error in case the ErrNoRunningNode indicating the node hasn't
// been initialized or an ErrInvalidWhisperService in case the Whisper
// service is not accessible.
func (m *manager) WhisperService() (*whisper.Whisper, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snode, err := m.availableNode()
	if err != nil {
		return nil, err
	}

	return snode.getWhisperService()
}

// AccountManager gets the assiciated node account manager. Additionally it
// will return an error in case the ErrNoRunningNode indicating the node hasn't
// been initialized or an ErrInvalidAccountManager in case the account manager
// is not set.
func (m *manager) AccountManager() (*accounts.Manager, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snode, err := m.availableNode()
	if err != nil {
		return nil, err
	}

	return snode.getAccountManager()
}

// AccountKeyStore gets the assiciated node account keyStore. Additionally
// it  will return an error in case the ErrNoRunningNode indicating the node
// hasn't been initialized or an ErrAccountKeyStoreMissing in case the
// KeyStore of the backend is not correctly set.
func (m *manager) AccountKeyStore() (*keystore.KeyStore, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snode, err := m.availableNode()
	if err != nil {
		return nil, err
	}

	return snode.getAccountKeyStore()
}

// RPCClient exposes reference to RPC client connected to the running node.
func (m *manager) RPCClient() *rpc.Client {
	m.mu.Lock()
	defer m.mu.Unlock()

	snode, err := m.availableNode()
	if err != nil {
		return nil
	}

	return snode.getRPCClient()
}

// IsNodeRunning confirm that node is running
func (m *manager) IsNodeRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, err := m.availableNode()

	return err == nil
}

// EnsureSync waits until blockchain synchronization is complete and returns
// in case it's not a local private chain (testing), otherwise it will directly
// return.
func (m *manager) EnsureSync(ctx context.Context) error {
	snode, err := m.availableNode()
	if err != nil {
		return err
	}

	// Don't wait for any blockchain sync for the local private chain as blocks
	// are never mined.
	if snode.isLocalPrivateChain() {
		return nil
	}

	return snode.ensureSync(ctx)
}

// availableNode gets the related StatusNode assigned to the current manager
// in case it's available. Additionally it will returns an ErrNoRunningNode
// both in case it's not yet initialized or started.
func (m *manager) availableNode() (*statusNode, error) {
	if m.snode == nil {
		return nil, ErrNoRunningNode
	}
	if !m.snode.isAvailable() {
		return nil, ErrNoRunningNode
	}

	return m.snode, nil
}

// initLog initializes global logger parameters based on
// provided node configurations.
func (m *manager) initLog(config *params.NodeConfig) {
	log.SetLevel(config.LogLevel)

	if config.LogFile != "" {
		err := log.SetLogFile(config.LogFile)
		if err != nil {
			fmt.Println("Failed to open log file, using stdout")
		}
	}
}
