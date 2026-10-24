package serverless

import (
	"context"
	"testing"

	"github.com/tetratelabs/wazero/api"
	"go.uber.org/zap"
)

// feat-941: get_caller_attestation is part of the host ABI, a contract with
// deployed WASM: under every module name it takes nothing and returns the
// packed ptr<<32|len of a JSON object, as get_caller_capability does.
func TestEngine_HostModule_GetCallerAttestationExported(t *testing.T) {
	engine, err := NewEngine(nil, NewMockRegistry(), NewMockHostServices(), zap.NewNop())
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer func() { _ = engine.runtime.Close(context.Background()) }()

	for _, name := range []string{"env", "host", "orama"} {
		def, ok := engine.runtime.Module(name).ExportedFunctionDefinitions()["get_caller_attestation"]
		if !ok {
			t.Fatalf("%s.get_caller_attestation is not exported", name)
		}
		if len(def.ParamTypes()) != 0 {
			t.Errorf("%s.get_caller_attestation params = %v, want none", name, def.ParamTypes())
		}
		if results := def.ResultTypes(); len(results) != 1 || results[0] != api.ValueTypeI64 {
			t.Errorf("%s.get_caller_attestation results = %v, want (i64 ptr<<32|len)", name, results)
		}
	}
}
