package communitytrend

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"time"
)

// RobotsFetcher fetches robots.txt. accessible=false signals an anti-bot wall
// (HTTP 403/429/timeout) which we treat as disallowed (decisions.md D-006).
type RobotsFetcher interface {
	Fetch(ctx context.Context, robotsURL string) (body string, accessible bool, err error)
}

// Suggestion is the transient AI result for a worksheet, held until a human
// confirms. Titles are NOT stored here (copyright guardrail) — only derived data.
type Suggestion struct {
	CommunityID  int
	StatDate     time.Time
	Output       TaggerOutput
	Fingerprints []string
	TotalPosts   int
}

// SuggestionStore holds AI suggestions transiently (e.g. TTL cache) for the
// admin UI to fetch before confirmation.
type SuggestionStore interface {
	Put(ctx context.Context, s Suggestion) error
	Get(ctx context.Context, communityID int, date time.Time) (Suggestion, bool, error)
}

// CommunityResult records the per-community outcome of a daily run.
type CommunityResult struct {
	Key    string
	Mode   string // 'auto' | 'manual'
	Status string // 'suggested' | 'pending' | 'error'
	Reason string
}

// Pipeline orchestrates the daily auto-collection: robots gate → fetch → dedup
// → AI single-pass suggest → worksheet 'suggested'. Disallowed/adapter-less
// communities fall back to a manual pending worksheet.
type Pipeline struct {
	communities CommunityRepository
	tags        TagRepository
	axes        AxisRepository
	robotsRepo  RobotsRepository
	seen        SeenRepository
	worksheets  WorksheetRepository
	registry    *AdapterRegistry
	fetcher     RobotsFetcher
	tagger      Tagger
	suggestions SuggestionStore
	memes       MemeRepository
	minCount    int
}

// NewPipeline wires the daily-run dependencies.
func NewPipeline(
	communities CommunityRepository, tags TagRepository, axes AxisRepository,
	robotsRepo RobotsRepository, seen SeenRepository, worksheets WorksheetRepository,
	registry *AdapterRegistry, fetcher RobotsFetcher, tagger Tagger,
	suggestions SuggestionStore, memes MemeRepository, minCount int,
) *Pipeline {
	return &Pipeline{
		communities: communities, tags: tags, axes: axes,
		robotsRepo: robotsRepo, seen: seen, worksheets: worksheets,
		registry: registry, fetcher: fetcher, tagger: tagger,
		suggestions: suggestions, memes: memes, minCount: minCount,
	}
}

// RunDaily processes every enabled community for the given date.
func (p *Pipeline) RunDaily(ctx context.Context, date time.Time) ([]CommunityResult, error) {
	comms, err := p.communities.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list communities: %w", err)
	}
	var results []CommunityResult
	for _, c := range comms {
		if !c.Enabled {
			continue
		}
		results = append(results, p.runCommunity(ctx, c, date))
	}
	return results, nil
}

func (p *Pipeline) runCommunity(ctx context.Context, c Community, date time.Time) CommunityResult {
	res := CommunityResult{Key: c.Key}

	adapter, ok := p.registry.Get(c.Key)
	if !ok {
		return p.fallbackManual(ctx, c, date, "no adapter")
	}

	// robots gate
	if url := adapter.RobotsURL(); url != "" {
		body, accessible, ferr := p.fetcher.Fetch(ctx, url)
		allowed := accessible
		note := "accessible"
		hash := ""
		if !accessible {
			note = "blocked (403/429/timeout)"
			if ferr != nil {
				note = "fetch error: " + ferr.Error()
			}
		} else {
			rules := ParseRobots(body)
			allowed = rules.AllPathsAllowed(adapter.BestBoardPaths())
			sum := sha256.Sum256([]byte(body))
			hash = hex.EncodeToString(sum[:])
			if !allowed {
				note = "disallowed by robots"
			}
		}
		if _, rerr := p.robotsRepo.Record(ctx, c.ID, allowed, hash, note); rerr != nil {
			return p.errorResult(ctx, c, date, "robots record: "+rerr.Error())
		}
		if !allowed {
			return p.fallbackManual(ctx, c, date, note)
		}
	}

	items, ferr := adapter.FetchRecent(ctx)
	if ferr != nil {
		return p.fallbackManual(ctx, c, date, "fetch error: "+ferr.Error())
	}

	seen, serr := p.seen.LoadSeen(ctx, c.ID)
	if serr != nil {
		return p.errorResult(ctx, c, date, "load seen: "+serr.Error())
	}
	fresh, fps := FilterUnseen(c.Key, items, seen)

	titles := make([]string, len(fresh))
	for i, it := range fresh {
		comments := it.Engagement["comments"]
		upvotes := it.Engagement["upvotes"]
		if comments > 0 || upvotes > 0 {
			titles[i] = fmt.Sprintf("%s (댓글 %d, 추천 %d)", it.TextUnit, comments, upvotes)
		} else {
			titles[i] = it.TextUnit
		}
	}

	taxonomy, terr := p.buildTaxonomy(ctx)
	if terr != nil {
		return p.errorResult(ctx, c, date, terr.Error())
	}

	out, aerr := p.tagger.Analyze(ctx, TaggerInput{
		CommunityKey: c.Key,
		Titles:       titles,
		Taxonomy:     taxonomy,
		MinCount:     p.minCount,
	})
	if aerr != nil {
		return p.fallbackManual(ctx, c, date, "ai error: "+aerr.Error())
	}

	// Go-side deterministic weight calculation
	commentDenom := 30.0
	upvoteDenom := 50.0
	if c.Key == "clien" {
		commentDenom = 20.0
		upvoteDenom = 30.0
	} else if c.Key == "fmkorea" {
		commentDenom = 150.0
		upvoteDenom = 300.0
	} else if c.Key == "theqoo" {
		commentDenom = 100.0
		upvoteDenom = 1.0
	}

	for i := range out.Tags {
		tag := &out.Tags[i]
		var sum float64
		for _, idx := range tag.PostIndices {
			if idx >= 1 && idx <= len(fresh) {
				item := fresh[idx-1]
				comments := float64(item.Engagement["comments"])
				upvotes := float64(item.Engagement["upvotes"])
				var val float64
				if c.Key == "theqoo" {
					val = comments / commentDenom
				} else {
					val = comments/commentDenom + upvotes/upvoteDenom
				}
				weight := 1.0 + math.Log10(1.0+val)
				sum += weight
			}
		}
		tag.Count = math.Round(sum*100) / 100
	}

	for i := range out.MemeMatches {
		m := &out.MemeMatches[i]
		count := 0
		for _, idx := range m.PostIndices {
			if idx >= 1 && idx <= len(fresh) {
				count++
			}
		}
		m.Count = count
	}

	for i := range out.MemeCandidates {
		mc := &out.MemeCandidates[i]
		count := 0
		for _, idx := range mc.PostIndices {
			if idx >= 1 && idx <= len(fresh) {
				count++
			}
		}
		mc.HitCount = count
	}

	// Persist AI-discovered meme candidates so humans can review them (blacklisted
	// expressions are ignored by the repo). Best-effort: a candidate write failure
	// must not sink the whole community's run.
	if p.memes != nil {
		for _, mc := range out.MemeCandidates {
			_ = p.memes.UpsertCandidate(ctx, mc.Expression, date)
		}
	}

	if _, werr := p.worksheets.Ensure(ctx, c.ID, date, "auto"); werr != nil {
		return p.errorResult(ctx, c, date, "ensure worksheet: "+werr.Error())
	}
	if perr := p.suggestions.Put(ctx, Suggestion{
		CommunityID: c.ID, StatDate: date, Output: out, Fingerprints: fps, TotalPosts: len(fresh),
	}); perr != nil {
		return p.errorResult(ctx, c, date, "store suggestion: "+perr.Error())
	}

	res.Mode = "auto"
	res.Status = "suggested"
	res.Reason = fmt.Sprintf("%d fresh items, %d tag suggestions", len(fresh), len(out.Tags))
	return res
}

func (p *Pipeline) buildTaxonomy(ctx context.Context) ([]TaxonomyTag, error) {
	axes, err := p.axes.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list axes: %w", err)
	}
	axisKey := make(map[int]string, len(axes))
	for _, a := range axes {
		axisKey[a.ID] = a.Key
	}
	tags, err := p.tags.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	out := make([]TaxonomyTag, len(tags))
	for i, t := range tags {
		out[i] = TaxonomyTag{ID: t.ID, AxisKey: axisKey[t.AxisID], Name: t.Name}
	}
	return out, nil
}

func (p *Pipeline) fallbackManual(ctx context.Context, c Community, date time.Time, reason string) CommunityResult {
	res := CommunityResult{Key: c.Key, Mode: "manual", Status: "pending", Reason: reason}
	if _, err := p.worksheets.Ensure(ctx, c.ID, date, "manual"); err != nil {
		res.Status = "error"
		res.Reason = "ensure manual worksheet: " + err.Error()
	}
	return res
}

func (p *Pipeline) errorResult(_ context.Context, c Community, _ time.Time, reason string) CommunityResult {
	return CommunityResult{Key: c.Key, Status: "error", Reason: reason}
}
