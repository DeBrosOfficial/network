package decommission

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/Orama-Network/orama/core/pkg/install"
	"github.com/Orama-Network/orama/core/pkg/install/installers"
	"github.com/Orama-Network/orama/core/pkg/privhelper"
)

// installerDirs are the packages that write the host, relative to the module root.
var installerDirs = []string{"pkg/install", "pkg/install/installers"}

// Why a host path the installer names is not one a wipe removes.
const (
	notWrittenRead   = "read or executed, never written"
	notWrittenShared = "a shared directory; the installer writes only named files in it, each listed for the wipe"
	notWrittenMsg    = "part of an error message"
	keptNSS          = "an edit adding the systemd source to the OS's own nsswitch.conf (the distro default on Ubuntu); kept, as it changes nothing without dynamic users"
	restoredResolv   = "replaced by the resolver restore (resolverRestoreBlock), not removed"
	removedByPurge   = "installed by the tor package, which --nuclear purges"
)

// hostPathsNotRemoved are the host paths the installer's code names that no wipe removes,
// with the reason. Every other one must be a path a wipe removes, or lie under one.
var hostPathsNotRemoved = map[string]string{
	"/etc/os-release": notWrittenRead,
	"/etc/os-release has no VERSION_CODENAME; cannot pick the Tor apt suite": notWrittenMsg,
	"/usr/sbin/nologin":                           notWrittenRead,
	"/usr/sbin:/usr/bin:/sbin:/bin":               notWrittenRead,
	"/usr/bin/apt-get":                            notWrittenRead,
	"/usr/bin/ipfs":                               notWrittenRead,
	"/usr/bin/ipfs-cluster-service":               notWrittenRead,
	"/usr/bin/olric-server":                       notWrittenRead,
	"/usr/bin/tor":                                removedByPurge,
	"/usr/bin/tor -f ":                            removedByPurge,
	"/lib/*/libnss_systemd.so.2":                  notWrittenRead,
	"/lib/libnss_systemd.so.2":                    notWrittenRead,
	"/usr/lib/*/libnss_systemd.so.2":              notWrittenRead,
	"/usr/lib/libnss_systemd.so.2":                notWrittenRead,
	"/usr/bin":                                    notWrittenShared,
	"/var/lib/":                                   notWrittenShared,
	"/etc/systemd/system":                         notWrittenShared,
	"/etc/systemd/system/multi-user.target.wants": notWrittenShared,
	"/etc/systemd/resolved.conf.d":                notWrittenShared,
	"/etc/wireguard":                              notWrittenShared,
	"/etc/nsswitch.conf":                          keptNSS,
	installers.ResolvConfPath:                     restoredResolv,
}

// hostRoots are the top-level host directories a path the installer writes can live in.
var hostRoots = []string{"/etc/", "/var/", "/usr/", "/lib/"}

// installerHostPaths are the string constants and literals in the installer's code that name
// a host path.
func installerHostPaths(t *testing.T) map[string]string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	moduleRoot := filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "..")
	found := map[string]string{}
	fset := token.NewFileSet()
	for _, dir := range installerDirs {
		files, err := filepath.Glob(filepath.Join(moduleRoot, dir, "*.go"))
		if err != nil || len(files) == 0 {
			t.Fatalf("no Go files in %s: %v", dir, err)
		}
		for _, path := range files {
			if strings.HasSuffix(path, "_test.go") {
				continue
			}
			f, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			ast.Inspect(f, func(n ast.Node) bool {
				lit, ok := n.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					return true
				}
				s, err := strconv.Unquote(lit.Value)
				if err != nil {
					return true
				}
				for _, root := range hostRoots {
					if strings.HasPrefix(s, root) {
						found[s] = fset.Position(lit.Pos()).String()
					}
				}
				return true
			})
		}
	}
	return found
}

// removedByAWipe reports whether a wipe, plain or nuclear, removes path: it is a path the
// wipe removes, lies under one, or is orama-named in the unit directory (the wipe sweeps
// those).
func removedByAWipe(path string) bool {
	removed := append(append(append(wipedPaths(), globalPaths()...), nuclearPaths()...),
		installers.LegacyAnyonePaths(install.OramaDir)...)
	removed = append(removed, installers.NodeJSCommandLinks()...)
	removed = append(removed, filepath.Join(systemdUnitDir, privhelper.SocketUnitName), filepath.Join(systemdUnitDir, privhelper.ServiceUnitName))
	if strings.HasPrefix(path, systemdUnitDir+"/orama-") {
		return true
	}
	for _, r := range removed {
		if path == r || strings.HasPrefix(path, r+"/") {
			return true
		}
	}
	return false
}

// Every host path the installer names is removed by a wipe, or is listed with the reason it
// is not. The log rotation policy was neither: kept after a nuclear wipe, it named the deleted
// orama user and failed logrotate.service every night (testnet, 2026-10-24).
func TestWipe_removesEveryHostPathTheInstallerWrites(t *testing.T) {
	found := installerHostPaths(t)
	for path, at := range found {
		if _, ok := hostPathsNotRemoved[path]; ok {
			continue
		}
		if !removedByAWipe(path) {
			t.Errorf("%s: the installer names %s, which no wipe removes; add it to install.HostFiles (every wipe) or install.HardeningHostFiles (--nuclear), or to hostPathsNotRemoved with the reason", at, path)
		}
	}
	for path := range hostPathsNotRemoved {
		if _, ok := found[path]; !ok {
			t.Errorf("hostPathsNotRemoved entry %q is named nowhere in the installer any more; remove it", path)
		}
	}
}

// The log rotation policy and the legacy sudoers rules go with every wipe, the coredump
// drop-in only with --nuclear, and each is checked for afterwards.
func TestWipeScript_removesTheHostFiles(t *testing.T) {
	for _, nuclear := range []bool{false, true} {
		removal := removalPart(wipeScript(nuclear))
		checked := checkedPaths(leftoverCheck(nuclear))
		for _, path := range []string{install.LogrotateConfigPath, install.LegacySudoersPath} {
			if !strings.Contains(removal, "rm -f "+strings.Join(install.HostFiles(), " ")) || !slices.Contains(checked, path) {
				t.Errorf("nuclear=%v: %s is not removed and checked", nuclear, path)
			}
		}
		if got := slices.Contains(checked, install.CoredumpDropInPath); got != nuclear {
			t.Errorf("nuclear=%v: the coredump drop-in is checked: %v", nuclear, got)
		}
	}
	script := removalPart(wipeScript(false))
	nuclearAt := strings.Index(script, `if [ -n "$NUCLEAR" ]`)
	if at := strings.Index(script, "rm -f "+install.CoredumpDropInPath); at < 0 || nuclearAt < 0 || at < nuclearAt {
		t.Errorf("the coredump drop-in (at %d) is not removed inside the nuclear branch (at %d): it is RAM hygiene, which only --nuclear removes", at, nuclearAt)
	}
}

// hostRoot runs a script block with the host paths it names moved into root.
func hostRoot(block, root string) string {
	for _, prefix := range []string{"/etc/", "/run/", "/usr/"} {
		block = strings.ReplaceAll(block, " "+prefix, " "+root+prefix)
	}
	return block
}

func runBlock(t *testing.T, script string, fakes map[string]string) string {
	t.Helper()
	cmd := exec.Command(bashPath(t), "-c", script)
	cmd.Env = []string{"PATH=" + fakeBin(t, fakes) + ":/usr/bin:/bin"}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	return string(out)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// The resolver goes back to systemd-resolved: the drop-ins go, resolved restarts, and the
// resolv.conf the installer wrote links to the stub again. One the operator changed stays,
// and one still Orama's that cannot be relinked is a leftover.
func TestResolverRestore(t *testing.T) {
	cases := []struct {
		name, resolv string
		stub, linked bool
	}{
		{"orama's file, stub present", installers.OramaResolvConf, true, true},
		{"operator's file", "nameserver 192.0.2.53\n", true, false},
		{"orama's file, no stub", installers.OramaResolvConf, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := t.TempDir()
			log := filepath.Join(t.TempDir(), "calls")
			resolv := root + installers.ResolvConfPath
			writeFile(t, resolv, c.resolv)
			for _, dropin := range installers.ResolvedDropInPaths() {
				writeFile(t, root+dropin, "[Resolve]\n")
			}
			if c.stub {
				writeFile(t, root+installers.ResolvedStubResolvConf, "nameserver 127.0.0.53\n")
			}
			fakes := map[string]string{"systemctl": `echo "systemctl $*" >> "` + log + `"`}
			out := runBlock(t, hostRoot(resolverRestoreBlock()+"\nleftovers=0\nleft() { echo \"LEFTOVER: $*\"; }\n"+resolverLeftoverCheck(), root), fakes)
			for _, dropin := range installers.ResolvedDropInPaths() {
				if _, err := os.Lstat(root + dropin); err == nil {
					t.Errorf("%s was left", dropin)
				}
			}
			if calls, _ := os.ReadFile(log); strings.TrimSpace(string(calls)) != "systemctl try-restart "+resolvedService {
				t.Errorf("systemctl calls: %q", calls)
			}
			target, _ := os.Readlink(resolv)
			if linked := target == root+installers.ResolvedStubResolvConf; linked != c.linked {
				t.Errorf("resolv.conf linked to the stub: %v, want %v", linked, c.linked)
			}
			if !c.linked {
				if data, _ := os.ReadFile(resolv); string(data) != c.resolv {
					t.Errorf("resolv.conf changed to %q", data)
				}
			}
			if reported := strings.Contains(out, "LEFTOVER: "+installers.ResolvConfPath); reported != (c.resolv == installers.OramaResolvConf && !c.linked) {
				t.Errorf("leftover reported: %v\n%s", reported, out)
			}
		})
	}
}

// A nuclear wipe removes the Node.js releases and the command links into them, and nothing
// else in /usr/bin.
func TestNuclearHostFiles_removeNodeJSAndOnlyItsLinks(t *testing.T) {
	root := t.TempDir()
	release := root + installers.NodeJSInstallRoot + "/node-v1-linux-x64/bin"
	writeFile(t, filepath.Join(release, "node"), "")
	links := installers.NodeJSCommandLinks()
	if err := os.MkdirAll(filepath.Dir(root+links[0]), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(release, "node"), root+links[0]); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/opt/someone-else/npm", root+links[1]); err != nil {
		t.Fatal(err)
	}
	runBlock(t, hostRoot(nuclearHostFilesBlock(), root), nil)
	if _, err := os.Lstat(root + links[0]); err == nil {
		t.Errorf("%s still links into the releases", links[0])
	}
	if _, err := os.Lstat(root + links[1]); err != nil {
		t.Errorf("%s is not Orama's and was removed", links[1])
	}
	if _, err := os.Stat(root + installers.NodeJSInstallRoot); err == nil {
		t.Errorf("%s was left", installers.NodeJSInstallRoot)
	}
}
