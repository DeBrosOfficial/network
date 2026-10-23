package namespacecmd

import (
	cli "github.com/Orama-Network/orama/core/cmd/orama/internal"
	"github.com/Orama-Network/orama/core/cmd/orama/internal/printer"
	"github.com/spf13/cobra"
)

var attestationPolicyCmd = &cobra.Command{
	Use:   "attestation-policy",
	Short: "Show or set which app builds may call a namespace (device attestation)",
	Long: `With no flags, show the namespace's attestation policy. With flags, change them; a
flag left out keeps its value.

A namespace that attests admits to its functions and data plane only the genuine
build of its app on a genuine device: iOS through App Attest, Android (GrapheneOS
included) through hardware key attestation. A script holding a valid session and
the app's API key is refused.

  --mode off|monitor|enforce   monitor refuses nothing and counts what enforce
                               would refuse (orama status traffic); enforce refuses
  --ios-team-id, --ios-bundle-id   the iOS App ID accepted
  --allow-development          accept App Attest's development environment (devnet)
  --min-ios-version            the oldest CFBundleVersion accepted (iOS 27 and later
                               sign it; older iOS is accepted with the version unknown)
  --android-package            the Android package accepted
  --android-signing-digest     a lowercase-hex SHA-256 of an accepted signing
                               certificate; repeat for several (replaces the list)
  --allow-grapheneos           accept GrapheneOS beside stock verified boot
  --min-android-version-code   the oldest versionCode accepted
  --max-patch-age-months       refuse an Android OS patch older than this (0: no limit)
  --reattest-interval          how long an Android attestation lasts (default 168h)

Changing the policy needs write access to the namespace.

  orama namespace attestation-policy --namespace anchat --mode monitor \
    --android-package chat.anchat --android-signing-digest <sha256> --allow-grapheneos`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ns, _ := cmd.Flags().GetString("namespace")
		return cli.NamespaceAttestationPolicy(printer.For(cmd), ns, attestationChange(cmd))
	},
}

// attestationChange is the flags the caller set, and only those.
func attestationChange(cmd *cobra.Command) cli.AttestationPolicyChange {
	f := cmd.Flags()
	str := func(name string) *string {
		if !f.Changed(name) {
			return nil
		}
		v, _ := f.GetString(name)
		return &v
	}
	boolean := func(name string) *bool {
		if !f.Changed(name) {
			return nil
		}
		v, _ := f.GetBool(name)
		return &v
	}
	var change cli.AttestationPolicyChange
	change.Mode = str("mode")
	change.IOSTeamID = str("ios-team-id")
	change.IOSBundleID = str("ios-bundle-id")
	change.AllowDevelopment = boolean("allow-development")
	change.MinIOSVersion = str("min-ios-version")
	change.AndroidPackage = str("android-package")
	change.AllowGrapheneOS = boolean("allow-grapheneos")
	change.ReattestInterval = str("reattest-interval")
	if f.Changed("android-signing-digest") {
		v, _ := f.GetStringArray("android-signing-digest")
		change.SigningDigests = &v
	}
	if f.Changed("min-android-version-code") {
		v, _ := f.GetInt64("min-android-version-code")
		change.MinVersionCode = &v
	}
	if f.Changed("max-patch-age-months") {
		v, _ := f.GetInt("max-patch-age-months")
		change.MaxPatchAgeMonths = &v
	}
	return change
}

func init() {
	f := attestationPolicyCmd.Flags()
	f.String("namespace", "", "Namespace name")
	f.String("mode", "", "off | monitor | enforce")
	f.String("ios-team-id", "", "Apple team id of the iOS app")
	f.String("ios-bundle-id", "", "Bundle id of the iOS app")
	f.Bool("allow-development", false, "Accept App Attest's development environment (devnet)")
	f.String("min-ios-version", "", "Oldest CFBundleVersion accepted (\"\" for none)")
	f.String("android-package", "", "Package name of the Android app")
	f.StringArray("android-signing-digest", nil, "Lowercase-hex SHA-256 of an accepted signing certificate; repeat for several")
	f.Bool("allow-grapheneos", false, "Accept GrapheneOS beside stock verified boot")
	f.Int64("min-android-version-code", 0, "Oldest versionCode accepted (0 for none)")
	f.Int("max-patch-age-months", 0, "Refuse an Android OS patch older than this many months (0: no limit)")
	f.String("reattest-interval", "", "How long an Android attestation lasts, e.g. 168h")
	Cmd.AddCommand(attestationPolicyCmd)
}
