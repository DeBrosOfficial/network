// Package releasedownload is the shell a node runs to download a release
// archive from the release repository itself and check it against the length
// and SHA-256 of the signed targets metadata, which reach the node over the SSH
// session and not through the repository. `orama setup` (a machine with no orama
// yet) and `orama upgrade` (a node that verifies the archive against the release
// root it adopted) both have their nodes download the release this way, so the
// operator's computer fetches only the metadata.
package releasedownload

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"

	"github.com/Orama-Network/orama/core/cmd/orama/internal/production/clusterops"
)

const (
	// connectSeconds, stallSeconds and stallBytes are curl's limits on a download
	// that does not move: no connection in 30 s, or less than 1 KiB/s for a
	// minute, ends it with an error instead of leaving it to the caller's budget.
	connectSeconds = 30
	stallSeconds   = 60
	stallBytes     = 1024
	// maxRedirects: GitHub release assets redirect once or twice.
	maxRedirects = 5
	// fileLimitBlock is the unit `ulimit -f` is told in. bash counts 1024 bytes,
	// POSIX mode 512; the smaller is assumed, so the cap is never below the archive.
	fileLimitBlock = 512
	// fileLimitSlack is the blocks the cap has above the archive: enough that an
	// archive of exactly the signed length is written whole, and a longer one is
	// cut at the cap and found too long by the size check.
	fileLimitSlack = 2
)

// sha256Hex is a SHA-256 in lower case hex.
var sha256Hex = regexp.MustCompile(`^[0-9a-f]{64}$`)

// Archive is a release archive as the verified metadata names it.
type Archive struct {
	// URL is where the repository serves the archive.
	URL string
	// Length and SHA256 (lower case hex) are the archive's size and digest in the
	// signed targets metadata.
	Length int64
	SHA256 string
}

// Lines are the shell lines that download a into file and refuse it unless it
// has the signed length and SHA-256; the caller's script runs them under
// `set -eu -o pipefail` and removes file when they fail. file must be a
// shell word that is safe as it is (a fixed path, or a quoted variable).
//
// The download is bounded whatever the server sends: curl stops at the length
// it was told, and `ulimit -f` caps what the process can write when the server
// gives no Content-Length. -q keeps a ~/.curlrc out of it and --globoff makes
// the URL literal. The protocol is the repository's own; a redirect may only go
// to https, so a repository cannot send the node to a plain-HTTP mirror. Every
// value interpolated is quoted or checked here: the URL is quoted, the digest is
// hex and the length a number.
func (a Archive) Lines(file string) ([]string, error) {
	u, err := url.Parse(a.URL)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return nil, fmt.Errorf("the release URL %q is not an http(s) URL", a.URL)
	}
	if !sha256Hex.MatchString(a.SHA256) {
		return nil, fmt.Errorf("the release digest %q is not a SHA-256", a.SHA256)
	}
	if a.Length <= 0 {
		return nil, fmt.Errorf("the release length %d is not a size", a.Length)
	}
	length := strconv.FormatInt(a.Length, 10)
	return []string{
		fmt.Sprintf("( ulimit -f %d; curl -q --fail --silent --show-error --location --globoff --proto %s --proto-redir '=https' --max-redirs %d "+
			"--connect-timeout %d --speed-limit %d --speed-time %d --max-filesize %s --output %s %s )",
			a.Length/fileLimitBlock+fileLimitSlack, clusterops.ShellQuote("="+u.Scheme), maxRedirects, connectSeconds,
			stallBytes, stallSeconds, length, file, clusterops.ShellQuote(a.URL)),
		"size=$(stat -c %s " + file + ")",
		fmt.Sprintf(`[ "$size" = %s ] || { echo "the download is $size bytes, the signed metadata says %s" >&2; exit 1; }`, length, length),
		"sum=$(sha256sum " + file + " | cut -d' ' -f1)",
		fmt.Sprintf(`[ "$sum" = %s ] || { echo "the download has sha256 $sum, the signed metadata says %s" >&2; exit 1; }`, a.SHA256, a.SHA256),
	}, nil
}

// curlExit is how curl reports a failure in its message ("curl: (6) ...").
var curlExit = regexp.MustCompile(`curl: \((\d+)\)`)

// unreachableCurlExits are the curl exit codes of a repository the node cannot
// reach: the proxy or host does not resolve (5, 6), the connection is refused or
// times out (7, 28), the TLS handshake or certificate fails (35, 51, 58, 59, 60,
// 77), or the transfer is cut (52, 55, 56). A repository that answered 404 (22),
// a download of the wrong size or a digest that differs are not these.
var unreachableCurlExits = map[int]bool{5: true, 6: true, 7: true, 28: true, 35: true, 51: true, 52: true, 55: true, 56: true, 58: true, 59: true, 60: true, 77: true}

// Unreachable says whether err, the failure of a script that ran Lines, is a
// node that could not reach the repository: the case an upload from the
// operator's computer is for.
func Unreachable(err error) bool {
	m := curlExit.FindStringSubmatch(err.Error())
	if m == nil {
		return false
	}
	code, convErr := strconv.Atoi(m[1])
	return convErr == nil && unreachableCurlExits[code]
}
