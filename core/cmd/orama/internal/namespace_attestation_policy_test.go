package cli

import (
	"encoding/json"
	"testing"
)

// A PUT names only the fields the caller changed, so a flag left out keeps
// the gateway's value; no field at all reads instead.
func TestAttestationPolicyChange_bodyCarriesOnlyNamedFields(t *testing.T) {
	if raw, err := (AttestationPolicyChange{}).body(); err != nil || raw != nil {
		t.Fatalf("no flags made a body %q, %v; want a read", raw, err)
	}
	mode, pkg, graphene := "monitor", "chat.anchat", true
	digests := []string{"ab"}
	raw, err := AttestationPolicyChange{Mode: &mode, AndroidPackage: &pkg, AllowGrapheneOS: &graphene, SigningDigests: &digests}.body()
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got["mode"] != "monitor" || got["ios"] != nil || got["reattest_interval"] != nil {
		t.Errorf("top level %v", got)
	}
	android, _ := got["android"].(map[string]any)
	if android["package"] != "chat.anchat" || android["allow_grapheneos"] != true || len(android) != 3 {
		t.Errorf("android %v", android)
	}
}
