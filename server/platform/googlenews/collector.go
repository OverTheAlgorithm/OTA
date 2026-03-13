package googlenews

import (
	"context"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"ota/domain/collector"
)

const userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

// FeedTopic defines a Google News topic feed to collect.
type FeedTopic struct {
	Category string // mapped category: "general", "entertainment", "business", etc.
	URL      string
}

// The hidden rules for google rss searching: https://www.newscatcherapi.com/blog-posts/google-news-rss-search-parameters-the-missing-documentaiton
// DefaultTopics returns the default Google News Korea topic feeds.
func DefaultTopics() []FeedTopic {
	return []FeedTopic{
		{Category: "general", URL: "https://news.google.com/rss?hl=ko&gl=KR&ceid=KR:ko"},
		{Category: "entertainment", URL: "https://news.google.com/rss/topics/CAAqJggKIiBDQkFTRWdvSUwyMHZNREpxYW5RU0FtdHZHZ0pMVWlnQVAB?hl=ko&gl=KR&ceid=KR:ko"},
		{Category: "business", URL: "https://news.google.com/rss/topics/CAAqJggKIiBDQkFTRWdvSUwyMHZNRGx6TVdZU0FtdHZHZ0pMVWlnQVAB?hl=ko&gl=KR&ceid=KR:ko"},
		{Category: "sports", URL: "https://news.google.com/rss/topics/CAAqJggKIiBDQkFTRWdvSUwyMHZNRFp1ZEdvU0FtdHZHZ0pMVWlnQVAB?hl=ko&gl=KR&ceid=KR:ko"},
		{Category: "technology", URL: "https://news.google.com/rss/topics/CAAqJggKIiBDQkFTRWdvSUwyMHZNRGRqTVhZU0FtdHZHZ0pMVWlnQVAB?hl=ko&gl=KR&ceid=KR:ko"},
		{Category: "science", URL: "https://news.google.com/rss/topics/CAAqJggKIiBDQkFTRWdvSUwyMHZNRFp0Y1RjU0FtdHZHZ0pMVWlnQVAB?hl=ko&gl=KR&ceid=KR:ko"},
		{Category: "health", URL: "https://news.google.com/rss/topics/CAAqIQgKIhtDQkFTRGdvSUwyMHZNR3QwTlRFU0FtdHZLQUFQAQ?hl=ko&gl=KR&ceid=KR:ko"},
	}
}

// Collector fetches news topics from Google News Korea RSS feeds.
type Collector struct {
	topics []FeedTopic
	client *http.Client
}

// NewCollector creates a Google News RSS collector.
func NewCollector(topics []FeedTopic) *Collector {
	return &Collector{
		topics: topics,
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *Collector) Name() string {
	return "google_news"
}

// Collect fetches all configured topic feeds in parallel and deduplicates by title.
func (c *Collector) Collect(ctx context.Context) ([]collector.TrendingItem, error) {
	type feedResult struct {
		items []collector.TrendingItem
		err   error
	}

	var wg sync.WaitGroup
	results := make([]feedResult, len(c.topics))

	for i, topic := range c.topics {
		wg.Go(func() {
			items, err := c.fetchFeed(ctx, topic)
			results[i] = feedResult{items: items, err: err}
		})
	}
	wg.Wait()

	var allItems []collector.TrendingItem
	var errs []string
	for i, r := range results {
		if r.err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", c.topics[i].Category, r.err))
			continue
		}
		allItems = append(allItems, r.items...)
	}

	if len(errs) == len(c.topics) {
		return nil, fmt.Errorf("all feeds failed: %s", strings.Join(errs, "; "))
	}

	deduped := dedup(allItems)
	return deduped, nil
}

func (c *Collector) fetchFeed(ctx context.Context, topic FeedTopic) ([]collector.TrendingItem, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, topic.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept-Language", "ko-KR,ko;q=0.9")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	return ParseFeed(body, topic.Category)
}

// ParseFeed parses Google News RSS XML into TrendingItems.
// Each RSS item represents a topic cluster with multiple related articles.
// Exported for testing.
func ParseFeed(data []byte, category string) ([]collector.TrendingItem, error) {
	var feed rssFeed
	if err := xml.Unmarshal(data, &feed); err != nil {
		return nil, fmt.Errorf("xml unmarshal: %w", err)
	}

	var items []collector.TrendingItem
	for _, item := range feed.Channel.Items {
		pubTime := parseRSSDate(item.PubDate)

		// Title format: "headline - 매체명" → strip media suffix
		headline := stripMediaSuffix(item.Title)
		if headline == "" {
			continue
		}

		// Extract related articles from description HTML
		articleURLs, articleTitles := parseDescriptionArticles(item.Description)

		// If no articles extracted from description, use the main link
		if len(articleURLs) == 0 && item.Link != "" {
			articleURLs = []string{item.Link}
			articleTitles = []string{headline}
		}

		items = append(items, collector.TrendingItem{
			Keyword:       headline,
			Source:        "google_news",
			Traffic:       0, // no traffic data, scored by cluster size in aggregator
			Category:      category,
			ArticleURLs:   articleURLs,
			ArticleTitles: articleTitles,
			PublishedAt:   pubTime,
		})
	}

	return items, nil
}

// RSS XML structures

type rssFeed struct {
	XMLName xml.Name   `xml:"rss"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Title string    `xml:"title"`
	Items []rssItem `xml:"item"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	PubDate     string `xml:"pubDate"`
	Description string `xml:"description"`
	Source      string `xml:"source"`
}

// stripMediaSuffix removes the trailing " - 매체명" from Google News titles.
// e.g. "headline text - 한겨레" → "headline text"
func stripMediaSuffix(title string) string {
	idx := strings.LastIndex(title, " - ")
	if idx > 0 {
		return strings.TrimSpace(title[:idx])
	}
	return strings.TrimSpace(title)
}

// Regex to extract articles from Google News description HTML.
// Description format: <ol><li><a href="URL">Title</a>&nbsp;&nbsp;<font>Media</font></li>...</ol>
var descArticleRe = regexp.MustCompile(`<a href="([^"]+)"[^>]*>([^<]+)</a>`)

// parseDescriptionArticles extracts article URLs and titles from the HTML description.
func parseDescriptionArticles(desc string) (urls []string, titles []string) {
	decoded := html.UnescapeString(desc)
	matches := descArticleRe.FindAllStringSubmatch(decoded, -1)
	for _, m := range matches {
		url := strings.TrimSpace(m[1])
		title := strings.TrimSpace(m[2])
		if url != "" && title != "" {
			urls = append(urls, url)
			titles = append(titles, title)
		}
	}
	return
}

// dedup removes duplicate items by headline across multiple topic feeds.
// Keeps the first occurrence (general feed items take priority).
func dedup(items []collector.TrendingItem) []collector.TrendingItem {
	seen := make(map[string]bool)
	var unique []collector.TrendingItem
	for _, item := range items {
		key := strings.ToLower(item.Keyword)
		if seen[key] {
			continue
		}
		seen[key] = true
		unique = append(unique, item)
	}
	return unique
}

func parseRSSDate(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	formats := []string{
		time.RFC1123Z,
		time.RFC1123,
		"Mon, 02 Jan 2006 15:04:05 -0700",
		"Mon, 02 Jan 2006 15:04:05 MST",
		"Mon, 2 Jan 2006 15:04:05 -0700",
		"Mon, 2 Jan 2006 15:04:05 MST",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t
		}
	}
	return time.Time{}
}
