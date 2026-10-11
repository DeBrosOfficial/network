//go:build e2e_fleet

package chainexplorer

import (
	"net/http"
	"net/url"
	"strconv"
	"testing"
)

// maxListPage is the deepest numbered page of the lists that grow with the chain
// (chain/indexer/cursor.go MaxListPage, core/pkg/gateway/handlers/chainread indexMaxListPage).
const maxListPage = 10

type blockPage struct {
	Blocks []struct {
		Height int64 `json:"height"`
	} `json:"blocks"`
	NextCursor string `json:"next_cursor"`
}

// TestExplorerLists_aDeepPageIsReadByCursor: the block list's full page carries next_cursor, and
// the same route with cursor=<next_cursor> answers the blocks older than the page, with no page
// field. A page past maxListPage, a page with a cursor and a cursor that is not one are refused
// (website/src/docs/blockchain/reading-the-chain.mdx#the-chain-indexer).
func TestExplorerLists_aDeepPageIsReadByCursor(t *testing.T) {
	t.Parallel()
	requireIndexer(t)

	var first blockPage
	decode(t, read(t, "/index/blocks", url.Values{"limit": {"2"}}), &first)
	if len(first.Blocks) != 2 || first.NextCursor == "" {
		t.Fatalf("a full first page of 2 blocks with a next_cursor, got %+v", first)
	}
	var next blockPage
	decode(t, read(t, "/index/blocks", url.Values{"limit": {"2"}, "cursor": {first.NextCursor}}), &next)
	if len(next.Blocks) == 0 || next.Blocks[0].Height >= first.Blocks[1].Height {
		t.Fatalf("the cursor page must hold blocks older than %d, got %+v", first.Blocks[1].Height, next)
	}

	for _, q := range []url.Values{
		{"page": {strconv.Itoa(maxListPage + 1)}},
		{"page": {"2"}, "cursor": {first.NextCursor}},
		{"cursor": {"not-a-cursor"}},
	} {
		if resp := read(t, "/index/txs", q); resp.Status != http.StatusBadRequest {
			t.Errorf("txs %v: HTTP %d, want 400", q, resp.Status)
		}
	}
	if resp := read(t, "/index/txs", url.Values{"page": {strconv.Itoa(maxListPage)}}); resp.Status != http.StatusOK {
		t.Errorf("txs page %d: HTTP %d, want 200", maxListPage, resp.Status)
	}
}
