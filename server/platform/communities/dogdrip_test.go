package communities

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"
)

// TestDogdrip_ParseFixture verifies parsing against a saved real page (no network).
func TestDogdrip_ParseFixture(t *testing.T) {
	f, err := os.Open("fixtures/dogdrip_boomupbest.html")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()

	d := NewDogdrip()
	items, err := d.parse(f)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(items) < 10 {
		t.Fatalf("expected >=10 items from fixture, got %d", len(items))
	}

	for i, it := range items {
		if it.SourceID == "" {
			t.Fatalf("item %d: empty SourceID", i)
		}
		if _, err := strconv.Atoi(it.SourceID); err != nil {
			t.Fatalf("item %d: SourceID %q not numeric", i, it.SourceID)
		}
		if it.TextUnit == "" {
			t.Fatalf("item %d: empty TextUnit", i)
		}
	}

	// 중복 SourceID 없어야 함
	seen := map[string]bool{}
	for _, it := range items {
		if seen[it.SourceID] {
			t.Fatalf("duplicate SourceID %s", it.SourceID)
		}
		seen[it.SourceID] = true
	}

	t.Logf("parsed %d items; first: id=%s title=%q comments=%d",
		len(items), items[0].SourceID, items[0].TextUnit, items[0].Engagement["comments"])
}

// TestDogdrip_LiveSmoke hits the real site. Guarded by CT_LIVE_SMOKE=1 so CI stays
// deterministic; run manually to prove the adapter really works end-to-end.
func TestDogdrip_LiveSmoke(t *testing.T) {
	if os.Getenv("CT_LIVE_SMOKE") != "1" {
		t.Skip("set CT_LIVE_SMOKE=1 to run live adapter smoke test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	items, err := NewDogdrip().FetchRecent(ctx)
	if err != nil {
		t.Fatalf("live fetch: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("live fetch returned 0 items")
	}
	t.Logf("LIVE dogdrip: %d items; sample: id=%s title=%q comments=%d",
		len(items), items[0].SourceID, items[0].TextUnit, items[0].Engagement["comments"])
}
