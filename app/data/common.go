package data

import (
	"errors"
	"fmt"
	"math/big"
	"strings"

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

// Removes prepended `0{x, X}` from hex string
func remove0x(num string) string {
	return strings.Replace(strings.Replace(num, "0x", "", -1), "0X", "", -1)
}

// HexToDecimal - Converts hex encoded uint64 to decimal string
func HexToDecimal(num hexutil.Uint64) string {

	_num := big.NewInt(0)
	_num.SetString(remove0x(num.String()), 16)

	return _num.String()

}

// BigHexToBigDecimal - Given a hex encoded big number, converts it to
// decimal big integer
func BigHexToBigDecimal(num *hexutil.Big) *big.Int {

	_num := big.NewInt(0)
	_num.SetString(remove0x(num.String()), 16)

	return _num

}

// BigIntToBigFloat - Given big number, attempts to convert it to
// big floating point number
func BigIntToBigFloat(num *big.Int) (*big.Float, error) {

	_res := big.NewFloat(0)

	if _, ok := _res.SetString(num.String()); !ok {
		return nil, errors.New("Failed to convert big int to big float")
	}

	return _res, nil

}

// BigHexToBigFloat - Converts hex encoded big integer to big floating point number
//
// @note To be used for better precision flaoting point arithmetic
func BigHexToBigFloat(num *hexutil.Big) (*big.Float, error) {

	return BigIntToBigFloat(BigHexToBigDecimal(num))

}

// HumanReadableGasPrice - Returns gas price paid for tx
// in Gwei unit
func HumanReadableGasPrice(num *hexutil.Big) string {

	_num, err := BigHexToBigFloat(num)
	if err != nil {
		return "0 Gwei"
	}

	_den, err := BigIntToBigFloat(big.NewInt(1_000_000_000))
	if err != nil {
		return "0 Gwei"
	}

	_res := big.NewFloat(0)
	_res.Quo(_num, _den)

	return fmt.Sprintf("%s Gwei", _res.String())

}
