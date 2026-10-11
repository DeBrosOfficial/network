package report

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/DeBrosOfficial/network/pkg/constants"
	"github.com/DeBrosOfficial/network/pkg/cosmovisor"
)

// currentPlanPath is x/upgrade's query for the plan the chain has scheduled.
const currentPlanPath = "/cosmos/upgrade/v1beta1/current_plan"

// chainHome is the cosmovisor DAEMON_HOME the staged binaries are looked for in. It is a variable
// so tests can stand in for the node.
var chainHome = constants.ChainHome

// ChainUpgrade is the software upgrade the chain has scheduled, and whether this node holds the
// binary for it. cosmovisor switches to a staged binary at Height; with none staged the chain
// halts there until one is.
type ChainUpgrade struct {
	Name   string `json:"name"`
	Height int64  `json:"height"`
	// Staged is whether upgrades/<Name>/bin holds both the chain binary and the shielded verifier
	// that rides with it.
	Staged bool `json:"staged"`
	// StageError is why Staged could not be decided (a plan name cosmovisor cannot stage, a
	// layout that could not be read). Empty when Staged is true or the binary is simply absent.
	StageError string `json:"stage_error,omitempty"`
}

// readChainUpgrade fills r.Upgrade from the chain's REST API at apiBase and the cosmovisor layout.
// A failed query is UpgradeError and leaves the RPC section responsive, like the signing queries.
func readChainUpgrade(ctx context.Context, apiBase string, r *ChainReport) {
	r.Upgrade, r.UpgradeError = nil, ""
	body, err := httpGet(ctx, apiBase+currentPlanPath)
	if err != nil {
		r.UpgradeError = shortChainError(fmt.Errorf("read the scheduled upgrade: %w", err))
		return
	}
	plan, err := parseChainUpgrade(body)
	if err != nil {
		r.UpgradeError = shortChainError(err)
		return
	}
	if plan == nil {
		return
	}
	plan.Staged, plan.StageError = upgradeStaged(chainHome, plan.Name)
	r.Upgrade = plan
}

// parseChainUpgrade reads x/upgrade's current_plan answer; nil when no upgrade is scheduled.
// Protobuf JSON carries the int64 height as a string.
func parseChainUpgrade(body []byte) (*ChainUpgrade, error) {
	var resp struct {
		Plan *struct {
			Name   string `json:"name"`
			Height int64  `json:"height,string"`
		} `json:"plan"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("the chain answered a malformed upgrade plan: %w", err)
	}
	if resp.Plan == nil || resp.Plan.Name == "" {
		return nil, nil
	}
	return &ChainUpgrade{Name: resp.Plan.Name, Height: resp.Plan.Height}, nil
}

// upgradeStaged reports whether home's cosmovisor layout holds the binary and the verifier for the
// plan name. A name cosmovisor cannot stage is reported, not treated as absent.
func upgradeStaged(home, name string) (staged bool, problem string) {
	layout := cosmovisor.Layout{Home: home, Daemon: constants.ChainDaemonName}
	binary, err := layout.UpgradeBinary(name)
	if err != nil {
		return false, shortChainError(err)
	}
	verifier := filepath.Join(filepath.Dir(binary), constants.ChainVerifierBinary)
	for _, path := range []string{binary, verifier} {
		info, err := os.Lstat(path)
		switch {
		case os.IsNotExist(err):
			return false, ""
		case err != nil:
			return false, shortChainError(fmt.Errorf("check %s: %w", path, err))
		case !info.Mode().IsRegular():
			return false, shortChainError(fmt.Errorf("%s is not a regular file", path))
		}
	}
	return true, ""
}
