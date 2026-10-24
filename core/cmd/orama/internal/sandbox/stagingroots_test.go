package sandbox

import (
	"os"
	"testing"

	"github.com/Orama-Network/orama/core/cmd/orama/internal"
	"github.com/Orama-Network/orama/core/pkg/clustertrust"
)

// Create records the sandbox environment with the staging roots as its CA, so
// the CLI can reach a cluster whose certificates come from staging, and makes
// it the active environment.
func TestRegisterEnvironment_trustsTheStagingRootsForTheSandboxDomain(t *testing.T) {
	state := testSandbox(t)
	cfg := &Config{Domain: "sbx.example.com"}

	if err := registerEnvironment(cfg, state); err != nil {
		t.Fatal(err)
	}
	env, err := cli.GetActiveEnvironment()
	if err != nil {
		t.Fatal(err)
	}
	if env.Name != "sandbox" || env.GatewayURL != "https://sbx.example.com" {
		t.Fatalf("active environment = %+v, want sandbox at https://sbx.example.com", env)
	}
	data, err := os.ReadFile(env.CAFile)
	if err != nil {
		t.Fatalf("the sandbox environment's CA file: %v", err)
	}
	roots, err := clustertrust.StagingRoots()
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(roots) {
		t.Fatal("the sandbox environment's CA file is not the staging roots")
	}
	if err := cli.TrustEnvironmentCAs(); err != nil {
		t.Fatalf("the CLI cannot load the sandbox CA: %v", err)
	}
}
