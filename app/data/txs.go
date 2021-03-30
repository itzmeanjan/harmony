package data

type TxList interface {
	len() int
	cap() int

	findInsertionPoint(int, int, *MemPoolTx) int
	findTx(int, int, *MemPoolTx) int
}

// Insert - Insert into array of sorted ( in terms of gas price paid )
// mempool txs & keep it sorted
//
// If more memory allocation is required for inserting new element, it'll
// be done & new slice to be returned
func Insert(txs TxList, tx *MemPoolTx) MemPoolTxsDesc {

	n := txs.len()
	idx := txs.findInsertionPoint(0, n-1, tx)

	if n+1 <= cap(txs) {

		_txs := txs[:n+1]

		copy(_txs[idx+1:], txs[idx:])
		copy(_txs[idx:], []*MemPoolTx{tx})

		return _txs

	}

	_txs := make([]*MemPoolTx, 0, n+1)

	copy(_txs, txs[:idx])
	copy(_txs[idx:], []*MemPoolTx{tx})
	copy(_txs[idx+1:], txs[idx:])

	return _txs

}

// Remove - Removes existing entry from sorted ( in terms of gas price paid ) slice of txs
func Remove(txs MemPoolTxsDesc, tx *MemPoolTx) MemPoolTxsDesc {

	n := len(txs)
	idx := findTx(txs, 0, n-1, tx)
	if idx == -1 {
		// denotes nothing to delete
		return txs
	}

	copy(txs[idx:], txs[idx+1:])
	txs[n-1] = nil
	txs = txs[:n-1]

	return txs

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
