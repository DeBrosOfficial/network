package rqlite

import "testing"

func TestBindHost(t *testing.T) {
	tests := []struct {
		adv  string
		want string
	}{
		{"", "127.0.0.1"},
		{"10.0.0.4:10100", "10.0.0.4"},
		{"0.0.0.0:10100", "127.0.0.1"},
		{"10.0.0.4", "10.0.0.4"},
		{"[::]:10100", "127.0.0.1"},
	}
	for _, tt := range tests {
		if got := BindHost(tt.adv); got != tt.want {
			t.Errorf("BindHost(%q) = %q, want %q", tt.adv, got, tt.want)
		}
	}
}

func TestBindAddr_refusesWildcard(t *testing.T) {
	got, err := BindAddr("10.0.0.4:10100", 10100)
	if err != nil {
		t.Fatal(err)
	}
	if got != "10.0.0.4:10100" {
		t.Fatalf("got %q", got)
	}
}
