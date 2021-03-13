package data

import (
	"context"
	"log"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rpc"
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

// PendingDuplicates - Find duplicate tx(s), given txHash, present
// in pending mempool
func (m *MemPool) PendingDuplicates(hash common.Hash) []*MemPoolTx {
	return m.Pending.DuplicateTxs(hash)
}

// QueuedDuplicates - Find duplicate tx(s), given txHash, present
// in queued mempool
func (m *MemPool) QueuedDuplicates(hash common.Hash) []*MemPoolTx {
	return m.Queued.DuplicateTxs(hash)
}

// PendingPoolLength - Returning current pending tx queue length
func (m *MemPool) PendingPoolLength() uint64 {
	return m.Pending.Count()
}

// QueuedPoolLength - Returning current queued tx queue length
func (m *MemPool) QueuedPoolLength() uint64 {
	return m.Queued.Count()
}

// PendingForGTE - Returning list of tx(s), pending for more than
// x time unit
func (m *MemPool) PendingForGTE(x time.Duration) []*MemPoolTx {
	return m.Pending.OlderThanX(x)
}

// PendingForLTE - Returning list of tx(s), pending for less than
// x time unit
func (m *MemPool) PendingForLTE(x time.Duration) []*MemPoolTx {
	return m.Pending.FresherThanX(x)
}

// QueuedForGTE - Returning list of tx(s), queued for more than
// x time unit
func (m *MemPool) QueuedForGTE(x time.Duration) []*MemPoolTx {
	return m.Queued.OlderThanX(x)
}

// QueuedForLTE - Returning list of tx(s), queued for less than
// x time unit
func (m *MemPool) QueuedForLTE(x time.Duration) []*MemPoolTx {
	return m.Queued.FresherThanX(x)
}

// PendingFrom - List of tx(s) pending from address
//
// @note These are going to be same nonce tx(s), only one of them will
// make to next block, others to be dropped
func (m *MemPool) PendingFrom(address common.Address) []*MemPoolTx {
	return m.Pending.SentFrom(address)
}

// PendingTo - List of tx(s) living in pending pool, sent to specified address
func (m *MemPool) PendingTo(address common.Address) []*MemPoolTx {
	return m.Pending.SentTo(address)
}

// QueuedFrom - List of stuck tx(s) from specified address, due to nonce gap
func (m *MemPool) QueuedFrom(address common.Address) []*MemPoolTx {
	return m.Queued.SentFrom(address)
}

// QueuedTo - List of stuck tx(s) present in queued pool, sent to specified
// address
func (m *MemPool) QueuedTo(address common.Address) []*MemPoolTx {
	return m.Queued.SentTo(address)
}

// TopXPendingWithHighGasPrice - Returns a list of top `X` pending tx(s)
// where high gas price tx(s) are prioritized
func (m *MemPool) TopXPendingWithHighGasPrice(x uint64) []*MemPoolTx {
	return m.Pending.TopXWithHighGasPrice(x)
}

// TopXQueuedWithHighGasPrice - Returns a list of top `X` queued tx(s)
// where high gas price tx(s) are prioritized
func (m *MemPool) TopXQueuedWithHighGasPrice(x uint64) []*MemPoolTx {
	return m.Queued.TopXWithHighGasPrice(x)
}

// TopXPendingWithLowGasPrice - Returns a list of top `X` pending tx(s)
// where low gas price tx(s) are prioritized
func (m *MemPool) TopXPendingWithLowGasPrice(x uint64) []*MemPoolTx {
	return m.Pending.TopXWithLowGasPrice(x)
}

// TopXQueuedWithLowGasPrice - Returns a list of top `X` queued tx(s)
// where low gas price tx(s) are prioritized
func (m *MemPool) TopXQueuedWithLowGasPrice(x uint64) []*MemPoolTx {
	return m.Queued.TopXWithLowGasPrice(x)
}

// Process - Process all current pending & queued tx pool content & populate our in-memory buffer
func (m *MemPool) Process(ctx context.Context, rpc *rpc.Client, pubsub *redis.Client, pending map[string]map[string]*MemPoolTx, queued map[string]map[string]*MemPoolTx) {

	if v := m.Queued.RemoveUnstuck(ctx, rpc, pubsub, m.Pending, pending, queued); v != 0 {
		log.Printf("[➖] Removed %d unstuck tx(s) from queued tx pool\n", v)
	}

	if v := m.Queued.AddQueued(ctx, pubsub, queued); v != 0 {
		log.Printf("[➕] Added %d tx(s) to queued tx pool\n", v)
	}

	if v := m.Pending.RemoveConfirmed(ctx, rpc, pubsub, pending); v != 0 {
		log.Printf("[➖] Removed %d confirmed tx(s) from pending tx pool\n", v)
	}

	if v := m.Pending.AddPendings(ctx, pubsub, pending); v != 0 {
		log.Printf("[➕] Added %d tx(s) to pending tx pool\n", v)
	}

}

// Stat - Log current mempool state
func (m *MemPool) Stat(start time.Time) {

	log.Printf("❇️ Pending Tx(s) : %d | Queued Tx(s) : %d, in %s\n", m.PendingPoolLength(), m.QueuedPoolLength(), time.Now().UTC().Sub(start))

}
