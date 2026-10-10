package indexer

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// homeAPI indexes seven blocks of the economy chain, then adds `ahead` more the index has not seen.
func homeAPI(t *testing.T, ahead int) (*API, *fakeChain, *Store) {
	t.Helper()
	chain, _ := econChain(t)
	addEconBlocks(t, chain, 1, 7)
	store, _ := follow(t, chain, t.TempDir(), 1)
	t.Cleanup(func() { _ = store.Close() })
	addEconBlocks(t, chain, 8, int64(7+ahead))
	return NewAPI(store, chain), chain, store
}

func askedFor(chain *fakeChain, method string) int {
	n := 0
	for _, key := range chain.asked {
		if strings.HasPrefix(key, method+"@") {
			n++
		}
	}
	return n
}

func TestHome_oneAnswerHoldsWhatThePageShows(t *testing.T) {
	api, chain, _ := homeAPI(t, 2)

	code, body := get(t, api, http.MethodGet, "/index/v1/home")
	require.Equal(t, http.StatusOK, code)

	status := body["status"].(map[string]any)
	require.Equal(t, 7.0, status["cursor"])
	require.Equal(t, 9.0, status["tip"])
	require.Equal(t, 2.0, status["lag"], "the tip is two blocks ahead of the index")
	require.Equal(t, 1.0, status["aggregates_from"])

	blocks := body["blocks"].([]any)
	require.Len(t, blocks, 7)
	require.Equal(t, 7.0, blocks[0].(map[string]any)["height"], "newest first, from the cursor")
	require.Len(t, body["hours"], HourlyWindow)
	require.Equal(t, 3.0, body["epoch"].(map[string]any)["number"], "two epochs closed, the third is in progress")
	require.Equal(t, float64(epochBlocks), body["epoch"].(map[string]any)["duration_seconds"])
	require.Equal(t, "7", body["base_fee"])
	require.Equal(t, 9.0, body["supply_height"])
	require.Equal(t, float64(chain.ledger.supply(9)), parseNumber(t, body["supply"]))
	require.Contains(t, body, "examples")
}

func parseNumber(t *testing.T, v any) float64 {
	t.Helper()
	s, ok := v.(string)
	require.True(t, ok)
	var n float64
	for _, c := range s {
		require.True(t, c >= '0' && c <= '9', s)
		n = n*10 + float64(c-'0')
	}
	return n
}

func TestHome_exampleTargetsAreFromTheNewestTransactions(t *testing.T) {
	txs := []Tx{{Hash: "h3", Signer: "b"}, {Hash: "h2", Signer: "a"}, {Hash: "h1", Signer: "b"}, {Hash: "h0"}}
	ex := examplesOf(txs)
	require.Equal(t, "h3", ex.LatestTx)
	require.Equal(t, "b", ex.BusyAccount, "b signed twice, a once; a transaction with no signer counts for nobody")

	tie := examplesOf([]Tx{{Hash: "x", Signer: "b"}, {Hash: "y", Signer: "a"}})
	require.Equal(t, "a", tie.BusyAccount, "a tie goes to the first address in order, so the answer does not flicker")

	require.Equal(t, HomeExamples{}, examplesOf(nil), "nothing is invented for an empty index")
	require.Empty(t, examplesOf([]Tx{{Hash: "unsigned"}}).BusyAccount)
}

func TestHome_isBuiltOncePerBlockNotOncePerReader(t *testing.T) {
	api, chain, _ := homeAPI(t, 0)
	clock := time.Unix(1_800_000_000, 0)
	api.now = func() time.Time { return clock }
	before := askedFor(chain, querySupplyOf)

	for i := 0; i < 20; i++ {
		code, _ := get(t, api, http.MethodGet, "/index/v1/home")
		require.Equal(t, http.StatusOK, code)
	}
	require.Equal(t, before+1, askedFor(chain, querySupplyOf), "twenty readers, one set of chain reads")
	require.Equal(t, 1, askedFor(chain, queryBaseFee))

	clock = clock.Add((HeadMaxAge + 1) * time.Second)
	get(t, api, http.MethodGet, "/index/v1/home")
	require.Equal(t, before+2, askedFor(chain, querySupplyOf), "a block's time later, the snapshot is built again")
}

func TestHome_aNewBlockInTheIndexIsBuiltAtOnce(t *testing.T) {
	api, chain, store := homeAPI(t, 3)
	clock := time.Unix(1_800_000_000, 0)
	api.now = func() time.Time { return clock }
	_, first := get(t, api, http.MethodGet, "/index/v1/home")
	require.Equal(t, 7.0, first["status"].(map[string]any)["cursor"])

	f, err := NewFollower(chain, store, 1)
	require.NoError(t, err)
	_, err = f.Step(context.Background())
	require.NoError(t, err)

	_, second := get(t, api, http.MethodGet, "/index/v1/home")
	require.Equal(t, 10.0, second["status"].(map[string]any)["cursor"], "the cursor moved, so the cached snapshot is not served")
	require.Equal(t, 0.0, second["status"].(map[string]any)["lag"])
}

func TestHome_concurrentReadersShareOneBuild(t *testing.T) {
	api, chain, _ := homeAPI(t, 0)
	before := askedFor(chain, querySupplyOf)
	var wg sync.WaitGroup
	codes := make([]int, 16)
	for i := range codes {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rec := httptest.NewRecorder()
			api.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/index/v1/home", nil))
			codes[i] = rec.Code
		}()
	}
	wg.Wait()
	for _, c := range codes {
		require.Equal(t, http.StatusOK, c)
	}
	require.Equal(t, before+1, askedFor(chain, querySupplyOf))
}

func TestHome_aChainThatCannotBeReadIsAnErrorNotAGuess(t *testing.T) {
	api, chain, _ := homeAPI(t, 0)
	chain.failing = map[string]error{queryKey(querySupplyOf, 7): errors.New("connection refused")}

	code, body := get(t, api, http.MethodGet, "/index/v1/home")
	require.Equal(t, http.StatusBadGateway, code)
	require.Equal(t, "chain unreachable", body["error"])

	delete(chain.failing, queryKey(querySupplyOf, 7))
	code, _ = get(t, api, http.MethodGet, "/index/v1/home")
	require.Equal(t, http.StatusOK, code, "a failure is not cached")
}

func TestHome_anUnreachableTipIsABadGateway(t *testing.T) {
	store := openStore(t, t.TempDir())
	defer store.Close()
	api := NewAPI(store, fixedTip{err: errors.New("connection refused")})

	code, _ := get(t, api, http.MethodGet, "/index/v1/home")
	require.Equal(t, http.StatusBadGateway, code)
}

func TestHome_anEmptyIndexIsAnEmptyHomeNotAnError(t *testing.T) {
	chain := newFakeChain()
	chain.add()
	store := openStore(t, t.TempDir())
	defer store.Close()
	api := NewAPI(store, chain)

	code, body := get(t, api, http.MethodGet, "/index/v1/home")
	require.Equal(t, http.StatusOK, code)
	require.Equal(t, 0.0, body["status"].(map[string]any)["cursor"])
	require.Empty(t, body["blocks"])
	require.NotNil(t, body["blocks"])
	require.NotNil(t, body["txs"])
	require.NotContains(t, body, "epoch", "no epoch before the aggregates begin")
	require.Equal(t, "", body["examples"].(map[string]any)["latest_tx"])
}

func TestHome_takesNoQueryAndCachesForABlock(t *testing.T) {
	api, _, _ := homeAPI(t, 0)
	code, _ := get(t, api, http.MethodGet, "/index/v1/home?limit=1")
	require.Equal(t, http.StatusBadRequest, code)

	rec := httptest.NewRecorder()
	api.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/index/v1/home", nil))
	require.Equal(t, "public, max-age=2", rec.Header().Get("Cache-Control"))
}

func TestHomeCache_aReaderThatGivesUpDoesNotCancelTheBuild(t *testing.T) {
	var c homeCache
	release := make(chan struct{})
	built := make(chan struct{})
	build := func(context.Context) ([]byte, error) {
		<-release
		close(built)
		return []byte("snapshot"), nil
	}
	now := func() time.Time { return time.Unix(1, 0) }

	leader, cancelLeader := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := c.get(leader, 1, now, build)
		done <- err
	}()
	for {
		c.mu.Lock()
		started := c.pending != nil
		c.mu.Unlock()
		if started {
			break
		}
		time.Sleep(time.Millisecond)
	}
	waiter, cancelWaiter := context.WithCancel(context.Background())
	cancelWaiter()
	_, err := c.get(waiter, 1, now, build)
	require.ErrorIs(t, err, context.Canceled, "a reader that is gone stops waiting")

	cancelLeader()
	close(release)
	require.NoError(t, <-done, "the build runs on after the request that began it is cancelled")
	<-built
	body, err := c.get(context.Background(), 1, now, build)
	require.NoError(t, err)
	require.Equal(t, "snapshot", string(body))
}
