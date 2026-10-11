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

// TestNodes_aPageOfTheLargestRecordsStopsAtItsByteBoundUnderTheQueryGasLimit: 100 records of 64
// maximum-length endpoints and 64 bindings (the largest the genesis parameters allow) read more
// than the node's query-gas-limit (2,000,000). On the caller's meter a page ends at
// MaxNodesPageBytes instead, under the limit, and its next_key resumes at the next node, so every
// node is read without the query ever running out of gas.
func TestNodes_aPageOfTheLargestRecordsStopsAtItsByteBoundUnderTheQueryGasLimit(t *testing.T) {
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

	q := keeper.NewQueryServerImpl(f.Keeper)
	seen := map[string]bool{}
	var key []byte
	for pages := 0; ; pages++ {
		require.Less(t, pages, keeper.MaxNodesPerPage, "the pages never reach the last node")
		bounded := f.Ctx.WithGasMeter(storetypes.NewGasMeter(queryGasLimit))
		res, err := q.Nodes(bounded, &types.QueryNodesRequest{Pagination: &query.PageRequest{Key: key}})
		require.NoError(t, err, "a page runs on the caller's meter and stays under the query gas limit")
		require.NotEmpty(t, res.Nodes)
		require.Less(t, len(res.Nodes), keeper.MaxNodesPerPage, "a page of the largest records ends at its byte bound")
		for _, n := range res.Nodes {
			require.False(t, seen[n.NodeId], "node %s on two pages", n.NodeId)
			seen[n.NodeId] = true
		}
		key = res.Pagination.NextKey
		if len(key) == 0 {
			break
		}
	}
	require.Len(t, seen, keeper.MaxNodesPerPage, "every node is read once")
}

// The node list has one order; a reverse page is refused rather than answered in id order.
func TestNodes_aReversePageIsRefused(t *testing.T) {
	f := newTestFixture(t)
	_, err := keeper.NewQueryServerImpl(f.Keeper).Nodes(f.Ctx, &types.QueryNodesRequest{Pagination: &query.PageRequest{Key: []byte{}, Reverse: true}})
	require.Error(t, err, "the node list is read in id order only")
}
