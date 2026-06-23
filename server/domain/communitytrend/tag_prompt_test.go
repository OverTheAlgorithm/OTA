package communitytrend

import (
	"strings"
	"testing"
)

func TestBuildTagPrompt_InjectsTaxonomyAndRules(t *testing.T) {
	in := TaggerInput{
		CommunityKey: "fmkorea",
		Titles:       []string{"여성 무고 사건 또 터졌다", "정부 규제 비판"},
		Taxonomy: []TaxonomyTag{
			{ID: 11, AxisKey: "gender_topic", Name: "남성 인권"},
			{ID: 22, AxisKey: "political_topic", Name: "우파 지지"},
		},
		Memes:     []MemeRef{{ID: 1, Name: "킹받다", Aliases: []string{"킹받네"}}},
		Blacklist: []string{"노잼"},
		MinCount:  3,
	}
	p := BuildTagPrompt(in)

	mustContain := []string{
		"남성 인권", "우파 지지", // taxonomy tags
		"gender_topic", // axis key
		"3건 이상",        // conservative threshold from MinCount
		"우파 지지",        // precise naming example
		"킹받다", "킹받네",   // meme + alias
		"노잼",              // blacklist
		"여성 무고 사건 또 터졌다",  // a title
		"meme_candidates", // output schema
		"JSON",            // output instruction
	}
	for _, s := range mustContain {
		if !strings.Contains(p, s) {
			t.Errorf("prompt missing %q", s)
		}
	}
}

func TestBuildTagPrompt_EmptyTaxonomy(t *testing.T) {
	p := BuildTagPrompt(TaggerInput{Titles: []string{"x"}, MinCount: 5})
	if !strings.Contains(p, "(없음)") {
		t.Fatal("expected (없음) for empty taxonomy/memes")
	}
	if !strings.Contains(p, "5건 이상") {
		t.Fatal("expected threshold 5건 이상")
	}
}
