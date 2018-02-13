package node

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/eth/downloader"
	"github.com/ethereum/go-ethereum/les"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p/discover"
	whisper "github.com/ethereum/go-ethereum/whisper/whisperv5"
	"github.com/labstack/gommon/log"
	"github.com/status-im/status-go/geth/params"
	"github.com/status-im/status-go/geth/rpc"
)

// tickerResolution is the delta to check blockchain sync progress.
const tickerResolution = time.Second

// RPCClientError reported when rpc client is initialized.
type RPCClientError error

// EthNodeError is reported when node crashed on start up.
type EthNodeError error

type statusNode struct {
	node           *node.Node         // reference to Geth P2P stack/node
	config         *params.NodeConfig // Status node configuration
	whisperService *whisper.Whisper   // reference to Whisper service
	lesService     *les.LightEthereum // reference to LES service
	rpcClient      *rpc.Client        // reference to RPC client
}

// newStatusNode returns an initilaized and started new statusNode and the
// associated error in case something is broken.
func newStatusNode(config *params.NodeConfig, fn node.ServiceConstructor) (*statusNode, error) {
	snode := statusNode{}

	if err := snode.init(config, fn); err != nil {
		return nil, err
	}
	if err := snode.start(); err != nil {
		return nil, err
	}

	return &snode, nil
}

// init initializes a statusNode based on the given config and mailServer and
// returns an error if needed
func (s *statusNode) init(config *params.NodeConfig, mailServer node.ServiceConstructor) error {
	ethNode, err := MakeNode(config)
	if err != nil {
		return err
	}

	s.node = ethNode
	s.config = config

	// activate MailService required for Offline Inboxing
	if err := ethNode.Register(mailServer); err != nil {
		return err
	}

	return nil
}

// start starts the underlying node
func (s *statusNode) start() error {
	// start underlying node
	if err := s.node.Start(); err != nil {
		return EthNodeError(err)
	}
	// init RPC client for this node
	localRPCClient, err := s.node.Attach()
	if err == nil {
		s.rpcClient, err = rpc.NewClient(localRPCClient, s.config.UpstreamConfig)
	}
	if err != nil {
		log.Error("Failed to create an RPC client", "error", err)
		return RPCClientError(err)
	}
	// populate static peers exits when node stopped
	go func() {
		if err := s.populateStaticPeers(); err != nil {
			log.Error("Static peers population", "error", err)
		}
	}()

	return nil
}

// stop resets the current statusNode to its initial status
func (s *statusNode) stop() error {
	if err := s.node.Stop(); err != nil {
		return err
	}
	s.node = nil
	s.config = nil
	s.lesService = nil
	s.whisperService = nil
	s.rpcClient = nil

	return nil
}

// populateStaticPeers connects current node with status publicly available
// LES/SHH/Swarm cluster defined on the config
func (s *statusNode) populateStaticPeers() error {
	if !s.config.BootClusterConfig.Enabled {
		log.Info("Boot cluster is disabled")
		return nil
	}

	for _, enode := range s.config.BootClusterConfig.BootNodes {
		err := s.addPeer(enode)
		if err != nil {
			log.Warn("Boot node addition failed", "error", err)
			continue
		}
		log.Info("Boot node added", "enode", enode)
	}

	return nil
}

// addPeer adds new static peer to the underlying node
func (s *statusNode) addPeer(url string) error {
	// Try to add the url as a static peer and return
	parsedNode, err := discover.ParseNode(url)
	if err != nil {
		return err
	}
	s.node.Server().AddPeer(parsedNode)
	return nil
}

// peerCount gets the count of the node attached peers
func (s *statusNode) peerCount() int {
	return s.node.Server().PeerCount()
}

// isAvailable check if we have a node running and make sure is fully started
func (s *statusNode) isAvailable() bool {
	if s.node == nil || s.node.Server() == nil {
		return false
	}
	return true
}

// getNode returns the unrerlying node
func (s *statusNode) getNode() *node.Node {
	return s.node
}

// getLightEthereumService returns the LES service or error if there is any
// problem accessing it
func (s *statusNode) getLightEthereumService() (*les.LightEthereum, error) {
	if s.lesService == nil {
		if err := s.node.Service(&s.lesService); err != nil {
			log.Warn("Cannot obtain LES service", "error", err)
			return nil, ErrInvalidLightEthereumService
		}
	}
	if s.lesService == nil {
		return nil, ErrInvalidLightEthereumService
	}
	return s.lesService, nil
}

// getWhisperService returns the whisper service or error if there is any
// problem accessing it
func (s *statusNode) getWhisperService() (*whisper.Whisper, error) {
	if s.whisperService == nil {
		if err := s.node.Service(&s.whisperService); err != nil {
			log.Warn("Cannot obtain whisper service", "error", err)
			return nil, ErrInvalidWhisperService
		}
	}
	if s.whisperService == nil {
		return nil, ErrInvalidWhisperService
	}
	return s.whisperService, nil
}

// getAccountManager exposes reference to node's accounts manager
func (s *statusNode) getAccountManager() (*accounts.Manager, error) {
	accountManager := s.node.AccountManager()
	if accountManager == nil {
		return nil, ErrInvalidAccountManager
	}
	return accountManager, nil
}

// getAccountKeyStore exposes reference to accounts key store
func (s *statusNode) getAccountKeyStore() (*keystore.KeyStore, error) {
	accountManager, err := s.getAccountManager()
	if err != nil {
		return nil, err
	}

	backends := accountManager.Backends(keystore.KeyStoreType)
	if len(backends) == 0 {
		return nil, ErrAccountKeyStoreMissing
	}

	keyStore, ok := backends[0].(*keystore.KeyStore)
	if !ok {
		return nil, ErrAccountKeyStoreMissing
	}

	return keyStore, nil
}

// getRPCClient exposes reference to RPC client connected to the running node.
func (s *statusNode) getRPCClient() *rpc.Client {
	return s.rpcClient
}

// getConfig returns the current statusNode configuration
func (s *statusNode) getConfig() *params.NodeConfig {
	return s.config
}

// resetChainData removes chain data if node is not running.
func (s *statusNode) resetChainData() error {
	config := s.getConfig()
	chainDataDir := filepath.Join(config.DataDir, config.Name, "lightchaindata")
	if _, err := os.Stat(chainDataDir); os.IsNotExist(err) {
		// is it really an error, if we want to remove it as next step?
		return err
	}
	err := os.RemoveAll(chainDataDir)
	if err == nil {
		log.Info("Chain data has been removed", "dir", chainDataDir)
	}

	return err
}

// getLESDownloader gets the related LES downloader or an error
func (s *statusNode) getLESDownloader() (*downloader.Downloader, error) {
	les, err := s.getLightEthereumService()
	if err != nil {
		return nil, fmt.Errorf("failed to get LES service: %v", err)
	}

	downloader := les.Downloader()
	if downloader == nil {
		return nil, errors.New("LightEthereumService downloader is nil")
	}

	return downloader, nil
}

// isSyncCompleted returns true in case the blockchain synchronization process
// is complete
func (s *statusNode) isSyncCompleted(progress ethereum.SyncProgress) bool {
	return s.peerCount() > 0 && progress.CurrentBlock >= progress.HighestBlock
}

// isLocalPrivateChain check if the configured network id is the same as the
// default test network (private chain)
func (s *statusNode) isLocalPrivateChain() bool {
	return s.config.NetworkID == params.StatusChainNetworkID
}

// ensureSync waits until blockchain synchronization is complete.
func (s *statusNode) ensureSync(ctx context.Context) error {
	downloader, err := s.getLESDownloader()
	if err != nil {
		return err
	}

	progress := downloader.Progress()
	if s.isSyncCompleted(progress) {
		log.Debug("Synchronization completed", "current block", progress.CurrentBlock, "highest block", progress.HighestBlock)
		return nil
	}

	ticker := time.NewTicker(tickerResolution)
	defer ticker.Stop()

	progressTicker := time.NewTicker(time.Minute)
	defer progressTicker.Stop()

	for {
		if done, err := s.tickManager(ctx, downloader, ticker, progressTicker); done {
			return err
		}
	}
}

func (s *statusNode) tickManager(ctx context.Context, downloader *downloader.Downloader, ticker, progressTicker *time.Ticker) (bool, error) {
	select {
	case <-ctx.Done():
		return true, errors.New("timeout during node synchronization")
	case <-ticker.C:
		if s.peerCount() == 0 {
			log.Debug("No established connections with any peers, continue waiting for a sync")
			return false, nil
		}
		if downloader.Synchronising() {
			log.Debug("Synchronization is in progress")
			return false, nil
		}
		progress := downloader.Progress()
		if progress.CurrentBlock >= progress.HighestBlock {
			log.Info("Synchronization completed", "current block", progress.CurrentBlock, "highest block", progress.HighestBlock)
			return true, nil
		}
		log.Debug("Synchronization is not finished", "current", progress.CurrentBlock, "highest", progress.HighestBlock)
	case <-progressTicker.C:
		progress := downloader.Progress()
		log.Warn("Synchronization is not finished", "current", progress.CurrentBlock, "highest", progress.HighestBlock)
	}

	return false, nil
}
