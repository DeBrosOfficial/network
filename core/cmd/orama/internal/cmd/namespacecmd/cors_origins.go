package namespacecmd

import (
	cli "github.com/Orama-Network/orama/core/cmd/orama/internal"
	"github.com/Orama-Network/orama/core/cmd/orama/internal/clierr"
	"github.com/Orama-Network/orama/core/cmd/orama/internal/printer"
	"github.com/spf13/cobra"
)

var corsOriginsCmd = &cobra.Command{
	Use:   "cors-origins",
	Short: "Show or set the browser origins a namespace's apps are served from",
	Long: `With no flags, show the origins a browser page may call the namespace from,
beyond the ones every gateway admits: the base domain and the names under it,
localhost, any *.localhost name, the loopback addresses, and the Tauri webviews
(tauri://localhost on macOS, Linux and iOS; http(s)://tauri.localhost on
Windows and Android).

--set replaces the list (repeat it for several origins, at most 20); --clear
empties it. An origin is scheme://host[:port], http, https or tauri, with no
path and no wildcard. The list applies to requests addressed to the
namespace's host (ns-<name>.<base domain>), at once on the gateway that
recorded it and within 30 seconds on every other.

Changing the list needs write access to the namespace (an owner or an admin).

  orama namespace cors-origins --namespace anchat --set https://app.anchat.chat --set https://anchat.chat`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ns, _ := cmd.Flags().GetString("namespace")
		clear, _ := cmd.Flags().GetBool("clear")
		set, _ := cmd.Flags().GetStringArray("set")
		switch {
		case clear && len(set) > 0:
			return clierr.Usage("--set and --clear are each the whole list; give one of them")
		case clear:
			return cli.NamespaceCORSOrigins(printer.For(cmd), ns, []string{})
		case len(set) > 0:
			return cli.NamespaceCORSOrigins(printer.For(cmd), ns, set)
		}
		return cli.NamespaceCORSOrigins(printer.For(cmd), ns, nil)
	},
}

func init() {
	corsOriginsCmd.Flags().String("namespace", "", "Namespace name")
	corsOriginsCmd.Flags().StringArray("set", nil, "An allowed origin; repeat for several (replaces the list)")
	corsOriginsCmd.Flags().Bool("clear", false, "Remove every allowed origin")
	Cmd.AddCommand(corsOriginsCmd)
}
