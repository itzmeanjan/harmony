package bootup

import (
	"context"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/itzmeanjan/harmony/app/config"
	"github.com/itzmeanjan/harmony/app/data"
)

// SetGround - This is to be called when starting application
// for doing basic ground work(s), so that all required resources
// are available for further usage during application lifetime
func SetGround(ctx context.Context, file string) (*data.Resource, error) {

	if err := config.Read(file); err != nil {
		return nil, err
	}

	client, err := rpc.DialContext(ctx, config.Get("RPCUrl"))

	if err != nil {
		return nil, err
	}

	return &data.Resource{
		RPCClient: client,
		Pool: &data.MemPool{
			Pending: &data.PendingPool{
				Transactions: make(map[common.Hash]*data.MemPoolTx),
				Lock:         &sync.RWMutex{},
			},
			Queued: &data.QueuedPool{
				Transactions: make(map[common.Hash]*data.MemPoolTx),
				Lock:         &sync.RWMutex{},
			},
		}}, nil

}
