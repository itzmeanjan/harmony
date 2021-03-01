package data

import (
	"sync"

	"github.com/ethereum/go-ethereum/common"
)

// MemPool - Current state of mempool, where all pending/ queued tx(s)
// are present. Among these pending tx(s), any of them can be picked up during next
// block mining phase, but any tx(s) present in queued pool, can't be picked up
// until some problem with sender address is resolved.
//
// Tx(s) ending up in queued pool, happens very commonly due to account nonce gaps
type MemPool struct {
	Pending         *PendingPool
	PendingPoolLock *sync.RWMutex
	QueuedPool      *QueuedPool
	QueuedPoolLock  *sync.RWMutex
}

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
	Addresses    map[*TxIdentifier]common.Hash
}

// QueuedPool - Currently present queued tx(s) i.e. these tx(s) are stuck
// due to some reasons, one very common reason is nonce gap
//
// What it essentially denotes is, these tx(s) are not ready to be picked up
// when next block is going to be picked, when these tx(s) are going to be
// moved to pending pool, only they can be considered before mining
type QueuedPool struct {
	Transactions map[common.Hash]*MemPoolTx
	Addresses    map[*TxIdentifier]common.Hash
}
