package sandbox

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// sleeper is a Process around a real child that runs until signalled, started
// without the sandbox's credentials (which need root).
func sleeper(t *testing.T, name string) *Process {
	t.Helper()
	cmd := exec.Command("sleep", "300")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start sleep: %v", err)
	}
	t.Cleanup(func() {
		if cmd.ProcessState == nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	})
	return &Process{Config: Config{Name: name}, cmd: cmd}
}

func TestProcess_stopEndsTheChild(t *testing.T) {
	p := sleeper(t, "rqlite")
	if !p.IsRunning() {
		t.Fatal("a started child is not running")
	}
	done := make(chan error, 1)
	go func() { done <- p.Stop() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Stop did not return: SIGTERM did not end the child")
	}
	if p.IsRunning() {
		t.Error("the child still runs after Stop")
	}
}

func TestProcess_neverStartedIsNotRunningAndStopsCleanly(t *testing.T) {
	p := &Process{Config: Config{Name: "olric"}}
	if p.IsRunning() {
		t.Error("a process that never started reports running")
	}
	if err := p.Stop(); err != nil {
		t.Errorf("stopping a process that never started: %v", err)
	}
}

// StopAll must stop every service StartAll starts. Its order is a list of its
// own, so a service added to the configs and not to that list would be left
// running at shutdown.
func TestSupervisor_stopAllStopsEveryDefaultService(t *testing.T) {
	s := NewSupervisor()
	for _, cfg := range defaultServiceConfigs() {
		s.processes[cfg.Name] = sleeper(t, cfg.Name)
	}
	for name, running := range s.GetStatus() {
		if !running {
			t.Fatalf("%s is not running before StopAll", name)
		}
	}
	s.StopAll()
	for name, running := range s.GetStatus() {
		if running {
			t.Errorf("%s still runs after StopAll", name)
		}
	}
}

func TestSupervisor_restartingAnUnknownServiceIsAnError(t *testing.T) {
	if err := NewSupervisor().RestartService("nope"); err == nil || !strings.Contains(err.Error(), "nope") {
		t.Errorf("got %v, want an error naming the service", err)
	}
}

// Every service runs as its own unprivileged user with an absolute binary,
// a seccomp allowlist of its own, and a log under the logs directory.
func TestDefaultServiceConfigs_eachServiceIsolated(t *testing.T) {
	uids := map[uint32]string{}
	for _, cfg := range defaultServiceConfigs() {
		if cfg.User == 0 || cfg.Group == 0 {
			t.Errorf("%s runs as root (uid %d gid %d)", cfg.Name, cfg.User, cfg.Group)
		}
		if other, dup := uids[cfg.User]; dup {
			t.Errorf("%s and %s share uid %d", cfg.Name, other, cfg.User)
		}
		uids[cfg.User] = cfg.Name
		if !filepath.IsAbs(cfg.Binary) {
			t.Errorf("%s binary %q is not absolute, so PATH would choose it", cfg.Name, cfg.Binary)
		}
		if _, ok := ServiceSyscalls[cfg.Name]; !ok {
			t.Errorf("%s has no service syscall list, so its profile is the base set only", cfg.Name)
		}
		if !strings.HasPrefix(cfg.LogFile, "/opt/orama/.orama/logs/") {
			t.Errorf("%s logs to %q, outside the logs directory", cfg.Name, cfg.LogFile)
		}
	}
}
