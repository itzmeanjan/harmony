package listen

import (
	"context"
	"log"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

// CaughtTx - Tx caught by block head subscriber, passed to
// pending pool watcher, so that it can prune its state
type CaughtTx struct {
	Hash  common.Hash
	Nonce uint64
}

// CaughtTxs - Just a slice of txs, which we found to be present in a recently
// mined block
type CaughtTxs []*CaughtTx

// SubscribeHead - Subscribe to block headers & as soon as new block gets mined
// its txs are picked up & published on a go channel, which will be listened
// to by pending pool watcher, so that it can prune its state
func SubscribeHead(ctx context.Context, client *ethclient.Client, lastSeenBlock uint64, commChan chan<- CaughtTxs, lastSeenBlockChan chan<- uint64, healthChan chan struct{}) {

	retryTable := make(map[*big.Int]struct{})
	lastRetried := time.Now()
	headerChan := make(chan *types.Header, 64)
	subs, err := client.SubscribeNewHead(ctx, headerChan)
	if err != nil {
		log.Printf("❗️ Failed to subscribe to block headers : %s\n", err.Error())
		return
	}

	for {

		select {

		case <-ctx.Done():
			subs.Unsubscribe()
			return

		case err := <-subs.Err():
			if err != nil {
				log.Printf("❗️ Block header subscription failed : %s\n", err.Error())
			} else {
				log.Printf("❗️ Block header subscription failed\n")
			}

			// Notify supervisor this worker is dying
			close(healthChan)
			return

		case header := <-headerChan:

			// If this go routine dies in mid, supervisor will spawn a new one
			// after some delay, which will require processing missed blocks
			if lastSeenBlock != 0 && header.Number.Uint64()-lastSeenBlock > 1 {

				for i := lastSeenBlock + 1; i < header.Number.Uint64(); i++ {
					retryTable[big.NewInt(int64(i))] = struct{}{}
				}

			}

			if !ProcessBlock(ctx, client, header.Number, commChan, lastSeenBlockChan) {
				// Put entry in table that we failed to fetch this block, to be
				// attempted in some time future
				retryTable[header.Number] = struct{}{}
			}

			lastSeenBlock = header.Number.Uint64()

		case <-time.After(time.Duration(64) * time.Millisecond):

			if !(time.Since(lastRetried) > time.Duration(2)*time.Minute) {
				break
			}

			pendingC := len(retryTable)
			if pendingC == 0 {
				break
			}

			log.Printf("🔁 Retrying %d block(s)\n", pendingC)
			lastRetried = time.Now()

			successC := 0
			for num := range retryTable {
				if ProcessBlock(ctx, client, num, commChan, lastSeenBlockChan) {
					delete(retryTable, num)
					successC++
				}
			}
			log.Printf("🎉 Processed %d pending block(s)\n", successC)

		}

	}

}

// ProcessBlock - Fetches all txs present in mined block & passes those to pending pool pruning worker
func ProcessBlock(ctx context.Context, client *ethclient.Client, number *big.Int, commChan chan<- CaughtTxs, lastSeenBlockChan chan<- uint64) bool {

	block, err := client.BlockByNumber(ctx, number)
	if err != nil {

		log.Printf("❗️ Failed to fetch block : %d\n", number)
		return false

	}

	txCount := len(block.Transactions())
	log.Printf("🧱 Block %d mined with %d tx(s)\n", number, txCount)

	// We've nothing to share with pruning worker
	if txCount == 0 {
		return true
	}

	txs := make([]*CaughtTx, 0, txCount)

	for _, tx := range block.Transactions() {

		txs = append(txs, &CaughtTx{
			Hash:  tx.Hash(),
			Nonce: tx.Nonce(),
		})

	}

	commChan <- txs
	lastSeenBlockChan <- number.Uint64()
	return true

}
