package tlsstatus

import (
	"errors"
	"os/exec"
	"slices"
	"strconv"
	"testing"
)

// exitWith is a real *exec.ExitError of the given status, as journalctl's would be.
func exitWith(t *testing.T, code int) error {
	t.Helper()
	err := exec.Command("sh", "-c", "exit "+strconv.Itoa(code)).Run()
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("sh did not exit %d: %v", code, err)
	}
	return err
}

func TestJournalArgs_grepBeforeTheCount(t *testing.T) {
	got := journalArgs(CaddyUnit, journalLines)
	grep, count := slices.Index(got, "--grep"), slices.Index(got, "-n")
	if grep < 0 || count < 0 || got[grep+1] != certificateLoggerPattern || got[count+1] != strconv.Itoa(journalLines) {
		t.Fatalf("journalctl %v: want --grep %s and -n %d", got, certificateLoggerPattern, journalLines)
	}
	if !slices.Contains(got, "cat") || !slices.Contains(got, CaddyUnit) {
		t.Fatalf("journalctl %v: want the unit's messages alone", got)
	}
}

func TestJournalResult(t *testing.T) {
	cases := []struct {
		name        string
		out, stderr string
		err         error
		want        string
		wantErr     bool
	}{
		{name: "lines", out: rateLimitedLine + "\n", want: rateLimitedLine + "\n"},
		{name: "no match, nothing printed", err: exitWith(t, 1)},
		{name: "no match, the no-entries line", out: "-- No entries --\n", err: exitWith(t, 1)},
		{name: "exit 1 with an error on stderr", stderr: "Failed to open journal: permission denied", err: exitWith(t, 1), wantErr: true},
		{name: "exit 1 with other output", out: "something else", err: exitWith(t, 1), wantErr: true},
		{name: "another exit status", err: exitWith(t, 2), wantErr: true},
		{name: "journalctl not found", err: exec.ErrNotFound, wantErr: true},
	}
	for _, c := range cases {
		got, err := journalResult(CaddyUnit, []byte(c.out), []byte(c.stderr), c.err)
		if (err != nil) != c.wantErr || got != c.want {
			t.Errorf("%s: got %q, %v; want %q, error %v", c.name, got, err, c.want, c.wantErr)
		}
	}
}
