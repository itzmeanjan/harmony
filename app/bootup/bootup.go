package bootup

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/go-redis/redis/v8"
	"github.com/itzmeanjan/harmony/app/config"
	"github.com/itzmeanjan/harmony/app/data"
)

// GetNetwork - Make RPC call for reading network ID
func GetNetwork(ctx context.Context, rpc *rpc.Client) (uint64, error) {

	var result string

	if err := rpc.CallContext(ctx, &result, "net_version"); err != nil {
		return 0, err
	}

	_result, err := strconv.ParseUint(result, 10, 64)
	if err != nil {
		return 0, err
	}

	return _result, nil

}

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

	var options *redis.Options

	// If password is given in config file
	if config.Get("RedisPassword") != "" {

		options = &redis.Options{
			Network:  config.Get("RedisConnection"),
			Addr:     config.Get("RedisAddress"),
			Password: config.Get("RedisPassword"),
			DB:       int(config.GetRedisDBIndex()),
		}

	} else {
		// If password is not given, attempting to connect with out it
		//
		// Though this is not recommended in production environment
		options = &redis.Options{
			Network: config.Get("RedisConnection"),
			Addr:    config.Get("RedisAddress"),
			DB:      int(config.GetRedisDBIndex()),
		}

	}

	_redis := redis.NewClient(options)
	// Checking whether connection was successful or not
	if err := _redis.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	// Attempt to read current network ID
	network, err := GetNetwork(ctx, client)
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
		},
		Redis:     _redis,
		StartedAt: time.Now().UTC(),
		NetworkID: network}, nil

}
