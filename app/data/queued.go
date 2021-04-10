package data

import (
	"context"
	"log"
	"runtime"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/gammazero/workerpool"
	"github.com/go-redis/redis/v8"
	"github.com/itzmeanjan/harmony/app/config"
)

// QueuedPool - Currently present queued tx(s) i.e. these tx(s) are stuck
// due to some reasons, one very common reason is nonce gap
//
// What it essentially denotes is, these tx(s) are not ready to be picked up
// when next block is going to be picked, when these tx(s) are going to be
// moved to pending pool, only they can be considered before mining
type QueuedPool struct {
	Transactions      map[common.Hash]*MemPoolTx
	AscTxsByGasPrice  TxList
	DescTxsByGasPrice TxList
	Lock              *sync.RWMutex
	IsPruning         bool
	AddTxChan         chan AddRequest
	RemoveTxChan      chan RemovedUnstuckTx
	TxExistsChan      chan ExistsRequest
	GetTxChan         chan GetRequest
	CountTxsChan      chan CountRequest
	ListTxsChan       chan ListRequest
	PubSub            *redis.Client
	RPC               *rpc.Client
	PendingPool       *PendingPool
}

// Start - This method is supposed to be started as a
// seperate go routine which will manage queued pool ops
// through out its life
func (q *QueuedPool) Start(ctx context.Context) {

	txRemover := func(txHash common.Hash) *MemPoolTx {

		// -- Safely reading from map, begins
		q.Lock.RLock()

		tx, ok := q.Transactions[txHash]
		if !ok {

			q.Lock.RUnlock()
			return nil

		}

		q.Lock.RUnlock()
		// -- reading ends

		tx.UnstuckAt = time.Now().UTC()

		// -- Safe writing begins, critical section
		q.Lock.Lock()

		// Remove from sorted tx list, keep it sorted
		q.AscTxsByGasPrice = Remove(q.AscTxsByGasPrice, tx)
		q.DescTxsByGasPrice = Remove(q.DescTxsByGasPrice, tx)

		delete(q.Transactions, txHash)

		q.Lock.Unlock()
		// -- ends

		// Publishing unstuck tx, this is probably going to
		// enter pending pool now
		q.PublishRemoved(ctx, q.PubSub, q.Transactions[txHash])

		return tx

	}

	// Closure for checking whether adding new tx triggers
	// condition for dropping some other tx
	//
	// Selecting which tx to be dropped
	//
	// - Tx with lowest gas price paid ✅
	// - Oldest tx living in mempool ❌
	// - Oldest tx with lowest gas price paid ❌
	//
	// ✅ : Implemented
	// ❌ : Not yet
	needToDropTxs := func() bool {

		q.Lock.RLock()
		defer q.Lock.RUnlock()

		return uint64(q.AscTxsByGasPrice.len())+1 > config.GetQueuedPoolSize()

	}

	pickTxWithLowestGasPrice := func() *MemPoolTx {

		q.Lock.RLock()
		defer q.Lock.RUnlock()

		return q.AscTxsByGasPrice.get()[0]

	}

	// Silently drop some tx, before adding
	// new one, so that we don't exceed limit
	// set up by user
	dropTx := func() {

		// because this is the only scheme currently
		// supported
		tx := pickTxWithLowestGasPrice()

		// -- Safe writing, critical section begins
		q.Lock.Lock()
		defer q.Lock.Unlock()

		// Remove from sorted tx list, keep it sorted
		q.AscTxsByGasPrice = Remove(q.AscTxsByGasPrice, tx)
		q.DescTxsByGasPrice = Remove(q.DescTxsByGasPrice, tx)

		delete(q.Transactions, tx.Hash)

	}

	for {

		select {

		case <-ctx.Done():
			return
		case req := <-q.AddTxChan:

			// This is what we're trying to add
			tx := req.Tx

			// -- Safe reading begins
			q.Lock.RLock()

			if _, ok := q.Transactions[tx.Hash]; ok {
				req.ResponseChan <- false

				q.Lock.RUnlock()
				break
			}

			q.Lock.RUnlock()
			// -- ends here

			if needToDropTxs() {
				dropTx()
				log.Printf("🧹 Dropped queued tx, was about to hit limit\n")
			}

			// Marking we found this tx in mempool now
			tx.QueuedAt = time.Now().UTC()
			tx.Pool = "queued"

			// -- Critical section begins
			q.Lock.Lock()

			// Creating entry
			q.Transactions[tx.Hash] = tx

			// Insert into sorted pending tx list, keep sorted
			q.AscTxsByGasPrice = Insert(q.AscTxsByGasPrice, tx)
			q.DescTxsByGasPrice = Insert(q.DescTxsByGasPrice, tx)

			q.Lock.Unlock()
			// -- ends here

			// As soon as we find new entry for queued pool
			// we publish that tx to pubsub topic
			q.PublishAdded(ctx, q.PubSub, tx)

			req.ResponseChan <- true

		case req := <-q.RemoveTxChan:

			// if removed will return non-nil reference to removed tx
			req.ResponseChan <- txRemover(req.Hash)

		case req := <-q.TxExistsChan:

			// -- Attempt to read safely, begins
			q.Lock.RLock()

			_, ok := q.Transactions[req.Tx]

			q.Lock.RUnlock()
			// -- ends here

			req.ResponseChan <- ok

		case req := <-q.GetTxChan:

			// -- Safe reading, begins
			q.Lock.RLock()

			if tx, ok := q.Transactions[req.Tx]; ok {
				req.ResponseChan <- tx

				q.Lock.RUnlock()
				break
			}

			q.Lock.RUnlock()
			// -- ends here

			req.ResponseChan <- nil

		case req := <-q.CountTxsChan:

			// -- Safe reading starting here
			q.Lock.RLock()

			req.ResponseChan <- uint64(q.AscTxsByGasPrice.len())

			q.Lock.RUnlock()
			// -- ending here

		case req := <-q.ListTxsChan:

			if req.Order == ASC {
				// -- Safe reading starting here
				q.Lock.RLock()

				// If empty, just return nil
				if q.AscTxsByGasPrice.len() == 0 {
					req.ResponseChan <- nil

					q.Lock.RUnlock()
					break
				}

				copied := make([]*MemPoolTx, q.AscTxsByGasPrice.len())
				copy(copied, q.AscTxsByGasPrice.get())

				req.ResponseChan <- copied

				q.Lock.RUnlock()
				// -- ending here
				break
			}

			if req.Order == DESC {
				// -- Safe reading starting here
				q.Lock.RLock()

				// If empty, just return nil
				if q.DescTxsByGasPrice.len() == 0 {
					req.ResponseChan <- nil

					q.Lock.RUnlock()
					break
				}

				copied := make([]*MemPoolTx, q.DescTxsByGasPrice.len())
				copy(copied, q.DescTxsByGasPrice.get())

				req.ResponseChan <- copied

				q.Lock.RUnlock()
				// -- ending here
			}

		}

	}

}

// Prune - Remove unstuck txs from queued pool & also
// attempt to place them in pending pool, if not present already
//
// @note Start this method as an independent go routine
func (q *QueuedPool) Prune(ctx context.Context) {

	// Creating worker pool, where jobs to be submitted
	// for concurrently checking whether tx has been unstuck or not
	// so that it can be moved to pending pool
	wp := workerpool.New(config.GetConcurrencyFactor())
	defer wp.Stop()

	internalChan := make(chan *TxStatus, 1024)
	var unstuck uint64

	for {

		select {

		case <-ctx.Done():
			return

		case <-time.After(time.Duration(500) * time.Millisecond):

			txs := q.DescListTxs()
			if txs == nil {
				break
			}

			for i := 0; i < len(txs); i++ {

				func(tx *MemPoolTx) {

					wp.Submit(func() {

						// Finally checking whether this tx is unstuck or not
						// by doing nonce comparison
						yes, err := tx.IsUnstuck(ctx, q.RPC)
						if err != nil {

							internalChan <- &TxStatus{Hash: tx.Hash, Status: STUCK}
							return

						}

						if yes {
							internalChan <- &TxStatus{Hash: tx.Hash, Status: UNSTUCK}
						} else {
							internalChan <- &TxStatus{Hash: tx.Hash, Status: STUCK}
						}

					})

				}(txs[i])

			}

		case txStat := <-internalChan:

			if txStat.Status == UNSTUCK {

				// Removing unstuck tx
				tx := q.Remove(ctx, txStat.Hash)
				if tx == nil {
					// probably just been removed by some competing worker
					// because it became eligible for that
					continue
				}

				unstuck++

				// Just check whether we need to add this tx into pending
				// pool first, if not required, we're not adding it
				q.PendingPool.VerifiedAdd(ctx, tx)

				if unstuck%10 == 0 {
					log.Printf("[➖] Removed 10 tx(s) from queued tx pool\n")
				}

			}

		}

	}

}

// Get - Given tx hash, attempts to find out tx in queued pool, if any
//
// Returns nil, if found nothing
func (q *QueuedPool) Get(hash common.Hash) *MemPoolTx {

	respChan := make(chan *MemPoolTx)

	q.GetTxChan <- GetRequest{Tx: hash, ResponseChan: respChan}

	return <-respChan

}

// Exists - Checks whether tx of given hash exists on queued pool or not
func (q *QueuedPool) Exists(hash common.Hash) bool {

	respChan := make(chan bool)

	q.TxExistsChan <- ExistsRequest{Tx: hash, ResponseChan: respChan}

	return <-respChan

}

// Count - How many tx(s) currently present in pending pool
func (q *QueuedPool) Count() uint64 {

	respChan := make(chan uint64)

	q.CountTxsChan <- CountRequest{ResponseChan: respChan}

	return <-respChan

}

// DuplicateTxs - Attempting to find duplicate tx(s) for given
// txHash.
//
// @note In duplicate tx list, the tx which was provided as input
// will not be included
//
// Considering one tx duplicate of given one, if this tx has same
// nonce & sender address, as of given ones
func (q *QueuedPool) DuplicateTxs(hash common.Hash) []*MemPoolTx {

	targetTx := q.Get(hash)
	if targetTx == nil {
		return nil
	}

	txs := q.DescListTxs()
	if txs == nil {
		return nil
	}

	txCount := uint64(len(txs))
	commChan := make(chan *MemPoolTx, txCount)
	result := make([]*MemPoolTx, 0, txCount)

	// Attempting to concurrently checking which txs are duplicate
	// of a given tx hash
	wp := workerpool.New(runtime.NumCPU())

	for i := 0; i < len(txs); i++ {

		func(tx *MemPoolTx) {

			wp.Submit(func() {

				if tx.IsDuplicateOf(targetTx) {
					commChan <- tx
					return
				}

				commChan <- nil

			})

		}(txs[i])

	}

	var received uint64
	mustReceive := txCount

	// Waiting for all go routines to finish
	for v := range commChan {

		if v != nil {
			result = append(result, v)
		}

		received++
		if received >= mustReceive {
			break
		}

	}

	// This call is irrelevant here, probably, but still being made
	//
	// Because all workers have exited, otherwise we could have never
	// reached this point
	wp.Stop()

	return result

}

// AscListTxs - Returns all tx(s) present in queued pool, as slice, ascending ordered as per gas price paid
func (q *QueuedPool) AscListTxs() []*MemPoolTx {

	respChan := make(chan []*MemPoolTx)

	q.ListTxsChan <- ListRequest{ResponseChan: respChan, Order: ASC}

	return <-respChan

}

// DescListTxs - Returns all tx(s) present in queued pool, as slice, descending ordered as per gas price paid
func (q *QueuedPool) DescListTxs() []*MemPoolTx {

	respChan := make(chan []*MemPoolTx)

	q.ListTxsChan <- ListRequest{ResponseChan: respChan, Order: DESC}

	return <-respChan

}

// TopXWithHighGasPrice - Returns only top `X` tx(s) present in queued mempool,
// where being top is determined by how much gas price paid by tx sender
func (q *QueuedPool) TopXWithHighGasPrice(x uint64) []*MemPoolTx {
	return q.DescListTxs()[:x]
}

// TopXWithLowGasPrice - Returns only top `X` tx(s) present in queued mempool,
// where being top is determined by how low gas price paid by tx sender
func (q *QueuedPool) TopXWithLowGasPrice(x uint64) []*MemPoolTx {
	return q.AscListTxs()[:x]
}

// SentFrom - Returns a list of queued tx(s) sent from
// specified address
func (q *QueuedPool) SentFrom(address common.Address) []*MemPoolTx {

	txs := q.DescListTxs()
	if txs == nil {
		return nil
	}

	txCount := uint64(len(txs))
	commChan := make(chan *MemPoolTx, txCount)
	result := make([]*MemPoolTx, 0, txCount)

	wp := workerpool.New(runtime.NumCPU())

	for i := 0; i < len(txs); i++ {

		func(tx *MemPoolTx) {

			wp.Submit(func() {

				if tx.IsSentFrom(address) {
					commChan <- tx
					return
				}

				commChan <- nil

			})

		}(txs[i])

	}

	var received uint64
	mustReceive := txCount

	// Waiting for all go routines to finish
	for v := range commChan {

		if v != nil {
			result = append(result, v)
		}

		received++
		if received >= mustReceive {
			break
		}

	}

	// This call is irrelevant here, probably, but still being made
	//
	// Because all workers have exited, otherwise we could have never
	// reached this point
	wp.Stop()

	return result

}

// SentTo - Returns a list of queued tx(s) sent to
// specified address
func (q *QueuedPool) SentTo(address common.Address) []*MemPoolTx {

	txs := q.DescListTxs()
	if txs == nil {
		return nil
	}

	txCount := uint64(len(txs))
	commChan := make(chan *MemPoolTx, txCount)
	result := make([]*MemPoolTx, 0, txCount)

	wp := workerpool.New(runtime.NumCPU())

	for i := 0; i < len(txs); i++ {

		func(tx *MemPoolTx) {

			wp.Submit(func() {

				if tx.IsSentTo(address) {
					commChan <- tx
					return
				}

				commChan <- nil

			})

		}(txs[i])

	}

	var received uint64
	mustReceive := txCount

	// Waiting for all go routines to finish
	for v := range commChan {

		if v != nil {
			result = append(result, v)
		}

		received++
		if received >= mustReceive {
			break
		}

	}

	// This call is irrelevant here, probably, but still being made
	//
	// Because all workers have exited, otherwise we could have never
	// reached this point
	wp.Stop()

	return result

}

// OlderThanX - Returns a list of all queued tx(s), which are
// living in mempool for more than or equals to `X` time unit
func (q *QueuedPool) OlderThanX(x time.Duration) []*MemPoolTx {

	txs := q.DescListTxs()
	if txs == nil {
		return nil
	}

	txCount := uint64(len(txs))
	commChan := make(chan *MemPoolTx, txCount)
	result := make([]*MemPoolTx, 0, txCount)

	wp := workerpool.New(runtime.NumCPU())

	for i := 0; i < len(txs); i++ {

		func(tx *MemPoolTx) {

			wp.Submit(func() {

				if tx.IsQueuedForGTE(x) {
					commChan <- tx
					return
				}

				commChan <- nil

			})

		}(txs[i])

	}

	var received uint64
	mustReceive := txCount

	// Waiting for all go routines to finish
	for v := range commChan {

		if v != nil {
			result = append(result, v)
		}

		received++
		if received >= mustReceive {
			break
		}

	}

	// This call is irrelevant here, probably, but still being made
	//
	// Because all workers have exited, otherwise we could have never
	// reached this point
	wp.Stop()

	return result

}

// FresherThanX - Returns a list of all queued tx(s), which are
// living in mempool for less than or equals to `X` time unit
func (q *QueuedPool) FresherThanX(x time.Duration) []*MemPoolTx {

	txs := q.DescListTxs()
	if txs == nil {
		return nil
	}

	txCount := uint64(len(txs))
	commChan := make(chan *MemPoolTx, txCount)
	result := make([]*MemPoolTx, 0, txCount)

	wp := workerpool.New(runtime.NumCPU())

	for i := 0; i < len(txs); i++ {

		func(tx *MemPoolTx) {

			wp.Submit(func() {

				if tx.IsQueuedForLTE(x) {
					commChan <- tx
					return
				}

				commChan <- nil

			})

		}(txs[i])

	}

	var received uint64
	mustReceive := txCount

	// Waiting for all go routines to finish
	for v := range commChan {

		if v != nil {
			result = append(result, v)
		}

		received++
		if received >= mustReceive {
			break
		}

	}

	// This call is irrelevant here, probably, but still being made
	//
	// Because all workers have exited, otherwise we could have never
	// reached this point
	wp.Stop()

	return result

}

// Add - Attempts to add new tx found in pending pool into
// harmony mempool, so that further manipulation can be performed on it
//
// If it returns `true`, it denotes, it's success, otherwise it's failure
// because this tx is already present in pending pool
func (q *QueuedPool) Add(ctx context.Context, tx *MemPoolTx) bool {

	respChan := make(chan bool)

	q.AddTxChan <- AddRequest{Tx: tx, ResponseChan: respChan}

	return <-respChan

}

// PublishAdded - Publish new tx, entered queued pool, ( in messagepack serialized format )
// to pubsub topic
func (q *QueuedPool) PublishAdded(ctx context.Context, pubsub *redis.Client, msg *MemPoolTx) {

	_msg, err := msg.ToMessagePack()
	if err != nil {

		log.Printf("[❗️] Failed to serialize into messagepack : %s\n", err.Error())
		return

	}

	if err := pubsub.Publish(ctx, config.GetQueuedTxEntryPublishTopic(), _msg).Err(); err != nil {
		log.Printf("[❗️] Failed to publish new queued tx : %s\n", err.Error())
	}

}

// Remove - Removes unstuck tx from queued pool
func (q *QueuedPool) Remove(ctx context.Context, txHash common.Hash) *MemPoolTx {

	respChan := make(chan *MemPoolTx)

	q.RemoveTxChan <- RemovedUnstuckTx{Hash: txHash, ResponseChan: respChan}

	return <-respChan

}

// PublishRemoved - Publish unstuck tx, leaving queued pool ( in messagepack serialized format )
// to pubsub topic
//
// These tx(s) are leaving queued pool i.e. they're ( probably ) going to
// sit in pending pool now, unless they're already mined & harmony
// failed to keep track of it
func (q *QueuedPool) PublishRemoved(ctx context.Context, pubsub *redis.Client, msg *MemPoolTx) {

	_msg, err := msg.ToMessagePack()
	if err != nil {

		log.Printf("[❗️] Failed to serialize into messagepack : %s\n", err.Error())
		return

	}

	if err := pubsub.Publish(ctx, config.GetQueuedTxExitPublishTopic(), _msg).Err(); err != nil {
		log.Printf("[❗️] Failed to publish unstuck tx : %s\n", err.Error())
	}

}

// AddQueued - Update latest queued pool state
func (q *QueuedPool) AddQueued(ctx context.Context, txs map[string]map[string]*MemPoolTx) uint64 {

	var count uint64

	for keyO := range txs {
		for keyI := range txs[keyO] {

			if q.Add(ctx, txs[keyO][keyI]) {
				count++
			}

		}
	}

	return count

}
