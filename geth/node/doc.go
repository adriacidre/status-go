/*
node - Status Node Management.

The node.Manager needs a params.NodeConfig object to be set up.
You can generate default configuration object (in JSON format):

  lib.GenerateConfig(datadir *C.char, networkId C.int) *C.char

Note: `GenerateConfig()` requires data directory and network id to be present, because for special networks (like testnet) default network parameters (genesis, for instance) are loaded.

To start a node with a given configuration:

  func StartNode(configJSON *C.char) *C.char

To stop the running node:

  func StopNode() *C.char

From time to time it might be necessary to wipe out the chain data, and re-sync, for that:

    func ResetChainData() *C.char

To add peer node:

    func AddPeer(url *C.char) *C.char // adds peer, which main node will listen to

*/
package node

//go:generate autoreadme -f
