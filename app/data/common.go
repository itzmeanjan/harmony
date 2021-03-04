package data

import "github.com/ethereum/go-ethereum/common"

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
