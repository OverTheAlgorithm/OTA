package storage

import (
	"context"
	"sync"
	"time"

	"ota/domain/communitytrend"
)

// CTSuggestionStore is an in-process store for transient AI suggestions held
// between the daily pipeline run and human confirmation. It deliberately keeps
// no titles/posts — only derived tag/meme data and fingerprints.
//
// In-process (not Redis) is acceptable: suggestions are regenerated each daily
// run and only needed for that day's review window. Entries older than the TTL
// are dropped on access.
type CTSuggestionStore struct {
	mu  sync.Mutex
	ttl time.Duration
	m   map[string]entry
}

type entry struct {
	s       communitytrend.Suggestion
	expires time.Time
}

func NewCTSuggestionStore(ttl time.Duration) *CTSuggestionStore {
	if ttl <= 0 {
		ttl = 48 * time.Hour
	}
	return &CTSuggestionStore{ttl: ttl, m: map[string]entry{}}
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
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[suggestionKey(sug.CommunityID, sug.StatDate)] = entry{s: sug, expires: nowCT().Add(s.ttl)}
	return nil
}

func (s *CTSuggestionStore) Get(_ context.Context, communityID int, date time.Time) (communitytrend.Suggestion, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := suggestionKey(communityID, date)
	e, ok := s.m[k]
	if !ok {
		return communitytrend.Suggestion{}, false, nil
	}
	if nowCT().After(e.expires) {
		delete(s.m, k)
		return communitytrend.Suggestion{}, false, nil
	}
	return e.s, true, nil
}

// nowCT is a seam kept tiny; time.Now is fine here (not a workflow script).
func nowCT() time.Time { return time.Now() }
