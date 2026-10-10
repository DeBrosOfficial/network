package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"os/exec"

	"github.com/DeBrosOfficial/network/cmd/orama/internal/clierr"
	"github.com/DeBrosOfficial/network/cmd/orama/internal/utils"
	"github.com/DeBrosOfficial/network/pkg/constants"
	"github.com/DeBrosOfficial/network/pkg/nodehealth"
	"github.com/DeBrosOfficial/network/pkg/rqlite"
	"github.com/DeBrosOfficial/network/pkg/systemd"
)

// timerUnits are the systemd operations a restart makes on the auto-update timer.
type timerUnits struct {
	// active reports whether the timer is installed and running.
	active func(unit string) bool
	stop   func(unit string) error
	start  func(unit string) error
}

// systemdTimerUnits acts on this machine's systemd.
var systemdTimerUnits = timerUnits{
	active: func(unit string) bool {
		if !utils.ServiceUnitExists(unit) {
			return false
		}
		running, _ := utils.IsServiceActive(unit)
		return running
	},
	stop:  func(unit string) error { return exec.Command("systemctl", "stop", unit).Run() },
	start: func(unit string) error { return exec.Command("systemctl", "start", unit).Run() },
}

// restartWithAutoUpdatePaused runs restart with the auto-update timer stopped,
// and starts it again afterwards when it was running before.
//
// The agent reads the cluster's settings from this node's own rqlite. A timer
// that falls due while the restart has the node down runs the agent against a
// half-stopped node, which fails ("connection refused") and leaves a failed
// unit in `orama status` until the next run, up to 15 minutes later.
//
// The timer's OnBootSec deadline is long past on a running host, so starting it
// makes it fire at once (within its randomized delay). It is therefore started
// only after serving reports the node answering: the agent's first run is
// against a node that is up. A timer that was stopped before the restart is
// left stopped, and a timer is started again whatever became of the restart, so
// a failed restart never leaves automatic updates off until the next boot.
func restartWithAutoUpdatePaused(units timerUnits, restart, serving func() error) error {
	const unit = systemd.AutoUpdateTimerName
	if !units.active(unit) {
		return restart()
	}
	if err := units.stop(unit); err != nil {
		return clierr.Failure("could not stop %s before the restart, so it could run the update agent against a node that is down: %v", unit, err)
	}
	fmt.Printf("  Stopped %s for the restart\n", unit)

	err := restart()
	if err == nil {
		err = serving()
	}
	if startErr := units.start(unit); startErr != nil {
		return errors.Join(err, clierr.Failure("could not start %s again: %v; start it with: systemctl start %s", unit, startErr, unit))
	}
	fmt.Printf("  Started %s\n", unit)
	return err
}

// waitNodeServing waits until this node answers: its index rqlite has a leader
// and its gateway serves. did is what was done to the services, for the message.
func waitNodeServing(did string) error {
	indexRQLite, err := rqlite.LocalNodeEndpoint()
	if err != nil {
		return clierr.Failure("Services were %s but the index RQLite cannot be addressed: %v", did, err)
	}
	if err := nodehealth.WaitReady(context.Background(), nodehealth.Target{
		RQLite:      indexRQLite,
		GatewayBase: fmt.Sprintf("http://localhost:%d", constants.GatewayAPIPort),
	}, nodehealth.Options{Budget: startReadyBudget}); err != nil {
		return clierr.Failure("Services were %s but the node is not serving: %v", did, err)
	}
	return nil
}
