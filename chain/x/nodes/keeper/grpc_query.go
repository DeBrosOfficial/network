package keeper

import (
	"context"

	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/DeBrosOfficial/network/chain/x/internal/pagebound"
	"github.com/DeBrosOfficial/network/chain/x/nodes/types"
)

var _ types.QueryServer = queryServer{}

// MaxUnbondingsPerQuery bounds one NodeUnbondings response: an operator can queue many small
// unbondings on a node, and a public query must not walk them all.
const MaxUnbondingsPerQuery = 1000

// MaxNodesPerPage bounds one Nodes page. A node record carries its bindings and endpoints, up to
// 64 of each at the largest genesis parameters (about 26 KB), so a full page of such records
// reads more than the node's query gas limit allows. The page is therefore bounded by entries,
// and the query runs on a meter of its own, as Invariants does.
const MaxNodesPerPage = 100

type queryServer struct {
	Keeper
}

// NewQueryServerImpl returns an implementation of x/nodes' QueryServer.
func NewQueryServerImpl(k Keeper) types.QueryServer {
	return queryServer{Keeper: k}
}

func (q queryServer) Params(goCtx context.Context, _ *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	p, err := q.Keeper.Params.Get(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &types.QueryParamsResponse{Params: p}, nil
}

func (q queryServer) Operator(goCtx context.Context, req *types.QueryOperatorRequest) (*types.QueryOperatorResponse, error) {
	if req == nil || req.Address == "" {
		return nil, status.Error(codes.InvalidArgument, "address is required")
	}
	ctx := sdk.UnwrapSDKContext(goCtx)
	op, err := q.Keeper.GetOperator(ctx, req.Address)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}
	return &types.QueryOperatorResponse{Operator: op}, nil
}

func (q queryServer) Node(goCtx context.Context, req *types.QueryNodeRequest) (*types.QueryNodeResponse, error) {
	if req == nil || req.NodeId == "" {
		return nil, status.Error(codes.InvalidArgument, "node_id is required")
	}
	ctx := sdk.UnwrapSDKContext(goCtx)
	node, err := q.Keeper.GetNode(ctx, req.NodeId)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}
	return &types.QueryNodeResponse{Node: node}, nil
}

func (q queryServer) Cluster(goCtx context.Context, req *types.QueryClusterRequest) (*types.QueryClusterResponse, error) {
	if req == nil || req.ClusterId == "" {
		return nil, status.Error(codes.InvalidArgument, "cluster_id is required")
	}
	ctx := sdk.UnwrapSDKContext(goCtx)
	cluster, err := q.Keeper.GetCluster(ctx, req.ClusterId)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}
	return &types.QueryClusterResponse{Cluster: cluster}, nil
}

func (q queryServer) NodeUnbondings(goCtx context.Context, req *types.QueryNodeUnbondingsRequest) (*types.QueryNodeUnbondingsResponse, error) {
	if req == nil || req.NodeId == "" {
		return nil, status.Error(codes.InvalidArgument, "node_id is required")
	}
	ctx := sdk.UnwrapSDKContext(goCtx)
	entries, err := q.Keeper.nodeUnbondingsUpTo(ctx, req.NodeId, MaxUnbondingsPerQuery)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if entries == nil {
		entries = []types.UnbondingEntry{}
	}
	return &types.QueryNodeUnbondingsResponse{Unbondings: entries}, nil
}

// Nodes pages through every registered node in node id order, whatever its status. A page key
// resumes after the last node of the previous page.
func (q queryServer) Nodes(goCtx context.Context, req *types.QueryNodesRequest) (*types.QueryNodesResponse, error) {
	if req == nil {
		req = &types.QueryNodesRequest{}
	}
	pagination, err := pagebound.Clamp(req.Pagination, MaxNodesPerPage)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	ctx := sdk.UnwrapSDKContext(goCtx).WithGasMeter(storetypes.NewInfiniteGasMeter())
	nodes, page, err := query.CollectionPaginate(ctx, q.Keeper.Nodes, pagination,
		func(_ string, node types.Node) (types.Node, error) { return node, nil })
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if nodes == nil {
		nodes = []types.Node{}
	}
	return &types.QueryNodesResponse{Nodes: nodes, Pagination: page}, nil
}

func (q queryServer) Invariants(goCtx context.Context, _ *types.QueryInvariantsRequest) (*types.QueryInvariantsResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	got, err := q.Keeper.CheckInvariants(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &types.QueryInvariantsResponse{
		BalanceMatches:    got.BalanceMatches,
		ActiveRolesBonded: got.ActiveRolesBonded,
		CapacityBacked:    got.CapacityBacked,
		Detail:            got.Detail,
	}, nil
}
