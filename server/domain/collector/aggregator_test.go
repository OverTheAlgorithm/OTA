package collector

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

// mockSourceCollector implements SourceCollector for testing.
type mockSourceCollector struct {
	name  string
	items []TrendingItem
	err   error
}

func (m *mockSourceCollector) Name() string { return m.name }
func (m *mockSourceCollector) Collect(_ context.Context) ([]TrendingItem, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.items, nil
}

func TestAggregator_Collect_Success(t *testing.T) {
	trends := &mockSourceCollector{
		name: "google_trends",
		items: []TrendingItem{
			{Keyword: "RTX 5090", Source: "google_trends", Traffic: 500, ArticleURLs: []string{"https://example.com/1"}},
			{Keyword: "코스피", Source: "google_trends", Traffic: 200, ArticleURLs: []string{"https://example.com/2"}},
		},
	}
	news := &mockSourceCollector{
		name: "google_news",
		items: []TrendingItem{
			{Keyword: "엔비디아 RTX 5090 출시", Source: "google_news", Category: "technology", ArticleURLs: []string{"https://news.google.com/1", "https://news.google.com/2"}},
			{Keyword: "코스피 신고가 경신", Source: "google_news", Category: "business", ArticleURLs: []string{"https://news.google.com/3"}},
		},
	}

	agg := NewAggregator(trends, news)
	data, err := agg.Collect(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(data.Items) != 4 {
		t.Errorf("expected 4 items, got %d", len(data.Items))
	}

	if data.FormattedText == "" {
		t.Error("expected non-empty formatted text")
	}

	if !strings.Contains(data.FormattedText, "## Source: google_trends") {
		t.Error("formatted text missing google_trends header")
	}
	if !strings.Contains(data.FormattedText, "## Source: google_news") {
		t.Error("formatted text missing google_news header")
	}
}

func TestAggregator_Collect_PartialFailure(t *testing.T) {
	good := &mockSourceCollector{
		name: "google_trends",
		items: []TrendingItem{
			{Keyword: "test", Source: "google_trends", Traffic: 100},
		},
	}
	bad := &mockSourceCollector{
		name:  "google_news",
		items: nil,
		err:   fmt.Errorf("network error"),
	}

	agg := NewAggregator(good, bad)
	data, err := agg.Collect(context.Background())
	if err != nil {
		t.Fatalf("expected success on partial failure, got: %v", err)
	}

	if len(data.Items) != 1 {
		t.Errorf("expected 1 item from good source, got %d", len(data.Items))
	}
}

func TestAggregator_Collect_AllFail(t *testing.T) {
	bad1 := &mockSourceCollector{name: "source1", err: fmt.Errorf("fail1")}
	bad2 := &mockSourceCollector{name: "source2", err: fmt.Errorf("fail2")}

	agg := NewAggregator(bad1, bad2)
	_, err := agg.Collect(context.Background())
	if err == nil {
		t.Error("expected error when all sources fail")
	}
}

func TestFormatTrends(t *testing.T) {
	items := []TrendingItem{
		{
			Keyword:       "RTX 5090",
			Source:        "google_trends",
			Traffic:       500,
			ArticleURLs:   []string{"https://example.com/rtx"},
			ArticleTitles: []string{"RTX 5090 출시"},
			PublishedAt:   time.Date(2026, 2, 23, 0, 0, 0, 0, time.UTC),
		},
		{
			Keyword: "날씨",
			Source:  "google_trends",
			Traffic: 200,
		},
	}

	result := FormatTrends(items)

	if !strings.Contains(result, "## Source: google_trends (2 items)") {
		t.Error("missing google_trends header with count")
	}
	if !strings.Contains(result, "**RTX 5090** [traffic: 500]") {
		t.Error("missing RTX 5090 with traffic")
	}
	if !strings.Contains(result, "Related articles (1)") {
		t.Error("missing related articles count")
	}
}

func TestFormatNews(t *testing.T) {
	items := []TrendingItem{
		{
			Keyword:       "엔비디아 신제품",
			Source:        "google_news",
			Category:      "technology",
			ArticleURLs:   []string{"https://news.google.com/1", "https://news.google.com/2"},
			ArticleTitles: []string{"Title 1", "Title 2"},
		},
	}

	result := FormatNews(items)

	if !strings.Contains(result, "## Source: google_news (1 items)") {
		t.Error("missing google_news header with count")
	}
	if !strings.Contains(result, "[category: technology]") {
		t.Error("missing category tag")
	}
	if !strings.Contains(result, "Related articles (2)") {
		t.Error("missing related articles count")
	}
}

func TestFormatTrends_Empty(t *testing.T) {
	result := FormatTrends(nil)
	if result != "" {
		t.Errorf("expected empty string for nil items, got: %q", result)
	}
}

func TestFormatNews_Empty(t *testing.T) {
	result := FormatNews(nil)
	if result != "" {
		t.Errorf("expected empty string for nil items, got: %q", result)
	}
}
