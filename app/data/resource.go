package data

import (
	"log"
	"time"

	"github.com/ethereum/go-ethereum/rpc"
	"github.com/go-redis/redis/v8"
)

// Resource - Shared resources among multiple go routines
//
// Needs to be released carefully when shutting down
type Resource struct {
	RPCClient *rpc.Client
	Pool      *MemPool
	Redis     *redis.Client
	StartedAt time.Time
}

// Release - To be called when application will receive shut down request
// from system, to gracefully deallocate all resources
func (r *Resource) Release() {

	r.RPCClient.Close()
	if err := r.Redis.Close(); err != nil {
		log.Printf("[❗️] Failed to close redis client : %s\n", err.Error())
	}

}
