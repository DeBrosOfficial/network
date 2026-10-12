package globalcmd

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/DeBrosOfficial/network/cmd/orama/internal/clierr"
)

func writeInfo(t *testing.T, info string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "info.json")
	if err := os.WriteFile(path, []byte(info), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func planInfoFile(t *testing.T, oramad, verifier string) string {
	t.Helper()
	platform := runtime.GOOS + "/" + runtime.GOARCH
	return writeInfo(t, `{"binaries":{"`+platform+`":"https://x.example/oramad?checksum=sha256:`+oramad+`"},`+
		`"verifiers":{"`+platform+`":"https://x.example/orama-orchard-verifier?checksum=sha256:`+verifier+`"}}`+"\n")
}

func TestCheckStagedAgainstPlan_holdsBothFilesToTheChecksumsThePlanNames(t *testing.T) {
	dir := t.TempDir()
	oramad := filepath.Join(dir, "oramad")
	verifier := filepath.Join(dir, "orama-orchard-verifier")
	for path, content := range map[string]string{oramad: "oramad v2", verifier: "verifier v2"} {
		if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	oramadSum, err := sha256File(oramad)
	if err != nil {
		t.Fatal(err)
	}
	verifierSum, err := sha256File(verifier)
	if err != nil {
		t.Fatal(err)
	}
	other := strings.Repeat("0", 64)

	tests := []struct {
		name    string
		info    string
		wantErr string
	}{
		{"the plan names both", planInfoFile(t, oramadSum, verifierSum), ""},
		{"another oramad", planInfoFile(t, other, verifierSum), "names an oramad with sha256 000000000000"},
		{"another verifier", planInfoFile(t, oramadSum, other), "names an orama-orchard-verifier with sha256 000000000000"},
		{"no checksums", writeInfo(t, "upgrade to v2"), "carries no upgrade-info JSON"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkStagedAgainstPlan(context.Background(), "v2", tt.info, oramad, verifier)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("a binary the plan names was refused: %v", err)
				}
				return
			}
			if clierr.CodeOf(err) != clierr.CodeConflict || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err = %v, want a refusal naming %q", err, tt.wantErr)
			}
		})
	}
}

func TestCheckStagedAgainstPlan_unreadableOrOversizedInfoIsRefused(t *testing.T) {
	if err := checkStagedAgainstPlan(context.Background(), "v2", filepath.Join(t.TempDir(), "missing"), "a", "b"); err == nil || !strings.Contains(err.Error(), "--plan-info") {
		t.Errorf("a missing info file: %v", err)
	}
	huge := writeInfo(t, strings.Repeat("x", planInfoLimit+1))
	if err := checkStagedAgainstPlan(context.Background(), "v2", huge, "a", "b"); clierr.CodeOf(err) != clierr.CodeUsage {
		t.Errorf("an oversized info file: %v", err)
	}
}

func TestCheckStagedAgainstPlan_aMissingBinaryIsAFailureNotAMatch(t *testing.T) {
	sum := strings.Repeat("a", 64)
	err := checkStagedAgainstPlan(context.Background(), "v2", planInfoFile(t, sum, sum), filepath.Join(t.TempDir(), "missing"), "b")
	if err == nil || !strings.Contains(err.Error(), "read ") {
		t.Fatalf("err = %v, want the unreadable binary reported", err)
	}
}
