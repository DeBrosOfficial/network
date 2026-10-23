package build

import (
	"fmt"
	"os"
	"path/filepath"
)

// pinnedCacheDirName is where release tarballs whose SHA-256 is pinned are
// kept between builds, under the user's cache directory. Every build used to
// download them again, and a network blip on the Kubo download host failed a deploy
// for a file it already had.
const pinnedCacheDirName = "orama-build-pinned"

// fetchPinned puts the tarball at url into destPath, verified against its
// pinned SHA-256. A cached copy with that digest is used without the network;
// a download is verified, then cached under its digest. A cached copy whose
// digest no longer matches is removed and downloaded again. destPath is a data
// file either way (archiveFileMode, what a download is created with): the
// archive takes an entry's mode from the file, so the first build on a machine
// and every later one ship the same bytes.
func fetchPinned(url, destPath, name, arch string, pins map[string]string) error {
	want, ok := pins[arch]
	if !ok {
		return fmt.Errorf("no SHA-256 is pinned for %s on %s; pin the release digest in pkg/constants/release_digests.go", name, arch)
	}
	cached, err := pinnedCachePath(want)
	if err != nil {
		return err
	}
	if err := copyFileMode(cached, destPath, archiveFileMode); err == nil {
		if verifyPinnedSHA256(destPath, name, arch, pins) == nil {
			return nil
		}
		if err := os.Remove(cached); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove the corrupt cached %s: %w", cached, err)
		}
	}
	if err := downloadFile(url, destPath); err != nil {
		return err
	}
	if err := verifyPinnedSHA256(destPath, name, arch, pins); err != nil {
		return err
	}
	return storeInPinnedCache(destPath, cached)
}

// pinnedCachePath is the cache file for a digest.
func pinnedCachePath(sha string) (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("find the user cache directory for release tarballs: %w", err)
	}
	return filepath.Join(base, pinnedCacheDirName, sha), nil
}

// storeInPinnedCache copies a verified tarball into the cache atomically.
func storeInPinnedCache(src, cached string) error {
	if err := os.MkdirAll(filepath.Dir(cached), 0o700); err != nil {
		return fmt.Errorf("create the release cache %s: %w", filepath.Dir(cached), err)
	}
	tmp := cached + ".tmp"
	if err := copyFileMode(src, tmp, archiveFileMode); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("cache %s: %w", src, err)
	}
	if err := os.Rename(tmp, cached); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("cache %s: %w", src, err)
	}
	return nil
}
