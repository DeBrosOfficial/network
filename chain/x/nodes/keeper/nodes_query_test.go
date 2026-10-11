package keeper_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
	"github.com/cosmos/cosmos-sdk/types/query"

	"github.com/DeBrosOfficial/network/chain/x/nodes/keeper"
	"github.com/DeBrosOfficial/network/chain/x/nodes/types"
)

func TestNodes_pagesEveryNodeInIDOrder(t *testing.T) {
	f := newTestFixture(t)
	q := keeper.NewQueryServerImpl(f.Keeper)
	op := newAccount(t)
	f.fund(op, 100)
	f.registerOperator(t, op)
	for _, id := range []string{"node-d", "node-a", "node-c", "node-b", "node-e"} {
		f.registerNode(t, op, newAccount(t), id, []types.Role{types.RoleStorage}, []types.Binding{edBinding(t, testChainID, op.String(), "ipfs")})
	}

	var got []string
	var key []byte
	for pages := 0; pages < 10; pages++ {
		res, err := q.Nodes(f.Ctx, &types.QueryNodesRequest{Pagination: &query.PageRequest{Key: key, Limit: 2}})
		require.NoError(t, err)
		require.LessOrEqual(t, len(res.Nodes), 2)
		for _, node := range res.Nodes {
			got = append(got, node.NodeId)
			require.Equal(t, op.String(), node.Operator)
			require.Equal(t, []types.Role{types.RoleStorage}, node.Roles)
		}
		if key = res.Pagination.NextKey; key == nil {
			break
		}
	}
	require.Equal(t, []string{"node-a", "node-b", "node-c", "node-d", "node-e"}, got, "every node once, in id order")
}

func TestNodes_listsRetiredNodesWithTheirStatus(t *testing.T) {
	f := newTestFixture(t)
	q := keeper.NewQueryServerImpl(f.Keeper)
	op, hot := newAccount(t), newAccount(t)
	f.fund(op, 100)
	f.registerOperator(t, op)
	f.registerNode(t, op, hot, "node-1", []types.Role{types.RoleStorage}, []types.Binding{edBinding(t, testChainID, op.String(), "ipfs")})
	_, err := f.Msg.RetireNode(f.Ctx, &types.MsgRetireNode{Operator: op.String(), NodeId: "node-1"})
	require.NoError(t, err)

	res, err := q.Nodes(f.Ctx, &types.QueryNodesRequest{})
	require.NoError(t, err)
	require.Len(t, res.Nodes, 1)
	require.Equal(t, types.NodeStatusRetired, res.Nodes[0].Status)
}

func TestNodes_emptyListIsEmptyNotAnError(t *testing.T) {
	f := newTestFixture(t)
	res, err := keeper.NewQueryServerImpl(f.Keeper).Nodes(f.Ctx, &types.QueryNodesRequest{})
	require.NoError(t, err)
	require.NotNil(t, res.Nodes)
	require.Empty(t, res.Nodes)
}

func TestNodes_aPageIsBoundedAndPagesByKey(t *testing.T) {
	f := newTestFixture(t)
	q := keeper.NewQueryServerImpl(f.Keeper)

	_, err := q.Nodes(f.Ctx, &types.QueryNodesRequest{Pagination: &query.PageRequest{Offset: 5}})
	require.Error(t, err, "an offset skips keys one by one, so the query refuses it")

	for i := 0; i < keeper.MaxNodesPerPage+50; i++ {
		id := fmt.Sprintf("node-%03d", i)
		require.NoError(t, f.Keeper.Nodes.Set(f.Ctx, id, types.Node{NodeId: id, Status: types.NodeStatusActive}))
	}
	res, err := q.Nodes(f.Ctx, &types.QueryNodesRequest{Pagination: &query.PageRequest{Limit: 10 * keeper.MaxNodesPerPage, CountTotal: true}})
	require.NoError(t, err)
	require.Len(t, res.Nodes, keeper.MaxNodesPerPage, "a page holds at most 100 nodes")
	require.NotNil(t, res.Pagination.NextKey)
	require.Zero(t, res.Pagination.Total, "a total would walk every node, so it is not computed")
}

// TestNodes_aFullPageOfTheLargestRecordsAnswersUnderTheQueryGasLimit pins why the query runs on a
// meter of its own: 100 records of 64 maximum-length endpoints and 64 bindings (the largest the
// genesis parameters allow) read more than the node's query-gas-limit (2,000,000), and the
// public route must answer them whole.
func TestNodes_aFullPageOfTheLargestRecordsAnswersUnderTheQueryGasLimit(t *testing.T) {
	const queryGasLimit = 2_000_000
	const largest = 64
	f := newTestFixture(t)
	op := newAccount(t)
	endpoints := make([]string, largest)
	bindings := make([]types.Binding, largest)
	for i := range endpoints {
		endpoints[i] = "https://" + strings.Repeat("h", types.MaxEndpointLen-len("https://:443")-3) + fmt.Sprintf("%03d", i) + ":443"
		bindings[i] = types.Binding{Service: fmt.Sprintf("service-%02d", i), KeyType: types.KeyTypeEd25519, Pubkey: make([]byte, 32), Signature: make([]byte, 64)}
	}
	for i := 0; i < keeper.MaxNodesPerPage; i++ {
		id := fmt.Sprintf("node-%03d", i)
		require.NoError(t, f.Keeper.Nodes.Set(f.Ctx, id, types.Node{NodeId: id, Operator: op.String(), Endpoints: endpoints, Bindings: bindings, Status: types.NodeStatusActive}))
	}

	bounded := f.Ctx.WithGasMeter(storetypes.NewGasMeter(queryGasLimit))
	res, err := keeper.NewQueryServerImpl(f.Keeper).Nodes(bounded, &types.QueryNodesRequest{})
	require.NoError(t, err)
	require.Len(t, res.Nodes, keeper.MaxNodesPerPage)

	// The same reads on the caller's meter run out of gas: the reason for the meter of its own.
	over := f.Ctx.WithGasMeter(storetypes.NewGasMeter(queryGasLimit))
	require.Panics(t, func() {
		for i := 0; i < keeper.MaxNodesPerPage; i++ {
			_, _ = f.Keeper.Nodes.Get(over, fmt.Sprintf("node-%03d", i))
		}
	})
}
