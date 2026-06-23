package communitytrend

import (
	"context"
	"testing"
)

// fakeCommunityRepo is an in-memory CommunityRepository for unit tests.
type fakeCommunityRepo struct {
	items    map[int]Community
	metaTags map[int][]int
	nextID   int
	keys     map[string]bool
}

func newFakeCommunityRepo() *fakeCommunityRepo {
	return &fakeCommunityRepo{
		items: map[int]Community{}, metaTags: map[int][]int{}, nextID: 1, keys: map[string]bool{},
	}
}

func (f *fakeCommunityRepo) Create(_ context.Context, c Community) (Community, error) {
	if f.keys[c.Key] {
		return Community{}, errDuplicate
	}
	c.ID = f.nextID
	f.nextID++
	f.items[c.ID] = c
	f.keys[c.Key] = true
	return c, nil
}
func (f *fakeCommunityRepo) List(_ context.Context) ([]Community, error) {
	var out []Community
	for _, c := range f.items {
		out = append(out, c)
	}
	return out, nil
}
func (f *fakeCommunityRepo) Update(_ context.Context, id int, name, homeURL string, enabled bool) (Community, error) {
	c := f.items[id]
	c.Name, c.HomeURL, c.Enabled = name, homeURL, enabled
	f.items[id] = c
	return c, nil
}
func (f *fakeCommunityRepo) Delete(_ context.Context, id int) error { delete(f.items, id); return nil }
func (f *fakeCommunityRepo) SetMetaTags(_ context.Context, id int, tagIDs []int) error {
	f.metaTags[id] = tagIDs
	return nil
}
func (f *fakeCommunityRepo) GetMetaTags(_ context.Context, id int) ([]int, error) {
	return f.metaTags[id], nil
}

// errDuplicate is a sentinel for the fake.
var errDuplicate = &dupErr{}

type dupErr struct{}

func (*dupErr) Error() string { return "duplicate key" }

func TestService_CreateCommunity_ValidatesKey(t *testing.T) {
	svc := NewService(newFakeCommunityRepo(), nil, nil)
	ctx := context.Background()

	tests := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{"valid lower alnum", "fmkorea", false},
		{"valid with dash", "mlb-park", false},
		{"valid with underscore", "the_qoo", false},
		{"empty", "", true},
		{"uppercase", "FmKorea", true},
		{"space", "fm korea", true},
		{"korean", "더쿠", true},
		{"slash", "fm/korea", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.CreateCommunity(ctx, Community{Key: tc.key, Name: "X"})
			if tc.wantErr && err == nil {
				t.Fatalf("key %q: expected error, got nil", tc.key)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("key %q: unexpected error %v", tc.key, err)
			}
		})
	}
}

func TestService_CreateCommunity_RequiresName(t *testing.T) {
	svc := NewService(newFakeCommunityRepo(), nil, nil)
	_, err := svc.CreateCommunity(context.Background(), Community{Key: "valid", Name: ""})
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestService_ListCommunities_PopulatesMetaTags(t *testing.T) {
	repo := newFakeCommunityRepo()
	svc := NewService(repo, nil, nil)
	ctx := context.Background()

	c, _ := svc.CreateCommunity(ctx, Community{Key: "k", Name: "N"})
	_ = svc.SetMetaTags(ctx, c.ID, []int{10, 20})

	list, err := svc.ListCommunities(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 || len(list[0].MetaTagIDs) != 2 {
		t.Fatalf("expected meta tags populated, got %+v", list)
	}
}
