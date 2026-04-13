package collector

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type RunStatus string

const (
	RunStatusRunning RunStatus = "running"
	RunStatusSuccess RunStatus = "success"
	RunStatusFailed  RunStatus = "failed"
)

type CollectionRun struct {
	ID           uuid.UUID
	StartedAt    time.Time
	CompletedAt  *time.Time
	Status       RunStatus
	ErrorMessage *string
	RawResponse  *string
}

// DetailItem represents a single detail entry with a title (short heading) and content (expanded text).
type DetailItem struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

type ContextItem struct {
	ID              uuid.UUID
	CollectionRunID uuid.UUID
	Category        string       `json:"category"`
	Priority        string       `json:"priority"`
	BrainCategory   string       `json:"brain_category"`
	Rank            int          `json:"rank"`
	Topic           string       `json:"topic"`
	Summary         string       `json:"summary"`
	Detail          string       `json:"detail"`
	Details         []DetailItem `json:"details"`
	BuzzScore       int          `json:"buzz_score"`
	Sources         []string     `json:"sources"`
	ImagePath       *string      `json:"-"` // local file path, not included in AI JSON
}

type CollectionResult struct {
	Run   CollectionRun
	Items []ContextItem
}

// Phase1Topic is a single topic from Phase 1 AI clustering response.
type Phase1Topic struct {
	TopicHint     string   `json:"topic_hint"`
	Category      string   `json:"category"`
	Priority      string   `json:"priority"`
	BrainCategory string   `json:"brain_category"`
	BuzzScore     int      `json:"buzz_score"`
	Sources       []string `json:"sources"`
}

// Phase2Result is the AI-written content for a single topic from Phase 2.
// Poll is optional — nil means the topic is not opinion-worthy and no poll row is created.
type Phase2Result struct {
	Topic   string       `json:"topic"`
	Summary string       `json:"summary"`
	Detail  string       `json:"detail"`
	Details []DetailItem `json:"details"`
	Poll    *PollData    `json:"poll"`
}

// QuizData is the AI-generated quiz for a single topic, parsed from a separate AI call.
// It is NOT part of Phase2Result — quiz generation is a separate pipeline stage.
type QuizData struct {
	Question     string   `json:"question"`
	Options      []string `json:"options"`
	CorrectIndex int      `json:"correct_index"`
}

// PollData is the optional opinion-poll payload embedded in Phase2Result.
// Produced inline by the Phase 2 AI prompt; validated + persisted after Stage 5.
type PollData struct {
	Question string   `json:"question"`
	Options  []string `json:"options"`
}

// FetchedArticle holds the plain-text body fetched from a source URL.
type FetchedArticle struct {
	URL  string `json:"url"`
	Body string `json:"body"`  // plain text, max 3000 chars
	Err  error  `json:"-"`     // non-nil if fetch failed; excluded from serialization
}

// UnmarshalDetails decodes a JSON blob into []DetailItem with backward
// compatibility: old data stored as ["string", ...] is converted to
// [{Title: "string", Content: ""}].
func UnmarshalDetails(data []byte) []DetailItem {
	var items []DetailItem
	if err := json.Unmarshal(data, &items); err == nil {
		return items
	}
	// Fallback: old format was a plain string array.
	var strings []string
	if err := json.Unmarshal(data, &strings); err == nil && len(strings) > 0 {
		items = make([]DetailItem, len(strings))
		for i, s := range strings {
			items[i] = DetailItem{Title: s, Content: ""}
		}
		return items
	}
	return nil
}
