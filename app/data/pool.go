package data

import (
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

// TxIdentifier - Helps in identifying single tx uniquely using
// tx sender address & nonce
type TxIdentifier struct {
	Sender common.Address
	Nonce  uint64
}

// PendingPool - Currently present pending tx(s) i.e. which are ready to
// be mined in next block
type PendingPool struct {
	Transactions map[common.Hash]*MemPoolTx
	Lock         *sync.RWMutex
}

// Count - How many tx(s) currently present in pending pool
func (p *PendingPool) Count() uint64 {

	p.Lock.RLock()
	defer p.Lock.RUnlock()

	return uint64(len(p.Transactions))

}

// Add - Attempts to add new tx found in pending pool into
// harmony mempool, so that further manipulation can be performed on it
//
// If it returns `true`, it denotes, it's success, otherwise it's failure
// because this tx is already present in pending pool
func (p *PendingPool) Add(tx *MemPoolTx) bool {

	p.Lock.Lock()
	defer p.Lock.Unlock()

	if _, ok := p.Transactions[tx.Hash]; ok {
		return false
	}

	// Marking we found this tx in mempool now
	tx.DiscoveredAt = time.Now().UTC()

	// Creating entry
	p.Transactions[tx.Hash] = tx

	return true

}

// Remove - Removes already existing tx from pending tx pool
// denoting it has been mined i.e. confirmed
func (p *PendingPool) Remove(txHash common.Hash) bool {

	p.Lock.Lock()
	defer p.Lock.Unlock()

	if _, ok := p.Transactions[txHash]; !ok {
		return false
	}

	delete(p.Transactions, txHash)

	return true

}

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

// MemPool - Current state of mempool, where all pending/ queued tx(s)
// are present. Among these pending tx(s), any of them can be picked up during next
// block mining phase, but any tx(s) present in queued pool, can't be picked up
// until some problem with sender address is resolved.
//
// Tx(s) ending up in queued pool, happens very commonly due to account nonce gaps
type MemPool struct {
	Pending *PendingPool
	Queued  *QueuedPool
}

// PendingPoolLength - Returning current pending tx queue length
func (m *MemPool) PendingPoolLength() uint64 {

	return m.PendingPoolLength()

}

// QueuedPoolLength - Returning current queued tx queue length
func (m *MemPool) QueuedPoolLength() uint64 {

	m.Queued.Lock.RLock()
	defer m.Queued.Lock.RUnlock()

	return uint64(len(m.Queued.Transactions))

}
