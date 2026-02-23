package collector

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// Aggregator runs all SourceCollectors in parallel and produces
// a formatted text block suitable for AI prompt input.
type Aggregator struct {
	collectors []SourceCollector
}

// NewAggregator creates an Aggregator with the given source collectors.
func NewAggregator(collectors []SourceCollector) *Aggregator {
	return &Aggregator{collectors: collectors}
}

// CollectedData holds the raw trending items and the AI-ready formatted text.
type CollectedData struct {
	Items         []TrendingItem // all collected items across sources
	FormattedText string         // human-readable text for AI prompt
}

// Collect runs all source collectors in parallel and returns merged data.
// Partial failures are tolerated — if at least one source succeeds, results are returned.
func (a *Aggregator) Collect(ctx context.Context) (CollectedData, error) {
	type result struct {
		items []TrendingItem
		err   error
	}

	var wg sync.WaitGroup
	results := make([]result, len(a.collectors))

	for i, c := range a.collectors {
		wg.Add(1)
		go func(idx int, sc SourceCollector) {
			defer wg.Done()
			items, err := sc.Collect(ctx)
			results[idx] = result{items: items, err: err}
		}(i, c)
	}
	wg.Wait()

	var allItems []TrendingItem
	var errs []string
	for i, r := range results {
		if r.err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", a.collectors[i].Name(), r.err))
			continue
		}
		allItems = append(allItems, r.items...)
	}

	if len(allItems) == 0 {
		return CollectedData{}, fmt.Errorf("all sources failed: %s", strings.Join(errs, "; "))
	}

	formatted := FormatForAI(allItems)
	return CollectedData{
		Items:         allItems,
		FormattedText: formatted,
	}, nil
}

// FormatForAI converts collected trending items into a structured text block
// that provides the AI with concrete data for clustering and summarization.
func FormatForAI(items []TrendingItem) string {
	var b strings.Builder

	// Group by source
	bySource := make(map[string][]TrendingItem)
	var sourceOrder []string
	for _, item := range items {
		if _, exists := bySource[item.Source]; !exists {
			sourceOrder = append(sourceOrder, item.Source)
		}
		bySource[item.Source] = append(bySource[item.Source], item)
	}

	for _, source := range sourceOrder {
		sourceItems := bySource[source]
		b.WriteString(fmt.Sprintf("## Source: %s (%d items)\n\n", source, len(sourceItems)))

		for i, item := range sourceItems {
			b.WriteString(fmt.Sprintf("%d. **%s**", i+1, item.Keyword))
			if item.Traffic > 0 {
				b.WriteString(fmt.Sprintf(" [traffic: %d]", item.Traffic))
			}
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
	}

	return b.String()
}
