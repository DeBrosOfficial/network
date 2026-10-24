package functions

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

// VersionsCmd lists all versions of a function.
var VersionsCmd = &cobra.Command{
	Use:   "versions <name>",
	Short: "List all versions of a function",
	Long: `Shows all deployed versions of a specific function. WASM SHA256 is the
SHA-256 of the WASM each version was deployed with, as uploaded, to compare
with 'shasum -a 256 function.wasm'; "-" for a version deployed before the
gateway recorded it. The WASM CID is of the sealed blob and differs on every
deploy.`,
	Args: functionNameArgs(1),
	RunE: runVersions,
}

func runVersions(cmd *cobra.Command, args []string) error {
	name := args[0]

	result, err := apiGet("/v1/functions/" + name + "/versions")
	if err != nil {
		return err
	}

	versions, ok := result["versions"].([]interface{})
	if !ok || len(versions) == 0 {
		fmt.Printf("No versions found for function %q.\n", name)
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "VERSION\tWASM SHA256\tWASM CID\tSTATUS\tCREATED")
	fmt.Fprintln(w, "-------\t-----------\t--------\t------\t-------")

	for _, v := range versions {
		ver, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		version := valNum(ver, "version")
		wasmSHA256 := valStrOrDash(ver, "wasm_sha256")
		wasmCID := valStr(ver, "wasm_cid")
		status := valStr(ver, "status")
		created := valStr(ver, "created_at")

		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n", version, wasmSHA256, wasmCID, status, created)
	}
	w.Flush()

	fmt.Printf("\nTotal: %d version(s)\n", len(versions))
	return nil
}
