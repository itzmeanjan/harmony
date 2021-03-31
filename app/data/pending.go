package data

import (
	"context"
	"log"
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
	Transactions      map[common.Hash]*MemPoolTx
	AscTxsByGasPrice  TxList
	DescTxsByGasPrice TxList
	Lock              *sync.RWMutex
}

// Get - Given tx hash, attempts to find out tx in pending pool, if any
//
// Returns nil, if found nothing
func (p *PendingPool) Get(hash common.Hash) *MemPoolTx {

	p.Lock.RLock()
	defer p.Lock.RUnlock()

	if tx, ok := p.Transactions[hash]; ok {
		return tx
	}

	return nil

}

// Exists - Checks whether tx of given hash exists on pending pool or not
func (p *PendingPool) Exists(hash common.Hash) bool {

	p.Lock.RLock()
	defer p.Lock.RUnlock()

	_, ok := p.Transactions[hash]
	return ok

}

// Count - How many tx(s) currently present in pending pool
func (p *PendingPool) Count() uint64 {

	p.Lock.RLock()
	defer p.Lock.RUnlock()

	return uint64(p.AscTxsByGasPrice.len())

}

// DuplicateTxs - Attempting to find duplicate tx(s) for given
// txHash.
//
// @note In duplicate tx list, the tx which was provided as input
// will not be included
//
// Considering one tx duplicate of given one, if this tx has same
// nonce & sender address, as of given ones
func (p *PendingPool) DuplicateTxs(hash common.Hash) []*MemPoolTx {

	p.Lock.RLock()
	defer p.Lock.RUnlock()

	// Denotes tx hash itself doesn't exists in pending pool
	//
	// So we can't find duplicate tx(s) for that txHash
	targetTx, ok := p.Transactions[hash]
	if !ok {
		return nil
	}

	result := make([]*MemPoolTx, 0, p.Count())

	for _, tx := range p.DescTxsByGasPrice.get() {

		// First checking if tx under radar is the one for which
		// we're finding duplicate tx(s). If yes, we will move to next one
		//
		// Now we can check whether current tx under radar is having same nonce
		// and sender address, as of target tx ( for which we had txHash, as input )
		// or not
		//
		// If yes, we'll include it considerable duplicate tx list, for given
		// txHash
		if tx.IsDuplicateOf(targetTx) {
			result = append(result, tx)
		}

	}

	return result

}

// ListTxs - Returns all tx(s) present in pending pool, as slice
func (p *PendingPool) ListTxs() []*MemPoolTx {

	p.Lock.RLock()
	defer p.Lock.RUnlock()

	return p.DescTxsByGasPrice.get()

}

// TopXWithHighGasPrice - Returns only top `X` tx(s) present in pending mempool,
// where being top is determined by how much gas price paid by tx sender
func (p *PendingPool) TopXWithHighGasPrice(x uint64) []*MemPoolTx {

	p.Lock.RLock()
	defer p.Lock.RUnlock()

	return p.DescTxsByGasPrice.get()[:x]

}

// TopXWithLowGasPrice - Returns only top `X` tx(s) present in pending mempool,
// where being top is determined by how low gas price paid by tx sender
func (p *PendingPool) TopXWithLowGasPrice(x uint64) []*MemPoolTx {

	p.Lock.RLock()
	defer p.Lock.RUnlock()

	return p.AscTxsByGasPrice.get()[:x]

}

// SentFrom - Returns a list of pending tx(s) sent from
// specified address
func (p *PendingPool) SentFrom(address common.Address) []*MemPoolTx {

	p.Lock.RLock()
	defer p.Lock.RUnlock()

	result := make([]*MemPoolTx, 0, p.Count())

	for _, tx := range p.DescTxsByGasPrice.get() {

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

	result := make([]*MemPoolTx, 0, p.Count())

	for _, tx := range p.DescTxsByGasPrice.get() {

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

	result := make([]*MemPoolTx, 0, p.Count())

	for _, tx := range p.DescTxsByGasPrice.get() {

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

	result := make([]*MemPoolTx, 0, p.Count())

	for _, tx := range p.DescTxsByGasPrice.get() {

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

	// Insert into sorted pending tx list, keep sorted
	p.AscTxsByGasPrice = Insert(p.AscTxsByGasPrice, tx)
	p.DescTxsByGasPrice = Insert(p.DescTxsByGasPrice, tx)

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
func (p *PendingPool) Remove(ctx context.Context, pubsub *redis.Client, txStat *TxStatus) bool {

	p.Lock.Lock()
	defer p.Lock.Unlock()

	tx, ok := p.Transactions[txStat.Hash]
	if !ok {
		return false
	}

	// Tx got confirmed/ dropped, to be used when computing
	// how long it spent in pending pool
	if txStat.Status == DROPPED {
		tx.Pool = "dropped"
		tx.DroppedAt = time.Now().UTC()
	}

	if txStat.Status == CONFIRMED {
		tx.Pool = "confirmed"
		tx.ConfirmedAt = time.Now().UTC()
	}

	// Publishing this confirmed tx
	p.PublishRemoved(ctx, pubsub, tx)

	// Remove from sorted tx list, keep it sorted
	p.AscTxsByGasPrice = Remove(p.AscTxsByGasPrice, tx)
	p.DescTxsByGasPrice = Remove(p.DescTxsByGasPrice, tx)

	delete(p.Transactions, txStat.Hash)

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
func (p *PendingPool) RemoveConfirmedAndDropped(ctx context.Context, rpc *rpc.Client, pubsub *redis.Client, txs map[string]map[string]*MemPoolTx) uint64 {

	if len(p.Transactions) == 0 {
		return 0
	}

	// -- Attempt to safely find out which txHashes
	// are confirmed i.e. included in a recent block
	//
	// So we can also remove those from our pending pool
	p.Lock.RLock()

	// Creating worker pool, where jobs to be submitted
	// for concurrently checking status of tx(s)
	wp := workerpool.New(config.GetConcurrencyFactor())

	commChan := make(chan *TxStatus, len(p.Transactions))

	for _, tx := range p.Transactions {

		func(tx *MemPoolTx) {

			wp.Submit(func() {

				// If this pending tx is present in current
				// pending pool state obtained, no need
				// check whether tx is confirmed or not
				if IsPresentInCurrentPool(txs, tx.Hash) {

					commChan <- &TxStatus{Hash: tx.Hash, Status: PENDING}
					return

				}

				// Checking whether this nonce is used
				// in any mined tx ( including this )
				yes, err := tx.IsNonceExhausted(ctx, rpc)
				if err != nil {

					log.Printf("[❗️] Failed to check if nonce exhausted : %s\n", err.Error())

					commChan <- &TxStatus{Hash: tx.Hash, Status: PENDING}
					return

				}

				if !yes {

					commChan <- &TxStatus{Hash: tx.Hash, Status: PENDING}
					return

				}

				// Tx got confirmed/ dropped, to be used when computing
				// how long it spent in pending pool
				dropped, _ := tx.IsDropped(ctx, rpc)
				if dropped {

					commChan <- &TxStatus{Hash: tx.Hash, Status: DROPPED}
					return

				}

				commChan <- &TxStatus{Hash: tx.Hash, Status: CONFIRMED}

			})

		}(tx)

	}

	buffer := make([]*TxStatus, 0, len(p.Transactions))

	var received int
	mustReceive := len(p.Transactions)

	// Waiting for all go routines to finish
	for v := range commChan {

		if v.Status == CONFIRMED || v.Status == DROPPED {
			buffer = append(buffer, v)
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

		if p.Remove(ctx, pubsub, v) {
			count++
		}

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
