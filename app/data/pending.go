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
	Lock              *sync.Mutex
	IsPruning         bool
	AddTxChan         chan AddRequest
	RemoveTxChan      chan RemoveRequest
	RemoveTxsChan     chan RemoveTxsFromPendingPool
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
		p.PublishRemoved(ctx, p.PubSub, tx)

		// Remove from sorted tx list, keep it sorted
		p.AscTxsByGasPrice = Remove(p.AscTxsByGasPrice, tx)
		p.DescTxsByGasPrice = Remove(p.DescTxsByGasPrice, tx)

		delete(p.Transactions, txStat.Hash)

		return true

	}

	for {

		select {

		case <-ctx.Done():
			return

		case req := <-p.AddTxChan:

			// This is what we're trying to add
			tx := req.Tx

			if _, ok := p.Transactions[tx.Hash]; ok {
				req.ResponseChan <- false
				break
			}

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

		case req := <-p.RemoveTxsChan:

			if p.IsPruning {
				req.ResponseChan <- false
				break
			}

			if p.DescTxsByGasPrice.len() == 0 {
				req.ResponseChan <- false
				break
			}

			p.IsPruning = true
			go func() {

				defer func() {
					p.IsPruning = false
				}()

				start := time.Now().UTC()

				// Creating worker pool, where jobs to be submitted
				// for concurrently checking status of tx(s)
				wp := workerpool.New(config.GetConcurrencyFactor())

				txs := p.DescTxsByGasPrice.get()
				txCount := uint64(len(txs))
				commChan := make(chan *TxStatus, txCount)

				for i := 0; i < len(txs); i++ {

					func(tx *MemPoolTx) {

						wp.Submit(func() {

							// If it's present in current pool, it's pending
							if IsPresentInCurrentPool(req.Txs, tx.Hash) {

								commChan <- &TxStatus{Hash: tx.Hash, Status: PENDING}
								return

							}

							// Checking whether this nonce is used
							// in any mined tx ( including this )
							yes, err := tx.IsNonceExhausted(ctx, p.RPC)
							if err != nil {

								commChan <- &TxStatus{Hash: tx.Hash, Status: PENDING}
								return

							}

							if !yes {

								commChan <- &TxStatus{Hash: tx.Hash, Status: PENDING}
								return

							}

							// Tx got confirmed/ dropped, to be used when computing
							// how long it spent in pending pool
							dropped, _ := tx.IsDropped(ctx, p.RPC)
							if dropped {

								commChan <- &TxStatus{Hash: tx.Hash, Status: DROPPED}
								return

							}

							commChan <- &TxStatus{Hash: tx.Hash, Status: CONFIRMED}

						})

					}(txs[i])

				}

				buffer := make([]*TxStatus, 0, txCount)

				var received uint64
				mustReceive := txCount

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

				// All pending tx(s) present in last iteration
				// also present in now
				//
				// Nothing has changed, so we can't remove any older tx(s)
				if len(buffer) == 0 {
					return
				}

				var count uint64

				// Iteratively removing entries which are
				// not supposed to be present in pending mempool
				// anymore
				for i := 0; i < len(buffer); i++ {

					p.Lock.Lock()

					if txRemover(buffer[i]) {
						count++
					}

					p.Lock.Unlock()

				}

				log.Printf("[➖] Removed %d tx(s) from pending tx pool, in %s\n", count, time.Now().UTC().Sub(start))

			}()

			req.ResponseChan <- true

		case req := <-p.TxExistsChan:

			_, ok := p.Transactions[req.Tx]
			req.ResponseChan <- ok

		case req := <-p.GetTxChan:

			if tx, ok := p.Transactions[req.Tx]; ok {
				req.ResponseChan <- tx
				break
			}

			req.ResponseChan <- nil

		case req := <-p.CountTxsChan:

			req.ResponseChan <- uint64(p.AscTxsByGasPrice.len())

		case req := <-p.ListTxsChan:

			if req.Order == ASC {
				req.ResponseChan <- p.AscTxsByGasPrice.get()
				break
			}

			if req.Order == DESC {
				req.ResponseChan <- p.DescTxsByGasPrice.get()
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

	// Attempting to concurrently checking which txs are duplicate
	// of a given tx hash
	wp := workerpool.New(config.GetConcurrencyFactor())

	txs := p.DescListTxs()
	txCount := uint64(len(txs))
	commChan := make(chan *MemPoolTx, txCount)
	result := make([]*MemPoolTx, 0, txCount)

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

	wp := workerpool.New(config.GetConcurrencyFactor())

	txs := p.DescListTxs()
	txCount := uint64(len(txs))
	commChan := make(chan *MemPoolTx, txCount)
	result := make([]*MemPoolTx, 0, txCount)

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

	wp := workerpool.New(config.GetConcurrencyFactor())

	txs := p.DescListTxs()
	txCount := uint64(len(txs))
	commChan := make(chan *MemPoolTx, txCount)
	result := make([]*MemPoolTx, 0, txCount)

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

	wp := workerpool.New(config.GetConcurrencyFactor())

	txs := p.DescListTxs()
	txCount := uint64(len(txs))
	commChan := make(chan *MemPoolTx, txCount)
	result := make([]*MemPoolTx, 0, txCount)

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

	wp := workerpool.New(config.GetConcurrencyFactor())

	txs := p.DescListTxs()
	txCount := uint64(len(txs))
	commChan := make(chan *MemPoolTx, txCount)
	result := make([]*MemPoolTx, 0, txCount)

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

// RemoveDroppedAndConfirmed - Given current tx list attempt to remove
// txs which are dropped/ confirmed
func (p *PendingPool) RemoveDroppedAndConfirmed(ctx context.Context, txs map[string]map[string]*MemPoolTx) bool {

	resp := make(chan bool)
	p.RemoveTxsChan <- RemoveTxsFromPendingPool{Txs: txs, ResponseChan: resp}

	return <-resp

}
