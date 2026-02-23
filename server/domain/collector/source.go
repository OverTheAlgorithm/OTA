package collector

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// TrendingItem represents a single trending topic collected from any source.
// Each source (Google Trends, News RSS, etc.) produces these items.
type TrendingItem struct {
	Keyword       string    // trending keyword or headline
	Source        string    // source identifier: "google_trends", "news_yonhap", etc.
	Traffic       int       // raw search volume (Google Trends: "500+" → 500)
	Category      string    // topic category if available from source
	ArticleURLs   []string  // related article URLs (verified, real URLs)
	ArticleTitles []string  // related article titles (parallel to ArticleURLs)
	PublishedAt   time.Time // when the trend/article was published
}

// SourceCollector fetches trending topics from a specific data source.
// Implementations must be safe for concurrent use.
type SourceCollector interface {
	Name() string
	Collect(ctx context.Context) ([]TrendingItem, error)
}

// TrendingItemRepository persists raw trending data for tracking and analysis.
type TrendingItemRepository interface {
	SaveTrendingItems(ctx context.Context, runID uuid.UUID, items []TrendingItem) error
	GetTrendingItemsByRunID(ctx context.Context, runID uuid.UUID) ([]TrendingItem, error)
}
