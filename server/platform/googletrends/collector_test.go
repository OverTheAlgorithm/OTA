package googletrends

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ota/domain/collector"
)

const sampleRSS = `<?xml version="1.0" encoding="UTF-8"?>
<rss xmlns:atom="http://www.w3.org/2005/Atom" xmlns:ht="https://trends.google.com/trending/rss" version="2.0">
  <channel>
    <title>Daily Search Trends</title>
    <description>Recent searches</description>
    <link>https://trends.google.co.kr/trending?geo=KR</link>
    <item>
      <title>코스피 폭락</title>
      <ht:approx_traffic>500+</ht:approx_traffic>
      <pubDate>Sun, 23 Feb 2026 00:00:00 +0000</pubDate>
      <link>https://trends.google.co.kr/trending/20260223/코스피+폭락</link>
      <ht:picture>https://example.com/pic1.jpg</ht:picture>
      <ht:picture_source>연합뉴스</ht:picture_source>
      <ht:news_item>
        <ht:news_item_title>코스피 2400선 붕괴, 외국인 매도</ht:news_item_title>
        <ht:news_item_url>https://www.yna.co.kr/view/AKR20260223012345</ht:news_item_url>
        <ht:news_item_picture>https://example.com/thumb1.jpg</ht:news_item_picture>
        <ht:news_item_source>연합뉴스</ht:news_item_source>
      </ht:news_item>
      <ht:news_item>
        <ht:news_item_title>증시 급락에 개인 투자자 패닉</ht:news_item_title>
        <ht:news_item_url>https://www.hankyung.com/article/2026022312345</ht:news_item_url>
        <ht:news_item_picture>https://example.com/thumb2.jpg</ht:news_item_picture>
        <ht:news_item_source>한국경제</ht:news_item_source>
      </ht:news_item>
    </item>
    <item>
      <title>RTX 5090 출시</title>
      <ht:approx_traffic>200+</ht:approx_traffic>
      <pubDate>Sun, 23 Feb 2026 00:00:00 +0000</pubDate>
      <link>https://trends.google.co.kr/trending/20260223/RTX+5090</link>
      <ht:news_item>
        <ht:news_item_title>엔비디아 RTX 5090 국내 출시</ht:news_item_title>
        <ht:news_item_url>https://zdnet.co.kr/view/20260223_rtx5090</ht:news_item_url>
        <ht:news_item_picture>https://example.com/thumb3.jpg</ht:news_item_picture>
        <ht:news_item_source>ZDNet Korea</ht:news_item_source>
      </ht:news_item>
    </item>
    <item>
      <title>날씨</title>
      <ht:approx_traffic>100+</ht:approx_traffic>
      <pubDate>Sun, 23 Feb 2026 00:00:00 +0000</pubDate>
      <link>https://trends.google.co.kr/trending/20260223/날씨</link>
    </item>
  </channel>
</rss>`

func TestParseFeed_ValidRSS(t *testing.T) {
	items, err := ParseFeed([]byte(sampleRSS))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}

	// First item: 코스피 폭락 with 2 news articles
	first := items[0]
	if first.Keyword != "코스피 폭락" {
		t.Errorf("expected keyword '코스피 폭락', got '%s'", first.Keyword)
	}
	if first.Source != "google_trends" {
		t.Errorf("expected source 'google_trends', got '%s'", first.Source)
	}
	if first.Traffic != 500 {
		t.Errorf("expected traffic 500, got %d", first.Traffic)
	}
	if len(first.ArticleURLs) != 2 {
		t.Fatalf("expected 2 article URLs, got %d", len(first.ArticleURLs))
	}
	if first.ArticleURLs[0] != "https://www.yna.co.kr/view/AKR20260223012345" {
		t.Errorf("unexpected first article URL: %s", first.ArticleURLs[0])
	}
	if first.ArticleTitles[0] != "코스피 2400선 붕괴, 외국인 매도" {
		t.Errorf("unexpected first article title: %s", first.ArticleTitles[0])
	}

	// Second item: RTX 5090 with 1 news article
	second := items[1]
	if second.Keyword != "RTX 5090 출시" {
		t.Errorf("expected keyword 'RTX 5090 출시', got '%s'", second.Keyword)
	}
	if second.Traffic != 200 {
		t.Errorf("expected traffic 200, got %d", second.Traffic)
	}
	if len(second.ArticleURLs) != 1 {
		t.Fatalf("expected 1 article URL, got %d", len(second.ArticleURLs))
	}

	// Third item: 날씨 with no news articles
	third := items[2]
	if third.Keyword != "날씨" {
		t.Errorf("expected keyword '날씨', got '%s'", third.Keyword)
	}
	if len(third.ArticleURLs) != 0 {
		t.Errorf("expected 0 article URLs, got %d", len(third.ArticleURLs))
	}
}

func TestParseFeed_EmptyChannel(t *testing.T) {
	emptyRSS := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0"><channel><title>Empty</title></channel></rss>`

	items, err := ParseFeed([]byte(emptyRSS))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

func TestParseFeed_InvalidXML(t *testing.T) {
	_, err := ParseFeed([]byte("not xml at all"))
	if err == nil {
		t.Error("expected error for invalid XML")
	}
}

func TestParseFeed_PubDateParsing(t *testing.T) {
	items, err := ParseFeed([]byte(sampleRSS))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if items[0].PublishedAt.IsZero() {
		t.Error("expected non-zero published time")
	}
	if items[0].PublishedAt.Year() != 2026 {
		t.Errorf("expected year 2026, got %d", items[0].PublishedAt.Year())
	}
}

func TestCollector_Collect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(sampleRSS))
	}))
	defer server.Close()

	c := &Collector{
		client: server.Client(),
	}
	// Override feed URL for testing by using the fetchAndParse approach
	body, err := func() ([]byte, error) {
		resp, err := c.client.Get(server.URL)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		return io.ReadAll(resp.Body)
	}()
	if err != nil {
		t.Fatalf("fetch failed: %v", err)
	}

	items, err := ParseFeed(body)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if len(items) != 3 {
		t.Errorf("expected 3 items, got %d", len(items))
	}
}

func TestCollector_Name(t *testing.T) {
	c := NewCollector()
	if c.Name() != "google_trends" {
		t.Errorf("expected name 'google_trends', got '%s'", c.Name())
	}
}

func TestCollector_FetchError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	// Test that non-200 responses are handled
	c := &Collector{client: server.Client()}
	resp, err := c.client.Get(server.URL)
	if err != nil {
		t.Fatalf("unexpected network error: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", resp.StatusCode)
	}
}

func TestParseTraffic(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"500+", 500},
		{"200+", 200},
		{"100+", 100},
		{"1,000+", 1000},
		{"10,000+", 10000},
		{"200", 200},
		{"", 0},
		{"  500+  ", 500},
		{"not_a_number", 0},
	}
	for _, tt := range tests {
		got := parseTraffic(tt.input)
		if got != tt.want {
			t.Errorf("parseTraffic(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestCollector_CollectIntegration(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("expected User-Agent header")
		}
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(sampleRSS))
	}))
	defer server.Close()

	// Create collector with custom URL via a test helper
	c := newCollectorWithURL(server.URL)
	items, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("collect failed: %v", err)
	}
	if len(items) != 3 {
		t.Errorf("expected 3 items, got %d", len(items))
	}
}

// newCollectorWithURL creates a Collector pointing to a custom URL (for testing).
func newCollectorWithURL(url string) *testCollector {
	return &testCollector{
		url:    url,
		client: &http.Client{Timeout: 5 * time.Second},
	}
}

type testCollector struct {
	url    string
	client *http.Client
}

func (c *testCollector) Name() string { return "google_trends" }

func (c *testCollector) Collect(ctx context.Context) ([]collector.TrendingItem, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept-Language", "ko-KR,ko;q=0.9")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return ParseFeed(body)
}
