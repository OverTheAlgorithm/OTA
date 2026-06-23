// Package communitytrend collects and aggregates the trending "topics" of
// Korean online communities as derived tag counts. It shares no business
// logic with the news collector package.
package communitytrend

import "time"

// Axis groups tags into a dimension (e.g. 성향축, 정치논제축).
type Axis struct {
	ID           int    `json:"id"`
	Key          string `json:"key"`
	Label        string `json:"label"`
	DisplayOrder int    `json:"display_order"`
}

// Tag is a single classification label belonging to one axis.
// The same tag may be attached as community meta (성향) or daily topic (decisions.md D-003).
type Tag struct {
	ID          int       `json:"id"`
	AxisID      int       `json:"axis_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedBy   string    `json:"created_by"` // 'seed' | 'ai' | 'admin'
	CreatedAt   time.Time `json:"created_at"`
}

// Community is a tracked site. Key is the natural identifier linking to a
// code-side SourceAdapter (decisions.md D-004).
type Community struct {
	ID         int       `json:"id"`
	Key        string    `json:"key"`
	Name       string    `json:"name"`
	HomeURL    string    `json:"home_url"`
	Enabled    bool      `json:"enabled"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	MetaTagIDs []int     `json:"meta_tag_ids"` // populated on read; cohort dimension
}
