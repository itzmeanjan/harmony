package data

type TxList interface {
	len() int
	cap() int
	get() []*MemPoolTx

	findInsertionPoint(int, int, *MemPoolTx) int
	findTx(int, int, *MemPoolTx) int
}

// Insert - Insert tx into slice of sorted mempool txs, while keeping it sorted
//
// If more memory allocation is required for inserting new element, it'll
// be done & new slice to be returned
func Insert(txs TxList, tx *MemPoolTx) TxList {

	n := txs.len()
	idx := txs.findInsertionPoint(0, n-1, tx)

	if n+1 <= txs.cap() {

		_txs := txs.get()[:n+1]

		copy(_txs[idx+1:], txs.get()[idx:])
		copy(_txs[idx:], []*MemPoolTx{tx})

		switch txs.(type) {

		case MemPoolTxsAsc:
			return (MemPoolTxsAsc)(_txs)
		case MemPoolTxsDesc:
			return (MemPoolTxsDesc)(_txs)
		case TxsFromAddressAsc:
			return (TxsFromAddressAsc)(_txs)
		default:
			return nil

		}

	}

	_txs := make([]*MemPoolTx, n+1)

	copy(_txs, txs.get()[:idx])
	copy(_txs[idx:], []*MemPoolTx{tx})
	copy(_txs[idx+1:], txs.get()[idx:])

	// Previous array now only contains `nil`
	for i := 0; i < txs.len(); i++ {
		txs.get()[i] = nil
	}

	switch txs.(type) {

	case MemPoolTxsAsc:
		return (MemPoolTxsAsc)(_txs)
	case MemPoolTxsDesc:
		return (MemPoolTxsDesc)(_txs)
	case TxsFromAddressAsc:
		return (TxsFromAddressAsc)(_txs)
	default:
		return nil

	}

}

// Remove - Removes existing entry from sorted slice of txs
func Remove(txs TxList, tx *MemPoolTx) TxList {

	n := txs.len()
	idx := txs.findTx(0, n-1, tx)
	if idx == -1 {
		// denotes nothing to delete
		return txs
	}

	copy(txs.get()[idx:], txs.get()[idx+1:])
	txs.get()[n-1] = nil
	_txs := txs.get()[:n-1]

	switch txs.(type) {

	case MemPoolTxsAsc:
		return (MemPoolTxsAsc)(_txs)
	case MemPoolTxsDesc:
		return (MemPoolTxsDesc)(_txs)
	case TxsFromAddressAsc:
		return (TxsFromAddressAsc)(_txs)
	default:
		return nil

	}

}

// findTxFromSlice - Given a slice of txs, attempt to linearly find
// out tx for which we've txHash given
func findTxFromSlice(txs []*MemPoolTx, tx *MemPoolTx) int {

	idx := -1

	// Don't copy tx elements from slice, rather access them by
	// pointer ( directly from index )
	for i := 0; i < len(txs); i++ {

		if txs[i].Hash == tx.Hash {
			idx = i
			break
		}

	}

	return idx

}

// CleanSlice - When we're done using one slice of txs, it's better
// to clean those up, so that it becomes eligible for GC
func CleanSlice(txs []*MemPoolTx) {

	for i := 0; i < len(txs); i++ {
		txs[i] = nil
	}

}
