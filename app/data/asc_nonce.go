package data

// TxsFromAddressAsc - List of txs, sent from same address
// sorted by their nonce
type TxsFromAddressAsc []*MemPoolTx

// len - Number of tx(s) living in pool, from this address
func (t TxsFromAddressAsc) len() int {
	return len(t)
}

// cap - How many txs can be kept in slice, without further allocation
func (t TxsFromAddressAsc) cap() int {
	return cap(t)
}

// get - Return all txs living in pool, sent from specific address
func (t TxsFromAddressAsc) get() []*MemPoolTx {
	return t
}

// findInsertionPoint - When attempting to insert new tx into this slice,
// find index where to insert, so that it stays sorted ( ascending ), as per
// nonce field of tx
func (t TxsFromAddressAsc) findInsertionPoint(low int, high int, tx *MemPoolTx) int {

	if low > high {
		return 0
	}

	if low == high {

		if t[low].Nonce > tx.Nonce {
			return low
		}

		return low + 1

	}

	mid := (low + high) / 2
	if t[mid].Nonce > tx.Nonce {

		return t.findInsertionPoint(low, mid, tx)

	}

	return t.findInsertionPoint(mid+1, high, tx)

}

// findTx - Find index of tx, which is already present in this sorted slice
// of txs, sent from some specific address
func (t TxsFromAddressAsc) findTx(low int, high int, tx *MemPoolTx) int {

	if low > high {
		return -1
	}

	if low == high {

		if t[low].Hash == tx.Hash {
			return low
		}

		return -1

	}

	mid := (low + high) / 2
	if t[mid].Nonce >= tx.Nonce {
		return t.findTx(low, mid, tx)
	}

	return t.findTx(mid+1, high, tx)

}
