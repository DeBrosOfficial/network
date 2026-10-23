package node

import (
	"errors"
	"testing"
)

// bug-933: doctor passed "1 member(s)" on a node whose index Olric had split
// from the other four. The member count is held to the index cluster's node
// count.
func TestOlricMembershipCheck(t *testing.T) {
	for _, tc := range []struct {
		name             string
		members, cluster int
		clusterErr       error
		want             string
	}{
		{"whole", 5, 5, nil, "PASS"},
		{"partitioned", 1, 5, nil, "FAIL"},
		{"the majority side of a split", 3, 5, nil, "FAIL"},
		{"a removed node lingers", 6, 5, nil, "WARN"},
		{"cluster size unknown", 1, 0, errors.New("rqlite down"), "WARN"},
	} {
		if got := olricMembershipCheck("10.0.0.1:10102", tc.members, tc.cluster, tc.clusterErr); got.Status != tc.want {
			t.Errorf("%s: %s (%s), want %s", tc.name, got.Status, got.Detail, tc.want)
		}
	}
}
