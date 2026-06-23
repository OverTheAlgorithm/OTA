package communitytrend

import (
	"context"
	"time"
)

// TrendItem is the source-neutral unit an adapter emits. It is an "observation",
// not a "post" — YouTube/Twitter adapters could emit the same shape later.
// TextUnit holds the topic-extraction target (e.g. a title) and is transient:
// it is never persisted (copyright guardrail).
type TrendItem struct {
	SourceID   string         // stable per-site id (post number, video id). Fingerprint material.
	TextUnit   string         // one line to extract topic from. Transient. Never stored.
	Engagement map[string]int // generic metrics: {"comments":..,"upvotes":..,"views":..}
	ObservedAt time.Time
}

// SourceAdapter normalizes a site's representative board into TrendItems.
type SourceAdapter interface {
	Key() string              // "dogdrip" — matches ct_communities.key
	RobotsURL() string        // "" skips the robots gate (e.g. API sources)
	BestBoardPaths() []string // paths checked for generic-UA allowance in robots
	FetchRecent(ctx context.Context) ([]TrendItem, error)
}

// AdapterRegistry maps community keys to their code-side adapter.
type AdapterRegistry struct {
	adapters map[string]SourceAdapter
}

// NewAdapterRegistry builds a registry from the given adapters, keyed by Key().
func NewAdapterRegistry(adapters ...SourceAdapter) *AdapterRegistry {
	m := make(map[string]SourceAdapter, len(adapters))
	for _, a := range adapters {
		m[a.Key()] = a
	}
	return &AdapterRegistry{adapters: m}
}

// Get returns the adapter for a community key, if registered.
func (r *AdapterRegistry) Get(key string) (SourceAdapter, bool) {
	a, ok := r.adapters[key]
	return a, ok
}

// Keys returns all registered adapter keys.
func (r *AdapterRegistry) Keys() []string {
	keys := make([]string, 0, len(r.adapters))
	for k := range r.adapters {
		keys = append(keys, k)
	}
	return keys
}
