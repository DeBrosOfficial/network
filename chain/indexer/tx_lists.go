package indexer

import "fmt"

// The transaction lists are newest first and are read straight off an index per filter, so a page
// costs the page and the rows skipped to reach it, not the transactions that do not match.

func kindTxPrefix(kind string) []byte { return []byte(pfxKindTx + kind + "/") }
func kindTxKey(kind string, height int64, index uint32) []byte {
	return join(kindTxPrefix(kind), be64(uint64(height)), be32(index))
}
func kindAcctPrefix(addr, kind string) []byte { return []byte(pfxKindAcct + addr + "/" + kind + "/") }
func kindAcctKey(addr, kind string, height int64, index uint32) []byte {
	return join(kindAcctPrefix(addr, kind), be64(uint64(height)), be32(index))
}

// addKinds lists the transaction under each filter it matches (besides "all", which the latest
// list and the account index already are).
func (w *writer) addKinds(kinds []string, height int64, index uint32, hash []byte) error {
	for _, kind := range kinds {
		if err := w.set(kindTxKey(kind, height, index), hash); err != nil {
			return err
		}
	}
	return nil
}

// addAccountKinds lists the transaction under each filter it matches in addr's own lists.
func (w *writer) addAccountKinds(addr string, kinds []string, height int64, index uint32, hash []byte) error {
	for _, kind := range kinds {
		if err := w.set(kindAcctKey(addr, kind, height, index), hash); err != nil {
			return err
		}
	}
	return nil
}

// LatestTxs returns the newest transactions, newest first.
func (s *Store) LatestTxs(limit int) ([]Tx, error) { return s.TxsFiltered(FilterAll, 1, limit) }

// TxsFiltered returns one page of the transactions that match filter, newest first. page starts at 1.
func (s *Store) TxsFiltered(filter string, page, limit int) ([]Tx, error) {
	prefix := []byte(pfxLatest)
	if filter != FilterAll {
		prefix = kindTxPrefix(filter)
	}
	return s.txPage(prefix, page, limit, "the transaction list")
}

// AccountTxs returns one page of the transactions whose events name addr, newest first. page starts at 1.
func (s *Store) AccountTxs(addr string, page, limit int) ([]Tx, error) {
	return s.AccountTxsFiltered(addr, FilterAll, page, limit)
}

// AccountTxsFiltered returns one page of the transactions whose events name addr and that match
// filter, newest first.
func (s *Store) AccountTxsFiltered(addr, filter string, page, limit int) ([]Tx, error) {
	prefix := accountPrefix(addr)
	if filter != FilterAll {
		prefix = kindAcctPrefix(addr, filter)
	}
	return s.txPage(prefix, page, limit, addr)
}

// txPage reads one page of the hashes listed under prefix, newest first, and the transactions they
// name, from one snapshot so a block committed between the two cannot tear the page.
func (s *Store) txPage(prefix []byte, page, limit int, what string) ([]Tx, error) {
	snap := s.db.NewSnapshot()
	defer snap.Close()
	return txPageFrom(snap, prefix, page, limit, what)
}

// txPageFrom is txPage over a snapshot the caller holds.
func txPageFrom(snap iterable, prefix []byte, page, limit int, what string) ([]Tx, error) {
	rows, err := scan(snap, prefix, true, page, limit)
	if err != nil {
		return nil, err
	}
	out := make([]Tx, 0, len(rows))
	for _, row := range rows {
		var t Tx
		ok, err := getJSON(snap, txKey(row.value), &t)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("index of %s names transaction %x but the index does not hold it", what, row.value)
		}
		out = append(out, t)
	}
	return out, nil
}
