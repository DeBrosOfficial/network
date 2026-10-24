package functions

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

var (
	deleteForce   bool
	deletePurge   bool
	deleteVersion int
)

// DeleteCmd deletes a deployed function.
var DeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a deployed function",
	Long: `Deletes a function from the Orama Network: every version, or only the one
named with --version. A deleted version is never invoked again and cannot be
enabled or rolled back to, but its row stays and 'orama function list --all'
still shows it.

--purge removes the rows for good, with the function's environment variables,
triggers, invocation history, logs and jobs. Purging takes the namespace's
owner or an admin. The namespace's secrets are shared by every function and
stay. Purging the current version leaves the function with none until
'orama function rollback'.`,
	Args: functionNameArgs(1),
	RunE: runDelete,
}

func init() {
	DeleteCmd.Flags().BoolVarP(&deleteForce, "force", "f", false, "Skip confirmation prompt")
	DeleteCmd.Flags().BoolVar(&deletePurge, "purge", false, "Remove the function's rows, triggers and history for good (owner or admin)")
	DeleteCmd.Flags().IntVar(&deleteVersion, "version", 0, "Delete only this version")
}

func runDelete(cmd *cobra.Command, args []string) error {
	name := args[0]
	if deleteVersion < 0 {
		return fmt.Errorf("--version must be a version number, not %d", deleteVersion)
	}

	target := strconv.Quote(name)
	if deleteVersion > 0 {
		target = fmt.Sprintf("version %d of %q", deleteVersion, name)
	}
	if !deleteForce {
		verb := "delete"
		if deletePurge {
			verb = "purge"
		}
		fmt.Printf("Are you sure you want to %s function %s? This cannot be undone. [y/N] ", verb, target)
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	endpoint := "/v1/functions/" + name
	if deleteVersion > 0 {
		endpoint += "@" + strconv.Itoa(deleteVersion)
	}
	if deletePurge {
		endpoint += "?purge=true"
	}
	result, err := apiDelete(endpoint)
	if err != nil {
		return err
	}

	if msg, ok := result["message"]; ok {
		fmt.Println(msg)
	} else {
		fmt.Printf("Function %s deleted.\n", target)
	}

	return nil
}
