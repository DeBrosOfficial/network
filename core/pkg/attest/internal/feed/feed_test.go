package feed

import (
	"net/http"
	"testing"
	"time"
)

func TestFreshness_isMaxAgeLessAge(t *testing.T) {
	for _, c := range []struct {
		cacheControl, age string
		want              time.Duration
	}{
		{"public, max-age=86400", "", 86400 * time.Second},
		{"public, max-age=86400", "70776", 15624 * time.Second},
		{"Max-Age=60", "", time.Minute},
		{"no-cache", "", 0},
		{"", "", 0},
		{"max-age=abc", "", 0},
		{"max-age=60", "120", -time.Minute},
	} {
		h := http.Header{}
		h.Set("Cache-Control", c.cacheControl)
		if c.age != "" {
			h.Set("Age", c.age)
		}
		if got := freshness(h); got != c.want {
			t.Errorf("Cache-Control %q Age %q: %s, want %s", c.cacheControl, c.age, got, c.want)
		}
	}
}

func TestClamp_keepsRefreshesWithinBounds(t *testing.T) {
	for in, want := range map[time.Duration]time.Duration{
		-time.Minute:     MinRefresh,
		30 * time.Minute: MinRefresh,
		5 * time.Hour:    5 * time.Hour,
		72 * time.Hour:   MaxRefresh,
	} {
		if got := clamp(in); got != want {
			t.Errorf("clamp(%s) = %s, want %s", in, got, want)
		}
	}
}
