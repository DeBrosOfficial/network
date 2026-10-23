package push

import (
	"crypto/sha256"
	"fmt"
	"slices"
	"strings"

	"github.com/Orama-Network/orama/core/pkg/archivetrust"
)

// Skipping a node that already runs the build being pushed.
//
// A push re-run after an interrupted one must not upload again to the nodes
// it already reached: over a slow uplink each upload is many minutes, and a
// re-run that starts from the first node again never gets further. Before it
// uploads, the push asks the node to check its staged tree against the
// archive's identity: the digests of manifest.json and manifest.sig, and of
// every file the manifest lists. Only a check that passes skips the node; a
// mismatch, a missing file, an unreadable tree or a node that does not answer
// gets the upload and the stage as before. A skipped node is exactly as a
// stage of this archive would leave it, because the manifest signature and
// every file it signs are byte-identical to the archive's.

// stagedCheck is the sha256sum check list, relative to /opt/orama, of the
// tree the archive at path stages. ok is false for an archive no staged tree
// can be identified with — one without manifest.sig, whose stage is refused
// anyway: such a push always runs the stage.
func stagedCheck(path string) (list string, ok bool, err error) {
	manifestJSON, sig, err := archivetrust.ReadArchiveManifest(path)
	if err != nil {
		return "", false, err
	}
	if sig == nil {
		return "", false, nil
	}
	m, err := archivetrust.ParseManifest(manifestJSON)
	if err != nil {
		return "", false, err
	}
	lines := []string{
		checkLine(archivetrust.ManifestDigest(manifestJSON), archivetrust.ManifestName),
		checkLine(fmt.Sprintf("%x", sha256.Sum256(sig)), archivetrust.SignatureName),
	}
	keys := make([]string, 0, len(m.Checksums))
	for k := range m.Checksums {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		rel, err := archivetrust.EntryPath(k)
		if err != nil {
			return "", false, fmt.Errorf("manifest checksum %q: %w", k, err)
		}
		sum := strings.ToLower(m.Checksums[k])
		if !cliSumPattern.MatchString(sum) {
			return "", false, fmt.Errorf("manifest checksum of %s is not a sha256: %q", rel, sum)
		}
		lines = append(lines, checkLine(sum, rel))
	}
	return strings.Join(lines, "\n") + "\n", true, nil
}

// checkLine is one sha256sum check line.
func checkLine(sum, rel string) string {
	return sum + "  " + rel
}

// stagedCheckScript is the node-side script that exits 0 only when every file
// in list matches under base (/opt/orama on a node). Every value in list is checked by
// stagedCheck: hex digests and manifest entry paths.
func stagedCheckScript(base, list string) string {
	return "set -eu\ncd " + base + "\nsha256sum -c --quiet --strict - <<'ORAMA_STAGED'\n" + list + "ORAMA_STAGED\n"
}

// alreadyStaged runs the check through run, which executes a script on the
// node as root and returns an error unless it exited 0. Only a clean pass
// counts: any error, from the check or from reaching the node, means upload.
func alreadyStaged(run func(script string) error, base, list string) bool {
	return run(stagedCheckScript(base, list)) == nil
}
