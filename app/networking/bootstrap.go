package networking

import (
	"context"
	"errors"

	"github.com/itzmeanjan/harmony/app/data"
)

var memPool *data.MemPool
var parentCtx context.Context
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

// To be used for listening to event when `harmony` asks its
// workers to stop gracefully
func InitParentContext(ctx context.Context) {
	parentCtx = ctx
}

// Setup - Bootstraps `harmony`'s p2p networking stack
func Setup(ctx context.Context, comm chan struct{}) error {

	if memPool == nil {
		return errors.New("mempool instance not initialised")
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
