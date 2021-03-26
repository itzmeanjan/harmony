package networking

import (
	"context"
	"log"
	"time"

	"github.com/itzmeanjan/harmony/app/config"
	_discovery "github.com/libp2p/go-libp2p-core/discovery"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
	"github.com/libp2p/go-libp2p-core/protocol"
	discovery "github.com/libp2p/go-libp2p-discovery"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/multiformats/go-multiaddr"
)

// BootstrapPeers - Returns addresses of bootstrap nodes, if none are given
// using default ones
func BootstrapPeers() []multiaddr.Multiaddr {

	addr, err := multiaddr.NewMultiaddr(config.GetBootstrapPeer())
	if err != nil {

		log.Printf("[❗️] Using default bootstrap nodes : %s\n", err.Error())
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

				log.Printf("[❗️] Failed to establish connection with bootstrap node : %s\n", addr)
				return

			}

			log.Printf("➕ Connected to bootstrap node : %s\n", addr)
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

// LookForPeers - Asks this node to start looking for peers, does for some time & attempts to connect
// to them, while setting up stream for further chit-chat
func LookForPeers(ctx context.Context, _host host.Host, _dht *dht.IpfsDHT, routing *discovery.RoutingDiscovery, comm chan struct{}) {

	defer close(comm)

	peerChan, err := routing.FindPeers(ctx, config.GetNetworkingRendezvous())
	if err != nil {

		log.Printf("[❗️] Failed to start finding peers : %s\n", err.Error())
		return

	}

	{
	OUTER:
		for {

		INNER:
			select {

			// Keeping track of signal, whether main go routine is asking
			// this one to stop, because application is going done, so it's better to
			// attempt graceful shutdown
			case <-ctx.Done():

				if err := _dht.Close(); err != nil {
					log.Printf("[❗️] Failed to stop peer discovery mechanism : %s\n", err.Error())
				}

				break OUTER

			case found := <-peerChan:

				if found.ID.String() == "" {
					break OUTER
				}

				// this is me 😅
				if found.ID == _host.ID() {
					break INNER
				}

				// Adding peer info in local peer store, so that we can attempt to connect to
				// this peer in near future, if connection is found to be lost due to some
				// unforeseeable reasons
				_host.Peerstore().AddAddrs(found.ID, found.Addrs, peerstore.PermanentAddrTTL)

				stream, err := _host.NewStream(ctx, found.ID, protocol.ID(config.GetNetworkingStream()))
				if err != nil {

					log.Printf("[❗️] Failed to connect to discovered peer : %s\n", found)
					break INNER

				}

				func(stream network.Stream) {
					go HandleStream(stream)
				}(stream)

				log.Printf("✅ Connected to new discovered peer : %s\n", found)

			}

		}
	}

}

// SetUpPeerDiscovery - Setting up peer discovery mechanism, by connecting
// to bootstrap nodes first, then advertises self with rendezvous & attempts to
// discover peers with same rendezvous, which are to be eventually connected with
func SetUpPeerDiscovery(ctx context.Context, _host host.Host, comm chan struct{}) {

	_dht, err := dht.New(ctx, _host, dht.Mode(dht.ModeOpt(config.GetPeerDiscoveryMode())))
	if err != nil {

		log.Printf("[❗️] Failed to create DHT : %s\n", err.Error())

		close(comm)
		return

	}

	if err := _dht.Bootstrap(ctx); err != nil {

		log.Printf("[❗️] Failed to keep refreshing DHT : %s\n", err.Error())

		close(comm)
		return

	}

	connected, total := ConnectToBootstraps(ctx, _host)
	log.Printf("✅ Connected to %d/ %d bootstrap nodes\n", connected, total)

	routingDiscovery := discovery.NewRoutingDiscovery(_dht)
	routingDiscovery.Advertise(
		ctx,
		config.GetNetworkingRendezvous(),
		_discovery.TTL(time.Duration(1)*time.Hour), // Published record of self to stay floating for <= 1 hour
		_discovery.Limit(100))

	log.Printf("✅ Advertised self with rendezvous\n")

	workerComm := make(chan struct{}, 1)

	for {

		log.Printf("✅ Started looking for peers\n")
		go LookForPeers(ctx, _host, _dht, routingDiscovery, workerComm)
		<-workerComm

		log.Printf("✅ Stopped looking for peers\n")
		<-time.After(time.Minute * time.Duration(5))

	}

}
