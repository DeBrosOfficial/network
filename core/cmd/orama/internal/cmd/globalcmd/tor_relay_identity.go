package globalcmd

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/DeBrosOfficial/network/cmd/orama/internal/clierr"
	"github.com/DeBrosOfficial/network/cmd/orama/internal/cmdmeta"
	"github.com/DeBrosOfficial/network/pkg/constants"
	"github.com/DeBrosOfficial/network/pkg/globalbind"
	"github.com/DeBrosOfficial/network/pkg/tornet"
	"github.com/spf13/cobra"
)

var relayIdentityFlags struct {
	chainID, operator, nodeID, home string
}

var relayIdentityCmd = &cobra.Command{
	Use:   "relay-identity",
	Short: "Sign what registering this node's Tor relay on the chain needs (run as root, on the relay's node)",
	Long: `Print the relay's RSA fingerprint, the binding that makes its ed25519 master identity the
operator's service key "relay", and the cross-certificate that ties the fingerprint to the node.
x/relay pays the relay that a directory authority's vote names by that ed25519 identity and that
fingerprint, so the chain needs both, signed by the relay's own key.

The signatures are made here with the relay's key in its DataDirectory
(` + constants.GlobalTorRelayHome + `); the key is never printed or copied. 'orama setup' runs this on the
node and sends the registration; nothing is submitted by this command.`,
	Args: cobra.NoArgs,
	RunE: runRelayIdentity,
}

func init() {
	f := relayIdentityCmd.Flags()
	f.StringVar(&relayIdentityFlags.chainID, "chain-id", "", "Chain id the binding is for [required]")
	f.StringVar(&relayIdentityFlags.operator, "operator", "", "Operator account (orama1...) [required]")
	f.StringVar(&relayIdentityFlags.nodeID, "node-id", "", "The chain id of the node the relay runs on [required]")
	f.StringVar(&relayIdentityFlags.home, "home", constants.GlobalTorRelayHome, "The relay's tor DataDirectory")
	torCmd.AddCommand(cmdmeta.MarkNodeLocal(relayIdentityCmd))
}

func runRelayIdentity(cmd *cobra.Command, _ []string) error {
	f := relayIdentityFlags
	if f.chainID == "" || f.operator == "" || f.nodeID == "" {
		return clierr.Usage("--chain-id, --operator and --node-id are required")
	}
	if err := clierr.RequireRoot("reading the relay's keys"); err != nil {
		return err
	}
	doc, err := relayIdentityDoc(f.home, f.chainID, f.operator, f.nodeID)
	if err != nil {
		return err
	}
	_, err = cmd.OutOrStdout().Write(doc)
	return err
}

// relayIdentityDoc reads the relay keys of the DataDirectory home and returns the signed identity as
// indented JSON.
func relayIdentityDoc(home, chainID, operator, nodeID string) ([]byte, error) {
	keys, err := tornet.ReadRelayKeys(home)
	if err != nil {
		return nil, clierr.Failure("%v", err)
	}
	identity, err := globalbind.NewRelayIdentity(keys.Ed25519Expanded, keys.Fingerprint, chainID, operator, nodeID)
	if err != nil {
		return nil, clierr.Usage("%v", err)
	}
	if err := checkEd25519ID(keys, identity.Binding.Pubkey); err != nil {
		return nil, clierr.Failure("%v", err)
	}
	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	enc.SetIndent("", "  ")
	if err := enc.Encode(identity); err != nil {
		return nil, clierr.Failure("write the relay identity: %w", err)
	}
	return out.Bytes(), nil
}

// checkEd25519ID refuses a key file whose public key is not the identity tor wrote beside it: the
// consensus names the relay by the written one, so a binding of another key would never match a vote.
func checkEd25519ID(keys tornet.RelayKeys, pub []byte) error {
	if keys.Ed25519ID == "" {
		return nil
	}
	if want := base64.RawStdEncoding.EncodeToString(pub); want != keys.Ed25519ID {
		return fmt.Errorf("the relay's ed25519 secret key is for %s but tor wrote the identity %s", want, keys.Ed25519ID)
	}
	return nil
}
