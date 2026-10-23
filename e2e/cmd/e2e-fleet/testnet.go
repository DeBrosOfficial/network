package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/Orama-Network/orama/e2e/harness/config"
	"github.com/Orama-Network/orama/e2e/harness/fleet"
	"github.com/Orama-Network/orama/e2e/harness/oramacli"
)

// refuseTestnetEnv refuses cmd, which provisions, changes or destroys
// servers or cloud resources, when E2E_FLEET_STATE names a testnet state:
// the existing testnet cluster is only ever tested. The state is read
// without the run guards on purpose (it is only asked what its target is);
// a state that cannot be read is not a testnet state, and cmd's own loading
// refuses it or never looks at it.
func refuseTestnetEnv(cmd string) error {
	path := strings.TrimSpace(os.Getenv(config.EnvState))
	if path == "" {
		return nil
	}
	st, err := fleet.Load(path)
	if err != nil {
		return nil
	}
	return refuseTestnetState(cmd, st)
}

// refuseTestnetState is the refusal for a loaded state.
func refuseTestnetState(cmd string, st *fleet.State) error {
	if st.IsTestnet() {
		return fmt.Errorf("`e2e-fleet %s` is not available on the testnet target: the existing testnet cluster is only tested (`e2e-fleet test --stage N`), never provisioned, changed, swept or destroyed", cmd)
	}
	return nil
}

// signInTestnetOperator makes the operator's wallet session, in the
// namespace the operator owns, the testnet HOME's current credential, as
// provisioning does for a fleet it builds. Tests create namespaces and call
// the operator routes as the operator: the gateway refuses the first to any
// credential but a wallet session (a service account the owner's own work
// left current: "creating a namespace requires a signed-in wallet"), and the
// second to a session holding no admin grant (the lobby).
func signInTestnetOperator(ctx context.Context, st *fleet.State) error {
	cli := oramacli.ForState(st, nil)
	if err := cli.Check(); err != nil {
		return err
	}
	res, err := cli.Run(ctx, "auth", "login", "--namespace", st.OperatorNamespace)
	if err != nil {
		return fmt.Errorf("failed to sign the operator in to testnet: %w", err)
	}
	if res.Exit != 0 {
		return fmt.Errorf("orama auth login exited %d signing the operator in to testnet (is the dev RootWallet agent running? ~/orama-testnet-handoff/rwdev.sh): %s", res.Exit, strings.TrimSpace(res.Stderr))
	}
	return nil
}
