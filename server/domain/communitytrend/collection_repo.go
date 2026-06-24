package communitytrend

import (
	"context"
	"time"
)

type RobotsStatus struct {
	CommunityID   int       `json:"community_id"`
	CommunityKey  string    `json:"community_key"`
	CommunityName string    `json:"community_name"`
	CheckedAt     time.Time `json:"checked_at"`
	Allowed       bool      `json:"allowed"`
	SnapshotHash  string    `json:"snapshot_hash"`
	Note          string    `json:"note"`
}

type RobotsTransition struct {
	ID            int       `json:"id"`
	CommunityID   int       `json:"community_id"`
	CommunityName string    `json:"community_name"`
	FromAllowed   *bool     `json:"from_allowed"`
	ToAllowed     bool      `json:"to_allowed"`
	ChangedAt     time.Time `json:"changed_at"`
}

// RobotsRepository persists daily robots.txt allowance status and transitions.
type RobotsRepository interface {
	// Record stores a status sample and, if the allowance changed from the most
	// recent prior sample (or this is the first), records a transition.
	// Returns whether a transition was recorded.
	Record(ctx context.Context, communityID int, allowed bool, snapshotHash, note string) (changed bool, err error)
	// LatestAllowed returns the most recent allowance, found=false if none yet.
	LatestAllowed(ctx context.Context, communityID int) (allowed bool, found bool, err error)
	// ListStatus returns the latest robots status for all communities.
	ListStatus(ctx context.Context) ([]RobotsStatus, error)
	// ListTransitions returns the recent transitions, sorted by changed_at DESC.
	ListTransitions(ctx context.Context, limit int) ([]RobotsTransition, error)
}

// SeenRepository persists/loads dedup fingerprints (ct_seen_posts).
type SeenRepository interface {
	// LoadSeen returns the set of fingerprints already counted for a community.
	LoadSeen(ctx context.Context, communityID int) (map[string]bool, error)
	// Prune removes seen rows older than the given date (best board shows ~1 day).
	Prune(ctx context.Context, before time.Time) (int64, error)
}
