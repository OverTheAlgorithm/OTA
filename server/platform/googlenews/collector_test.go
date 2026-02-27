package googlenews

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ota/domain/collector"
)

const sampleGoogleNewsRSS = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:media="http://search.yahoo.com/mrss/">
  <channel>
    <title>주요 뉴스 - Google 뉴스</title>
    <link>https://news.google.com/?hl=ko&amp;gl=KR&amp;ceid=KR:ko</link>
    <item>
      <title>정부, 엘리엇 ISDS 취소소송 승소 - 경향신문</title>
      <link>https://news.google.com/rss/articles/CBMi_ARTICLE_1</link>
      <guid isPermaLink="false">CBMi_GUID_1</guid>
      <pubDate>Mon, 23 Feb 2026 12:00:00 GMT</pubDate>
      <description>&lt;ol&gt;&lt;li&gt;&lt;a href="https://news.google.com/rss/articles/CBMi_ARTICLE_1" target="_blank"&gt;정부, 엘리엇 ISDS 취소소송 승소&lt;/a&gt;&amp;nbsp;&amp;nbsp;&lt;font color="#6f6f6f"&gt;경향신문&lt;/font&gt;&lt;/li&gt;&lt;li&gt;&lt;a href="https://news.google.com/rss/articles/CBMi_ARTICLE_2" target="_blank"&gt;엘리엇 ISDS 배상소송 뒤집혀&lt;/a&gt;&amp;nbsp;&amp;nbsp;&lt;font color="#6f6f6f"&gt;연합뉴스TV&lt;/font&gt;&lt;/li&gt;&lt;li&gt;&lt;a href="https://news.google.com/rss/articles/CBMi_ARTICLE_3" target="_blank"&gt;론스타 이어 엘리엇 승소&lt;/a&gt;&amp;nbsp;&amp;nbsp;&lt;font color="#6f6f6f"&gt;한겨레&lt;/font&gt;&lt;/li&gt;&lt;/ol&gt;</description>
      <source url="https://www.khan.co.kr">경향신문</source>
    </item>
    <item>
      <title>한-브라질 전략적 동반자 관계 격상 - 한겨레</title>
      <link>https://news.google.com/rss/articles/CBMi_ARTICLE_4</link>
      <guid isPermaLink="false">CBMi_GUID_2</guid>
      <pubDate>Mon, 23 Feb 2026 09:00:00 GMT</pubDate>
      <description>&lt;ol&gt;&lt;li&gt;&lt;a href="https://news.google.com/rss/articles/CBMi_ARTICLE_4" target="_blank"&gt;한-브라질 전략적 동반자 관계 격상&lt;/a&gt;&amp;nbsp;&amp;nbsp;&lt;font color="#6f6f6f"&gt;한겨레&lt;/font&gt;&lt;/li&gt;&lt;li&gt;&lt;a href="https://news.google.com/rss/articles/CBMi_ARTICLE_5" target="_blank"&gt;룰라 대통령 국빈 방한&lt;/a&gt;&amp;nbsp;&amp;nbsp;&lt;font color="#6f6f6f"&gt;연합뉴스&lt;/font&gt;&lt;/li&gt;&lt;/ol&gt;</description>
      <source url="https://www.hani.co.kr">한겨레</source>
    </item>
    <item>
      <title>코스피 2800선 회복 - 매일경제</title>
      <link>https://news.google.com/rss/articles/CBMi_ARTICLE_6</link>
      <guid isPermaLink="false">CBMi_GUID_3</guid>
      <pubDate>Mon, 23 Feb 2026 06:00:00 GMT</pubDate>
      <description>&lt;ol&gt;&lt;li&gt;&lt;a href="https://news.google.com/rss/articles/CBMi_ARTICLE_6" target="_blank"&gt;코스피 2800선 회복&lt;/a&gt;&amp;nbsp;&amp;nbsp;&lt;font color="#6f6f6f"&gt;매일경제&lt;/font&gt;&lt;/li&gt;&lt;/ol&gt;</description>
      <source url="https://www.mk.co.kr">매일경제</source>
    </item>
  </channel>
</rss>`

func TestParseFeed_ValidRSS(t *testing.T) {
	items, err := ParseFeed([]byte(sampleGoogleNewsRSS), "general")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}

	// First item: 엘리엇 ISDS with 3 related articles
	first := items[0]
	if first.Keyword != "정부, 엘리엇 ISDS 취소소송 승소" {
		t.Errorf("expected keyword without media suffix, got '%s'", first.Keyword)
	}
	if first.Source != "google_news" {
		t.Errorf("expected source 'google_news', got '%s'", first.Source)
	}
	if first.Category != "general" {
		t.Errorf("expected category 'general', got '%s'", first.Category)
	}
	if len(first.ArticleURLs) != 3 {
		t.Fatalf("expected 3 article URLs, got %d", len(first.ArticleURLs))
	}
	if first.ArticleTitles[0] != "정부, 엘리엇 ISDS 취소소송 승소" {
		t.Errorf("unexpected first article title: %s", first.ArticleTitles[0])
	}
	if first.ArticleTitles[1] != "엘리엇 ISDS 배상소송 뒤집혀" {
		t.Errorf("unexpected second article title: %s", first.ArticleTitles[1])
	}

	// Second item: 한-브라질 with 2 related articles
	second := items[1]
	if second.Keyword != "한-브라질 전략적 동반자 관계 격상" {
		t.Errorf("unexpected keyword: '%s'", second.Keyword)
	}
	if len(second.ArticleURLs) != 2 {
		t.Errorf("expected 2 article URLs, got %d", len(second.ArticleURLs))
	}

	// Third item: 코스피 with 1 article
	third := items[2]
	if third.Keyword != "코스피 2800선 회복" {
		t.Errorf("unexpected keyword: '%s'", third.Keyword)
	}
	if len(third.ArticleURLs) != 1 {
		t.Errorf("expected 1 article URL, got %d", len(third.ArticleURLs))
	}
}

func TestParseFeed_EmptyChannel(t *testing.T) {
	emptyRSS := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0"><channel><title>Empty</title></channel></rss>`

	items, err := ParseFeed([]byte(emptyRSS), "general")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

func TestParseFeed_InvalidXML(t *testing.T) {
	_, err := ParseFeed([]byte("not xml"), "general")
	if err == nil {
		t.Error("expected error for invalid XML")
	}
}

func TestStripMediaSuffix(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"정부, 엘리엇 ISDS 취소소송 승소 - 경향신문", "정부, 엘리엇 ISDS 취소소송 승소"},
		{"RTX 5090 출시 - ZDNet Korea", "RTX 5090 출시"},
		{"제목에 대시 없음", "제목에 대시 없음"},
		{"A - B - C", "A - B"},       // last " - " is stripped
		{"", ""},                       // empty
		{"  spaces  - media  ", "spaces"}, // trimmed
	}
	for _, tt := range tests {
		got := stripMediaSuffix(tt.input)
		if got != tt.want {
			t.Errorf("stripMediaSuffix(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestParseDescriptionArticles(t *testing.T) {
	desc := `<ol><li><a href="https://example.com/1" target="_blank">Title One</a>&nbsp;&nbsp;<font color="#6f6f6f">Media1</font></li><li><a href="https://example.com/2" target="_blank">Title Two</a>&nbsp;&nbsp;<font color="#6f6f6f">Media2</font></li></ol>`

	urls, titles := parseDescriptionArticles(desc)
	if len(urls) != 2 {
		t.Fatalf("expected 2 URLs, got %d", len(urls))
	}
	if urls[0] != "https://example.com/1" {
		t.Errorf("unexpected URL: %s", urls[0])
	}
	if titles[1] != "Title Two" {
		t.Errorf("unexpected title: %s", titles[1])
	}
}

func TestParseDescriptionArticles_Empty(t *testing.T) {
	urls, titles := parseDescriptionArticles("")
	if len(urls) != 0 || len(titles) != 0 {
		t.Error("expected empty results for empty description")
	}
}

func TestDedup(t *testing.T) {
	items := []collector.TrendingItem{
		{Keyword: "Topic A", Source: "google_news", Category: "general"},
		{Keyword: "Topic B", Source: "google_news", Category: "general"},
		{Keyword: "topic a", Source: "google_news", Category: "business"}, // duplicate (case-insensitive)
		{Keyword: "Topic C", Source: "google_news", Category: "sports"},
	}
	result := dedup(items)
	if len(result) != 3 {
		t.Fatalf("expected 3 unique items, got %d", len(result))
	}
	// First occurrence wins
	if result[0].Category != "general" {
		t.Errorf("expected first occurrence (general), got %s", result[0].Category)
	}
}

func TestCollector_Name(t *testing.T) {
	c := NewCollector(nil)
	if c.Name() != "google_news" {
		t.Errorf("expected name 'google_news', got '%s'", c.Name())
	}
}

func TestCollector_Collect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(sampleGoogleNewsRSS))
	}))
	defer server.Close()

	topics := []FeedTopic{
		{Category: "general", URL: server.URL},
	}
	c := &Collector{
		topics:     topics,
		client:     server.Client(),
	}

	items, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("collect failed: %v", err)
	}
	if len(items) != 3 {
		t.Errorf("expected 3 items, got %d", len(items))
	}
}

func TestCollector_MultipleFeedsDedup(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(sampleGoogleNewsRSS))
	}))
	defer server.Close()

	// Two feeds returning the same content → should dedup
	topics := []FeedTopic{
		{Category: "general", URL: server.URL},
		{Category: "business", URL: server.URL},
	}
	c := &Collector{
		topics:     topics,
		client:     server.Client(),
	}

	items, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("collect failed: %v", err)
	}
	// Same 3 items from both feeds, deduped to 3
	if len(items) != 3 {
		t.Errorf("expected 3 deduped items, got %d", len(items))
	}
}

func TestCollector_AllFeedsFail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	topics := []FeedTopic{
		{Category: "general", URL: server.URL},
	}
	c := &Collector{
		topics: topics,
		client: server.Client(),
	}

	_, err := c.Collect(context.Background())
	if err == nil {
		t.Error("expected error when all feeds fail")
	}
}

func TestCollector_PartialFailure(t *testing.T) {
	goodServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(sampleGoogleNewsRSS))
	}))
	defer goodServer.Close()

	badServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer badServer.Close()

	topics := []FeedTopic{
		{Category: "general", URL: goodServer.URL},
		{Category: "business", URL: badServer.URL},
	}
	c := &Collector{
		topics:     topics,
		client:     &http.Client{Timeout: 5 * time.Second},
	}

	items, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("expected no error on partial failure, got: %v", err)
	}
	if len(items) != 3 {
		t.Errorf("expected 3 items from good feed, got %d", len(items))
	}
}

func TestParseFeed_PubDateParsing(t *testing.T) {
	items, err := ParseFeed([]byte(sampleGoogleNewsRSS), "general")
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
