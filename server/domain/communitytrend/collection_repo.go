package communitytrend

import (
	"context"
	"time"
)

// RobotsRepository persists daily robots.txt allowance status and transitions.
type RobotsRepository interface {
	// Record stores a status sample and, if the allowance changed from the most
	// recent prior sample (or this is the first), records a transition.
	// Returns whether a transition was recorded.
	Record(ctx context.Context, communityID int, allowed bool, snapshotHash, note string) (changed bool, err error)
	// LatestAllowed returns the most recent allowance, found=false if none yet.
	LatestAllowed(ctx context.Context, communityID int) (allowed bool, found bool, err error)
}

// SeenRepository persists/loads dedup fingerprints (ct_seen_posts).
type SeenRepository interface {
	// LoadSeen returns the set of fingerprints already counted for a community.
	LoadSeen(ctx context.Context, communityID int) (map[string]bool, error)
	// Prune removes seen rows older than the given date (best board shows ~1 day).
	Prune(ctx context.Context, before time.Time) (int64, error)
}
