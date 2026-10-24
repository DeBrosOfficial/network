package install

// The host files the installer writes into the operating system's own
// directories, outside the trees Orama owns whole (/opt/orama, /etc/orama, the
// orama-* entries of the unit directory). The installer writes each through the
// name here, and `orama node wipe` removes each and then checks it is gone
// (core/cmd/orama/internal/production/decommission). A file written there and
// missing from these lists survives every wipe: the log rotation policy did,
// and once a nuclear wipe had deleted the orama user, its `su orama orama`
// failed logrotate.service every night on every wiped machine (testnet,
// 2026-10-24). The sysctl drop-ins are not here: each records the values it
// replaced (sysctl_prior.go), and the wipe puts those back before it removes
// the drop-in.
const (
	// LogrotateConfigPath is the Orama log rotation policy. Debian and
	// Ubuntu's logrotate reads every file in /etc/logrotate.d daily.
	LogrotateConfigPath = "/etc/logrotate.d/orama"

	// CoredumpDropInPath turns systemd-coredump storage off, part of the RAM
	// hygiene with RAMHygieneSysctlPath.
	CoredumpDropInPath = "/etc/systemd/coredump.conf.d/orama.conf"

	// LegacySudoersPath held the wildcard sudoers rules the privileged helper
	// replaced. Install deletes it; a wipe of a node never upgraded past them
	// deletes it too.
	LegacySudoersPath = "/etc/sudoers.d/orama-namespaces"
)

// HostFiles are the host files every wipe removes: each configures something
// only for an Orama installation, and names its tree or its accounts.
func HostFiles() []string {
	return []string{LogrotateConfigPath, LegacySudoersPath}
}

// HardeningHostFiles are the RAM hygiene host files besides its sysctl
// drop-in. They harden the machine whatever runs on it, so a plain wipe keeps
// them and only --nuclear removes them, with RAMHygieneSysctlPath.
func HardeningHostFiles() []string {
	return []string{CoredumpDropInPath}
}
