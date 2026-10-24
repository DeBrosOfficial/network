package releasedownload

import (
	"errors"
	"strings"
	"testing"
)

const testSum = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func testArchive() Archive {
	return Archive{URL: "https://releases.example/targets/nightly/orama-0.4.0-linux-amd64.tar.gz", Length: 13, SHA256: testSum}
}

func TestLines_downloadBoundedToTheSignedLengthThenChecksSizeAndDigest(t *testing.T) {
	lines, err := testArchive().Lines("/tmp/orama-push.AbCd1234/archive.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	script := strings.Join(lines, "\n")
	order := []string{
		"ulimit -f 2;",
		"curl -q --fail --silent --show-error --location --globoff --proto '=https' --proto-redir '=https' --max-redirs 5",
		"--max-filesize 13 --output /tmp/orama-push.AbCd1234/archive.tar.gz 'https://releases.example/",
		`[ "$size" = 13 ]`,
		`[ "$sum" = ` + testSum + ` ]`,
	}
	last := -1
	for _, want := range order {
		i := strings.Index(script, want)
		if i < 0 || i < last {
			t.Fatalf("missing or out of order %q in:\n%s", want, script)
		}
		last = i
	}
}

func TestLines_refusesWhatCouldNotGoIntoAShellCommand(t *testing.T) {
	for name, change := range map[string]func(a *Archive){
		"a URL that is not http(s)": func(a *Archive) { a.URL = "ftp://releases.example/x" },
		"a URL with no host":        func(a *Archive) { a.URL = "https:///x" },
		"text that is no URL":       func(a *Archive) { a.URL = "; reboot" },
		"a digest that is not hex":  func(a *Archive) { a.SHA256 = "abc; reboot" },
		"an upper case digest":      func(a *Archive) { a.SHA256 = strings.ToUpper(testSum) },
		"a length of zero":          func(a *Archive) { a.Length = 0 },
	} {
		a := testArchive()
		change(&a)
		if lines, err := a.Lines("/tmp/f"); err == nil {
			t.Errorf("%s was accepted: %v", name, lines)
		}
	}
}

func TestUnreachable_tellsARepositoryTheNodeCannotReachFromOneThatRefused(t *testing.T) {
	for msg, want := range map[string]bool{
		"curl: (6) Could not resolve host: releases.example":    true,
		"curl: (7) Failed to connect":                           true,
		"curl: (28) Connection timed out":                       true,
		"curl: (60) SSL certificate problem":                    true,
		"curl: (22) The requested URL returned error: 404":      false,
		"the download has sha256 x, the signed metadata says y": false,
	} {
		if got := Unreachable(errors.New("run on 203.0.113.7: exit status 1 (stderr: " + msg + ")")); got != want {
			t.Errorf("%q: unreachable = %v, want %v", msg, got, want)
		}
	}
}
