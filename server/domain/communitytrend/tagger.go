package communitytrend

import "context"

// TaxonomyTag is one tag presented to the AI, with its axis for context.
type TaxonomyTag struct {
	ID      int
	AxisKey string
	Name    string
}

// MemeRef is a confirmed meme (name + aliases) the AI matches against.
type MemeRef struct {
	ID      int
	Name    string
	Aliases []string
}

// TaggerInput is the single-pass analysis request (decisions.md D-009):
// topic tagging + meme matching + meme-candidate discovery in one call.
type TaggerInput struct {
	CommunityKey string
	Titles       []string // fresh item TextUnits (transient; never persisted)
	Taxonomy     []TaxonomyTag
	Memes        []MemeRef
	Blacklist    []string // expressions never to re-propose as meme candidates
	MinCount     int      // conservative threshold (CT_MIN_TAG_COUNT)
}

// TagSuggestion is a proposed topic tag. Existing tags carry TagID>0; brand-new
// proposals carry TagID==0 with NewAxisKey set, pending human creation+confirm.
type TagSuggestion struct {
	TagID      int     `json:"tag_id"`
	Name       string  `json:"name"`
	AxisKey    string  `json:"axis_key"`
	Count      float64 `json:"count"`
	IsNew      bool    `json:"is_new"`
	NewAxisKey string  `json:"new_axis_key"`
	PostIndices []int  `json:"post_indices"`
}

// MemeMatch is a confirmed meme detected in the titles.
type MemeMatch struct {
	MemeID      int   `json:"meme_id"`
	Name        string `json:"name"`
	Count       int    `json:"count"`
	PostIndices []int  `json:"post_indices"`
}

// MemeCandidate is a novel repeated expression not in the taxonomy/memes/blacklist.
type MemeCandidate struct {
	Expression  string `json:"expression"`
	HitCount    int    `json:"hit_count"`
	PostIndices []int  `json:"post_indices"`
}

// TaggerOutput is the structured single-pass result.
type TaggerOutput struct {
	Tags           []TagSuggestion `json:"tags"`
	MemeMatches    []MemeMatch     `json:"meme_matches"`
	MemeCandidates []MemeCandidate `json:"meme_candidates"`
}

// Tagger runs the AI single-pass analysis. Implemented in platform/gemini;
// the domain depends only on this interface (no AI SDK import here).
type Tagger interface {
	Analyze(ctx context.Context, in TaggerInput) (TaggerOutput, error)
}
