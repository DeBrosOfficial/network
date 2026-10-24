package decommission

import (
	"strings"

	"github.com/Orama-Network/orama/core/pkg/install"
	"github.com/Orama-Network/orama/core/pkg/install/installers"
)

// The host files the installer writes into the operating system's own directories
// (pkg/install HostFiles, HardeningHostFiles, and the resolver and Node.js files of the
// installers), removed by the wipe through the same names.

// resolvedService is the unit that reads the resolver drop-ins.
const resolvedService = "systemd-resolved"

// hostFilesBlock removes the host files every wipe removes.
func hostFilesBlock() string {
	return `# Host files that configure the operating system for this installation only. The log
# rotation policy names the orama user and its log directory: kept, logrotate.service failed
# every night once a nuclear wipe had deleted the user.
rm -f ` + strings.Join(install.HostFiles(), " ")
}

// oramaResolvConfTest is a shell test that is true while /etc/resolv.conf is still the file
// the installer wrote.
func oramaResolvConfTest() string {
	want := strings.ReplaceAll(strings.TrimSuffix(installers.OramaResolvConf, "\n"), "\n", `\n`)
	return `[ "$(cat ` + installers.ResolvConfPath + ` 2>/dev/null)" = "$(printf "` + want + `")" ]`
}

// resolverRestoreBlock gives the host resolver back to systemd-resolved. The installer turned
// its stub listener off so CoreDNS could own :53, turned off LLMNR and mDNS, and pointed
// /etc/resolv.conf at CoreDNS on 127.0.0.1, which the wipe has removed. The drop-ins go,
// resolved restarts with its stub listener, and /etc/resolv.conf links to its stub file
// again (the Debian and Ubuntu default the installer replaced, and the installer only touched
// it on a machine running resolved). A resolv.conf the operator changed since is theirs and
// stays; one still Orama's that cannot be relinked is a leftover.
func resolverRestoreBlock() string {
	return `# The resolver: the drop-ins go, resolved restarts with its stub listener, and
# /etc/resolv.conf, while still the file the installer wrote, links to its stub again.
resolved_dropins=0
for dropin in ` + strings.Join(installers.ResolvedDropInPaths(), " ") + `; do
    if [ -e "$dropin" ]; then resolved_dropins=1; rm -f "$dropin"; fi
done
if [ "$resolved_dropins" = 1 ]; then systemctl try-restart ` + resolvedService + `; fi
if ` + oramaResolvConfTest() + ` && [ -e ` + installers.ResolvedStubResolvConf + ` ]; then
    ln -sfn ` + installers.ResolvedStubResolvConf + ` ` + installers.ResolvConfPath + `
fi`
}

// resolverLeftoverCheck reports a resolv.conf that still names CoreDNS.
func resolverLeftoverCheck() string {
	return `if ` + oramaResolvConfTest() + `; then left "` + installers.ResolvConfPath + ` (still the file the installer wrote, naming CoreDNS; ` + installers.ResolvedStubResolvConf + ` is missing)"; fi
`
}

// nuclearHostFilesBlock is the nuclear wipe's part for the host files a plain wipe keeps: the
// hardening files besides the RAM hygiene sysctl drop-in (which ramHygieneRestoreBlock
// removes), and the Node.js runtime, its releases and the command links into them. A command
// in /usr/bin that does not link into the releases is not Orama's and stays.
func nuclearHostFilesBlock() string {
	return `    rm -f ` + strings.Join(install.HardeningHostFiles(), " ") + `
` + forEachNodeJSLink(`rm -f "$link"`) + `
    rm -rf ` + installers.NodeJSInstallRoot
}

// nodeJSLeftoverCheck reports command links still pointing into the Node.js releases.
func nodeJSLeftoverCheck() string {
	return forEachNodeJSLink(`left "$link"`) + "\n"
}

// forEachNodeJSLink runs action for each Node.js command link that points into the releases.
func forEachNodeJSLink(action string) string {
	return `    for link in ` + strings.Join(installers.NodeJSCommandLinks(), " ") + `; do
        case "$(readlink "$link")" in ` + installers.NodeJSInstallRoot + `/*) ` + action + ` ;; esac
    done`
}
