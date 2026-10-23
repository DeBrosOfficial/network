package namespace

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

// bug-276: the coordinator allocating a port block asks the node which ports
// of the namespace range are held there. The action reads; it needs no
// admission (nothing is started), and answers the held ports.
func TestSpawnHandler_probePortsAnswersTheHeldPorts(t *testing.T) {
	orig := boundNamespacePorts
	t.Cleanup(func() { boundNamespacePorts = orig })
	boundNamespacePorts = func() ([]int, error) { return []int{10006, 10007}, nil }

	adm := &fakeAdmitter{}
	h, key, _ := admissionHandler(t, adm)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, spawnRequestWith(t, key, `{"action":"probe-ports","namespace":"acme","node_id":"n1"}`))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var resp SpawnResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if !resp.Success || !reflect.DeepEqual(resp.BoundPorts, []int{10006, 10007}) {
		t.Fatalf("response %+v, want success with [10006 10007]", resp)
	}
	if len(adm.admitted) != 0 {
		t.Fatalf("a read-only probe took the namespace's lock: %v", adm.admitted)
	}
}
