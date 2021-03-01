package data

import (
	"github.com/ethereum/go-ethereum/common"
)

// TxIdentifier - Helps in identifying single tx uniquely using
// tx sender address & nonce
type TxIdentifier struct {
	Sender common.Address
	Nonce  uint64
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
	return m.Pending.Count()
}

// QueuedPoolLength - Returning current queued tx queue length
func (m *MemPool) QueuedPoolLength() uint64 {
	return m.Queued.Count()
}
