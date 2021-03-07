package data

import (
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

// BigHexToDecimal - Converts hex encoded big number to Gas Price in Gwei form
func BigHexToDecimal(num *hexutil.Big) string {

	_res := big.NewInt(0)
	_res.Div(BigHexToBigDecimal(num), big.NewInt(1_000_000_000))

	return fmt.Sprintf("%s Gwei", _res.String())

}
