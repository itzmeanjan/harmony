package bootup

import (
	"context"
	"strconv"
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

	// This is communication channel to be used between pending pool
	// & queued pool, so that when new tx gets added into pending pool
	// queued pool also gets notified & gets to update state if required
	alreadyInPendingPoolChan := make(chan *data.MemPoolTx, 4096)
	inPendingPoolChan := make(chan *data.MemPoolTx, 4096)
	lastSeenBlockChan := make(chan uint64, 16)

	// initialising pending pool
	pendingPool := &data.PendingPool{
		Transactions:             make(map[common.Hash]*data.MemPoolTx),
		TxsFromAddress:           make(map[common.Address]data.TxList),
		DroppedTxs:               make(map[common.Hash]time.Time),
		RemovedTxs:               make(map[common.Hash]time.Time),
		AscTxsByGasPrice:         make(data.MemPoolTxsAsc, 0, config.GetPendingPoolSize()),
		DescTxsByGasPrice:        make(data.MemPoolTxsDesc, 0, config.GetPendingPoolSize()),
		Done:                     0,
		LastSeenBlock:            0,
		LastSeenAt:               time.Now().UTC(),
		AddTxChan:                make(chan data.AddRequest, 1),
		AddFromQueuedPoolChan:    make(chan data.AddRequest, 1),
		RemoveTxChan:             make(chan data.RemoveRequest, 1),
		AlreadyInPendingPoolChan: alreadyInPendingPoolChan,
		InPendingPoolChan:        inPendingPoolChan,
		TxExistsChan:             make(chan data.ExistsRequest, 1),
		GetTxChan:                make(chan data.GetRequest, 1),
		CountTxsChan:             make(chan data.CountRequest, 1),
		ListTxsChan:              make(chan data.ListRequest, 1),
		TxsFromAChan:             make(chan data.TxsFromARequest, 1),
		DoneChan:                 make(chan chan uint64, 1),
		SetLastSeenBlockChan:     lastSeenBlockChan,
		LastSeenBlockChan:        make(chan chan data.LastSeenBlock, 1),
		PubSub:                   _redis,
		RPC:                      client,
	}

	// initialising queued pool
	queuedPool := &data.QueuedPool{
		Transactions:      make(map[common.Hash]*data.MemPoolTx),
		TxsFromAddress:    make(map[common.Address]data.TxList),
		DroppedTxs:        make(map[common.Hash]time.Time),
		RemovedTxs:        make(map[common.Hash]time.Time),
		AscTxsByGasPrice:  make(data.MemPoolTxsAsc, 0, config.GetQueuedPoolSize()),
		DescTxsByGasPrice: make(data.MemPoolTxsDesc, 0, config.GetQueuedPoolSize()),
		AddTxChan:         make(chan data.AddRequest, 1),
		RemoveTxChan:      make(chan data.RemovedUnstuckTx, 1),
		TxExistsChan:      make(chan data.ExistsRequest, 1),
		GetTxChan:         make(chan data.GetRequest, 1),
		CountTxsChan:      make(chan data.CountRequest, 1),
		ListTxsChan:       make(chan data.ListRequest, 1),
		TxsFromAChan:      make(chan data.TxsFromARequest, 1),
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
	caughtTxsChan := make(chan listen.CaughtTxs, 16)
	notFoundTxsChan := make(chan listen.CaughtTxs, 16)
	confirmedTxsChan := make(chan data.ConfirmedTx, 4096)

	// Starting pool life cycle manager go routine
	go pool.Pending.Start(ctx)
	// (a)
	//
	// After that this pool will also let (b) know that it can
	// update state of txs, which have become unstuck
	go pool.Pending.Prune(ctx, caughtTxsChan, confirmedTxsChan, notFoundTxsChan)
	go pool.Queued.Start(ctx)
	// (b)
	go pool.Queued.Prune(ctx, confirmedTxsChan, alreadyInPendingPoolChan)

	// This worker will supervise block header listener, so that it can keep
	// track of their health & if they die due to some abnormal reasons
	// it'll spawn a new one after a static delay of x time unit ( see below )
	go func() {

		var died bool

		healthChan := make(chan struct{})
		go listen.SubscribeHead(ctx, wsClient, pool.Pending.GetLastSeenBlock().Number, caughtTxsChan, lastSeenBlockChan, healthChan)

		for {

			if died {
				// Wait before we spawn new worker
				<-time.After(time.Duration(5) * time.Second)

				healthChan = make(chan struct{})
				go listen.SubscribeHead(ctx, wsClient, pool.Pending.GetLastSeenBlock().Number, caughtTxsChan, lastSeenBlockChan, healthChan)

				died = false
			}

			select {

			case <-ctx.Done():
				return

			case <-healthChan:
				died = true

			default:
				// sleep for a while
				<-time.After(time.Duration(1000) * time.Millisecond)
				// and go to work again

			}

		}

	}()

	go data.TrackNotFoundTxs(ctx, inPendingPoolChan, notFoundTxsChan, caughtTxsChan)

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
		WSClient:  wsClient,
		Pool:      pool,
		Redis:     _redis,
		StartedAt: time.Now().UTC(),
		NetworkID: network}, nil

}
