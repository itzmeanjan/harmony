package data

import (
	"context"
	"log"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/gammazero/workerpool"
	"github.com/go-redis/redis/v8"
	"github.com/itzmeanjan/harmony/app/config"
	"github.com/itzmeanjan/harmony/app/listen"
)

// PendingPool - Currently present pending tx(s) i.e. which are ready to
// be mined in next block
type PendingPool struct {
	Transactions      map[common.Hash]*MemPoolTx
	AscTxsByGasPrice  TxList
	DescTxsByGasPrice TxList
	Lock              *sync.RWMutex
	IsPruning         bool
	AddTxChan         chan AddRequest
	RemoveTxChan      chan RemoveRequest
	TxExistsChan      chan ExistsRequest
	GetTxChan         chan GetRequest
	CountTxsChan      chan CountRequest
	ListTxsChan       chan ListRequest
	PubSub            *redis.Client
	RPC               *rpc.Client
}

// Start - This method is supposed to be run as an independent
// go routine, maintaining pending pool state, through out its life time
func (p *PendingPool) Start(ctx context.Context) {

	// Just a closure, which will remove existing tx
	// from pending pool, assuming it has been confirmed/ dropped
	//
	// This is extracted out here, for ease of usability
	txRemover := func(txStat *TxStatus) bool {

		// -- Safe reading, begins
		p.Lock.RLock()

		tx, ok := p.Transactions[txStat.Hash]
		if !ok {

			p.Lock.RUnlock()
			return false

		}

		p.Lock.RUnlock()
		// -- reading ends

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

		// -- Safe writing, critical section begins
		p.Lock.Lock()

		// Remove from sorted tx list, keep it sorted
		p.AscTxsByGasPrice = Remove(p.AscTxsByGasPrice, tx)
		p.DescTxsByGasPrice = Remove(p.DescTxsByGasPrice, tx)

		delete(p.Transactions, txStat.Hash)

		p.Lock.Unlock()
		// -- writing ends

		// Publishing this confirmed/ dropped tx
		p.PublishRemoved(ctx, p.PubSub, tx)

		return true

	}

	// Closure for checking whether adding new tx triggers
	// condition for dropping some other tx
	//
	// Selecting which tx to be dropped
	//
	// - Tx with lowest gas price paid ✅
	// - Oldest tx living in mempool ❌
	// - Oldest tx with lowest gas price paid ❌
	//
	// ✅ : Implemented
	// ❌ : Not yet
	needToDropTxs := func() bool {

		p.Lock.RLock()
		defer p.Lock.RUnlock()

		return uint64(p.AscTxsByGasPrice.len())+1 > config.GetPendingPoolSize()

	}

	pickTxWithLowestGasPrice := func() *MemPoolTx {

		p.Lock.RLock()
		defer p.Lock.RUnlock()

		return p.AscTxsByGasPrice.get()[0]

	}

	// Silently drop some tx, before adding
	// new one, so that we don't exceed limit
	// set up by user
	dropTx := func() {

		// because this is the only scheme currently
		// supported
		tx := pickTxWithLowestGasPrice()

		// -- Safe writing, critical section begins
		p.Lock.Lock()
		defer p.Lock.Unlock()

		// Remove from sorted tx list, keep it sorted
		p.AscTxsByGasPrice = Remove(p.AscTxsByGasPrice, tx)
		p.DescTxsByGasPrice = Remove(p.DescTxsByGasPrice, tx)

		delete(p.Transactions, tx.Hash)

	}

	for {

		select {

		case <-ctx.Done():
			return

		case req := <-p.AddTxChan:

			// This is what we're trying to add
			tx := req.Tx

			// -- Safe reading begins
			p.Lock.RLock()

			if _, ok := p.Transactions[tx.Hash]; ok {
				req.ResponseChan <- false

				p.Lock.RUnlock()
				break
			}

			p.Lock.RUnlock()
			// -- reading ends

			// Marking we found this tx in mempool now
			tx.PendingFrom = time.Now().UTC()
			tx.Pool = "pending"

			// --- Critical section, starts
			p.Lock.Lock()

			// Creating entry
			p.Transactions[tx.Hash] = tx

			// Insert into sorted pending tx list, keep sorted
			p.AscTxsByGasPrice = Insert(p.AscTxsByGasPrice, tx)
			p.DescTxsByGasPrice = Insert(p.DescTxsByGasPrice, tx)

			p.Lock.Unlock()
			// --- ends here

			// After adding new tx in pending pool, also attempt to
			// publish it to pubsub topic
			p.PublishAdded(ctx, p.PubSub, tx)

			req.ResponseChan <- true

		case req := <-p.RemoveTxChan:

			req.ResponseChan <- txRemover(req.TxStat)

		case req := <-p.TxExistsChan:

			// -- Safe reading begins
			p.Lock.RLock()

			_, ok := p.Transactions[req.Tx]

			p.Lock.RUnlock()
			// -- ends

			req.ResponseChan <- ok

		case req := <-p.GetTxChan:

			// -- Safe reading begins
			p.Lock.RLock()

			if tx, ok := p.Transactions[req.Tx]; ok {
				req.ResponseChan <- tx

				p.Lock.RUnlock()
				break
			}

			p.Lock.RUnlock()
			// -- ends

			req.ResponseChan <- nil

		case req := <-p.CountTxsChan:

			// -- Safe reading begins
			p.Lock.RLock()

			req.ResponseChan <- uint64(p.AscTxsByGasPrice.len())

			p.Lock.RUnlock()
			// -- ends

		case req := <-p.ListTxsChan:

			if req.Order == ASC {
				// -- Safe reading begins
				p.Lock.RLock()

				// If empty, just return nil
				if p.AscTxsByGasPrice.len() == 0 {
					req.ResponseChan <- nil

					p.Lock.RUnlock()
					break
				}

				copied := make([]*MemPoolTx, p.AscTxsByGasPrice.len())
				copy(copied, p.AscTxsByGasPrice.get())

				req.ResponseChan <- copied

				p.Lock.RUnlock()
				// -- ends
				break
			}

			if req.Order == DESC {
				// -- Safe reading begins
				p.Lock.RLock()

				// If empty, just return nil
				if p.DescTxsByGasPrice.len() == 0 {
					req.ResponseChan <- nil

					p.Lock.RUnlock()
					break
				}

				copied := make([]*MemPoolTx, p.DescTxsByGasPrice.len())
				copy(copied, p.DescTxsByGasPrice.get())

				req.ResponseChan <- copied

				p.Lock.RUnlock()
				// -- ends
			}

		}

	}

}

// Prune - Remove confirmed/ dropped txs from pending pool
//
// @note This method is supposed to be run as independent go routine
func (p *PendingPool) Prune(ctx context.Context, commChan chan listen.CaughtTxs) {

	// Creating worker pool, where jobs to be submitted
	// for concurrently checking whether tx was dropped or not
	wp := workerpool.New(config.GetConcurrencyFactor())
	defer wp.Stop()

	internalChan := make(chan *TxStatus, 1024)
	var droppedOrConfirmed uint64

	for {

		select {

		case <-ctx.Done():
			return

		case txs := <-commChan:

			var expected uint64

			for i := 0; i < len(txs); i++ {

				func(tx *listen.CaughtTx) {

					wp.Submit(func() {

						prunables := p.Prunables(tx.Hash)
						if prunables == nil {
							return
						}

						// Atomic increment, concurrent-safe [ expecting to behave well on all platforms ]
						atomic.AddUint64(&expected, uint64(len(prunables)))

						for i := 0; i < len(prunables); i++ {

							func(idx int, tx *MemPoolTx) {

								// First element of this list is always
								// which tx got mined in block, so it's confirmed
								//
								// We might find some associated tx(s), which will non-zero-indexed
								// elements, so it's safe to avoid `IsDropped` check for first txHash
								if idx == 0 {
									internalChan <- &TxStatus{Hash: tx.Hash, Status: CONFIRMED}
									return
								}

								wp.Submit(func() {

									// Tx got confirmed/ dropped, to be used when computing
									// how long it spent in pending pool
									dropped, _ := tx.IsDropped(ctx, p.RPC)
									if dropped {

										internalChan <- &TxStatus{Hash: tx.Hash, Status: DROPPED}
										return

									}

									internalChan <- &TxStatus{Hash: tx.Hash, Status: CONFIRMED}

								})

							}(i, prunables[i])

						}

					})

				}(txs[i])

			}

		case tx := <-internalChan:

			if tx.Status == CONFIRMED || tx.Status == DROPPED {

				// Keep pruning as soon as we determined it can be pruned, rather than wait
				// for all to come & then doing it
				if p.Remove(ctx, tx) {
					droppedOrConfirmed++

					if droppedOrConfirmed%10 == 0 {
						log.Printf("[➖] Removed 10 tx(s) from pending tx pool\n")
					}
				}

			}

		}

	}

}

// Get - Given tx hash, attempts to find out tx in pending pool, if any
//
// Returns nil, if found nothing
func (p *PendingPool) Get(hash common.Hash) *MemPoolTx {

	respChan := make(chan *MemPoolTx)

	p.GetTxChan <- GetRequest{Tx: hash, ResponseChan: respChan}

	return <-respChan

}

// Exists - Checks whether tx of given hash exists on pending pool or not
func (p *PendingPool) Exists(hash common.Hash) bool {

	respChan := make(chan bool)

	p.TxExistsChan <- ExistsRequest{Tx: hash, ResponseChan: respChan}

	return <-respChan

}

// Count - How many tx(s) currently present in pending pool
func (p *PendingPool) Count() uint64 {

	respChan := make(chan uint64)

	p.CountTxsChan <- CountRequest{ResponseChan: respChan}

	return <-respChan

}

// Prunables - Given tx hash, with which a tx has been mined
// we're attempting to find out all txs which are living in pending pool
// now & having same sender address & same/ lower nonce, so that
// pruner can update state while removing mined txs from mempool
func (p *PendingPool) Prunables(hash common.Hash) []*MemPoolTx {

	targetTx := p.Get(hash)
	if targetTx == nil {
		return nil
	}

	txs := p.DescListTxs()
	if txs == nil {
		return nil
	}

	txCount := uint64(len(txs))
	commChan := make(chan *MemPoolTx, txCount)
	result := make([]*MemPoolTx, 0, txCount)
	// This is the tx which got mined
	result = append(result, targetTx)

	// Attempting to concurrently checking which txs are duplicate
	// of a given tx hash
	wp := workerpool.New(runtime.NumCPU())

	for i := 0; i < len(txs); i++ {

		func(tx *MemPoolTx) {

			wp.Submit(func() {

				if tx.IsLowerNonce(targetTx) {
					commChan <- tx
					return
				}

				commChan <- nil

			})

		}(txs[i])

	}

	var received uint64
	mustReceive := txCount

	// Waiting for all go routines to finish
	for v := range commChan {

		if v != nil {
			result = append(result, v)
		}

		received++
		if received >= mustReceive {
			break
		}

	}

	// This call is irrelevant here, probably, but still being made
	//
	// Because all workers have exited, otherwise we could have never
	// reached this point
	wp.Stop()

	return result

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

	targetTx := p.Get(hash)
	if targetTx == nil {
		return nil
	}

	txs := p.DescListTxs()
	if txs == nil {
		return nil
	}

	txCount := uint64(len(txs))
	commChan := make(chan *MemPoolTx, txCount)
	result := make([]*MemPoolTx, 0, txCount)

	// Attempting to concurrently checking which txs are duplicate
	// of a given tx hash
	wp := workerpool.New(runtime.NumCPU())

	for i := 0; i < len(txs); i++ {

		func(tx *MemPoolTx) {

			wp.Submit(func() {

				if tx.IsDuplicateOf(targetTx) {
					commChan <- tx
					return
				}

				commChan <- nil

			})

		}(txs[i])

	}

	var received uint64
	mustReceive := txCount

	// Waiting for all go routines to finish
	for v := range commChan {

		if v != nil {
			result = append(result, v)
		}

		received++
		if received >= mustReceive {
			break
		}

	}

	// This call is irrelevant here, probably, but still being made
	//
	// Because all workers have exited, otherwise we could have never
	// reached this point
	wp.Stop()

	return result

}

// AscListTxs - Returns all tx(s) present in pending pool, as slice, ascending ordered as per gas price paid
func (p *PendingPool) AscListTxs() []*MemPoolTx {

	respChan := make(chan []*MemPoolTx)

	p.ListTxsChan <- ListRequest{ResponseChan: respChan, Order: ASC}

	return <-respChan

}

// DescListTxs - Returns all tx(s) present in pending pool, as slice, descending ordered as per gas price paid
func (p *PendingPool) DescListTxs() []*MemPoolTx {

	respChan := make(chan []*MemPoolTx)

	p.ListTxsChan <- ListRequest{ResponseChan: respChan, Order: DESC}

	return <-respChan

}

// TopXWithHighGasPrice - Returns only top `X` tx(s) present in pending mempool,
// where being top is determined by how much gas price paid by tx sender
func (p *PendingPool) TopXWithHighGasPrice(x uint64) []*MemPoolTx {
	return p.DescListTxs()[:x]
}

// TopXWithLowGasPrice - Returns only top `X` tx(s) present in pending mempool,
// where being top is determined by how low gas price paid by tx sender
func (p *PendingPool) TopXWithLowGasPrice(x uint64) []*MemPoolTx {
	return p.AscListTxs()[:x]
}

// SentFrom - Returns a list of pending tx(s) sent from
// specified address
func (p *PendingPool) SentFrom(address common.Address) []*MemPoolTx {

	txs := p.DescListTxs()
	if txs == nil {
		return nil
	}

	txCount := uint64(len(txs))
	commChan := make(chan *MemPoolTx, txCount)
	result := make([]*MemPoolTx, 0, txCount)

	wp := workerpool.New(runtime.NumCPU())

	for i := 0; i < len(txs); i++ {

		func(tx *MemPoolTx) {

			wp.Submit(func() {

				if tx.IsSentFrom(address) {
					commChan <- tx
					return
				}

				commChan <- nil

			})

		}(txs[i])

	}

	var received uint64
	mustReceive := txCount

	// Waiting for all go routines to finish
	for v := range commChan {

		if v != nil {
			result = append(result, v)
		}

		received++
		if received >= mustReceive {
			break
		}

	}

	// This call is irrelevant here, probably, but still being made
	//
	// Because all workers have exited, otherwise we could have never
	// reached this point
	wp.Stop()

	return result

}

// SentTo - Returns a list of pending tx(s) sent to
// specified address
func (p *PendingPool) SentTo(address common.Address) []*MemPoolTx {

	txs := p.DescListTxs()
	if txs == nil {
		return nil
	}

	txCount := uint64(len(txs))
	commChan := make(chan *MemPoolTx, txCount)
	result := make([]*MemPoolTx, 0, txCount)

	wp := workerpool.New(runtime.NumCPU())

	for i := 0; i < len(txs); i++ {

		func(tx *MemPoolTx) {

			wp.Submit(func() {

				if tx.IsSentTo(address) {
					commChan <- tx
					return
				}

				commChan <- nil

			})

		}(txs[i])

	}

	var received uint64
	mustReceive := txCount

	// Waiting for all go routines to finish
	for v := range commChan {

		if v != nil {
			result = append(result, v)
		}

		received++
		if received >= mustReceive {
			break
		}

	}

	// This call is irrelevant here, probably, but still being made
	//
	// Because all workers have exited, otherwise we could have never
	// reached this point
	wp.Stop()

	return result

}

// OlderThanX - Returns a list of all pending tx(s), which are
// living in mempool for more than or equals to `X` time unit
func (p *PendingPool) OlderThanX(x time.Duration) []*MemPoolTx {

	txs := p.DescListTxs()
	if txs == nil {
		return nil
	}

	txCount := uint64(len(txs))
	commChan := make(chan *MemPoolTx, txCount)
	result := make([]*MemPoolTx, 0, txCount)

	wp := workerpool.New(runtime.NumCPU())

	for i := 0; i < len(txs); i++ {

		func(tx *MemPoolTx) {

			wp.Submit(func() {

				if tx.IsPendingForGTE(x) {
					commChan <- tx
					return
				}

				commChan <- nil

			})

		}(txs[i])

	}

	var received uint64
	mustReceive := txCount

	// Waiting for all go routines to finish
	for v := range commChan {

		if v != nil {
			result = append(result, v)
		}

		received++
		if received >= mustReceive {
			break
		}

	}

	// This call is irrelevant here, probably, but still being made
	//
	// Because all workers have exited, otherwise we could have never
	// reached this point
	wp.Stop()

	return result

}

// FresherThanX - Returns a list of all pending tx(s), which are
// living in mempool for less than or equals to `X` time unit
func (p *PendingPool) FresherThanX(x time.Duration) []*MemPoolTx {

	txs := p.DescListTxs()
	if txs == nil {
		return nil
	}

	txCount := uint64(len(txs))
	commChan := make(chan *MemPoolTx, txCount)
	result := make([]*MemPoolTx, 0, txCount)

	wp := workerpool.New(runtime.NumCPU())

	for i := 0; i < len(txs); i++ {

		func(tx *MemPoolTx) {

			wp.Submit(func() {

				if tx.IsPendingForLTE(x) {
					commChan <- tx
					return
				}

				commChan <- nil

			})

		}(txs[i])

	}

	var received uint64
	mustReceive := txCount

	// Waiting for all go routines to finish
	for v := range commChan {

		if v != nil {
			result = append(result, v)
		}

		received++
		if received >= mustReceive {
			break
		}

	}

	// This call is irrelevant here, probably, but still being made
	//
	// Because all workers have exited, otherwise we could have never
	// reached this point
	wp.Stop()

	return result

}

// Add - Attempts to add new tx found in pending pool into
// harmony mempool, so that further manipulation can be performed on it
//
// If it returns `true`, it denotes, it's success, otherwise it's failure
// because this tx is already present in pending pool
func (p *PendingPool) Add(ctx context.Context, tx *MemPoolTx) bool {

	respChan := make(chan bool)

	p.AddTxChan <- AddRequest{Tx: tx, ResponseChan: respChan}

	return <-respChan

}

// VerifiedAdd - Before adding tx from queued pool, just check do we
// really need to add this tx in pending pool i.e. is this tx really
// pending ?
func (p *PendingPool) VerifiedAdd(ctx context.Context, tx *MemPoolTx) bool {

	ok, err := tx.IsNonceExhausted(ctx, p.RPC)
	if err != nil {
		return false
	}

	if ok {
		return false
	}

	return p.Add(ctx, tx)

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
// denoting it has been mined i.e. confirmed/ dropped ( possible too )
func (p *PendingPool) Remove(ctx context.Context, txStat *TxStatus) bool {

	respChan := make(chan bool)

	p.RemoveTxChan <- RemoveRequest{TxStat: txStat, ResponseChan: respChan}

	return <-respChan

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

// AddPendings - Update latest pending pool state
func (p *PendingPool) AddPendings(ctx context.Context, txs map[string]map[string]*MemPoolTx) uint64 {

	var count uint64

	for keyO := range txs {
		for keyI := range txs[keyO] {

			if p.Add(ctx, txs[keyO][keyI]) {
				count++
			}

		}
	}

	return count

}
