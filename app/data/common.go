package data

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/itzmeanjan/harmony/app/graph/model"
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

// ToGraphQL - Given a list of mempool tx(s), convert them to
// compatible list of graphql tx(s)
func ToGraphQL(txs []*MemPoolTx) []*model.MemPoolTx {

	res := make([]*model.MemPoolTx, 0, len(txs))

	for _, tx := range txs {

		res = append(res, tx.ToGraphQL())

	}

	return res

}
