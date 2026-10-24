package rqlite

import "os"

// RaftPeer represents a peer entry in RQLite's peers.json recovery file
type RaftPeer struct {
	ID       string `json:"id"`
	Address  string `json:"address"`
	NonVoter bool   `json:"non_voter"`
}

// InstanceConfig contains configuration for spawning a RQLite instance
type InstanceConfig struct {
	Namespace      string   // Namespace this instance belongs to
	NodeID         string   // Node ID where this instance runs
	HTTPPort       int      // HTTP API port
	RaftPort       int      // Raft consensus port
	HTTPAdvAddress string   // Advertised HTTP address (e.g., "192.168.1.1:10000")
	RaftAdvAddress string   // Advertised Raft address (e.g., "192.168.1.1:10001")
	JoinAddresses  []string // Addresses to join (e.g., ["192.168.1.2:10001"])
	DataDir        string   // Data directory for this instance
	IsLeader       bool     // Whether this is the first node (creates cluster)
	AuthFile       string   // Path to RQLite auth JSON file. Empty = no auth enforcement.
	ExtraArgs      string   // Extra rqlited flags, set by the index from node config; tenants leave it empty. Any platform Raft timing flag not set here is added (WithDefaultRaftTimeouts), so every instance runs with the platform timing.
	// FreshStart marks this as the first start of a BRAND-NEW cluster, as
	// opposed to a restart/restore of an existing one. Bugboard #281: a
	// namespace delete that failed to remove the data directory left raft state
	// behind, and re-creating a namespace of the same name then booted on top of
	// it — the nodes disagreed on membership (one inherited a peer set including
	// a long-removed node and a different namespace's members) and never elected
	// a leader. On a fresh cluster any pre-existing raft state is garbage by
	// definition, so it is cleared rather than silently adopted. A restart must
	// NOT set this: reusing the raft directory is exactly what makes a restart a
	// restart.
	FreshStart bool
	// JoinVerifyURL is the HTTP base URL of the node this instance is about to
	// join (e.g. "http://10.0.0.1:10000"). Bugboard #275: rqlited joins whatever
	// answers at the -join address, with no check that the cluster belongs to
	// this namespace. When a port collision put another namespace's rqlited on
	// the expected port, a namespace node joined the FOREIGN raft group as a
	// Voter and served that namespace's database — identical row counts on a
	// namespace minutes old. Verifying the target's identity before starting
	// makes that impossible. Empty skips the check (the leader joins nothing).
	JoinVerifyURL string
}

// Instance represents a running RQLite instance
type Instance struct {
	Config  InstanceConfig
	Process *os.Process
	PID     int
}
