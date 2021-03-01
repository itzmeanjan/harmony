package data

import (
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
	tx.DiscoveredAt = time.Now().UTC()

	// Creating entry
	q.Transactions[tx.Hash] = tx

	return true

}

// Remove - Removes already existing tx from pending tx pool
// denoting it has been mined i.e. confirmed
func (q *QueuedPool) Remove(txHash common.Hash) bool {

	q.Lock.Lock()
	defer q.Lock.Unlock()

	if _, ok := q.Transactions[txHash]; !ok {
		return false
	}

	delete(q.Transactions, txHash)

	return true

}
