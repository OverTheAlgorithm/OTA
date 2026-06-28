package storage

import (
	"context"
	"time"

	"ota/cache"
	"ota/domain/communitytrend"
)

// CTSuggestionStore wraps a cache.Cache (which could be RedisCache or InProcessCache)
// to hold transient AI suggestions between the daily pipeline run and human confirmation.
// It deliberately keeps no titles/posts — only derived tag/meme data and fingerprints.
type CTSuggestionStore struct {
	cache cache.Cache
	ttl   time.Duration
}

func NewCTSuggestionStore(c cache.Cache, ttl time.Duration) *CTSuggestionStore {
	if ttl <= 0 {
		ttl = 48 * time.Hour
	}
	return &CTSuggestionStore{cache: c, ttl: ttl}
}

func suggestionKey(communityID int, date time.Time) string {
	return date.Format("2006-01-02") + ":" + itoaCT(communityID)
}

func itoaCT(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func (s *CTSuggestionStore) Put(_ context.Context, sug communitytrend.Suggestion) error {
	k := suggestionKey(sug.CommunityID, sug.StatDate)
	return s.cache.Set(k, sug, s.ttl)
}

func (s *CTSuggestionStore) Get(_ context.Context, communityID int, date time.Time) (communitytrend.Suggestion, bool, error) {
	k := suggestionKey(communityID, date)
	sug, ok := cache.GetTyped[communitytrend.Suggestion](s.cache, k)
	return sug, ok, nil
}
