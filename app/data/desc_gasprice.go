package data

// MemPoolTxsDesc - List of mempool tx(s)
//
// @note This structure to be used for sorting tx(s)
// in descending way, using gas price they're paying
type MemPoolTxsDesc []*MemPoolTx

// len - Number of txs present in slice
func (m MemPoolTxsDesc) len() int {
	return len(m)
}

// findInsertionPoint - Find index at which newly arrived tx should be entered to
// keep this slice sorted
func (m MemPoolTxsDesc) findInsertionPoint(low int, high int, tx *MemPoolTx) int {

	if low > high {
		return 0
	}

	if low == high {

		if !(BigHexToBigDecimal(m[low].GasPrice).Cmp(BigHexToBigDecimal(tx.GasPrice)) > 0) {
			return low
		}

		return low + 1

	}

	mid := (low + high) / 2
	if !(BigHexToBigDecimal(m[mid].GasPrice).Cmp(BigHexToBigDecimal(tx.GasPrice)) > 0) {

		return m.findInsertionPoint(low, mid, tx)

	}

	return m.findInsertionPoint(mid+1, high, tx)

}

// findTx - Find index of tx, which is already present in this sorted slice
func (m MemPoolTxsDesc) findTx(low int, high int, tx *MemPoolTx) int {

	if low > high {
		return -1
	}

	if low == high {

		idx := findTxFromSlice(m[low:], tx)
		if idx == -1 {
			return -1
		}

		return low + idx

	}

	mid := (low + high) / 2
	if !(BigHexToBigDecimal(m[mid].GasPrice).Cmp(BigHexToBigDecimal(tx.GasPrice)) > 0) {
		return m.findTx(low, mid, tx)
	}

	return m.findTx(mid+1, high, tx)

}
