// Package status reports what the Orama node on this machine is running.
package status

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/Orama-Network/orama/core/cmd/orama/internal/printer"
	"github.com/Orama-Network/orama/core/cmd/orama/internal/utils"
	"github.com/Orama-Network/orama/core/pkg/systemd"
)

// turnUnit is the shared TURN unit's name as GetReconciledServices lists it.
var turnUnit = strings.TrimSuffix(systemd.HostTURNServiceName, ".service")

// oramaDir is where a node keeps its configuration and data.
const oramaDir = "/opt/orama/.orama"

// Handle prints the state of this node's services.
//
// It used to probe a fixed list — orama-ipfs, orama-ipfs-cluster, orama-olric,
// orama-vault, orama-node — four of which are systemd.LeftoverHostUnits that
// the installer deliberately disables, because IndexSupervisor runs
// orama-namespace-*@index instead. So a correctly installed node printed four
// "Inactive" rows and did not list the units that were actually running.
//
// GetProductionServices is what every other command uses to decide what this
// node runs: the global units that are not leftovers, plus the namespace
// instances discovered from the data directory.
//
// The reconciler-owned host units (GetReconciledServices: the shared TURN
// unit) are listed after them. One that its reconciler does not want running
// is shown "inactive (not needed)" and is not counted as a service that
// should run.
func Handle(out *printer.Printer) error {
	services := utils.GetProductionServices()
	sort.Strings(services)
	reconciled := utils.GetReconciledServices()

	rows := make([][]string, 0, len(services)+len(reconciled))
	running, expected := 0, 0
	for _, svc := range services {
		state := "inactive"
		if active, _ := utils.IsServiceActive(svc); active {
			state = "active"
			running++
		}
		expected++
		rows = append(rows, []string{svc, state, describe(svc)})
	}
	for _, svc := range reconciled {
		active, _ := utils.IsServiceActive(svc.Name)
		state := reconciledState(svc, active)
		if active {
			running++
		}
		if svc.Wanted || active {
			expected++
		}
		rows = append(rows, []string{svc.Name, state, describe(svc.Name)})
	}

	if len(rows) == 0 {
		out.Printf("No Orama services are installed on this machine.\n")
		out.Printf("Install one with: sudo orama maint node install --vps-ip <ip>\n")
		return nil
	}

	if err := out.Table([]string{"SERVICE", "STATE", "WHAT IT IS"}, rows); err != nil {
		return err
	}

	out.Printf("\n%d of %d running\n", running, expected)
	if _, err := os.Stat(oramaDir); err != nil {
		out.Printf("\n%s is missing — this node's configuration and data are not there.\n", oramaDir)
	}
	out.Printf("\nLogs: orama node logs <service>\n")
	out.Printf("Fleet health from your own machine: orama status\n")
	return nil
}

// reconciledState is the STATE of a reconciler-owned unit: "inactive (not
// needed)" when its reconciler does not want it running, so an operator does
// not read the TURN unit of a node with no TURN allocation as down.
func reconciledState(svc utils.ReconciledService, active bool) string {
	switch {
	case active:
		return "active"
	case svc.Wanted:
		return "inactive"
	default:
		return "inactive (not needed)"
	}
}

// describe names what a unit is, for an operator who has not memorised them.
//
// Namespace units are template instances — orama-namespace-rqlite@anchat —
// so they are described by their role and their tenant rather than looked up.
func describe(service string) string {
	if role, namespace, ok := splitNamespaceUnit(service); ok {
		return fmt.Sprintf("%s for namespace %q", role, namespace)
	}
	switch service {
	case "orama-node":
		return "the supervisor: it runs the index stack and the gateway"
	case turnUnit:
		return "the shared TURN relay for every namespace on this host; runs while the node holds a TURN allocation"
	default:
		return ""
	}
}

// splitNamespaceUnit takes orama-namespace-<role>@<namespace>[.timer] apart.
func splitNamespaceUnit(service string) (role, namespace string, ok bool) {
	const prefix = "orama-namespace-"
	if !strings.HasPrefix(service, prefix) {
		return "", "", false
	}
	rest := strings.TrimPrefix(service, prefix)
	role, namespace, found := strings.Cut(rest, "@")
	// A timer-driven job is listed by its timer (orama-namespace-ipfs-gc@<ns>.timer).
	if ns, isTimer := strings.CutSuffix(namespace, ".timer"); isTimer {
		role, namespace = role+" timer", ns
	}
	if !found || role == "" || namespace == "" {
		return "", "", false
	}
	return role, namespace, true
}
