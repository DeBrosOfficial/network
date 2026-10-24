package corsorigins

import "testing"

func TestCanonical(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"https://App.Example.com", "https://app.example.com"},
		{"https://app.example.com/", "https://app.example.com"},
		{"https://app.example.com:443", "https://app.example.com"},
		{"http://app.example.com:80", "http://app.example.com"},
		{"http://localhost:5173", "http://localhost:5173"},
		{"tauri://localhost", "tauri://localhost"},
		{"http://[::1]:8080", "http://[::1]:8080"},
	} {
		got, err := Canonical(tc.in)
		if err != nil || got != tc.want {
			t.Errorf("Canonical(%q) = %q, %v; want %q", tc.in, got, err, tc.want)
		}
	}
	for _, bad := range []string{
		"", "*", "null", "https://*.example.com", "ftp://example.com", "https://example.com/path",
		"https://example.com?q=1", "https://user@example.com", "example.com", "https://",
	} {
		if got, err := Canonical(bad); err == nil {
			t.Errorf("Canonical(%q) = %q, want a refusal", bad, got)
		}
	}
}

// Tauri v2 serves an app from tauri://localhost on macOS, Linux and iOS, and
// from http://tauri.localhost (https:// with useHttpsScheme) on Windows and
// Android. All of them are this machine; a host that merely starts with one is
// not.
func TestBuiltIn(t *testing.T) {
	const base = "orama.test"
	for _, origin := range []string{
		"https://orama.test", "https://ns-anchat.orama.test", "http://localhost:3000",
		"tauri://localhost", "http://tauri.localhost", "https://tauri.localhost",
		"http://myapp.localhost:5173", "http://127.0.0.1:8080", "http://[::1]:8080",
	} {
		if !BuiltIn(origin, base) {
			t.Errorf("BuiltIn(%q) = false, want true", origin)
		}
	}
	for _, origin := range []string{
		"https://evil.com", "http://tauri.localhost.evil.com", "https://orama.test.evil.com",
		"https://notorama.test", "http://localhost.evil.com", "null", "", "tauri://evil.com",
	} {
		if BuiltIn(origin, base) {
			t.Errorf("BuiltIn(%q) = true, want false", origin)
		}
	}
}

func TestNormalize(t *testing.T) {
	got, err := Normalize([]string{"https://A.example.com", "https://a.example.com/", "https://b.example.com"})
	if err != nil || len(got) != 2 || got[0] != "https://a.example.com" || got[1] != "https://b.example.com" {
		t.Fatalf("Normalize = %v, %v", got, err)
	}
	many := make([]string, MaxOriginsPerNamespace+1)
	for i := range many {
		many[i] = "https://app" + string(rune('a'+i)) + ".example.com"
	}
	if _, err := Normalize(many); err == nil {
		t.Error("a list over the ceiling was accepted")
	}
	if _, err := Normalize([]string{"https://ok.example.com", "*"}); err == nil {
		t.Error("a wildcard inside a list was accepted")
	}
}

func TestNamespaceFromHost(t *testing.T) {
	for _, tc := range []struct{ host, want string }{
		{"ns-anchat.orama.test", "anchat"},
		{"NS-AnChat.orama.test:443", "anchat"},
		{"orama.test", ""},
		{"app.orama.test", ""},
		{"ns-a.b.orama.test", ""},
		{"ns-anchat.orama.test.evil.com", ""},
	} {
		if got := NamespaceFromHost(tc.host, "orama.test"); got != tc.want {
			t.Errorf("NamespaceFromHost(%q) = %q, want %q", tc.host, got, tc.want)
		}
	}
}
