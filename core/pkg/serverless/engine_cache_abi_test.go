package serverless

import (
	"context"
	"testing"

	"github.com/tetratelabs/wazero/api"
	"go.uber.org/zap"
)

// bug-421: guests had no way to remove a cache entry. cache_delete is part of
// the host ABI; its signature is a contract with deployed WASM, so pin it.
func TestEngine_HostModule_CacheDeleteExported(t *testing.T) {
	engine, err := NewEngine(nil, NewMockRegistry(), NewMockHostServices(), zap.NewNop())
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer func() { _ = engine.runtime.Close(context.Background()) }()

	for _, name := range []string{"env", "host", "orama"} {
		def, ok := engine.runtime.Module(name).ExportedFunctionDefinitions()["cache_delete"]
		if !ok {
			t.Fatalf("%s.cache_delete is not exported", name)
		}
		params, results := def.ParamTypes(), def.ResultTypes()
		if len(params) != 2 || params[0] != api.ValueTypeI32 || params[1] != api.ValueTypeI32 {
			t.Errorf("%s.cache_delete params = %v, want (i32 key_ptr, i32 key_len)", name, params)
		}
		if len(results) != 1 || results[0] != api.ValueTypeI32 {
			t.Errorf("%s.cache_delete results = %v, want (i32)", name, results)
		}
	}
}

// bug-741: cache_incr_ttl(key_ptr, key_len, delta i64, ttl_seconds i64) -> i64
// is a new export beside cache_incr_by, whose signature deployed modules fix.
func TestEngine_HostModule_CacheIncrTTLExported(t *testing.T) {
	engine, err := NewEngine(nil, NewMockRegistry(), NewMockHostServices(), zap.NewNop())
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer func() { _ = engine.runtime.Close(context.Background()) }()

	want := []api.ValueType{api.ValueTypeI32, api.ValueTypeI32, api.ValueTypeI64, api.ValueTypeI64}
	for _, name := range []string{"env", "host", "orama"} {
		def, ok := engine.runtime.Module(name).ExportedFunctionDefinitions()["cache_incr_ttl"]
		if !ok {
			t.Fatalf("%s.cache_incr_ttl is not exported", name)
		}
		params, results := def.ParamTypes(), def.ResultTypes()
		if len(params) != len(want) {
			t.Fatalf("%s.cache_incr_ttl params = %v, want %v", name, params, want)
		}
		for i := range want {
			if params[i] != want[i] {
				t.Errorf("%s.cache_incr_ttl params = %v, want %v", name, params, want)
			}
		}
		if len(results) != 1 || results[0] != api.ValueTypeI64 {
			t.Errorf("%s.cache_incr_ttl results = %v, want (i64)", name, results)
		}
		if def, ok := engine.runtime.Module(name).ExportedFunctionDefinitions()["cache_incr_by"]; !ok || len(def.ParamTypes()) != 3 {
			t.Errorf("%s.cache_incr_by changed signature or disappeared", name)
		}
	}
}
