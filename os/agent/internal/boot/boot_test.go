package boot

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func unlock(h http.Handler, method, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(method, "/v1/agent/unlock", strings.NewReader(body)))
	return w
}

func keyBody(key []byte) string {
	b, _ := json.Marshal(map[string]string{"key": base64.StdEncoding.EncodeToString(key)})
	return string(b)
}

func TestGenesisUnlock_a32ByteKeyIsHandedOver(t *testing.T) {
	keyCh := make(chan []byte, 1)
	key := bytes.Repeat([]byte{0xA5}, genesisUnlockKeySize)
	w := unlock(genesisUnlockHandler(keyCh), http.MethodPost, keyBody(key))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"unlocking"`) {
		t.Fatalf("HTTP %d %s, want 200 unlocking", w.Code, w.Body)
	}
	select {
	case got := <-keyCh:
		if !bytes.Equal(got, key) {
			t.Error("the key handed over is not the key sent")
		}
	default:
		t.Fatal("an accepted key was not handed over")
	}
}

func TestGenesisUnlock_refusesAnythingElse(t *testing.T) {
	for name, tc := range map[string]struct {
		method, body string
		status       int
	}{
		"GET":           {http.MethodGet, "", http.StatusMethodNotAllowed},
		"not JSON":      {http.MethodPost, "key=abc", http.StatusBadRequest},
		"not base64":    {http.MethodPost, `{"key":"***"}`, http.StatusBadRequest},
		"short key":     {http.MethodPost, keyBody(make([]byte, genesisUnlockKeySize-1)), http.StatusBadRequest},
		"long key":      {http.MethodPost, keyBody(make([]byte, genesisUnlockKeySize+1)), http.StatusBadRequest},
		"no key at all": {http.MethodPost, `{}`, http.StatusBadRequest},
	} {
		keyCh := make(chan []byte, 1)
		if w := unlock(genesisUnlockHandler(keyCh), tc.method, tc.body); w.Code != tc.status {
			t.Errorf("%s: HTTP %d, want %d", name, w.Code, tc.status)
		}
		if len(keyCh) != 0 {
			t.Errorf("%s: a refused request handed a key over", name)
		}
	}
}

// A second key sent while the first is being used is refused at once. It used
// to block its handler on the full channel for ever.
func TestGenesisUnlock_aSecondKeyIsRefusedNotHung(t *testing.T) {
	keyCh := make(chan []byte, 1)
	h := genesisUnlockHandler(keyCh)
	first := bytes.Repeat([]byte{1}, genesisUnlockKeySize)
	if w := unlock(h, http.MethodPost, keyBody(first)); w.Code != http.StatusOK {
		t.Fatalf("first key: HTTP %d", w.Code)
	}
	done := make(chan int, 1)
	go func() {
		done <- unlock(h, http.MethodPost, keyBody(bytes.Repeat([]byte{2}, genesisUnlockKeySize))).Code
	}()
	select {
	case code := <-done:
		if code != http.StatusConflict {
			t.Errorf("second key: HTTP %d, want 409", code)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the second key's request hung")
	}
	if got := <-keyCh; !bytes.Equal(got, first) {
		t.Error("the second key replaced the first")
	}
}

// Shutdown with nothing started (an enrollment that failed early) and a
// second Shutdown are both no-ops.
func TestShutdown_nothingStartedAndTwice(t *testing.T) {
	a, err := NewAgent()
	if err != nil {
		t.Fatal(err)
	}
	a.Shutdown()
	a.Shutdown()
	if !a.shutdown {
		t.Error("the agent does not record that it shut down")
	}
}
