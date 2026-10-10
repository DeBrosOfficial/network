package gateway

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DeBrosOfficial/network/pkg/logging"
)

// The explorer's reads of the chain indexer (/v1/chain/index/) draw on the general per-client bucket
// like every other route: they are not exempt, and a client that exhausts the bucket is limited on
// them with a 429.
func TestRateLimitMiddleware_theIndexRoutesDrawOnTheGeneralBucket(t *testing.T) {
	logger, _ := logging.NewColoredLogger(logging.ComponentGateway, false)
	g := &Gateway{logger: logger, rateLimiter: NewRateLimiter(60, 4)}
	served := 0
	handler := g.rateLimitMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) { served++ }))

	var lastCode int
	for _, path := range []string{
		"/v1/chain/index/home", "/v1/chain/index/txs?filter=failed", "/v1/chain/index/blocks", "/v1/chain/index/epochs/3/rewards",
		"/v1/chain/index/accounts/orama1x/rewards", "/v1/chain/index/status",
	} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, request("127.0.0.1:9999", "10.0.0.1, 198.51.100.7", path))
		lastCode = w.Code
	}
	if served != 4 || lastCode != http.StatusTooManyRequests {
		t.Errorf("served %d of 6 index reads with last code %d; want the burst of 4 and a 429", served, lastCode)
	}

	served = 0
	handler.ServeHTTP(httptest.NewRecorder(), request("127.0.0.1:9999", "203.0.113.9", "/v1/chain/index/home"))
	if served != 1 {
		t.Error("another client network has its own bucket")
	}
}

func TestRateLimitMiddleware_headStreamsHaveTheirOwnBucket(t *testing.T) {
	logger, _ := logging.NewColoredLogger(logging.ComponentGateway, false)
	g := &Gateway{
		logger:               logger,
		rateLimiter:          NewRateLimiter(100000, 100000),
		chainHeadRateLimiter: NewRateLimiter(60, 3),
	}
	served := 0
	handler := g.rateLimitMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) { served++ }))

	var lastCode int
	for i := 0; i < 10; i++ {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, request("127.0.0.1:9999", "10.0.0.1, 198.51.100.7", "/v1/chain/head/stream"))
		lastCode = w.Code
	}
	if served != 3 || lastCode != http.StatusTooManyRequests {
		t.Errorf("served %d of 10 head streams with last code %d; want the burst of 3 and a 429", served, lastCode)
	}

	served = 0
	for i := 0; i < 10; i++ {
		handler.ServeHTTP(httptest.NewRecorder(), request("127.0.0.1:9999", "198.51.100.7", "/v1/chain/index/home"))
	}
	if served != 10 {
		t.Errorf("%d of 10 index reads were served; the stream bucket must not apply to them", served)
	}
}

func TestChainHeadRateLimiter_isMuchTighterThanTheGeneralOne(t *testing.T) {
	logger, _ := logging.NewColoredLogger(logging.ComponentGateway, false)
	g := &Gateway{logger: logger}
	configureRateLimiters(g)
	if g.chainHeadRateLimiter == nil {
		t.Fatal("the head stream limiter is missing")
	}
	if g.chainHeadRateLimiter.rate >= g.rateLimiter.rate/10 || g.chainHeadRateLimiter.burst >= g.rateLimiter.burst/10 {
		t.Errorf("the head stream bucket (%.2f/s, burst %d) is not far tighter than the general one (%.2f/s, burst %d)",
			g.chainHeadRateLimiter.rate, g.chainHeadRateLimiter.burst, g.rateLimiter.rate, g.rateLimiter.burst)
	}
	// A page held open all day reconnects about once every hundred seconds; its bucket must not run dry.
	if perMinute := float64(60) / 100; g.chainHeadRateLimiter.rate*60 < perMinute {
		t.Errorf("the head stream bucket refills at %.3f per minute, below the %.2f a page open all day uses", g.chainHeadRateLimiter.rate*60, perMinute)
	}
}
