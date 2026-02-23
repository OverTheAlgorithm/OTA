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
	BrainCategory   string       `json:"brain_category"`
	Rank            int          `json:"rank"`
	Topic           string       `json:"topic"`
	Summary         string       `json:"summary"`
	Detail          string       `json:"detail"`
	Details         []DetailItem `json:"details"`
	BuzzScore       int          `json:"buzz_score"`
	Sources         []string     `json:"sources"`
}

type CollectionResult struct {
	Run   CollectionRun
	Items []ContextItem
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
