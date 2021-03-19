package networking

import (
	"context"
	"log"

	"github.com/itzmeanjan/harmony/app/config"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/multiformats/go-multiaddr"
)

// BootstrapPeers - Returns addresses of bootstrap nodes, if none are given
// using default ones
func BootstrapPeers() []multiaddr.Multiaddr {

	addr, err := multiaddr.NewMultiaddr(config.GetBootstrapPeer())
	if err != nil {

		log.Printf("[❗️] Failed to parse bootstrap node : %s\n", err.Error())
		return dht.DefaultBootstrapPeers

	}

	return []multiaddr.Multiaddr{addr}

}

// ConnectToBootstraps - Attempting to connect to bootstrap nodes concurrently
// Waiting for all of them to complete, after that returning back how many
// attempts went successful among total attempts, respectively
func ConnectToBootstraps(ctx context.Context, _host host.Host) (int, int) {

	bootstrapPeers := BootstrapPeers()
	expected := len(bootstrapPeers)
	connectBoot := make(chan bool, expected)

	for _, addr := range bootstrapPeers {

		go func(addr multiaddr.Multiaddr) {

			var status bool
			defer func() {

				connectBoot <- status

			}()

			_peer, err := peer.AddrInfoFromP2pAddr(addr)
			if err != nil {

				log.Printf("[❗️] Failed to get peer address from multi address : %s\n", err.Error())
				return

			}

			if err := _host.Connect(ctx, *_peer); err != nil {

				log.Printf("[❗️] Failed to establish connection with bootstrap node(s) : %s\n", err.Error())
				return

			}

			status = true

		}(addr)

	}

	var failure int
	var success int

	for v := range connectBoot {

		if v {
			success++
		} else {
			failure++
		}

		if success+failure == expected {

			break

		}
	}

	return success, expected

}

// SetUpPeerDiscovery - ...
func SetUpPeerDiscovery(ctx context.Context, _host host.Host) {

	_dht, err := dht.New(ctx, _host)
	if err != nil {

		log.Printf("[❗️] Failed to create DHT : %s\n", err.Error())
		return

	}

	if err := _dht.Bootstrap(ctx); err != nil {

		log.Printf("[❗️] Failed to keep refreshing DHT : %s\n", err.Error())
		return

	}

}
