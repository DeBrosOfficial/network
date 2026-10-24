package olric

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// A partitioned Olric ring never merges by itself (bug-933): memberlist
// push-pulls only with members it holds alive, and only a Join merges two
// partitions, which olric-server calls at start alone. So the node supervisor
// heals a partition by restarting the index Olric of exactly one node on a
// smaller side, which joins every peer again when it starts and merges the
// sides. The restarted member starts empty: its share of the cache is lost,
// which a cache with replica count 1 already accepts for any restart.
//
// DecideHeal is the decision every node computes from the same inputs, and
// Healer is the per-node pacing around it. Together they keep two nodes from
// restarting at once: only one node of the whole cluster is ever the actor
// for a given set of views, it acts only after seeing the same decision on
// consecutive checks, and while its Olric restarts it is unreachable to the
// others, which makes every other node wait.

// HealAction is what one node should do about the index Olric's membership.
type HealAction int

const (
	// HealNone: the ring has every node of the cluster.
	HealNone HealAction = iota
	// HealWait: the ring is partitioned but this node must not act now: a
	// node is unreachable (the network has not healed, or a node is simply
	// down, which is not a partition), or another node is the one to restart.
	HealWait
	// HealRestart: this node is the one to restart its Olric.
	HealRestart
)

// HealDecision is DecideHeal's answer with the reason, for the log.
type HealDecision struct {
	Action HealAction
	Reason string
}

// DecideHeal decides what self does. expected are the index Olric addresses
// of every node in the index cluster (its raft configuration). views holds,
// for each node whose Olric answered and whose raft peer is reachable, the
// member names its Olric reports. Every address is host:port as Olric names
// its members.
//
// The ring is whole when self sees every expected node. Otherwise nothing is
// done unless every expected node answered: a missing answer is either a
// network that has not healed or a node that is down, and restarting into
// either merges nothing. With every view in hand, the nodes are grouped by
// the member set they see; the side with the most members stays (ties go to
// the side whose lowest member name sorts first), and of the nodes on every
// other side only the one whose name sorts first restarts.
func DecideHeal(self string, expected []string, views map[string][]string) HealDecision {
	if len(views[self]) >= len(expected) {
		return HealDecision{HealNone, fmt.Sprintf("%d of %d members", len(views[self]), len(expected))}
	}
	var missing []string
	for _, n := range expected {
		if _, ok := views[n]; !ok {
			missing = append(missing, n)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return HealDecision{HealWait, "partitioned, but no answer from " + strings.Join(missing, ", ")}
	}

	sides := map[string][]string{} // member set -> nodes that see it
	for _, n := range expected {
		key := memberSetKey(views[n])
		sides[key] = append(sides[key], n)
	}
	winner := ""
	for key := range sides {
		if winner == "" || betterSide(key, winner) {
			winner = key
		}
	}
	actor := ""
	for key, nodes := range sides {
		if key == winner {
			continue
		}
		for _, n := range nodes {
			if actor == "" || n < actor {
				actor = n
			}
		}
	}
	switch actor {
	case "":
		return HealDecision{HealWait, "every node sees the same members, fewer than the cluster's nodes"}
	case self:
		return HealDecision{HealRestart, fmt.Sprintf("%d sides; this node is on a smaller one and sorts first", len(sides))}
	default:
		return HealDecision{HealWait, fmt.Sprintf("%d sides; %s restarts first", len(sides), actor)}
	}
}

// memberSetKey is a canonical name for a set of members.
func memberSetKey(members []string) string {
	s := append([]string(nil), members...)
	sort.Strings(s)
	return strings.Join(s, ",")
}

// betterSide reports whether side a stays rather than side b: more members,
// then the lower first member name.
func betterSide(a, b string) bool {
	na, nb := strings.Count(a, ",")+1, strings.Count(b, ",")+1
	if na != nb {
		return na > nb
	}
	return a < b
}

const (
	// HealCheckInterval is how often a node checks the index Olric's membership.
	HealCheckInterval = 30 * time.Second
	// healConfirmChecks is how many consecutive checks must decide HealRestart
	// before the node restarts: views taken while another node was restarting
	// or rejoining must agree twice, a check interval apart.
	healConfirmChecks = 2
	// healMinBackoff and healMaxBackoff bound the wait after a restart before
	// this node may restart again; it doubles while restarts do not heal.
	healMinBackoff = 5 * time.Minute
	healMaxBackoff = time.Hour
)

// Healer paces one node's restarts: a hold-down of healConfirmChecks
// consecutive HealRestart decisions, and a backoff after each restart that
// doubles until a check finds the ring whole.
type Healer struct {
	confirmed int
	backoff   time.Duration
	notBefore time.Time
}

// Observe records one decision and reports whether to restart now.
func (h *Healer) Observe(d HealDecision, now time.Time) bool {
	switch d.Action {
	case HealNone:
		h.confirmed, h.backoff = 0, 0
		return false
	case HealWait:
		h.confirmed = 0
		return false
	}
	h.confirmed++
	if h.confirmed < healConfirmChecks || now.Before(h.notBefore) {
		return false
	}
	if h.backoff == 0 {
		h.backoff = healMinBackoff
	} else {
		h.backoff = min(2*h.backoff, healMaxBackoff)
	}
	h.notBefore = now.Add(h.backoff)
	h.confirmed = 0
	return true
}

// HealState is what a node records about the restarts it made to heal a
// partition, for the node report.
type HealState struct {
	Restarts    int       `json:"restarts"`
	LastRestart time.Time `json:"last_restart"`
	LastReason  string    `json:"last_reason"`
}

// HealStateFile is where the node keeps HealState, under its data directory.
const HealStateFile = "olric-heal.json"

// RecordHealRestart adds one restart to the state file in dataDir.
func RecordHealRestart(dataDir, reason string, at time.Time) error {
	path := filepath.Join(dataDir, HealStateFile)
	st, err := ReadHealState(dataDir)
	if err != nil {
		return err
	}
	st.Restarts++
	st.LastRestart, st.LastReason = at.UTC(), reason
	data, err := json.Marshal(st)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	return os.Rename(tmp, path)
}

// ReadHealState reads the state file in dataDir; none is a zero state.
func ReadHealState(dataDir string) (HealState, error) {
	var st HealState
	data, err := os.ReadFile(filepath.Join(dataDir, HealStateFile))
	if os.IsNotExist(err) {
		return st, nil
	}
	if err != nil {
		return st, err
	}
	if err := json.Unmarshal(data, &st); err != nil {
		return st, fmt.Errorf("parse %s: %w", HealStateFile, err)
	}
	return st, nil
}
