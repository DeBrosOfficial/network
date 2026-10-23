package process

import (
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap"
)

func trustManager(t *testing.T, trustFile string) (*Manager, *recordingStager) {
	t.Helper()
	st := newRecordingStager()
	return NewManager(zap.NewNop(), Config{Stager: st, BaseDomain: "dbrs.space", ClusterTrustFile: trustFile}), st
}

func stagedEnv(t *testing.T, m *Manager, st *recordingStager) map[string]string {
	t.Helper()
	if err := m.writeEnvFile(testDeployment(), "orama-deploy-acme-api"); err != nil {
		t.Fatalf("writeEnvFile: %v", err)
	}
	return systemdReadEnvFile(st.env["acme-api"])
}

// On a cluster whose certificates need roots beyond the system's, every
// runtime the platform runs is pointed at them, so the app's default TLS
// client verifies ORAMA_GATEWAY_URL.
func TestWriteEnvFile_pointsTheRuntimeAtTheClusterRoots(t *testing.T) {
	trust := filepath.Join(t.TempDir(), "cluster-ca.pem")
	if err := os.WriteFile(trust, []byte("roots"), 0o644); err != nil {
		t.Fatal(err)
	}
	m, st := trustManager(t, trust)
	got := stagedEnv(t, m, st)
	for _, key := range ClusterTrustEnvKeys {
		if got[key] != trust {
			t.Errorf("%s = %q, want %q", key, got[key], trust)
		}
	}
}

// Where install removed the file (the system roots verify the cluster), the
// runtime keeps its defaults; the file is looked up at each start, so a cluster
// moved between ACME directories needs no gateway restart.
func TestWriteEnvFile_noClusterRootsNoTrustVariables(t *testing.T) {
	trust := filepath.Join(t.TempDir(), "cluster-ca.pem")
	m, st := trustManager(t, trust)
	got := stagedEnv(t, m, st)
	for _, key := range ClusterTrustEnvKeys {
		if v, ok := got[key]; ok {
			t.Errorf("%s = %q with no cluster roots on the node", key, v)
		}
	}

	if err := os.WriteFile(trust, []byte("roots"), 0o644); err != nil {
		t.Fatal(err)
	}
	got = stagedEnv(t, m, st)
	for _, key := range ClusterTrustEnvKeys {
		if got[key] != trust {
			t.Errorf("after the roots appeared: %s = %q, want %q", key, got[key], trust)
		}
	}
}

// A trust file that cannot be checked stops the start rather than running the
// app into a TLS failure on its first gateway call.
func TestWriteEnvFile_refusesAnUncheckableTrustFile(t *testing.T) {
	notADir := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(notADir, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	m, st := trustManager(t, filepath.Join(notADir, "cluster-ca.pem"))
	if err := m.writeEnvFile(testDeployment(), "orama-deploy-acme-api"); err == nil {
		t.Fatal("started with a trust file whose presence could not be told")
	}
	if _, ok := st.env["acme-api"]; ok {
		t.Error("an environment was staged anyway")
	}
}
