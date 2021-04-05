package data

import "github.com/ethereum/go-ethereum/common"

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
	ResponseChan chan TxList
}
