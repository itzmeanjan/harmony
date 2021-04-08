package listen

import (
	"context"
	"log"

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

// SubscribeHead - Subscribe to block headers & as soon as new block gets mined
// its txs are picked up & published on a go channel, which will be listened
// to by pending pool watcher, so that it can prune its state
func SubscribeHead(ctx context.Context, client *ethclient.Client, commChan chan *CaughtTx) {

	headerChan := make(chan *types.Header, 1)
	subs, err := client.SubscribeNewHead(ctx, headerChan)
	if err != nil {
		log.Printf("[!] Failed to subscribe to block headers : %s\n", err.Error())
		return
	}

	for {

		select {

		case <-ctx.Done():
			return

		case err := <-subs.Err():
			log.Printf("[!] Block header subscription failed : %s\n", err.Error())
			return

		case header := <-headerChan:

			block, err := client.BlockByNumber(ctx, header.Number)
			if err != nil {
				log.Printf("[!] Failed to fetch block : %d\n", header.Number.Uint64())
				break
			}

			for _, tx := range block.Transactions() {

				// Hopefully pending pool watcher will read this
				// & prune its state
				commChan <- &CaughtTx{
					Hash:  tx.Hash(),
					Nonce: tx.Nonce(),
				}

			}

		}

	}

}
