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
	Transactions map[common.Hash]*MemPoolTx
	Lock         *sync.RWMutex
}

// Count - How many tx(s) currently present in pending pool
func (q *QueuedPool) Count() uint64 {

	q.Lock.RLock()
	defer q.Lock.RUnlock()

	return uint64(len(q.Transactions))

}

// OlderThanX - Returns a list of all queued tx(s), which are
// living in mempool for more than or equals to `X` time unit
func (q *QueuedPool) OlderThanX(x time.Duration) []*MemPoolTx {

	q.Lock.RLock()
	defer q.Lock.RUnlock()

	result := make([]*MemPoolTx, 0, len(q.Transactions))

	for _, tx := range q.Transactions {

		if tx.IsQueuedForGTE(x) {
			result = append(result, tx)
		}

	}

	return result

}

// FresherThanX - Returns a list of all queued tx(s), which are
// living in mempool for less than or equals to `X` time unit
func (q *QueuedPool) FresherThanX(x time.Duration) []*MemPoolTx {

	q.Lock.RLock()
	defer q.Lock.RUnlock()

	result := make([]*MemPoolTx, 0, len(q.Transactions))

	for _, tx := range q.Transactions {

		if tx.IsQueuedForLTE(x) {
			result = append(result, tx)
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

	q.Lock.Lock()
	defer q.Lock.Unlock()

	if _, ok := q.Transactions[tx.Hash]; ok {
		return false
	}

	// Marking we found this tx in mempool now
	tx.QueuedAt = time.Now().UTC()
	tx.Pool = "queued"

	// Creating entry
	q.Transactions[tx.Hash] = tx

	// As soon as we find new entry for queued pool
	// we publish that tx to pubsub topic
	q.PublishAdded(ctx, pubsub, tx)
	return true

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

// Remove - Removes already existing tx from pending tx pool
// denoting it has been mined i.e. confirmed
func (q *QueuedPool) Remove(ctx context.Context, pubsub *redis.Client, txHash common.Hash) *MemPoolTx {

	q.Lock.Lock()
	defer q.Lock.Unlock()

	tx, ok := q.Transactions[txHash]
	if !ok {
		return nil
	}

	// Publishing unstuck tx, this is probably going to
	// enter pending pool now
	q.PublishRemoved(ctx, pubsub, q.Transactions[txHash])

	delete(q.Transactions, txHash)

	return tx

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

// RemoveUnstuck - Removes queued tx(s) from pool which have been unstuck
// & returns how many were removed. If 0 is returned, denotes all tx(s) queued last time
// are still in queued state
//
// Only when their respective blocking factor will get unblocked, they'll be pushed
// into pending pool
func (q *QueuedPool) RemoveUnstuck(ctx context.Context, rpc *rpc.Client, pubsub *redis.Client, pendingPool *PendingPool, pending map[string]map[string]*MemPoolTx, queued map[string]map[string]*MemPoolTx) uint64 {

	if len(q.Transactions) == 0 {
		return 0
	}

	buffer := make([]common.Hash, 0, len(q.Transactions))

	// -- Attempt to safely find out which txHashes
	// are absent in current mempool content, i.e. denoting
	// those tx(s) are unstuck & can be attempted to be mined
	// in next block
	//
	// So we can also remove those from our queued pool & consider
	// putting them in pending pool
	q.Lock.RLock()

	// Creating worker pool, where jobs to be submitted
	// for concurrently checking status of tx(s)
	wp := workerpool.New(config.GetConcurrencyFactor())

	commChan := make(chan TxStatus, len(q.Transactions))

	// Attempting to concurrently check status of tx(s)
	for _, tx := range q.Transactions {

		func(tx *MemPoolTx) {

			wp.Submit(func() {

				// If this queued tx is present in current
				// queued pool state obtained from RPC node, no need
				// check whether tx is unstuck or not
				//
				// @note Because it's definitely not unstuck
				if IsPresentInCurrentPool(queued, tx.Hash) {

					commChan <- TxStatus{Hash: tx.Hash, Status: false}
					return

				}

				// Now checking if tx has been moved to pending pool
				// or not, if yes, we can consider this tx is unstuck
				// it must be moved out of queued pool
				if IsPresentInCurrentPool(pending, tx.Hash) {

					commChan <- TxStatus{Hash: tx.Hash, Status: true}
					return

				}

				// Finally checking whether this tx is unstuck or not
				// by doing nonce comparison
				yes, err := tx.IsUnstuck(ctx, rpc)
				if err != nil {

					log.Printf("[❗️] Failed to check if tx unstuck : %s\n", err.Error())

					commChan <- TxStatus{Hash: tx.Hash, Status: false}
					return

				}

				commChan <- TxStatus{Hash: tx.Hash, Status: yes}

			})

		}(tx)

	}

	var received int
	mustReceive := len(q.Transactions)

	// Waiting for all go routines to finish
	for v := range commChan {

		if v.Status {
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
		return 0
	}

	var count uint64

	// Iteratively removing entries which are
	// not supposed to be present in queued mempool
	// anymore
	//
	// And attempting to add them to pending pool
	// if they're supposed to be added there
	for _, v := range buffer {

		// removing unstuck tx
		tx := q.Remove(ctx, pubsub, v)
		if tx == nil {

			log.Printf("[❗️] Failed to remove unstuck tx from queued pool\n")
			continue

		}

		// updating count of removed unstuck tx(s) from
		// queued pool
		count++

		// pushing unstuck tx into pending pool
		// because now it's eligible for it
		if !pendingPool.Add(ctx, pubsub, tx) {
			log.Printf("[❗️] Failed to push unstuck tx into pending pool\n")
		}

	}

	return count

}

// AddQueued - Update latest queued pool state
func (q *QueuedPool) AddQueued(ctx context.Context, pubsub *redis.Client, txs map[string]map[string]*MemPoolTx) uint64 {

	var count uint64

	for _, vOuter := range txs {
		for _, vInner := range vOuter {

			if q.Add(ctx, pubsub, vInner) {
				count++
			}

		}
	}

	return count

}
