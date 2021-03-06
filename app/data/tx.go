package data

import (
	"context"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/itzmeanjan/harmony/app/graph/model"

	"github.com/vmihailenco/msgpack/v5"
)

// MemPoolTx - This is how tx is placed in mempool, after performing
// RPC call for fetching currently pending/ queued tx(s) in mempool
// it'll be destructured into this format, for further computation
type MemPoolTx struct {
	BlockHash        *common.Hash    `json:"blockHash"`
	BlockNumber      *hexutil.Big    `json:"blockNumber"`
	From             common.Address  `json:"from"`
	Gas              hexutil.Uint64  `json:"gas"`
	GasPrice         *hexutil.Big    `json:"gasPrice"`
	Hash             common.Hash     `json:"hash"`
	Input            hexutil.Bytes   `json:"input"`
	Nonce            hexutil.Uint64  `json:"nonce"`
	To               *common.Address `json:"to"`
	TransactionIndex *hexutil.Uint64 `json:"transactionIndex"`
	Value            *hexutil.Big    `json:"value"`
	Type             hexutil.Uint64  `json:"type"`
	ChainID          *hexutil.Big    `json:"chainId,omitempty"`
	V                *hexutil.Big    `json:"v"`
	R                *hexutil.Big    `json:"r"`
	S                *hexutil.Big    `json:"s"`
	PendingFrom      time.Time
	QueuedAt         time.Time
	Pool             string
}

// IsPendingForGTE - Test if this tx has been in pending pool
// for more than or equal to `X` time unit
func (m *MemPoolTx) IsPendingForGTE(x time.Duration) bool {

	if m.Pool != "pending" {
		return false
	}

	return time.Now().UTC().Sub(m.PendingFrom) >= x

}

// IsPendingForLTE - Test if this tx has been in pending pool
// for less than or equal to `X` time unit
func (m *MemPoolTx) IsPendingForLTE(x time.Duration) bool {

	if m.Pool != "pending" {
		return false
	}

	return time.Now().UTC().Sub(m.PendingFrom) <= x

}

// IsQueuedForGTE - Test if this tx has been in queued pool
// for more than or equal to `X` time unit
func (m *MemPoolTx) IsQueuedForGTE(x time.Duration) bool {

	if m.Pool != "queued" {
		return false
	}

	return time.Now().UTC().Sub(m.QueuedAt) >= x

}

// IsQueuedForLTE - Test if this tx has been in queued pool
// for less than or equal to `X` time unit
func (m *MemPoolTx) IsQueuedForLTE(x time.Duration) bool {

	if m.Pool != "queued" {
		return false
	}

	return time.Now().UTC().Sub(m.QueuedAt) <= x

}

// IsNonceExhausted - Multiple tx(s) of same/ different value
// can be sent to network with same nonce, where one of them
// which seems most profitable to miner, will be picked up, while mining next block
//
// This function will help us in checking whether nonce of this tx is exhausted or not
// i.e. whether some other tx is same nonce is mined or not
//
// If mined, we can drop this tx from mempool
func (m *MemPoolTx) IsNonceExhausted(ctx context.Context, rpc *rpc.Client) (bool, error) {

	var result hexutil.Uint64

	if err := rpc.CallContext(ctx, &result, "eth_getTransactionCount", m.From.Hex(), "latest"); err != nil {
		return false, err
	}

	return m.Nonce < result, nil

}

// IsUnstuck - Checking whether this tx is unstuck now
//
// @note Tx(s) generally get stuck in queued pool
// due to nonce gaps
func (m *MemPoolTx) IsUnstuck(ctx context.Context, rpc *rpc.Client) (bool, error) {

	var result hexutil.Uint64

	if err := rpc.CallContext(ctx, &result, "eth_getTransactionCount", m.From.Hex(), "latest"); err != nil {
		return false, err
	}

	return m.Nonce <= result, nil

}

// ToMessagePack - Serialize to message pack encoded byte array format
func (m *MemPoolTx) ToMessagePack() ([]byte, error) {

	return msgpack.Marshal(m)

}

// FromMessagePack - Given serialized byte array, attempts to deserialize
// into structured format
func FromMessagePack(data []byte) (*MemPoolTx, error) {

	var tx *MemPoolTx

	if err := msgpack.Unmarshal(data, tx); err != nil {
		return nil, err
	}

	return tx, nil

}

// ToGraphQL - Convert to graphql compatible type
func (m *MemPoolTx) ToGraphQL() *model.MemPoolTx {

	var gqlTx *model.MemPoolTx

	switch m.Pool {

	case "pending":

		gqlTx = &model.MemPoolTx{
			From:       m.From.Hex(),
			Gas:        HexToDecimal(m.Gas),
			Hash:       m.Hash.Hex(),
			Input:      m.Input.String(),
			Nonce:      HexToDecimal(m.Nonce),
			PendingFor: time.Now().UTC().Sub(m.PendingFrom).String(),
			QueuedFor:  "0 s",
			Pool:       m.Pool,
		}

		if !m.QueuedAt.Equal(time.Time{}) {
			gqlTx.QueuedFor = m.PendingFrom.Sub(m.QueuedAt).String()
		}

	case "queued":

		gqlTx = &model.MemPoolTx{
			From:       m.From.Hex(),
			Gas:        HexToDecimal(m.Gas),
			Hash:       m.Hash.Hex(),
			Input:      m.Input.String(),
			Nonce:      HexToDecimal(m.Nonce),
			PendingFor: "0 s",
			QueuedFor:  time.Now().UTC().Sub(m.PendingFrom).String(),
			Pool:       m.Pool,
		}

	}

	if m.To != nil {
		gqlTx.To = m.To.Hex()
	} else {
		gqlTx.To = "0x"
	}

	if m.GasPrice != nil {
		gqlTx.GasPrice = BigHexToDecimal(m.GasPrice)
	} else {
		gqlTx.GasPrice = "0"
	}

	if m.Value != nil {
		gqlTx.Value = m.Value.String()
	} else {
		gqlTx.Value = "0x"
	}

	if m.V != nil {
		gqlTx.V = m.V.String()
	} else {
		gqlTx.V = "0x"
	}

	if m.R != nil {
		gqlTx.R = m.R.String()
	} else {
		gqlTx.R = "0x"
	}

	if m.S != nil {
		gqlTx.S = m.S.String()
	} else {
		gqlTx.S = "0x"
	}

	return gqlTx

}

// TxStatus - When ever multiple go routines need to
// concurrently fetch status of tx, given hash
// they will communicate back to caller using this
// data structure, where `status` denotes result of
// intended check, which was performed concurrently
//
// @note Data to be sent in this form over communication
// channel
type TxStatus struct {
	Hash   common.Hash
	Status bool
}
