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
	QueuedAt         time.Time
	UnstuckAt        time.Time
	PendingFrom      time.Time
	ConfirmedAt      time.Time
	DroppedAt        time.Time
	Pool             string
	ReceivedFrom     string
}

// IsDuplicateOf - Checks whether one tx is duplicate of another one or not
//
// @note Two tx(s) are considered to be duplicate of each other when
// both of them having same from address & nonce
func (m *MemPoolTx) IsDuplicateOf(tx *MemPoolTx) bool {

	return m.Hash != tx.Hash && m.From == tx.From && m.Nonce == tx.Nonce

}

// IsLowerNonce - Objective is to find out whether `m` has same
// of lower nonce than `tx`
func (m *MemPoolTx) IsLowerNonce(tx *MemPoolTx) bool {

	return m.Hash != tx.Hash && m.From == tx.From && m.Nonce <= tx.Nonce

}

// IsSentFrom - Checks whether this tx was sent from specified address
// or not
func (m *MemPoolTx) IsSentFrom(address common.Address) bool {

	return m.From == address

}

// IsSentTo - Checks if this was sent to certain address ( EOA/ Contract )
//
// @note If it's a contract creation tx, it'll not have `to` address
func (m *MemPoolTx) IsSentTo(address common.Address) bool {

	if m.To == nil {
		return false
	}

	return *m.To == address

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

// IsDropped - Attempts to check whether this tx was dropped by node or not
//
// Dropping can happen due to higher priority tx from same account with same nonce
// was encountered
func (m *MemPoolTx) IsDropped(ctx context.Context, rpc *rpc.Client) (bool, error) {

	var result interface{}

	if err := rpc.CallContext(ctx, &result, "eth_getTransactionReceipt", m.Hash.Hex()); err != nil {
		return true, err
	}

	// Receipt is not available i.e. tx is dropped ( because nonce is exhausted, we already know )
	if result == nil {
		return true, nil
	}

	// tx receipt exists, meaning, tx got mined
	return false, nil

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
// into structured tx format
func FromMessagePack(data []byte) (*MemPoolTx, error) {

	var tx MemPoolTx

	if err := msgpack.Unmarshal(data, &tx); err != nil {
		return nil, err
	}

	return &tx, nil

}

// ToGraphQL - Convert to graphql compatible type
func (m *MemPoolTx) ToGraphQL() *model.MemPoolTx {

	var gqlTx *model.MemPoolTx

	switch m.Pool {

	case "queued":

		gqlTx = &model.MemPoolTx{
			From:       m.From.Hex(),
			Gas:        HexToDecimal(m.Gas),
			Hash:       m.Hash.Hex(),
			Input:      m.Input.String(),
			Nonce:      HexToDecimal(m.Nonce),
			PendingFor: "0 s",
			QueuedFor:  time.Now().UTC().Sub(m.QueuedAt).String(),
			Pool:       m.Pool,
		}

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

		if !m.QueuedAt.Equal(time.Time{}) && !m.UnstuckAt.Equal(time.Time{}) {

			gqlTx.QueuedFor = m.UnstuckAt.Sub(m.QueuedAt).String()

		}

	case "confirmed":

		gqlTx = &model.MemPoolTx{
			From:       m.From.Hex(),
			Gas:        HexToDecimal(m.Gas),
			Hash:       m.Hash.Hex(),
			Input:      m.Input.String(),
			Nonce:      HexToDecimal(m.Nonce),
			PendingFor: m.ConfirmedAt.Sub(m.PendingFrom).String(),
			QueuedFor:  "0 s",
			Pool:       m.Pool,
		}

		if !m.QueuedAt.Equal(time.Time{}) && !m.UnstuckAt.Equal(time.Time{}) {

			gqlTx.QueuedFor = m.UnstuckAt.Sub(m.QueuedAt).String()

		}

	case "dropped":

		gqlTx = &model.MemPoolTx{
			From:       m.From.Hex(),
			Gas:        HexToDecimal(m.Gas),
			Hash:       m.Hash.Hex(),
			Input:      m.Input.String(),
			Nonce:      HexToDecimal(m.Nonce),
			PendingFor: m.DroppedAt.Sub(m.PendingFrom).String(),
			QueuedFor:  "0 s",
			Pool:       m.Pool,
		}

		if !m.QueuedAt.Equal(time.Time{}) && !m.UnstuckAt.Equal(time.Time{}) {

			gqlTx.QueuedFor = m.UnstuckAt.Sub(m.QueuedAt).String()

		}

	default:
		// handle situation when deserilisation didn't work
		// properly
		break

	}

	// In that case, simply return
	if gqlTx == nil {
		return nil
	}

	if m.To != nil {
		gqlTx.To = m.To.Hex()
	} else {
		gqlTx.To = "0x"
	}

	if m.GasPrice != nil {
		gqlTx.GasPrice = HumanReadableGasPrice(m.GasPrice)
	} else {
		gqlTx.GasPrice = "0"
	}

	if m.Value != nil {
		gqlTx.Value = BigHexToBigDecimal(m.Value).String()
	} else {
		gqlTx.Value = "0"
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

const (
	STUCK = iota + 1
	UNSTUCK
	PENDING
	CONFIRMED
	DROPPED
)

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
	Status int
}
