package data

import (
	"context"
	"log"

	"github.com/go-redis/redis/v8"
)

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

// Process - Process all current pending & queued tx pool content & populate our in-memory buffer
func (m *MemPool) Process(ctx context.Context, pubsub *redis.Client, pending map[string]map[string]*MemPoolTx, queued map[string]map[string]*MemPoolTx) {

	if v := m.Queued.RemoveUnstuck(ctx, pubsub, m.Pending, pending, queued); v != 0 {
		log.Printf("☑️ Removed %d unstuck tx(s) from queued tx pool\n", v)
	}

	if v := m.Queued.AddQueued(ctx, pubsub, queued); v != 0 {
		log.Printf("☑️ Added %d tx(s) to queued tx pool\n", v)
	}

	if v := m.Pending.RemoveConfirmed(ctx, pubsub, pending); v != 0 {
		log.Printf("☑️ Removed %d confirmed tx(s) from pending tx pool\n", v)
	}

	if v := m.Pending.AddPendings(ctx, pubsub, pending); v != 0 {
		log.Printf("☑️ Added %d tx(s) to pending tx pool\n", v)
	}

}

// Stat - Log current mempool state
func (m *MemPool) Stat() {

	log.Printf("❇️ Pending Tx(s) : %d | Queued Tx(s) : %d\n", m.PendingPoolLength(), m.QueuedPoolLength())

}
