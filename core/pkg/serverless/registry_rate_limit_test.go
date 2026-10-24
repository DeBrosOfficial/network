package serverless

import (
	"context"
	"testing"
)

// bug-740: a function's own invoke limits are written on deploy and read back
// on every read path, so the engine can hand them to the limiter. The tier
// existed and nothing ever filled it.
func TestRegister_functionRateLimitRoundTrips(t *testing.T) {
	r, _ := newVersionedRegistry(t)
	ctx := context.Background()
	want := &FunctionRateLimit{
		PerCaller: RateLimitRule{Requests: 5, WindowSeconds: 60},
		PerIP:     RateLimitRule{Requests: 2, WindowSeconds: 3600},
	}
	if _, err := r.Register(ctx, &FunctionDefinition{Name: "user-create", Namespace: "ns", RateLimit: want}, []byte("wasm")); err != nil {
		t.Fatalf("deploy: %v", err)
	}
	if _, err := r.Register(ctx, &FunctionDefinition{Name: "open", Namespace: "ns"}, []byte("wasm")); err != nil {
		t.Fatalf("deploy: %v", err)
	}

	got, err := r.Get(ctx, "ns", "user-create", 0)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.RateLimit == nil || *got.RateLimit != *want {
		t.Errorf("Get read back %+v, want %+v", got.RateLimit, want)
	}
	list, err := r.List(ctx, "ns")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	for _, fn := range list {
		switch fn.Name {
		case "user-create":
			if fn.RateLimit == nil || *fn.RateLimit != *want {
				t.Errorf("List read back %+v", fn.RateLimit)
			}
		case "open":
			if fn.RateLimit != nil {
				t.Errorf("a function deployed with no limit read back %+v", fn.RateLimit)
			}
		}
	}
}

func TestFunctionRateLimit_Validate(t *testing.T) {
	valid := []*FunctionRateLimit{
		nil,
		{},
		{PerIP: RateLimitRule{Requests: 2, WindowSeconds: 3600}},
		{PerCaller: RateLimitRule{Requests: MaxRateLimitRequests, WindowSeconds: 86400}},
		{PerCaller: RateLimitRule{Requests: 1, WindowSeconds: 1}},
	}
	for _, l := range valid {
		if err := l.Validate(); err != nil {
			t.Errorf("%+v refused: %v", l, err)
		}
	}
	invalid := map[string]*FunctionRateLimit{
		"rate_limit.per_ip.requests":     {PerIP: RateLimitRule{Requests: 0, WindowSeconds: 60}},
		"rate_limit.per_caller.requests": {PerCaller: RateLimitRule{Requests: MaxRateLimitRequests + 1, WindowSeconds: 60}},
		"rate_limit.per_caller.window":   {PerCaller: RateLimitRule{Requests: 5}},
		"rate_limit.per_ip.window":       {PerIP: RateLimitRule{Requests: 5, WindowSeconds: 86401}},
	}
	for field, l := range invalid {
		err := l.Validate()
		ve, ok := err.(*ValidationError)
		if !ok || ve.Field != field {
			t.Errorf("%+v: got %v, want a ValidationError naming %s", l, err, field)
		}
	}
}
