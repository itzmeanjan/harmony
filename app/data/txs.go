package data

// Insert - Insert into array of sorted ( in terms of gas price paid )
// mempool txs & keep it sorted
//
// If more memory allocation is required for inserting new element, it'll
// be done & new slice to be returned
func Insert(txs MemPoolTxsDesc, tx *MemPoolTx) MemPoolTxsDesc {

	n := len(txs)
	idx := findInsertionPoint(txs, 0, n-1, tx)

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

// findInsertionPoint - Find index at which newly arrived tx should be entered to
// keep this slice sorted
func findInsertionPoint(txs MemPoolTxsDesc, low int, high int, tx *MemPoolTx) int {

	if low > high {
		return 0
	}

	if low == high {

		if BigHexToBigDecimal(txs[low].GasPrice).Cmp(BigHexToBigDecimal(tx.GasPrice)) > 0 {
			return low
		}

		return low + 1

	}

	mid := (low + high) / 2
	if BigHexToBigDecimal(txs[mid].GasPrice).Cmp(BigHexToBigDecimal(tx.GasPrice)) > 0 {

		return findInsertionPoint(txs, low, mid, tx)

	}

	return findInsertionPoint(txs, mid+1, high, tx)

}

// findTx - Find index of tx, which is already present in this slice
func findTx(txs MemPoolTxsDesc, low int, high int, tx *MemPoolTx) int {

	if low > high {
		return -1
	}

	if low == high {

		if txs[low].Hash == tx.Hash {
			return low
		}

		return -1

	}

	mid := (low + high) / 2
	if BigHexToBigDecimal(txs[mid].GasPrice).Cmp(BigHexToBigDecimal(tx.GasPrice)) >= 0 {
		return findTx(txs, low, mid, tx)
	}

	return findTx(txs, mid+1, high, tx)

}
