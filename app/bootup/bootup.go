package bootup

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/go-redis/redis/v8"
	"github.com/itzmeanjan/harmony/app/config"
	"github.com/itzmeanjan/harmony/app/data"
	"github.com/itzmeanjan/harmony/app/graph"
	"github.com/itzmeanjan/harmony/app/listen"
	"github.com/itzmeanjan/harmony/app/networking"
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

	wsClient, err := ethclient.DialContext(ctx, config.Get("WSUrl"))
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

	// Passed this redis client handle to graphql query resolver
	//
	// To be used when subscription requests are received from clients
	if err := graph.InitRedisClient(_redis); err != nil {
		return nil, err
	}

	// Redis client to be used in p2p networking communication
	// handling section for letting clients know of some newly
	// seen mempool tx
	if err := networking.InitRedisClient(_redis); err != nil {
		return nil, err
	}

	// Attempt to read current network ID
	network, err := GetNetwork(ctx, client)
	if err != nil {
		return nil, err
	}

	// initialising pending pool
	pendingPool := &data.PendingPool{
		Transactions:      make(map[common.Hash]*data.MemPoolTx),
		AscTxsByGasPrice:  make(data.MemPoolTxsAsc, 0, 1024),
		DescTxsByGasPrice: make(data.MemPoolTxsDesc, 0, 1024),
		Lock:              &sync.RWMutex{},
		IsPruning:         false,
		AddTxChan:         make(chan data.AddRequest, 1),
		RemoveTxChan:      make(chan data.RemoveRequest, 1),
		TxExistsChan:      make(chan data.ExistsRequest, 1),
		GetTxChan:         make(chan data.GetRequest, 1),
		CountTxsChan:      make(chan data.CountRequest, 1),
		ListTxsChan:       make(chan data.ListRequest, 1),
		PubSub:            _redis,
		RPC:               client,
	}

	// initialising queued pool
	queuedPool := &data.QueuedPool{
		Transactions:      make(map[common.Hash]*data.MemPoolTx),
		AscTxsByGasPrice:  make(data.MemPoolTxsAsc, 0, 1024),
		DescTxsByGasPrice: make(data.MemPoolTxsDesc, 0, 1024),
		Lock:              &sync.RWMutex{},
		IsPruning:         false,
		AddTxChan:         make(chan data.AddRequest, 1),
		RemoveTxChan:      make(chan data.RemovedUnstuckTx, 1),
		TxExistsChan:      make(chan data.ExistsRequest, 1),
		GetTxChan:         make(chan data.GetRequest, 1),
		CountTxsChan:      make(chan data.CountRequest, 1),
		ListTxsChan:       make(chan data.ListRequest, 1),
		PubSub:            _redis,
		RPC:               client,
		PendingPool:       pendingPool,
	}

	pool := &data.MemPool{
		Pending: pendingPool,
		Queued:  queuedPool,
	}

	// Block head listener & pending pool pruner
	// talks over this buffered channel
	commChan := make(chan *listen.CaughtTx, 1024)

	// Starting pool life cycle manager go routine
	go pool.Pending.Start(ctx)
	// (a)
	go pool.Pending.Prune(ctx, commChan)
	go pool.Queued.Start(ctx)
	go pool.Queued.Prune(ctx)
	// Listens for new block headers & informs 👆 (a) for pruning
	// txs which can be/ need to be
	go listen.SubscribeHead(ctx, wsClient, commChan)

	// Passed this mempool handle to graphql query resolver
	if err := graph.InitMemPool(pool); err != nil {
		return nil, err
	}

	// To be used when updating mempool state, in action of
	// seeing new tx
	if err := networking.InitMemPool(pool); err != nil {
		return nil, err
	}

	// Passing parent context to graphQL subscribers, so that
	// graceful system shutdown can be performed
	graph.InitParentContext(ctx)

	return &data.Resource{
		RPCClient: client,
		Pool:      pool,
		Redis:     _redis,
		StartedAt: time.Now().UTC(),
		NetworkID: network}, nil

}
