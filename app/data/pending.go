package data

import (
	"context"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/gammazero/workerpool"
	"github.com/go-redis/redis/v8"
	"github.com/itzmeanjan/harmony/app/config"
)

// PendingPool - Currently present pending tx(s) i.e. which are ready to
// be mined in next block
type PendingPool struct {
	Transactions map[common.Hash]*MemPoolTx
	Lock         *sync.RWMutex
}

// Count - How many tx(s) currently present in pending pool
func (p *PendingPool) Count() uint64 {

	p.Lock.RLock()
	defer p.Lock.RUnlock()

	return uint64(len(p.Transactions))

}

// ListTxs - Returns all tx(s) present in pending pool, as slice
func (p *PendingPool) ListTxs() []*MemPoolTx {

	p.Lock.RLock()
	defer p.Lock.RUnlock()

	result := make([]*MemPoolTx, 0, len(p.Transactions))

	for _, v := range p.Transactions {
		result = append(result, v)
	}

	return result

}

// TopXWithHighGasPrice - Returns only top `X` tx(s) present in pending mempool,
// where being top is determined by how much gas price paid by tx sender
func (p *PendingPool) TopXWithHighGasPrice(x uint64) []*MemPoolTx {

	txs := MemPoolTxs(p.ListTxs())

	if len(txs) == 0 {
		return txs
	}

	sort.Sort(&txs)

	return txs[:x]

}

// SentFrom - Returns a list of pending tx(s) sent from
// specified address
func (p *PendingPool) SentFrom(address common.Address) []*MemPoolTx {

	p.Lock.RLock()
	defer p.Lock.RUnlock()

	result := make([]*MemPoolTx, 0, len(p.Transactions))

	for _, tx := range p.Transactions {

		if tx.IsSentFrom(address) {
			result = append(result, tx)
		}

	}

	return result

}

// SentTo - Returns a list of pending tx(s) sent to
// specified address
func (p *PendingPool) SentTo(address common.Address) []*MemPoolTx {

	p.Lock.RLock()
	defer p.Lock.RUnlock()

	result := make([]*MemPoolTx, 0, len(p.Transactions))

	for _, tx := range p.Transactions {

		if tx.IsSentTo(address) {
			result = append(result, tx)
		}

	}

	return result

}

// OlderThanX - Returns a list of all pending tx(s), which are
// living in mempool for more than or equals to `X` time unit
func (p *PendingPool) OlderThanX(x time.Duration) []*MemPoolTx {

	p.Lock.RLock()
	defer p.Lock.RUnlock()

	result := make([]*MemPoolTx, 0, len(p.Transactions))

	for _, tx := range p.Transactions {

		if tx.IsPendingForGTE(x) {
			result = append(result, tx)
		}

	}

	return result

}

// FresherThanX - Returns a list of all pending tx(s), which are
// living in mempool for less than or equals to `X` time unit
func (p *PendingPool) FresherThanX(x time.Duration) []*MemPoolTx {

	p.Lock.RLock()
	defer p.Lock.RUnlock()

	result := make([]*MemPoolTx, 0, len(p.Transactions))

	for _, tx := range p.Transactions {

		if tx.IsPendingForLTE(x) {
			result = append(result, tx)
		}

	}

	return result

}

// Add - Attempts to add new tx found in pending pool into
// harmony mempool, so that further manipulation can be performed on it
//
// If it returns `true`, it denotes, it's success, otherwise it's failure
// because this tx is already present in pending pool
func (p *PendingPool) Add(ctx context.Context, pubsub *redis.Client, tx *MemPoolTx) bool {

	p.Lock.Lock()
	defer p.Lock.Unlock()

	if _, ok := p.Transactions[tx.Hash]; ok {
		return false
	}

	// Marking we found this tx in mempool now
	tx.PendingFrom = time.Now().UTC()
	tx.Pool = "pending"

	// Creating entry
	p.Transactions[tx.Hash] = tx

	// After adding new tx in pending pool, also attempt to
	// publish it to pubsub topic
	p.PublishAdded(ctx, pubsub, tx)
	return true

}

// PublishAdded - Publish new pending tx pool content ( in messagepack serialized format )
// to pubsub topic
func (p *PendingPool) PublishAdded(ctx context.Context, pubsub *redis.Client, msg *MemPoolTx) {

	_msg, err := msg.ToMessagePack()
	if err != nil {

		log.Printf("[❗️] Failed to serialize into messagepack : %s\n", err.Error())
		return

	}

	if err := pubsub.Publish(ctx, config.GetPendingTxEntryPublishTopic(), _msg).Err(); err != nil {

		log.Printf("[❗️] Failed to publish new pending tx : %s\n", err.Error())

	}

}

// Remove - Removes already existing tx from pending tx pool
// denoting it has been mined i.e. confirmed
func (p *PendingPool) Remove(ctx context.Context, pubsub *redis.Client, txHash common.Hash) bool {

	p.Lock.Lock()
	defer p.Lock.Unlock()

	if _, ok := p.Transactions[txHash]; !ok {
		return false
	}

	// Publishing this confirmed tx
	p.PublishRemoved(ctx, pubsub, p.Transactions[txHash])

	delete(p.Transactions, txHash)

	return true

}

// PublishRemoved - Publish old pending tx pool content ( in messagepack serialized format )
// to pubsub topic
//
// These tx(s) are leaving pending pool i.e. they're confirmed now
func (p *PendingPool) PublishRemoved(ctx context.Context, pubsub *redis.Client, msg *MemPoolTx) {

	_msg, err := msg.ToMessagePack()
	if err != nil {

		log.Printf("[❗️] Failed to serialize into messagepack : %s\n", err.Error())
		return

	}

	if err := pubsub.Publish(ctx, config.GetPendingTxExitPublishTopic(), _msg).Err(); err != nil {

		log.Printf("[❗️] Failed to publish confirmed tx : %s\n", err.Error())

	}

}

// RemoveConfirmed - Removes pending tx(s) from pool which have been confirmed
// & returns how many were removed. If 0 is returned, denotes all tx(s) pending last time
// are still in pending state
func (p *PendingPool) RemoveConfirmed(ctx context.Context, rpc *rpc.Client, pubsub *redis.Client, txs map[string]map[string]*MemPoolTx) uint64 {

	if len(p.Transactions) == 0 {
		return 0
	}

	buffer := make([]common.Hash, 0, len(p.Transactions))

	// -- Attempt to safely find out which txHashes
	// are confirmed i.e. included in a recent block
	//
	// So we can also remove those from our pending pool
	p.Lock.RLock()

	// Creating worker pool, where jobs to be submitted
	// for concurrently checking status of tx(s)
	wp := workerpool.New(config.GetConcurrencyFactor())

	commChan := make(chan TxStatus, len(p.Transactions))

	for _, tx := range p.Transactions {

		func(tx *MemPoolTx) {

			wp.Submit(func() {

				// If this pending tx is present in current
				// pending pool state obtained, no need
				// check whether tx is confirmed or not
				if IsPresentInCurrentPool(txs, tx.Hash) {

					commChan <- TxStatus{Hash: tx.Hash, Status: false}
					return

				}

				// Checking whether this nonce is used
				// in any mined tx ( including this )
				yes, err := tx.IsNonceExhausted(ctx, rpc)
				if err != nil {

					log.Printf("[❗️] Failed to check if nonce exhausted : %s\n", err.Error())

					commChan <- TxStatus{Hash: tx.Hash, Status: false}
					return

				}

				commChan <- TxStatus{Hash: tx.Hash, Status: yes}

			})

		}(tx)

	}

	var received int
	mustReceive := len(p.Transactions)

	// Waiting for all go routines to finish
	for v := range commChan {

		if v.Status {
			buffer = append(buffer, v.Hash)
		}

		received++
		if received >= mustReceive {
			break
		}

	}

	// This call is irrelevant here, but still being made
	//
	// Because all workers have exited, otherwise we could have never
	// reached this point
	wp.Stop()

	p.Lock.RUnlock()
	// -- Done with safely reading to be removed tx(s)

	// All pending tx(s) present in last iteration
	// also present in now
	//
	// Nothing has changed, so we can't remove any older tx(s)
	if len(buffer) == 0 {
		return 0
	}

	var count uint64

	// Iteratively removing entries which are
	// not supposed to be present in pending mempool
	// anymore
	for _, v := range buffer {

		if !p.Remove(ctx, pubsub, v) {
			log.Printf("[❗️] Failed to remove confirmed tx from pending pool\n")
			continue
		}

		count++

	}

	return count

}

// AddPendings - Update latest pending pool state
func (p *PendingPool) AddPendings(ctx context.Context, pubsub *redis.Client, txs map[string]map[string]*MemPoolTx) uint64 {

	var count uint64

	for _, vOuter := range txs {
		for _, vInner := range vOuter {

			if p.Add(ctx, pubsub, vInner) {
				count++
			}

		}
	}

	return count

}
