package clustertrust

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Orama-Network/orama/core/pkg/constants"
)

// The embedded bundle is exactly the two pinned Let's Encrypt staging roots.
func TestStagingRoots_areExactlyThePinnedRoots(t *testing.T) {
	if _, err := StagingRoots(); err != nil {
		t.Fatal(err)
	}
}

// A bundle with a root missing, or one added, is refused: the pin is the list.
func TestCheckStagingRoots_refusesAnythingElse(t *testing.T) {
	first := stagingRootsPEM[:strings.Index(string(stagingRootsPEM), "-----END CERTIFICATE-----")+len("-----END CERTIFICATE-----\n")]
	for name, bundle := range map[string][]byte{
		"empty":        nil,
		"one root":     first,
		"a root twice": append(append([]byte{}, stagingRootsPEM...), first...),
		"not PEM":      []byte("not a certificate"),
	} {
		if err := CheckStagingRoots(bundle); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestExtraRoots(t *testing.T) {
	for _, dir := range []string{"", constants.LetsEncryptProductionACME} {
		roots, known, err := ExtraRoots(dir)
		if err != nil || !known || roots != nil {
			t.Errorf("%q: roots %d bytes, known %v, err %v; want none, known", dir, len(roots), known, err)
		}
	}
	roots, known, err := ExtraRoots(constants.LetsEncryptStagingACME)
	if err != nil || !known || !bytes.Equal(roots, stagingRootsPEM) {
		t.Errorf("staging: known %v, err %v, roots are the staging roots: %v", known, err, bytes.Equal(roots, stagingRootsPEM))
	}
	roots, known, err = ExtraRoots("https://ca.example.com/acme/directory")
	if err != nil || known || roots != nil {
		t.Errorf("an operator's own CA: roots %d bytes, known %v, err %v; want none, unknown", len(roots), known, err)
	}
}
