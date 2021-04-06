package data

import (
	"context"
	"log"
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
	AddTxChan         chan AddRequest
	RemoveTxChan      chan RemovedUnstuckTx
	TxExistsChan      chan ExistsRequest
	GetTxChan         chan GetRequest
	CountTxsChan      chan CountRequest
	ListTxsChan       chan ListRequest
	PubSub            *redis.Client
	RPC               *rpc.Client
	PendingPool       *PendingPool
	LastPruned        time.Time
	PruneAfter        time.Duration
}

// Start - This method is supposed to be started as a
// seperate go routine which will manage queued pool ops
// through out its life
func (q *QueuedPool) Start(ctx context.Context) {

	txRemover := func(txHash common.Hash) *MemPoolTx {

		tx, ok := q.Transactions[txHash]
		if !ok {
			return nil
		}

		q.Transactions[txHash].UnstuckAt = time.Now().UTC()

		// Publishing unstuck tx, this is probably going to
		// enter pending pool now
		q.PublishRemoved(ctx, q.PubSub, q.Transactions[txHash])

		// Remove from sorted tx list, keep it sorted
		q.AscTxsByGasPrice = Remove(q.AscTxsByGasPrice, tx)
		q.DescTxsByGasPrice = Remove(q.DescTxsByGasPrice, tx)

		delete(q.Transactions, txHash)

		return tx

	}

	for {

		select {

		case <-ctx.Done():
			return
		case req := <-q.AddTxChan:

			// This is what we're trying to add
			tx := req.Tx

			if _, ok := q.Transactions[tx.Hash]; ok {
				req.ResponseChan <- false
				break
			}

			// Marking we found this tx in mempool now
			tx.QueuedAt = time.Now().UTC()
			tx.Pool = "queued"

			// Creating entry
			q.Transactions[tx.Hash] = tx

			// As soon as we find new entry for queued pool
			// we publish that tx to pubsub topic
			q.PublishAdded(ctx, q.PubSub, tx)

			// Insert into sorted pending tx list, keep sorted
			q.AscTxsByGasPrice = Insert(q.AscTxsByGasPrice, tx)
			q.DescTxsByGasPrice = Insert(q.DescTxsByGasPrice, tx)

			req.ResponseChan <- true

		case req := <-q.RemoveTxChan:

			// if removed will return non-nil reference to removed tx
			req.ResponseChan <- txRemover(req.Hash)

		case req := <-q.TxExistsChan:

			_, ok := q.Transactions[req.Tx]
			req.ResponseChan <- ok

		case req := <-q.GetTxChan:

			if tx, ok := q.Transactions[req.Tx]; ok {
				req.ResponseChan <- tx
				break
			}

			req.ResponseChan <- nil

		case req := <-q.CountTxsChan:

			req.ResponseChan <- uint64(q.AscTxsByGasPrice.len())

		case req := <-q.ListTxsChan:

			if req.Order == ASC {
				req.ResponseChan <- q.AscTxsByGasPrice.get()
				break
			}

			if req.Order == DESC {
				req.ResponseChan <- q.DescTxsByGasPrice.get()
			}

		case <-time.After(time.Duration(config.GetMemPoolPollingPeriod()) * time.Millisecond):

			if !(time.Now().UTC().Sub(q.LastPruned) >= q.PruneAfter) {
				break
			}

			q.LastPruned = time.Now().UTC()
			if q.AscTxsByGasPrice.len() == 0 {
				break
			}

			start := time.Now().UTC()
			// Creating worker pool, where jobs to be submitted
			// for concurrently checking status of tx(s)
			wp := workerpool.New(config.GetConcurrencyFactor())

			commChan := make(chan *TxStatus, q.Count())

			// Attempting to concurrently check status of tx(s)
			_txs := q.DescTxsByGasPrice.get()
			for i := 0; i < len(_txs); i++ {

				func(tx *MemPoolTx) {

					wp.Submit(func() {

						// Finally checking whether this tx is unstuck or not
						// by doing nonce comparison
						yes, err := tx.IsUnstuck(ctx, q.RPC)
						if err != nil {

							log.Printf("[❗️] Failed to check if tx unstuck : %s\n", err.Error())

							commChan <- &TxStatus{Hash: tx.Hash, Status: UNSTUCK}
							return

						}

						if yes {
							commChan <- &TxStatus{Hash: tx.Hash, Status: UNSTUCK}
						} else {
							commChan <- &TxStatus{Hash: tx.Hash, Status: STUCK}
						}

					})

				}(_txs[i])

			}

			buffer := make([]common.Hash, 0, q.Count())

			var received uint64
			mustReceive := q.Count()

			// Waiting for all go routines to finish
			for v := range commChan {

				if v.Status == UNSTUCK {
					buffer = append(buffer, v.Hash)
				}

				received++
				if received >= mustReceive {
					break
				}

			}

			// This call is irrelevant here, but still being made
			//
			// Because all workers have exited, otherwise we could have never
			// reached this point
			wp.Stop()

			q.Lock.RUnlock()
			// -- Done with safely reading to be removed tx(s)

			// All queued tx(s) present in last iteration
			// also present in now
			//
			// Nothing has changed, so we can't remove any older tx(s)
			if len(buffer) == 0 {
				break
			}

			var count uint64

			// Iteratively removing entries which are
			// not supposed to be present in queued mempool
			// anymore
			//
			// And attempting to add them to pending pool
			// if they're supposed to be added there
			for i := 0; i < len(buffer); i++ {

				// removing unstuck tx
				tx := txRemover(buffer[i])
				if tx == nil {

					log.Printf("[❗️] Failed to remove unstuck tx from queued pool\n")
					continue

				}

				// updating count of removed unstuck tx(s) from
				// queued pool
				count++

				// pushing unstuck tx into pending pool
				// because now it's eligible for it
				if !q.PendingPool.Add(ctx, tx) {
					log.Printf("[❗️] Failed to push unstuck tx into pending pool\n")
				}

			}

			log.Printf("[➖] Removed %d confirmed/ dropped tx(s) from pending tx pool, in %s\n", count, time.Now().UTC().Sub(start))

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
	result := make([]*MemPoolTx, 0, len(txs))

	for i := 0; i < len(txs); i++ {

		// First checking if tx under radar is the one for which
		// we're finding duplicate tx(s). If yes, we will move to next one
		//
		// Now we can check whether current tx under radar is having same nonce
		// and sender address, as of target tx ( for which we had txHash, as input )
		// or not
		//
		// If yes, we'll include it considerable duplicate tx list, for given
		// txHash
		if txs[i].IsDuplicateOf(targetTx) {
			result = append(result, txs[i])
		}

	}

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

// ListTxs - Returns all tx(s) present in queued pool, as slice
func (q *QueuedPool) ListTxs() []*MemPoolTx {
	return q.DescListTxs()
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
	result := make([]*MemPoolTx, 0, len(txs))

	for i := 0; i < len(txs); i++ {

		if txs[i].IsSentFrom(address) {
			result = append(result, txs[i])
		}

	}

	return result

}

// SentTo - Returns a list of queued tx(s) sent to
// specified address
func (q *QueuedPool) SentTo(address common.Address) []*MemPoolTx {

	txs := q.DescListTxs()
	result := make([]*MemPoolTx, 0, len(txs))

	for i := 0; i < len(txs); i++ {

		if txs[i].IsSentTo(address) {
			result = append(result, txs[i])
		}

	}

	return result

}

// OlderThanX - Returns a list of all queued tx(s), which are
// living in mempool for more than or equals to `X` time unit
func (q *QueuedPool) OlderThanX(x time.Duration) []*MemPoolTx {

	txs := q.DescListTxs()
	result := make([]*MemPoolTx, 0, len(txs))

	for i := 0; i < len(txs); i++ {

		if txs[i].IsQueuedForGTE(x) {
			result = append(result, txs[i])
		}

	}

	return result

}

// FresherThanX - Returns a list of all queued tx(s), which are
// living in mempool for less than or equals to `X` time unit
func (q *QueuedPool) FresherThanX(x time.Duration) []*MemPoolTx {

	txs := q.DescListTxs()
	result := make([]*MemPoolTx, 0, len(txs))

	for i := 0; i < len(txs); i++ {

		if txs[i].IsQueuedForLTE(x) {
			result = append(result, txs[i])
		}

	}

	return result

}

// Add - Attempts to add new tx found in pending pool into
// harmony mempool, so that further manipulation can be performed on it
//
// If it returns `true`, it denotes, it's success, otherwise it's failure
// because this tx is already present in pending pool
func (q *QueuedPool) Add(ctx context.Context, pubsub *redis.Client, tx *MemPoolTx) bool {

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
func (q *QueuedPool) Remove(ctx context.Context, pubsub *redis.Client, txHash common.Hash) *MemPoolTx {

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
func (q *QueuedPool) AddQueued(ctx context.Context, pubsub *redis.Client, txs map[string]map[string]*MemPoolTx) uint64 {

	var count uint64

	for keyO := range txs {
		for keyI := range txs[keyO] {

			if q.Add(ctx, pubsub, txs[keyO][keyI]) {
				count++
			}

		}
	}

	return count

}
