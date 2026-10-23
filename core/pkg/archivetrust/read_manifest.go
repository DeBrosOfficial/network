package archivetrust

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// ReadArchiveManifest returns manifest.json and manifest.sig from the build
// archive at path; sig is nil when the archive holds no manifest.sig. It reads the archive's tar stream
// and unpacks nothing; the manifest is the last entry of an archive, so the
// whole stream is read. ManifestDigest of the result is the digest a release's
// signed targets metadata names for the archive's manifest
// (releaseverify.ArchiveCustom.ManifestSHA256).
func ReadArchiveManifest(path string) (manifestJSON, sig []byte, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("open the archive: %w", err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, nil, fmt.Errorf("%s is not a gzip archive: %w", path, err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, nil, fmt.Errorf("read the archive %s: %w", path, err)
		}
		switch strings.TrimPrefix(hdr.Name, "./") {
		case SignatureName:
			if sig, err = io.ReadAll(io.LimitReader(tr, signatureLimit+1)); err != nil {
				return nil, nil, fmt.Errorf("read %s from %s: %w", SignatureName, path, err)
			}
			if len(sig) > signatureLimit {
				return nil, nil, fmt.Errorf("%s in %s is over %d bytes", SignatureName, path, signatureLimit)
			}
		case ManifestName:
			if manifestJSON, err = io.ReadAll(io.LimitReader(tr, manifestLimit+1)); err != nil {
				return nil, nil, fmt.Errorf("read %s from %s: %w", ManifestName, path, err)
			}
			if len(manifestJSON) > manifestLimit {
				return nil, nil, fmt.Errorf("%s in %s is over %d bytes", ManifestName, path, manifestLimit)
			}
		}
	}
	if manifestJSON == nil {
		return nil, nil, fmt.Errorf("%s has no %s: it is not an orama maint build archive", path, ManifestName)
	}
	return manifestJSON, sig, nil
}
