package sandbox

import (
	"encoding/json"
	"os"
	"slices"
	"testing"
)

func allowed(p *SeccompProfile) []string {
	var names []string
	for _, rule := range p.Syscalls {
		if rule.Action == ActionAllow {
			names = append(names, rule.Names...)
		}
	}
	return names
}

func TestBuildProfile_modeSetsTheDefaultAction(t *testing.T) {
	if got := BuildProfile("rqlite", SeccompEnforce).DefaultAction; got != ActionKillProcess {
		t.Errorf("enforce default %s, want %s", got, ActionKillProcess)
	}
	if got := BuildProfile("rqlite", SeccompAudit).DefaultAction; got != ActionLog {
		t.Errorf("audit default %s, want %s", got, ActionLog)
	}
}

func TestBuildProfile_baseSetPlusTheServicesOwn(t *testing.T) {
	for name, extra := range ServiceSyscalls {
		got := allowed(BuildProfile(name, SeccompEnforce))
		for _, sc := range append(slices.Clone(baseSyscalls), extra...) {
			if !slices.Contains(got, sc) {
				t.Errorf("%s: %s is not allowed", name, sc)
			}
		}
	}
	if got := allowed(BuildProfile("unknown-service", SeccompEnforce)); len(got) != len(baseSyscalls) {
		t.Errorf("an unknown service is allowed %d syscalls, want the %d of the base set", len(got), len(baseSyscalls))
	}
}

// Building one service's profile must not leak its syscalls into the base set
// or into another service's profile.
func TestBuildProfile_profilesDoNotShareState(t *testing.T) {
	base := slices.Clone(baseSyscalls)
	BuildProfile("ipfs", SeccompEnforce)
	BuildProfile("rqlite", SeccompEnforce)
	if !slices.Equal(base, baseSyscalls) {
		t.Fatal("building a profile changed the base syscall set")
	}
	if slices.Contains(allowed(BuildProfile("coredns", SeccompEnforce)), "splice") {
		t.Error("coredns is allowed splice, which only ipfs and the gateway asked for")
	}
}

// What a service never needs and an attacker inside one would want: changing
// the kernel, other processes' memory, namespaces and mounts, privilege.
func TestBuildProfile_noServiceMayEscape(t *testing.T) {
	forbidden := []string{
		"ptrace", "process_vm_readv", "process_vm_writev",
		"mount", "umount2", "pivot_root", "chroot", "unshare", "setns",
		"init_module", "finit_module", "delete_module", "kexec_load", "kexec_file_load",
		"bpf", "perf_event_open", "reboot", "swapon", "swapoff",
		"setuid", "setgid", "setreuid", "setregid", "setresuid", "setresgid", "capset",
		"keyctl", "add_key", "request_key",
	}
	for _, name := range append([]string{"unknown-service"}, keys(ServiceSyscalls)...) {
		got := allowed(BuildProfile(name, SeccompEnforce))
		for _, sc := range forbidden {
			if slices.Contains(got, sc) {
				t.Errorf("%s may call %s", name, sc)
			}
		}
	}
}

func keys(m map[string][]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// The file is the OCI/libseccomp shape, readable only by the agent.
func TestWriteProfile_ociShapeOwnerOnly(t *testing.T) {
	const name = "seccomp-test-service"
	path, err := WriteProfile(name, SeccompEnforce)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("profile mode %v, want 0600", info.Mode().Perm())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		DefaultAction string `json:"defaultAction"`
		Syscalls      []struct {
			Names  []string `json:"names"`
			Action string   `json:"action"`
		} `json:"syscalls"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.DefaultAction != string(ActionKillProcess) || len(doc.Syscalls) != 1 ||
		doc.Syscalls[0].Action != string(ActionAllow) || len(doc.Syscalls[0].Names) != len(baseSyscalls) {
		t.Errorf("profile on disk %s", data)
	}
}
