package collector

import (
	"context"
	"time"
)

// BrainCategory represents an action-oriented label that tells users
// HOW to use a piece of information (e.g., "대화할 때 꺼내보세요").
type BrainCategory struct {
	Key          string    `json:"key"`
	Emoji        string    `json:"emoji"`
	Label        string    `json:"label"`
	AccentColor  string    `json:"accent_color"`
	DisplayOrder int       `json:"display_order"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// BrainCategoryRepository provides CRUD access to brain categories.
type BrainCategoryRepository interface {
	GetAll(ctx context.Context) ([]BrainCategory, error)
	Create(ctx context.Context, bc BrainCategory) error
	Update(ctx context.Context, bc BrainCategory) error
	Delete(ctx context.Context, key string) error
}
