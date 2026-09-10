package enroll

import "testing"

func TestValidate_requiresTheConsoleCode(t *testing.T) {
	f := Flags{NodeIP: "203.0.113.10", Token: "tok", GatewayURL: "https://gw.example"}
	err := f.validate()
	if err == nil {
		t.Fatal("missing --code was accepted — the agent no longer serves the code")
	}
	if got := err.Error(); got == "" {
		t.Fatal("empty error")
	}

	f.Code = "a1b2c3d4e5f60718293a"
	if err := f.validate(); err != nil {
		t.Fatalf("valid flags refused: %v", err)
	}
}
