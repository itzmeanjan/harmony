package data

import (
	"log"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
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

// Add - Attempts to add new tx found in pending pool into
// harmony mempool, so that further manipulation can be performed on it
//
// If it returns `true`, it denotes, it's success, otherwise it's failure
// because this tx is already present in pending pool
func (q *QueuedPool) Add(tx *MemPoolTx) bool {

	q.Lock.Lock()
	defer q.Lock.Unlock()

	if _, ok := q.Transactions[tx.Hash]; ok {
		return false
	}

	// Marking we found this tx in mempool now
	tx.QueuedAt = time.Now().UTC()

	// Creating entry
	q.Transactions[tx.Hash] = tx

	return true

}

// Remove - Removes already existing tx from pending tx pool
// denoting it has been mined i.e. confirmed
func (q *QueuedPool) Remove(txHash common.Hash) *MemPoolTx {

	q.Lock.Lock()
	defer q.Lock.Unlock()

	tx, ok := q.Transactions[txHash]
	if !ok {
		return nil
	}

	delete(q.Transactions, txHash)

	return tx

}

// RemoveUnstuck - Removes queued tx(s) from pool which have been unstuck
// & returns how many were removed. If 0 is returned, denotes all tx(s) queued last time
// are still in queued state
//
// Only when their respective blocking factor will get unblocked, they'll be pushed
// into pending pool
func (q *QueuedPool) RemoveUnstuck(pendingPool *PendingPool, pending map[string]map[string]*MemPoolTx, queued map[string]map[string]*MemPoolTx) uint64 {

	buffer := make([]common.Hash, 0, len(q.Transactions))

	// -- Attempt to safely find out which txHashes
	// are absent in current mempool content, i.e. denoting
	// those tx(s) are unstuck & can be attempted to be mined
	// in next block
	//
	// So we can also remove those from our queued pool & consider
	// putting them in pending pool
	q.Lock.RLock()

	for hash := range q.Transactions {

		if !IsPresentInCurrentPool(queued, hash) {
			buffer = append(buffer, hash)
		}

	}

	q.Lock.RUnlock()
	// -- Done with safely reading to be removed tx(s)

	// All queued tx(s) present in last iteration
	// also present in now
	//
	// Nothing has changed, so we can't remove any older tx(s)
	if len(buffer) == 0 {
		return 0
	}

	// Iteratively removing entries which are
	// not supposed to be present in queued mempool
	// anymore
	//
	// And attempting to add them to pending pool
	// if they're supposed to be added there
	for _, v := range buffer {

		// removing unstuck tx
		tx := q.Remove(v)
		if tx == nil {
			continue
		}

		// If this tx is present in current pending pool
		// content, it'll be pushed into mempool
		if !IsPresentInCurrentPool(pending, v) {
			continue
		}

		if !pendingPool.Add(tx) {
			log.Printf("[❗️] Failed to push unstuck tx into pending pool\n")
		}

	}

	return uint64(len(buffer))

}

// AddQueued - Update latest queued pool state
func (q *QueuedPool) AddQueued(txs map[string]map[string]*MemPoolTx) uint64 {

	var count uint64

	for _, vOuter := range txs {
		for _, vInner := range vOuter {

			if q.Add(vInner) {
				count++
			}

		}
	}

	return count

}
