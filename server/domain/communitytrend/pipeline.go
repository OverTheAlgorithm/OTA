package communitytrend

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"math"
	"strings"
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
	Delete(ctx context.Context, communityID int, date time.Time) error
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
	minScore    float64
}

// NewPipeline wires the daily-run dependencies.
func NewPipeline(
	communities CommunityRepository, tags TagRepository, axes AxisRepository,
	robotsRepo RobotsRepository, seen SeenRepository, worksheets WorksheetRepository,
	registry *AdapterRegistry, fetcher RobotsFetcher, tagger Tagger,
	suggestions SuggestionStore, memes MemeRepository, minScore float64,
) *Pipeline {
	return &Pipeline{
		communities: communities, tags: tags, axes: axes,
		robotsRepo: robotsRepo, seen: seen, worksheets: worksheets,
		registry: registry, fetcher: fetcher, tagger: tagger,
		suggestions: suggestions, memes: memes, minScore: minScore,
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
	slog.Info("starting community-trend pipeline for community", "community", c.Key, "date", date.Format("2006-01-02"))
	res := CommunityResult{Key: c.Key}

	adapter, ok := p.registry.Get(c.Key)
	if !ok {
		slog.Warn("community-trend skip: no adapter registered", "community", c.Key)
		return p.fallbackManual(ctx, c, date, "no adapter")
	}

	// robots gate
	if url := adapter.RobotsURL(); url != "" {
		slog.Info("fetching robots.txt for community", "community", c.Key, "url", url)
		body, accessible, ferr := p.fetcher.Fetch(ctx, url)
		allowed := accessible
		note := "accessible"
		hash := ""
		if !accessible {
			note = "blocked (403/429/timeout)"
			if ferr != nil {
				note = "fetch error: " + ferr.Error()
			}
			slog.Warn("robots.txt not accessible, falling back to manual", "community", c.Key, "error", ferr)
		} else {
			rules := ParseRobots(body)
			allowed = rules.AllPathsAllowed(adapter.BestBoardPaths())
			sum := sha256.Sum256([]byte(body))
			hash = hex.EncodeToString(sum[:])
			if !allowed {
				note = "disallowed by robots"
				slog.Warn("community path disallowed by robots.txt, falling back to manual", "community", c.Key)
			}
		}
		if _, rerr := p.robotsRepo.Record(ctx, c.ID, allowed, hash, note); rerr != nil {
			slog.Error("failed to record robots status", "community", c.Key, "error", rerr)
			return p.errorResult(ctx, c, date, "robots record: "+rerr.Error())
		}
		if !allowed {
			return p.fallbackManual(ctx, c, date, note)
		}
	}

	slog.Info("fetching recent items for community", "community", c.Key)
	items, ferr := adapter.FetchRecent(ctx)
	if ferr != nil {
		slog.Error("failed to fetch recent items, falling back to manual", "community", c.Key, "error", ferr)
		return p.fallbackManual(ctx, c, date, "fetch error: "+ferr.Error())
	}
	slog.Info("fetched recent items successfully", "community", c.Key, "count", len(items))

	seen, serr := p.seen.LoadSeen(ctx, c.ID)
	if serr != nil {
		slog.Error("failed to load seen posts from DB", "community", c.Key, "error", serr)
		return p.errorResult(ctx, c, date, "load seen: "+serr.Error())
	}
	fresh, fps := FilterUnseen(c.Key, items, seen)
	slog.Info("filtered items with seen posts", "community", c.Key, "total", len(items), "fresh", len(fresh))

	titles := make([]string, len(fresh))
	for i, it := range fresh {
		titles[i] = it.TextUnit
	}

	taxonomy, terr := p.buildTaxonomy(ctx)
	if terr != nil {
		slog.Error("failed to build taxonomy", "community", c.Key, "error", terr)
		return p.errorResult(ctx, c, date, terr.Error())
	}

	var taggerMemes []MemeRef
	existingMemes := make(map[string]bool)
	if p.memes != nil {
		memes, err := p.memes.ListMemes(ctx, false)
		if err == nil {
			taggerMemes = make([]MemeRef, len(memes))
			for i, m := range memes {
				taggerMemes[i] = MemeRef{
					ID:      m.ID,
					Name:    m.Name,
					Aliases: m.Aliases,
				}
				existingMemes[strings.ToLower(strings.TrimSpace(m.Name))] = true
				for _, alias := range m.Aliases {
					existingMemes[strings.ToLower(strings.TrimSpace(alias))] = true
				}
			}
		} else {
			slog.Warn("failed to list memes", "community", c.Key, "error", err)
		}
	}

	slog.Info("running AI analysis for tagging and meme extraction", "community", c.Key, "titles_count", len(titles))
	out, aerr := p.tagger.Analyze(ctx, TaggerInput{
		CommunityKey: c.Key,
		Titles:       titles,
		Taxonomy:     taxonomy,
		Memes:        taggerMemes,
		MinScore:     p.minScore,
	})
	if aerr != nil {
		slog.Error("AI tagger analysis failed, falling back to manual", "community", c.Key, "error", aerr)
		return p.fallbackManual(ctx, c, date, "ai error: "+aerr.Error())
	}

	// Statically filter out meme candidates that are already confirmed memes (name or aliases)
	var filteredCandidates []MemeCandidate
	for _, mc := range out.MemeCandidates {
		lowerExp := strings.ToLower(strings.TrimSpace(mc.Expression))
		if existingMemes[lowerExp] {
			slog.Debug("statically filtered out confirmed meme from candidates", "expression", mc.Expression)
			continue
		}
		filteredCandidates = append(filteredCandidates, mc)
	}
	out.MemeCandidates = filteredCandidates
	slog.Info("AI analysis completed successfully", "community", c.Key, "tags_suggested", len(out.Tags), "meme_matches", len(out.MemeMatches), "meme_candidates", len(out.MemeCandidates))

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
		slog.Error("failed to ensure auto worksheet", "community", c.Key, "error", werr)
		return p.errorResult(ctx, c, date, "ensure worksheet: "+werr.Error())
	}
	if perr := p.suggestions.Put(ctx, Suggestion{
		CommunityID: c.ID, StatDate: date, Output: out, Fingerprints: fps, TotalPosts: len(fresh),
	}); perr != nil {
		slog.Error("failed to store AI suggestion", "community", c.Key, "error", perr)
		return p.errorResult(ctx, c, date, "store suggestion: "+perr.Error())
	}

	slog.Info("community-trend pipeline completed successfully", "community", c.Key, "fresh", len(fresh), "tags", len(out.Tags))
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
	axisType := make(map[int]string, len(axes))
	for _, a := range axes {
		axisKey[a.ID] = a.Key
		axisType[a.ID] = a.Type
	}
	tags, err := p.tags.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	var out []TaxonomyTag
	for _, t := range tags {
		if axisType[t.AxisID] == "topic" {
			out = append(out, TaxonomyTag{
				ID:          t.ID,
				AxisKey:     axisKey[t.AxisID],
				Name:        t.Name,
				Description: t.Description,
			})
		}
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
