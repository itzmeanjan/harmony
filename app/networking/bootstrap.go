package networking

import (
	"context"
	"errors"

	"github.com/go-redis/redis/v8"
)

var redisClient *redis.Client
var parentCtx context.Context

// InitRedisClient - Initializing redis client handle, so that all
// subscriptions can be done using this client
func InitRedisClient(client *redis.Client) error {

	if client != nil {
		redisClient = client
		return nil
	}

	return errors.New("bad redis client received in p2p networking handler")

}

// InitParentContext - Initializing parent context, to be listened by all
// p2p networking subscribers so that graceful shutdown can be done
func InitParentContext(ctx context.Context) {
	parentCtx = ctx
}

// Setup - Bootstraps `harmony`'s p2p networking stack
func Setup(ctx context.Context, comm chan struct{}) error {

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

	return nil

}
