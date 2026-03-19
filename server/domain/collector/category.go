package collector

import "context"

// Category represents a news category (e.g. general, entertainment, business).
type Category struct {
	Key          string `json:"key"`
	Label        string `json:"label"`
	DisplayOrder int    `json:"display_order"`
}

// NewsSource represents a category-based RSS feed URL.
type NewsSource struct {
	ID          int    `json:"id"`
	CategoryKey string `json:"category_key"`
	Provider    string `json:"provider"`
	URL         string `json:"url"`
	Enabled     bool   `json:"enabled"`
}

// CategoryRepository provides access to categories and news sources.
type CategoryRepository interface {
	GetAllCategories(ctx context.Context) ([]Category, error)
	GetEnabledNewsSources(ctx context.Context) ([]NewsSource, error)
}
