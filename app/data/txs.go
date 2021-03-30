package data

// MemPoolTxsDesc - List of mempool tx(s)
//
// @note This structure to be used for sorting tx(s)
// in descending way, using gas price they're paying
type MemPoolTxsDesc []*MemPoolTx

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

// findInsertionPoint - Find index at which newly arrived tx should be entered to
// keep this slice sorted
func findInsertionPoint(txs MemPoolTxsDesc, low int, high int, tx *MemPoolTx) int {

	if low > high {
		return 0
	}

	if low == high {

		if !(BigHexToBigDecimal(txs[low].GasPrice).Cmp(BigHexToBigDecimal(tx.GasPrice)) > 0) {
			return low
		}

		return low + 1

	}

	mid := (low + high) / 2
	if !(BigHexToBigDecimal(txs[mid].GasPrice).Cmp(BigHexToBigDecimal(tx.GasPrice)) > 0) {

		return findInsertionPoint(txs, low, mid, tx)

	}

	return findInsertionPoint(txs, mid+1, high, tx)

}

// findTxFromSlice - Given a slice where txs are ordered by gas price
// paid in descending order & one target tx, whose hash we already know
// we'll iteratively attempt to find out what is index of exactly that tx
//
// Only doing a binary search doesn't help, because there could be multiple
// tx(s) with same gas price & we need to find out exactly that specific entry
// matching tx hash
//
// Please note, `txs` slice is nothing but a view of original slice holding
// all ordered txs, where this subslice is starting from specific index which
// is starting point of tx(s) with this gas price paid by `tx`
func findTxFromSlice(txs MemPoolTxsDesc, tx *MemPoolTx) int {

	idx := -1

	for i, v := range txs {
		if v.Hash == tx.Hash {
			idx = i
			break
		}
	}

	return idx

}

// findTx - Find index of tx, which is already present in this slice
func findTx(txs MemPoolTxsDesc, low int, high int, tx *MemPoolTx) int {

	if low > high {
		return -1
	}

	if low == high {

		idx := findTxFromSlice(txs[low:], tx)
		if idx == -1 {
			return -1
		}

		return low + idx

	}

	mid := (low + high) / 2
	if !(BigHexToBigDecimal(txs[mid].GasPrice).Cmp(BigHexToBigDecimal(tx.GasPrice)) > 0) {
		return findTx(txs, low, mid, tx)
	}

	return findTx(txs, mid+1, high, tx)

}
