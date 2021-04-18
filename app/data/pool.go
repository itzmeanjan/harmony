package data

import (
	"context"
	"log"
	"time"

	"github.com/ethereum/go-ethereum/common"
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

// Get - Given a txhash, attempts to find out tx, if
// present in any of pending/ queued pool
func (m *MemPool) Get(hash common.Hash) *MemPoolTx {

	queued := m.Queued.Get(hash)
	if queued != nil {
		return queued
	}

	return m.Pending.Get(hash)

}

// Exists - Given a txHash, attempts to check whether this tx is present
// in either of pending/ queued pool
func (m *MemPool) Exists(hash common.Hash) bool {

	queued := m.Queued.Exists(hash)
	if queued {
		return queued
	}

	return m.Pending.Exists(hash)

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

// DoneTxCount - #-of tx(s) seen to processed during this node's life time
func (m *MemPool) DoneTxCount() uint64 {
	return m.Pending.Processed()
}

// LastSeenBlock - Last seen block by mempool & when it was seen, to be invoked
// by stat generator http request handler method
func (m *MemPool) LastSeenBlock() LastSeenBlock {
	return m.Pending.GetLastSeenBlock()
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
func (m *MemPool) Process(ctx context.Context, pending map[string]map[string]*MemPoolTx, queued map[string]map[string]*MemPoolTx) {

	start := time.Now().UTC()

	if addedQ := m.Queued.AddQueued(ctx, queued); addedQ != 0 {
		log.Printf("[➕] Added %d tx(s) to queued tx pool, in %s\n", addedQ, time.Now().UTC().Sub(start))
	}

	start = time.Now().UTC()

	if addedP := m.Pending.AddPendings(ctx, pending); addedP != 0 {
		log.Printf("[➕] Added %d tx(s) to pending tx pool, in %s\n", addedP, time.Now().UTC().Sub(start))
	}

}

// Stat - Log current mempool state
func (m *MemPool) Stat(start time.Time) {

	log.Printf("❇️ Pending Tx(s) : %d | Queued Tx(s) : %d, in %s\n", m.PendingPoolLength(), m.QueuedPoolLength(), time.Now().UTC().Sub(start))

}

// HandleTxFromPeer - When new chunk of deserialised in-flight tx ( i.e. entering/ leaving mempool )
// is received from any `harmony` peer, it will be checked against latest state
// of local mempool view, to decide whether this tx can be acted upon
// somehow or not
func (m *MemPool) HandleTxFromPeer(ctx context.Context, pubsub *redis.Client, tx *MemPoolTx) bool {

	// Checking whether we already have this tx included in pool
	// or not
	exists := m.Exists(tx.Hash)

	var status bool

	// @note Updating state needs to be made secure, proofs can be
	// considered, in future date
	switch tx.Pool {

	case "dropped":

		// If we already have entry for this tx & we just learnt
		// this tx got dropped, we'll try to update our state
		// same as our peer did
		if exists {
			status = m.Pending.Remove(ctx, &TxStatus{Hash: tx.Hash, Status: DROPPED})
		}

	case "confirmed":

		// If we already have entry for this tx & we just learnt
		// this tx got confirmed, we'll try to update our state
		// same as our peer did
		if exists {
			status = m.Pending.Remove(ctx, &TxStatus{Hash: tx.Hash, Status: CONFIRMED})
		}

	case "queued":

		// If we don't have it in our state, we'll add it
		if !exists {
			status = m.Queued.Add(ctx, tx)
		}

	case "pending":

		// If we don't have it in our state, we'll add it
		if !exists {
			status = m.Pending.Add(ctx, tx)
		}

	}

	return status

}
