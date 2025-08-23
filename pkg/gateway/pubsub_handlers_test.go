package gateway

import "testing"

func TestNamespaceHelpers(t *testing.T) {
	if p := namespacePrefix("ns"); p != "ns::ns::" {
		t.Fatalf("unexpected prefix: %q", p)
	}
	if tpc := namespacedTopic("ns", "topic"); tpc != "ns::ns::topic" {
		t.Fatalf("unexpected namespaced topic: %q", tpc)
	}
}
