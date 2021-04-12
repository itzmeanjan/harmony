package data

import (
	"context"
	"log"
	"runtime"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/gammazero/workerpool"
	"github.com/go-redis/redis/v8"
	"github.com/itzmeanjan/harmony/app/config"
)

// QueuedPool - Currently present queued tx(s) i.e. these tx(s) are stuck
// due to some reasons, one very common reason is nonce gap
//
// What it essentially denotes is, these tx(s) are not ready to be picked up
// when next block is going to be picked, when these tx(s) are going to be
// moved to pending pool, only they can be considered before mining
type QueuedPool struct {
	Transactions      map[common.Hash]*MemPoolTx
	TxsFromAddress    map[common.Address]TxList
	DroppedTxs        map[common.Hash]bool
	AscTxsByGasPrice  TxList
	DescTxsByGasPrice TxList
	AddTxChan         chan AddRequest
	RemoveTxChan      chan RemovedUnstuckTx
	TxExistsChan      chan ExistsRequest
	GetTxChan         chan GetRequest
	CountTxsChan      chan CountRequest
	ListTxsChan       chan ListRequest
	TxsFromAChan      chan TxsFromARequest
	PubSub            *redis.Client
	RPC               *rpc.Client
	PendingPool       *PendingPool
}

// hasBeenAllocatedFor - Checking whether memory has been allocated
// for storing all txs from certain address `A`, living in queued pool
func (q *QueuedPool) hasBeenAllocatedFor(addr common.Address) bool {

	_, ok := q.TxsFromAddress[addr]
	return ok

}

// allocateFor - Primarily allocate some memory for keeping
// track of which txs are sent from address `A`, in ascending order
// as per nonce
func (q *QueuedPool) allocateFor(addr common.Address) TxList {

	if q.hasBeenAllocatedFor(addr) {
		return q.TxsFromAddress[addr]
	}

	q.TxsFromAddress[addr] = make(TxsFromAddressAsc, 0, 16)
	return q.TxsFromAddress[addr]

}

// Start - This method is supposed to be started as a
// seperate go routine which will manage queued pool ops
// through out its life
func (q *QueuedPool) Start(ctx context.Context) {

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
	//
	// @note Don't accept tx which are already dropped
	needToDropTxs := func() bool {
		return uint64(q.AscTxsByGasPrice.len())+1 > config.GetQueuedPoolSize()
	}

	pickTxWithLowestGasPrice := func() *MemPoolTx {
		return q.AscTxsByGasPrice.get()[0]
	}

	// For adding new tx into queued pool, always
	// invoke this closure
	addTx := func(tx *MemPoolTx) {

		q.AscTxsByGasPrice = Insert(q.AscTxsByGasPrice, tx)
		q.DescTxsByGasPrice = Insert(q.DescTxsByGasPrice, tx)
		q.TxsFromAddress[tx.From] = Insert(q.allocateFor(tx.From), tx)
		q.Transactions[tx.Hash] = tx

	}

	// Plain simple tx removing logic. Rather than rewriting
	// same logic in multiple places, consider using this one
	removeTx := func(tx *MemPoolTx) {

		// Remove from sorted tx list, keep it sorted
		q.AscTxsByGasPrice = Remove(q.AscTxsByGasPrice, tx)
		q.DescTxsByGasPrice = Remove(q.DescTxsByGasPrice, tx)
		q.TxsFromAddress[tx.From] = Remove(q.TxsFromAddress[tx.From], tx)
		delete(q.Transactions, tx.Hash)

	}

	// Silently drop some tx, before adding
	// new one, so that we don't exceed limit
	// set up by user
	dropTx := func(tx *MemPoolTx) {

		removeTx(tx)
		// Marking that tx has been dropped, so that
		// it won't get picked up next time
		q.DroppedTxs[tx.Hash] = true

	}

	txAdder := func(tx *MemPoolTx) bool {

		if _, ok := q.Transactions[tx.Hash]; ok {
			return false
		}

		if _, ok := q.DroppedTxs[tx.Hash]; ok {
			return false
		}

		if needToDropTxs() {
			dropTx(pickTxWithLowestGasPrice())

			if len(q.DroppedTxs)%10 == 0 {
				log.Printf("🧹 Dropped 10 queued txs, was about to hit limit\n")
			}
		}

		// Marking we found this tx in mempool now
		tx.QueuedAt = time.Now().UTC()
		tx.Pool = "queued"

		addTx(tx)
		q.PublishAdded(ctx, q.PubSub, tx)

		return true

	}

	txRemover := func(txHash common.Hash) *MemPoolTx {

		tx, ok := q.Transactions[txHash]
		if !ok {
			return nil
		}

		tx.UnstuckAt = time.Now().UTC()

		removeTx(tx)
		q.PublishRemoved(ctx, q.PubSub, q.Transactions[txHash])

		return tx

	}

	for {

		select {

		case <-ctx.Done():
			return
		case req := <-q.AddTxChan:

			req.ResponseChan <- txAdder(req.Tx)

		case req := <-q.RemoveTxChan:

			// if removed will return non-nil reference to removed tx
			req.ResponseChan <- txRemover(req.Hash)

		case req := <-q.TxExistsChan:

			_, ok := q.Transactions[req.Tx]
			req.ResponseChan <- ok

		case req := <-q.GetTxChan:

			if tx, ok := q.Transactions[req.Tx]; ok {
				req.ResponseChan <- tx
				break
			}

			req.ResponseChan <- nil

		case req := <-q.CountTxsChan:

			req.ResponseChan <- uint64(q.AscTxsByGasPrice.len())

		case req := <-q.ListTxsChan:

			if req.Order == ASC {

				// If empty, just return nil
				if q.AscTxsByGasPrice.len() == 0 {
					req.ResponseChan <- nil
					break
				}

				copied := make([]*MemPoolTx, q.AscTxsByGasPrice.len())
				copy(copied, q.AscTxsByGasPrice.get())

				req.ResponseChan <- copied
				break

			}

			if req.Order == DESC {

				// If empty, just return nil
				if q.DescTxsByGasPrice.len() == 0 {
					req.ResponseChan <- nil
					break
				}

				copied := make([]*MemPoolTx, q.DescTxsByGasPrice.len())
				copy(copied, q.DescTxsByGasPrice.get())

				req.ResponseChan <- copied

			}

		case req := <-q.TxsFromAChan:

			if txs, ok := q.TxsFromAddress[req.From]; ok {

				if txs.len() == 0 {
					req.ResponseChan <- nil
					break
				}

				copied := make([]*MemPoolTx, txs.len())
				copy(copied, txs.get())

				req.ResponseChan <- copied
				break

			}

			req.ResponseChan <- nil

		}

	}

}

// Prune - Remove unstuck txs from queued pool & also
// attempt to place them in pending pool, if not present already
//
// @note Start this method as an independent go routine
func (q *QueuedPool) Prune(ctx context.Context, commChan chan ConfirmedTx) {

	// Creating worker pool, where jobs to be submitted
	// for concurrently checking whether tx has been unstuck or not
	// so that it can be moved to pending pool
	wp := workerpool.New(config.GetConcurrencyFactor())
	defer wp.Stop()

	internalChan := make(chan *TxStatus, 4096)
	var unstuck uint64

	for {

		select {

		case <-ctx.Done():
			return

		case mined := <-commChan:
			// As soon as we learn a new tx got mined
			// for which we've received txfrom & respective nonce
			// we'll attempt to find out how many txs from same address
			// currently live in queued pool
			//
			// If any, we'll attempt to go through all of those & see any of them
			// unstuck or not, if yes we're going to attempt to mark it as
			// unstuck
			txs := q.TxsFromA(mined.From)
			if txs == nil {
				break
			}

			for i := 0; i < len(txs); i++ {

				// We learnt last tx mined from `mined.From` was with nonce
				// `mined.Nonce`, we're expecting there might be some tx living
				// in queued pool with nonce +1 than that, which is now eligible to
				// be made unstuck & we're attempting that below
				//
				// We could have just checked `txs[i].Nonce == mined.Nonce + 1`, but
				// just to be sure, if any tx lower nonce still living in this part of pool
				// we're checking with `<=`, which is perfectly ok & doesn't harm
				//
				// @note As far as I understand the mempool behaviour 👆
				if txs[i].Nonce <= (mined.Nonce + 1) {
					internalChan <- &TxStatus{Hash: txs[i].Hash, Status: UNSTUCK}
				}

			}

		case txStat := <-internalChan:

			if txStat.Status == UNSTUCK {

				// Removing unstuck tx
				tx := q.Remove(ctx, txStat.Hash)
				if tx == nil {
					// probably just been removed by some competing worker
					// because it became eligible for that
					continue
				}

				unstuck++

				// Just check whether we need to add this tx into pending
				// pool first, if not required, we're not adding it
				q.PendingPool.VerifiedAdd(ctx, tx)

				if unstuck%10 == 0 {
					log.Printf("[➖] Removed 10 tx(s) from queued tx pool\n")
				}

			}

		}

	}

}

// Get - Given tx hash, attempts to find out tx in queued pool, if any
//
// Returns nil, if found nothing
func (q *QueuedPool) Get(hash common.Hash) *MemPoolTx {

	respChan := make(chan *MemPoolTx)

	q.GetTxChan <- GetRequest{Tx: hash, ResponseChan: respChan}

	return <-respChan

}

// Exists - Checks whether tx of given hash exists on queued pool or not
func (q *QueuedPool) Exists(hash common.Hash) bool {

	respChan := make(chan bool)

	q.TxExistsChan <- ExistsRequest{Tx: hash, ResponseChan: respChan}

	return <-respChan

}

// Count - How many tx(s) currently present in pending pool
func (q *QueuedPool) Count() uint64 {

	respChan := make(chan uint64)

	q.CountTxsChan <- CountRequest{ResponseChan: respChan}

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
func (q *QueuedPool) DuplicateTxs(hash common.Hash) []*MemPoolTx {

	targetTx := q.Get(hash)
	if targetTx == nil {
		return nil
	}

	txs := q.TxsFromA(targetTx.From)
	if txs == nil {
		return nil
	}

	txCount := uint64(len(txs))
	commChan := make(chan *MemPoolTx, txCount)
	result := make([]*MemPoolTx, 0, txCount)

	var wp *workerpool.WorkerPool

	if txCount > uint64(runtime.NumCPU()) {
		wp = workerpool.New(runtime.NumCPU())
	} else {
		wp = workerpool.New(int(txCount))
	}

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

	for v := range commChan {

		if v != nil {
			result = append(result, v)
		}

		received++
		if received >= txCount {
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

// AscListTxs - Returns all tx(s) present in queued pool, as slice, ascending ordered as per gas price paid
func (q *QueuedPool) AscListTxs() []*MemPoolTx {

	respChan := make(chan []*MemPoolTx)

	q.ListTxsChan <- ListRequest{ResponseChan: respChan, Order: ASC}

	return <-respChan

}

// DescListTxs - Returns all tx(s) present in queued pool, as slice, descending ordered as per gas price paid
func (q *QueuedPool) DescListTxs() []*MemPoolTx {

	respChan := make(chan []*MemPoolTx)

	q.ListTxsChan <- ListRequest{ResponseChan: respChan, Order: DESC}

	return <-respChan

}

// TxsFromA - Returns a slice of txs, where all of those are sent
// by address `A`
func (q *QueuedPool) TxsFromA(addr common.Address) []*MemPoolTx {

	respChan := make(chan []*MemPoolTx)

	q.TxsFromAChan <- TxsFromARequest{ResponseChan: respChan, From: addr}

	return <-respChan

}

// TopXWithHighGasPrice - Returns only top `X` tx(s) present in queued mempool,
// where being top is determined by how much gas price paid by tx sender
func (q *QueuedPool) TopXWithHighGasPrice(x uint64) []*MemPoolTx {

	txs := q.DescListTxs()
	if uint64(len(txs)) <= x {
		return txs
	}

	return txs[:x]

}

// TopXWithLowGasPrice - Returns only top `X` tx(s) present in queued mempool,
// where being top is determined by how low gas price paid by tx sender
func (q *QueuedPool) TopXWithLowGasPrice(x uint64) []*MemPoolTx {

	txs := q.AscListTxs()
	if uint64(len(txs)) <= x {
		return txs
	}

	return txs[:x]

}

// SentFrom - Returns a list of queued tx(s) sent from
// specified address
func (q *QueuedPool) SentFrom(address common.Address) []*MemPoolTx {
	return q.TxsFromA(address)
}

// SentTo - Returns a list of queued tx(s) sent to
// specified address
func (q *QueuedPool) SentTo(address common.Address) []*MemPoolTx {

	txs := q.DescListTxs()
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

// OlderThanX - Returns a list of all queued tx(s), which are
// living in mempool for more than or equals to `X` time unit
func (q *QueuedPool) OlderThanX(x time.Duration) []*MemPoolTx {

	txs := q.DescListTxs()
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

				if tx.IsQueuedForGTE(x) {
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

// FresherThanX - Returns a list of all queued tx(s), which are
// living in mempool for less than or equals to `X` time unit
func (q *QueuedPool) FresherThanX(x time.Duration) []*MemPoolTx {

	txs := q.DescListTxs()
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

				if tx.IsQueuedForLTE(x) {
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
func (q *QueuedPool) Add(ctx context.Context, tx *MemPoolTx) bool {

	respChan := make(chan bool)

	q.AddTxChan <- AddRequest{Tx: tx, ResponseChan: respChan}

	return <-respChan

}

// PublishAdded - Publish new tx, entered queued pool, ( in messagepack serialized format )
// to pubsub topic
func (q *QueuedPool) PublishAdded(ctx context.Context, pubsub *redis.Client, msg *MemPoolTx) {

	_msg, err := msg.ToMessagePack()
	if err != nil {

		log.Printf("[❗️] Failed to serialize into messagepack : %s\n", err.Error())
		return

	}

	if err := pubsub.Publish(ctx, config.GetQueuedTxEntryPublishTopic(), _msg).Err(); err != nil {
		log.Printf("[❗️] Failed to publish new queued tx : %s\n", err.Error())
	}

}

// Remove - Removes unstuck tx from queued pool
func (q *QueuedPool) Remove(ctx context.Context, txHash common.Hash) *MemPoolTx {

	respChan := make(chan *MemPoolTx)

	q.RemoveTxChan <- RemovedUnstuckTx{Hash: txHash, ResponseChan: respChan}

	return <-respChan

}

// PublishRemoved - Publish unstuck tx, leaving queued pool ( in messagepack serialized format )
// to pubsub topic
//
// These tx(s) are leaving queued pool i.e. they're ( probably ) going to
// sit in pending pool now, unless they're already mined & harmony
// failed to keep track of it
func (q *QueuedPool) PublishRemoved(ctx context.Context, pubsub *redis.Client, msg *MemPoolTx) {

	_msg, err := msg.ToMessagePack()
	if err != nil {

		log.Printf("[❗️] Failed to serialize into messagepack : %s\n", err.Error())
		return

	}

	if err := pubsub.Publish(ctx, config.GetQueuedTxExitPublishTopic(), _msg).Err(); err != nil {
		log.Printf("[❗️] Failed to publish unstuck tx : %s\n", err.Error())
	}

}

// AddQueued - Update latest queued pool state
func (q *QueuedPool) AddQueued(ctx context.Context, txs map[string]map[string]*MemPoolTx) uint64 {

	var count uint64

	for keyO := range txs {
		for keyI := range txs[keyO] {

			if q.Add(ctx, txs[keyO][keyI]) {
				count++
			}

		}
	}

	return count

}
