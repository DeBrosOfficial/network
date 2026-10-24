package provision

import (
	"fmt"

	"github.com/Orama-Network/orama/e2e/harness/fleet"
)

// refuseTestnet fails an operation that would create, change or destroy
// servers when st is the testnet target: that cluster is only ever tested.
func refuseTestnet(op string, st *fleet.State) error {
	if st != nil && st.IsTestnet() {
		return fmt.Errorf("%s is not available on the testnet target: the existing testnet cluster is only tested, never provisioned, changed or destroyed", op)
	}
	return nil
}
