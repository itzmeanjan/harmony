package data

import "github.com/ethereum/go-ethereum/common"

// Sorting direction representation
const (
	ASC = iota
	DESC
)

// When submitting async request for pruning pending/ queued
// pool, immediate response to be sent to client in any of these form(s)
const (
	EMPTY = iota
	PRUNING
	SCHEDULED
)

// AddRequest - For adding new tx into pool
type AddRequest struct {
	Tx           *MemPoolTx
	ResponseChan chan bool
}

// RemoveRequest - For removing existing tx into pool
type RemoveRequest struct {
	TxStat       *TxStatus
	ResponseChan chan bool
}

// RemovedUnstuckTx - Remove unstuck tx from queued pool, request to be
// sent in this form
type RemovedUnstuckTx struct {
	Hash         common.Hash
	ResponseChan chan *MemPoolTx
}

// RemoveTxsRequest - For checking which txs can be removed
// from pending pool, this request to be sent to pending pool manager
type RemoveTxsFromPendingPool struct {
	Txs          map[string]map[string]*MemPoolTx
	ResponseChan chan int
}

// RemoveTxsFromQueuedPool - For updating local queued pool state, request of
// this form to be sent to pool manager over channel, where it'll check which txs
// are likely to be unstuck & has moved to pending pool
type RemoveTxsFromQueuedPool struct {
	Pending      map[string]map[string]*MemPoolTx
	Queued       map[string]map[string]*MemPoolTx
	ResponseChan chan uint64
}

// ExistsRequest - Checking whether tx is present in pool or not
type ExistsRequest struct {
	Tx           common.Hash
	ResponseChan chan bool
}

// GetRequest - Obtaining reference to existing tx in pool
type GetRequest struct {
	Tx           common.Hash
	ResponseChan chan *MemPoolTx
}

// CountRequest - Getting #-of txs present in pool
type CountRequest struct {
	ResponseChan chan uint64
}

// ListRequest - Listing all txs in pool
type ListRequest struct {
	Order        int
	ResponseChan chan []*MemPoolTx
}
