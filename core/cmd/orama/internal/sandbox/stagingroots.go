package sandbox

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Orama-Network/orama/core/cmd/orama/internal"
	"github.com/Orama-Network/orama/core/pkg/clustertrust"
)

// Sandbox nodes get their certificates from Let's Encrypt staging
// (sandboxACMECA), which no system trusts. The CLI trusts staging's roots for
// the sandbox domain only, through the environment's CA file
// (website/src/docs/contributor/sandbox.mdx). The roots are pinned in
// pkg/clustertrust.

// stagingRootsFile names the roots written next to the sandbox state.
const stagingRootsFile = "letsencrypt-staging-roots.pem"

// writeStagingRoots writes the pinned staging roots to the sandboxes
// directory and returns the file's path.
func writeStagingRoots() (string, error) {
	roots, err := clustertrust.StagingRoots()
	if err != nil {
		return "", err
	}
	dir, err := sandboxesDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, stagingRootsFile)
	if err := os.WriteFile(path, roots, 0o600); err != nil {
		return "", fmt.Errorf("write the Let's Encrypt staging roots to %s: %w", path, err)
	}
	return path, nil
}

// registerEnvironment records the sandbox as the sandbox environment, trusting
// the staging roots for its domain only, and makes it the active environment.
func registerEnvironment(cfg *Config, state *SandboxState) error {
	gatewayURL := "https://" + cfg.Domain
	desc := fmt.Sprintf("Sandbox cluster: %s (%s)", state.Name, cfg.Domain)
	if err := cli.AddEnvironment(sandboxEnvironment, gatewayURL, desc); err != nil {
		return fmt.Errorf("register the %s environment: %w", sandboxEnvironment, err)
	}
	caFile, err := writeStagingRoots()
	if err != nil {
		return err
	}
	if err := cli.SetEnvironmentCA(sandboxEnvironment, caFile); err != nil {
		return fmt.Errorf("trust the Let's Encrypt staging roots for %s: %w", cfg.Domain, err)
	}
	if err := cli.SwitchEnvironment(sandboxEnvironment); err != nil {
		return fmt.Errorf("switch to the %s environment: %w", sandboxEnvironment, err)
	}
	return nil
}
