package data

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

// IsPresentInCurrentPool - Given tx hash, which was previously present in pending/ queued pool
// attempts to check whether it's present in current txpool content or not
//
// @note `txs` is current pending/ queued pool state received over
// RPC interface
func IsPresentInCurrentPool(txs map[string]map[string]*MemPoolTx, txHash common.Hash) bool {

	var present bool

	{
	OUTER:
		for _, vOuter := range txs {
			for _, vInner := range vOuter {

				if vInner.Hash == txHash {

					present = true
					break OUTER

				}

			}
		}
	}

	return present

}

// HexToDecimal - Converts hex encoded uint64 to decimal string
func HexToDecimal(num *hexutil.Uint64) string {

	_num := big.NewInt(0)
	_num.SetString(num.String(), 16)

	return _num.String()

}

// BigHexToDecimal - Converts hex encoded big number to decimal string
func BigHexToDecimal(num *hexutil.Big) string {

	_num := big.NewInt(0)
	_num.SetString(num.String(), 16)

	return _num.String()

}
