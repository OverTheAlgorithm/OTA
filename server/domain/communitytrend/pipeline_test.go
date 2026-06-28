package communitytrend

import (
	"context"
	"testing"
	"time"
)

// --- fakes ---

type fakeTagRepo struct{ tags []Tag }

func (f *fakeTagRepo) Create(context.Context, Tag) (Tag, error)       { return Tag{}, nil }
func (f *fakeTagRepo) List(context.Context) ([]Tag, error)            { return f.tags, nil }
func (f *fakeTagRepo) ListByAxis(context.Context, int) ([]Tag, error) { return nil, nil }
func (f *fakeTagRepo) Update(context.Context, int, string, string) (Tag, error) {
	return Tag{}, nil
}
func (f *fakeTagRepo) Delete(context.Context, int) error { return nil }

type fakeAxisRepo struct{ axes []Axis }

func (f *fakeAxisRepo) Create(context.Context, Axis) (Axis, error) { return Axis{}, nil }
func (f *fakeAxisRepo) List(context.Context) ([]Axis, error)       { return f.axes, nil }
func (f *fakeAxisRepo) Delete(context.Context, int) error          { return nil }

type fakeRobotsRepo struct{ recorded []bool }

func (f *fakeRobotsRepo) Record(_ context.Context, _ int, allowed bool, _, _ string) (bool, error) {
	f.recorded = append(f.recorded, allowed)
	return true, nil
}
func (f *fakeRobotsRepo) LatestAllowed(context.Context, int) (bool, bool, error) {
	return false, false, nil
}
func (f *fakeRobotsRepo) ListStatus(context.Context) ([]RobotsStatus, error) {
	return nil, nil
}
func (f *fakeRobotsRepo) ListTransitions(context.Context, int) ([]RobotsTransition, error) {
	return nil, nil
}

type fakeSeenRepo struct{ seen map[int]map[string]bool }

func (f *fakeSeenRepo) LoadSeen(_ context.Context, communityID int) (map[string]bool, error) {
	if f.seen == nil {
		return map[string]bool{}, nil
	}
	return f.seen[communityID], nil
}
func (f *fakeSeenRepo) Prune(context.Context, time.Time) (int64, error) { return 0, nil }

type fakeWorksheetRepo struct{ ensured map[int]string } // communityID -> mode

func (f *fakeWorksheetRepo) Ensure(_ context.Context, communityID int, _ time.Time, mode string) (Worksheet, error) {
	if f.ensured == nil {
		f.ensured = map[int]string{}
	}
	f.ensured[communityID] = mode
	return Worksheet{CommunityID: communityID, Mode: mode, Status: "pending"}, nil
}
func (f *fakeWorksheetRepo) ListByDate(context.Context, time.Time) ([]Worksheet, error) {
	return nil, nil
}
func (f *fakeWorksheetRepo) Confirm(context.Context, Confirmation) error { return nil }
func (f *fakeWorksheetRepo) Reset(context.Context, int, time.Time) error  { return nil }

type fakeAdapter struct {
	key, robotsURL string
	paths          []string
	items          []TrendItem
}

func (f *fakeAdapter) Key() string              { return f.key }
func (f *fakeAdapter) RobotsURL() string        { return f.robotsURL }
func (f *fakeAdapter) BestBoardPaths() []string { return f.paths }
func (f *fakeAdapter) FetchRecent(context.Context) ([]TrendItem, error) {
	return f.items, nil
}

type robotsResp struct {
	body       string
	accessible bool
}
type fakeRobotsFetcher struct{ byURL map[string]robotsResp }

func (f *fakeRobotsFetcher) Fetch(_ context.Context, url string) (string, bool, error) {
	if r, ok := f.byURL[url]; ok {
		return r.body, r.accessible, nil
	}
	return "User-agent: *\nAllow: /\n", true, nil // default allow
}

type fakeTagger struct{ gotTitles int }

func (f *fakeTagger) Analyze(_ context.Context, in TaggerInput) (TaggerOutput, error) {
	f.gotTitles = len(in.Titles)
	return TaggerOutput{Tags: []TagSuggestion{{Name: "남성 인권", AxisKey: "gender_topic", PostIndices: []int{1, 2}}}}, nil
}

type fakeSuggestionStore struct{ stored map[int]Suggestion }

func (f *fakeSuggestionStore) Put(_ context.Context, s Suggestion) error {
	if f.stored == nil {
		f.stored = map[int]Suggestion{}
	}
	f.stored[s.CommunityID] = s
	return nil
}
func (f *fakeSuggestionStore) Get(_ context.Context, communityID int, _ time.Time) (Suggestion, bool, error) {
	s, ok := f.stored[communityID]
	return s, ok, nil
}
func (f *fakeSuggestionStore) Delete(_ context.Context, communityID int, _ time.Time) error {
	if f.stored != nil {
		delete(f.stored, communityID)
	}
	return nil
}

// --- test ---

func TestPipeline_RunDaily(t *testing.T) {
	ctx := context.Background()
	date := time.Date(2026, 6, 24, 0, 0, 0, 0, time.UTC)

	commRepo := newFakeCommunityRepo()
	dogdrip, _ := commRepo.Create(ctx, Community{Key: "dogdrip", Name: "개드립", Enabled: true})
	theqoo, _ := commRepo.Create(ctx, Community{Key: "theqoo", Name: "더쿠", Enabled: true})
	nocomm, _ := commRepo.Create(ctx, Community{Key: "manualonly", Name: "수동", Enabled: true})

	registry := NewAdapterRegistry(
		&fakeAdapter{key: "dogdrip", robotsURL: "dd/robots", paths: []string{"/best"}, items: []TrendItem{
			{SourceID: "1", TextUnit: "글 A"},
			{SourceID: "2", TextUnit: "글 B"},
			{SourceID: "3", TextUnit: "글 C"},
		}},
		&fakeAdapter{key: "theqoo", robotsURL: "tq/robots", paths: []string{"/"}},
	)

	fetcher := &fakeRobotsFetcher{byURL: map[string]robotsResp{
		"dd/robots": {body: "User-agent: *\nAllow: /\n", accessible: true},
		"tq/robots": {body: "User-agent: *\nDisallow: /\n", accessible: true}, // disallows best path "/"
	}}

	// dedup: dogdrip item "2" already seen → only A,C fresh
	seen := &fakeSeenRepo{seen: map[int]map[string]bool{
		dogdrip.ID: {Fingerprint("dogdrip", "2"): true},
	}}

	tagger := &fakeTagger{}
	wsRepo := &fakeWorksheetRepo{}
	store := &fakeSuggestionStore{}

	p := NewPipeline(
		commRepo, &fakeTagRepo{}, &fakeAxisRepo{}, &fakeRobotsRepo{}, seen, wsRepo,
		registry, fetcher, tagger, store, nil, 3.0,
	)

	results, err := p.RunDaily(ctx, date)
	if err != nil {
		t.Fatalf("run daily: %v", err)
	}

	byKey := map[string]CommunityResult{}
	for _, r := range results {
		byKey[r.Key] = r
	}

	// dogdrip: allowed → auto/suggested
	if r := byKey["dogdrip"]; r.Mode != "auto" || r.Status != "suggested" {
		t.Fatalf("dogdrip: expected auto/suggested, got %+v", r)
	}
	// theqoo: robots disallow → manual/pending
	if r := byKey["theqoo"]; r.Mode != "manual" || r.Status != "pending" {
		t.Fatalf("theqoo: expected manual/pending, got %+v", r)
	}
	// manualonly: no adapter → manual/pending
	if r := byKey["manualonly"]; r.Mode != "manual" || r.Status != "pending" {
		t.Fatalf("manualonly: expected manual/pending, got %+v", r)
	}

	// dedup: tagger saw 2 fresh titles (A,C), not 3
	if tagger.gotTitles != 2 {
		t.Fatalf("expected 2 fresh titles after dedup, got %d", tagger.gotTitles)
	}

	// suggestion stored for dogdrip with 2 fresh + fingerprints
	s, ok := store.stored[dogdrip.ID]
	if !ok || s.TotalPosts != 2 || len(s.Fingerprints) != 2 {
		t.Fatalf("expected stored suggestion with 2 fresh, got %+v ok=%v", s, ok)
	}
	if len(s.Output.Tags) != 1 {
		t.Fatalf("expected 1 tag suggestion, got %d", len(s.Output.Tags))
	}

	// worksheets ensured with right modes
	if wsRepo.ensured[dogdrip.ID] != "auto" || wsRepo.ensured[theqoo.ID] != "manual" || wsRepo.ensured[nocomm.ID] != "manual" {
		t.Fatalf("worksheet modes wrong: %+v", wsRepo.ensured)
	}
}
