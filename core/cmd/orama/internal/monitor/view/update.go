package view

import (
	"fmt"
	"time"

	"github.com/DeBrosOfficial/network/pkg/telemetry/report"
	"github.com/DeBrosOfficial/network/pkg/updatenotice"
)

// noUpdateCell fills the UPDATE cell of a node whose auto-update agent has nothing to report.
const noUpdateCell = "-"

// UpdateCell is a node's auto-update finding in a few words: the release and what the policy does
// with it ("0.3.1 (notify)"), or that it was refused or rolled back.
func UpdateCell(t Theme, n *updatenotice.Notice) string {
	if n == nil {
		return noUpdateCell
	}
	candidate := n.Candidate
	if candidate == "" {
		candidate = unknownCell
	}
	switch n.State {
	case updatenotice.StateAvailable:
		return fmt.Sprintf("%s (%s)", candidate, n.Policy())
	case updatenotice.StateRefused:
		return t.Warn.Render("refused " + candidate)
	case updatenotice.StateFailed:
		return t.Warn.Render(candidate + " rolled back")
	}
	return unknownCell
}

// UpdateSentence is the finding as a sentence, with the policy's decision and why. It is empty
// when there is no finding.
func UpdateSentence(n *updatenotice.Notice) string {
	if n == nil {
		return ""
	}
	switch n.State {
	case updatenotice.StateAvailable:
		return fmt.Sprintf("release %s is available on the %s channel (running %s); the policy (%s) will %s: %s",
			n.Candidate, n.Channel, n.Current, n.Mode, n.Policy(), n.Reason)
	case updatenotice.StateRefused:
		return fmt.Sprintf("release %s was refused on the %s channel (running %s): %s",
			orUnknown(n.Candidate), n.Channel, n.Current, n.Reason)
	case updatenotice.StateFailed:
		return fmt.Sprintf("installing release %s failed and was rolled back (running %s): %s",
			orUnknown(n.Candidate), n.Current, n.Reason)
	}
	return ""
}

func orUnknown(s string) string {
	if s == "" {
		return "(unknown)"
	}
	return s
}

// UpgradeCountdown is how far a node's chain is from the height of the upgrade scheduled for it.
type UpgradeCountdown struct {
	// BlocksLeft is how many blocks remain; zero or negative once the height is reached.
	BlocksLeft int64
	// ETA is BlocksLeft at the chain's recent block time; HasETA is false when no block time was
	// measured or the height is reached.
	ETA    time.Duration
	HasETA bool
}

// CountdownTo is the distance from the chain's height to the upgrade's, at the node's average
// block time.
func CountdownTo(c *report.ChainReport, u *report.ChainUpgrade) UpgradeCountdown {
	left := u.Height - c.LatestHeight
	cd := UpgradeCountdown{BlocksLeft: left}
	if left > 0 && c.AvgBlockTimeSec > 0 {
		cd.ETA = time.Duration(float64(left) * c.AvgBlockTimeSec * float64(time.Second))
		cd.HasETA = true
	}
	return cd
}

// UpgradeSentence is the chain upgrade scheduled for a node as a sentence: its name and height,
// the blocks and time remaining, and whether this node holds the binary. It is empty when no
// upgrade is scheduled and the plan could be read.
func UpgradeSentence(c *report.ChainReport) string {
	if c == nil {
		return ""
	}
	if c.UpgradeError != "" {
		return "the scheduled upgrade could not be read: " + c.UpgradeError
	}
	u := c.Upgrade
	if u == nil {
		return ""
	}
	cd := CountdownTo(c, u)
	var when string
	switch {
	case cd.BlocksLeft <= 0:
		when = "the height is reached"
	case cd.HasETA:
		when = fmt.Sprintf("%d blocks left, about %s", cd.BlocksLeft, FormatAge(cd.ETA))
	default:
		when = fmt.Sprintf("%d blocks left", cd.BlocksLeft)
	}
	staged := "binary staged on this node"
	switch {
	case u.StageError != "":
		staged = "binary cannot be staged: " + u.StageError
	case !u.Staged:
		staged = "binary NOT staged on this node"
	}
	return fmt.Sprintf("%s at height %d (%s); %s", u.Name, u.Height, when, staged)
}
