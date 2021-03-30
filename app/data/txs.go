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
		default:
			return nil

		}

	}

	_txs := make([]*MemPoolTx, 0, n+1)

	copy(_txs, txs.get()[:idx])
	copy(_txs[idx:], []*MemPoolTx{tx})
	copy(_txs[idx+1:], txs.get()[idx:])

	switch txs.(type) {

	case MemPoolTxsAsc:
		return (MemPoolTxsAsc)(_txs)
	case MemPoolTxsDesc:
		return (MemPoolTxsDesc)(_txs)
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

	switch txs.(type) {

	case MemPoolTxsAsc:
		return (MemPoolTxsAsc)(txs.get()[:n-1])
	case MemPoolTxsDesc:
		return (MemPoolTxsDesc)(txs.get()[:n-1])
	default:
		return nil

	}

}

// findTxFromSlice - Given a slice of txs, attempt to linearly find
// out tx for which we've txHash given
func findTxFromSlice(txs []*MemPoolTx, tx *MemPoolTx) int {

	idx := -1

	for i, v := range txs {
		if v.Hash == tx.Hash {
			idx = i
			break
		}
	}

	return idx

}
