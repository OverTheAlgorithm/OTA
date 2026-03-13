package collector

import (
	"context"
	"fmt"
	"log"
	"strings"
)

// Aggregator runs source collectors and produces
// a formatted text block suitable for AI prompt input.
type Aggregator struct {
	trends SourceCollector
	news   SourceCollector
}

// NewAggregator creates an Aggregator with dedicated trends and news collectors.
func NewAggregator(trends, news SourceCollector) *Aggregator {
	return &Aggregator{trends: trends, news: news}
}

// CollectedData holds the raw trending items and the AI-ready formatted text.
type CollectedData struct {
	Items         []TrendingItem // all collected items across sources
	FormattedText string         // human-readable text for AI prompt
}

// Collect runs trends and news collectors, formats each separately, and merges.
// Partial failures are tolerated — if at least one source succeeds, results are returned.
func (a *Aggregator) Collect(ctx context.Context) (CollectedData, error) {
	type result struct {
		items []TrendingItem
		err   error
	}

	trendsCh := make(chan result, 1)
	newsCh := make(chan result, 1)

	go func() {
		items, err := a.trends.Collect(ctx)
		trendsCh <- result{items: items, err: err}
	}()
	go func() {
		items, err := a.news.Collect(ctx)
		newsCh <- result{items: items, err: err}
	}()

	trendsResult := <-trendsCh
	newsResult := <-newsCh

	var allItems []TrendingItem
	var errs []string
	var parts []string

	if trendsResult.err != nil {
		log.Printf("source %s failed: %v", a.trends.Name(), trendsResult.err)
		errs = append(errs, fmt.Sprintf("%s: %v", a.trends.Name(), trendsResult.err))
	} else {
		log.Printf("source %s: %d items collected", a.trends.Name(), len(trendsResult.items))
		allItems = append(allItems, trendsResult.items...)
		parts = append(parts, FormatTrends(trendsResult.items))
	}

	if newsResult.err != nil {
		log.Printf("source %s failed: %v", a.news.Name(), newsResult.err)
		errs = append(errs, fmt.Sprintf("%s: %v", a.news.Name(), newsResult.err))
	} else {
		log.Printf("source %s: %d items collected", a.news.Name(), len(newsResult.items))
		allItems = append(allItems, newsResult.items...)
		parts = append(parts, FormatNews(newsResult.items))
	}

	if len(allItems) == 0 {
		return CollectedData{}, fmt.Errorf("all sources failed: %s", strings.Join(errs, "; "))
	}

	return CollectedData{
		Items:         allItems,
		FormattedText: strings.Join(parts, "\n"),
	}, nil
}

// FormatTrends formats Google Trends items into a structured text block for AI.
func FormatTrends(items []TrendingItem) string {
	if len(items) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("## Source: google_trends (%d items)\n\n", len(items)))

	for i, item := range items {
		b.WriteString(fmt.Sprintf("%d. **%s**", i+1, item.Keyword))
		if item.Traffic > 0 {
			b.WriteString(fmt.Sprintf(" [traffic: %d]", item.Traffic))
		}
		b.WriteString("\n")

		if len(item.ArticleURLs) > 0 {
			b.WriteString(fmt.Sprintf("   Related articles (%d):\n", len(item.ArticleURLs)))
			for j, url := range item.ArticleURLs {
				title := ""
				if j < len(item.ArticleTitles) {
					title = item.ArticleTitles[j]
				}
				if title != "" {
					b.WriteString(fmt.Sprintf("   - %s (%s)\n", title, url))
				} else {
					b.WriteString(fmt.Sprintf("   - %s\n", url))
				}
			}
		}
		b.WriteString("\n")
	}

	return b.String()
}

// FormatNews formats Google News items into a structured text block for AI.
func FormatNews(items []TrendingItem) string {
	if len(items) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("## Source: google_news (%d items)\n\n", len(items)))

	for i, item := range items {
		b.WriteString(fmt.Sprintf("%d. **%s**", i+1, item.Keyword))
		if item.Category != "" {
			b.WriteString(fmt.Sprintf(" [category: %s]", item.Category))
		}
		b.WriteString("\n")

		if len(item.ArticleURLs) > 0 {
			b.WriteString(fmt.Sprintf("   Related articles (%d):\n", len(item.ArticleURLs)))
			for j, url := range item.ArticleURLs {
				title := ""
				if j < len(item.ArticleTitles) {
					title = item.ArticleTitles[j]
				}
				if title != "" {
					b.WriteString(fmt.Sprintf("   - %s (%s)\n", title, url))
				} else {
					b.WriteString(fmt.Sprintf("   - %s\n", url))
				}
			}
		}
		b.WriteString("\n")
	}

	return b.String()
}
