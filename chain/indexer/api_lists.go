package indexer

import (
	"errors"
	"log/slog"
	"net/http"
	"net/url"
)

// listParams are the parameters of a paged list: page and limit, and, for the transaction lists, a
// filter.
type listParams struct {
	page, limit int
	filter      string
}

// listQuery accepts page, limit and, if filtered, filter, each at most once. page is 1 to MaxPage,
// limit 1 to MaxLimit, filter one of the list filters.
func listQuery(w http.ResponseWriter, r *http.Request, filtered bool) (listParams, bool) {
	q, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad query")
		return listParams{}, false
	}
	p := listParams{page: defaultPage, limit: DefaultLimit, filter: FilterAll}
	for k, vs := range q {
		if len(vs) != 1 {
			writeError(w, http.StatusBadRequest, "repeated query parameter "+k)
			return listParams{}, false
		}
		if !p.set(k, vs[0], filtered) {
			writeError(w, http.StatusBadRequest, "bad query parameter "+k)
			return listParams{}, false
		}
	}
	return p, true
}

// set takes one parameter; it reports false for a name this list does not take or a value out of range.
func (p *listParams) set(key, value string, filtered bool) bool {
	switch {
	case key == "filter" && filtered:
		p.filter = value
		return IsListFilter(value)
	case key == "page" || key == "limit":
		n, ok := boundedInt(value, pageBounds[key])
		if key == "page" {
			p.page = n
		} else {
			p.limit = n
		}
		return ok
	}
	return false
}

// serveLatestTxs answers a page of the transactions, newest first, all of them or those that match
// a filter.
func (a *API) serveLatestTxs(w http.ResponseWriter, r *http.Request) {
	p, ok := listQuery(w, r, true)
	if !ok {
		return
	}
	txs, err := a.store.TxsFiltered(p.filter, p.page, p.limit)
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, map[string]any{"page": p.page, "limit": p.limit, "filter": p.filter, "txs": txs})
}

func (a *API) serveAccountTxs(w http.ResponseWriter, r *http.Request, addr string) {
	p, ok := listQuery(w, r, true)
	if !ok {
		return
	}
	if !IsAccountAddress(addr) {
		writeError(w, http.StatusBadRequest, "address must be a lowercase orama account address")
		return
	}
	txs, err := a.store.AccountTxsFiltered(addr, p.filter, p.page, p.limit)
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, map[string]any{"page": p.page, "limit": p.limit, "filter": p.filter, "txs": txs})
}

// serveBlocks answers a page of the blocks, newest first, without their transaction hashes.
func (a *API) serveBlocks(w http.ResponseWriter, r *http.Request) {
	p, ok := listQuery(w, r, false)
	if !ok {
		return
	}
	blocks, err := a.store.Blocks(p.page, p.limit)
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, map[string]any{"page": p.page, "limit": p.limit, "blocks": blocks})
}

// serveHome answers the snapshot the home page shows, built at most once per block.
func (a *API) serveHome(w http.ResponseWriter, r *http.Request) {
	if !noQuery(w, r) {
		return
	}
	st, err := a.store.Status()
	if err != nil {
		internalError(w, err)
		return
	}
	body, err := a.home.get(r.Context(), st.Cursor, a.now, a.buildHome)
	var chainErr chainReadError
	switch {
	case errors.As(err, &chainErr):
		slog.Error("home snapshot: chain unreachable", "err", err)
		writeError(w, http.StatusBadGateway, "chain unreachable")
		return
	case err != nil:
		internalError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if _, err := w.Write(body); err != nil {
		slog.Error("failed to write the home snapshot", "err", err)
	}
}
