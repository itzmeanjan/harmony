package data

import (
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/itzmeanjan/pubsub"
)

// Resource - Shared resources among multiple go routines
//
// Needs to be released carefully when shutting down
type Resource struct {
	RPCClient *rpc.Client
	WSClient  *ethclient.Client
	Pool      *MemPool
	PubSub    *pubsub.PubSub
	StartedAt time.Time
	NetworkID uint64
}

// Release - To be called when application will receive shut down request
// from system, to gracefully deallocate all resources
func (r *Resource) Release() {

	r.RPCClient.Close()
	r.WSClient.Close()

}
