package functions

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

// RollbackCmd makes an earlier version of a function its current version.
var RollbackCmd = &cobra.Command{
	Use:   "rollback <name> <version>",
	Short: "Make an earlier version of a function the current one",
	Long: `Makes <version> the function's current version. Only the current version
of a function is invoked: a deploy supersedes the version before it, and
name@N of a superseded version answers 404 FUNCTION_VERSION_SUPERSEDED.
Rolling back supersedes the version that was current, moves the function's
triggers to <version>, and serves it on every path (HTTP, WebSocket,
triggers). A deleted or pruned version cannot be rolled back to; see
'orama function versions <name>' for what is kept. The next deploy becomes
current again with the next version number.`,
	Args: func(cmd *cobra.Command, args []string) error {
		if err := functionNameArgs(2)(cmd, args); err != nil {
			return err
		}
		if v, err := strconv.Atoi(args[1]); err != nil || v <= 0 {
			return fmt.Errorf("invalid version %q: must be a positive version number", args[1])
		}
		return nil
	},
	RunE: runRollback,
}

func runRollback(cmd *cobra.Command, args []string) error {
	name, version := args[0], args[1]
	if _, err := apiPostNoBody("/v1/functions/" + name + "@" + version + "/rollback"); err != nil {
		return err
	}
	fmt.Printf("Function %q now runs version %s.\n", name, version)
	return nil
}
