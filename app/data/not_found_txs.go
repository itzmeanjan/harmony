package data

import (
	"context"

	"github.com/ethereum/go-ethereum/common"
	"github.com/itzmeanjan/harmony/app/listen"
)

// TrackNotFoundTxs - Those tx(s) which are found to be mined before actual pending tx is added
// into mempool, are kept track of here & pending pool pruner is informed about it, when it's found
// to be joining mempool, in some time future.
func TrackNotFoundTxs(ctx context.Context, inPendingPoolChan <-chan *MemPoolTx, notFoundTxsChan <-chan listen.CaughtTxs, alreadyMinedTxChan chan<- listen.CaughtTxs) {

	// Yes, for faster lookup, it's kept this
	keeper := make(map[common.Hash]*listen.CaughtTx)

	for {
		select {
		case <-ctx.Done():
			return

		case tx := <-inPendingPoolChan:
			// Just learnt about new tx which is considered to be pending
			// & added into pool, but this tx is mined in some previous block
			// which we've kept track of here in `keeper` structure
			//
			// We're letting pending pool pruner know, it must act in declaring this tx
			// to be mined

			if kept, ok := keeper[tx.Hash]; ok {
				alreadyMinedTxChan <- []*listen.CaughtTx{kept}
				delete(keeper, tx.Hash)
			}

		case txs := <-notFoundTxsChan:
			// New block was mined with some txs
			// which were not found in pending pool then,
			// which are being kept track of here
			//
			// To be used for letting pending pool know
			// some newly added pending pool tx is actually mined
			// in some later date

			for i := 0; i < len(txs); i++ {
				keeper[txs[i].Hash] = txs[i]
			}

		}
	}

}
