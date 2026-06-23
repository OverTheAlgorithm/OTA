package communitytrend

import "testing"

func TestFingerprint_StableAndScoped(t *testing.T) {
	a := Fingerprint("dogdrip", "123")
	if a != Fingerprint("dogdrip", "123") {
		t.Fatal("fingerprint not stable")
	}
	if a == Fingerprint("clien", "123") {
		t.Fatal("fingerprint should be scoped by community key")
	}
	if a == Fingerprint("dogdrip", "124") {
		t.Fatal("different source id should differ")
	}
	if len(a) != 64 {
		t.Fatalf("expected 64-hex sha256, got len %d", len(a))
	}
}

func TestFilterUnseen(t *testing.T) {
	items := []TrendItem{
		{SourceID: "1", TextUnit: "a"},
		{SourceID: "2", TextUnit: "b"},
		{SourceID: "3", TextUnit: "c"},
	}
	seen := map[string]bool{
		Fingerprint("dogdrip", "2"): true, // already counted on a prior day
	}
	fresh, fps := FilterUnseen("dogdrip", items, seen)
	if len(fresh) != 2 {
		t.Fatalf("expected 2 fresh (1,3), got %d", len(fresh))
	}
	if len(fps) != 2 {
		t.Fatalf("expected 2 fingerprints, got %d", len(fps))
	}
	for _, it := range fresh {
		if it.SourceID == "2" {
			t.Fatal("carryover item 2 should be filtered out")
		}
	}
}
