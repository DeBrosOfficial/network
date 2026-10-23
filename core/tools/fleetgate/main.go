// Command fleetgate is the release gate of core/scripts/release.sh: it exits
// 0 only when the fleet e2e report in -report is signed by a runner key
// compiled into this tool (core/pkg/releasepub/runner-keys.txt), tested
// exactly -commit with a clean working tree, and passed.
//
//	go run ./tools/fleetgate -report <artifact dir> -commit <sha>
//
// `orama maint release cut` applies the same check (releasepub.FleetGate) to
// the commit its archives were built from.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/Orama-Network/orama/core/pkg/releasepub"
)

func main() {
	fs := flag.NewFlagSet("fleetgate", flag.ContinueOnError)
	report := fs.String("report", "", "the artifact directory of the fleet e2e run (holds report.json and report.attestation.json)")
	commit := fs.String("commit", "", "the full SHA being released")
	if err := fs.Parse(os.Args[1:]); err != nil {
		os.Exit(2)
	}
	if err := run(*report, *commit); err != nil {
		fmt.Fprintln(os.Stderr, "fleetgate:", err)
		os.Exit(1)
	}
}

func run(report, commit string) error {
	keys, err := releasepub.RunnerKeys()
	if err != nil {
		return err
	}
	s, err := releasepub.FleetGate{ReportDir: report, RunnerKeys: keys}.Check(context.Background(), commit)
	if err != nil {
		return err
	}
	fmt.Printf("fleet e2e run %s passed on %s (report sha256 %s)\n", s.RunID, s.Commit, s.ReportSHA256)
	return nil
}
