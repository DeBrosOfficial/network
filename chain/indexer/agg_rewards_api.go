package indexer

import "net/http"

func (a *API) serveEpochRewards(w http.ResponseWriter, r *http.Request, raw string) {
	page, limit, ok := pageQuery(w, r)
	if !ok {
		return
	}
	epoch, ok := parseEpoch(w, raw)
	if !ok {
		return
	}
	rows, recorded, found, err := a.store.EpochRewards(epoch, page, limit)
	answer(w, map[string]any{"epoch": epoch, "recorded": recorded, "page": page, "limit": limit, "rewards": rows}, found, err)
}

func (a *API) serveAccountRewards(w http.ResponseWriter, r *http.Request, addr string) {
	page, limit, ok := pageQuery(w, r)
	if !ok {
		return
	}
	if !IsAccountAddress(addr) {
		writeError(w, http.StatusBadRequest, "address must be a lowercase orama account address")
		return
	}
	from, rows, err := a.store.AccountRewards(addr, page, limit)
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, map[string]any{"address": addr, "recorded_from_epoch": from, "page": page, "limit": limit, "rewards": rows})
}
