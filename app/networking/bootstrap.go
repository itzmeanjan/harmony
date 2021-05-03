package networking

import (
	"context"
	"errors"

	"github.com/itzmeanjan/harmony/app/data"
	"github.com/itzmeanjan/pubsub"
)

var memPool *data.MemPool
var pubsubHub *pubsub.PubSub
var connectionManager *ConnectionManager

// InitMemPool - Initializing mempool handle, in this module
// so that it can be used updating local mempool state, when new
// deserialisable tx chunk is received from any peer, over p2p network
func InitMemPool(pool *data.MemPool) error {

	if pool != nil {
		memPool = pool
		return nil
	}

	return errors.New("bad mempool received in p2p networking handler")

}

// InitRedisClient - Initializing redis client handle, so that all
// subscriptions can be done using this client
func InitRedisClient(client *pubsub.PubSub) error {

	if client != nil {
		pubsubHub = client
		return nil
	}

	return errors.New("bad pub/sub received in p2p networking handler")

}

// Setup - Bootstraps `harmony`'s p2p networking stack
func Setup(ctx context.Context, comm chan struct{}) error {

	if !(memPool != nil && pubsubHub != nil) {
		return errors.New("mempool/ pubsubHub instance not initialised")
	}

	// Attempt to create a new `harmony` node
	// with p2p networking capabilities
	host, err := CreateHost(ctx)
	if err != nil {
		return err
	}

	// Display info regarding this node
	ShowHost(host)
	// Start listening for incoming streams, for supported protocol
	Listen(host)

	go SetUpPeerDiscovery(ctx, host, comm)

	// Starting this worker as a seperate go routine,
	// so that they can manage their own life cycle independently
	connectionManager = NewConnectionManager()
	go connectionManager.Start(ctx)

	return nil

}
