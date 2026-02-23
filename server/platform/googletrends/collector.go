package googletrends

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"ota/domain/collector"
)

const (
	feedURL   = "https://trends.google.co.kr/trending/rss?geo=KR"
	userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
)

// RSS XML structures with Google Trends custom namespace (ht:)

type rssFeed struct {
	XMLName xml.Name   `xml:"rss"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Title string    `xml:"title"`
	Items []rssItem `xml:"item"`
}

type rssItem struct {
	Title       string        `xml:"title"`
	Link        string        `xml:"link"`
	PubDate     string        `xml:"pubDate"`
	Traffic     string        `xml:"https://trends.google.com/trending/rss approx_traffic"`
	Picture     string        `xml:"https://trends.google.com/trending/rss picture"`
	PicSource   string        `xml:"https://trends.google.com/trending/rss picture_source"`
	NewsItems   []rssNewsItem `xml:"https://trends.google.com/trending/rss news_item"`
}

type rssNewsItem struct {
	Title   string `xml:"https://trends.google.com/trending/rss news_item_title"`
	URL     string `xml:"https://trends.google.com/trending/rss news_item_url"`
	Picture string `xml:"https://trends.google.com/trending/rss news_item_picture"`
	Source  string `xml:"https://trends.google.com/trending/rss news_item_source"`
}

// Collector fetches trending topics from Google Trends Korea RSS.
type Collector struct {
	client *http.Client
}

// NewCollector creates a Google Trends RSS collector.
func NewCollector() *Collector {
	return &Collector{
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *Collector) Name() string {
	return "google_trends"
}

// Collect fetches the Google Trends Korea RSS feed and returns trending items.
func (c *Collector) Collect(ctx context.Context) ([]collector.TrendingItem, error) {
	body, err := c.fetchFeed(ctx)
	if err != nil {
		return nil, fmt.Errorf("google_trends: fetch failed: %w", err)
	}

	return ParseFeed(body)
}

func (c *Collector) fetchFeed(ctx context.Context) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feedURL, nil)
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

	return body, nil
}

// ParseFeed parses Google Trends RSS XML bytes into TrendingItems.
// Exported for testing with mock data.
func ParseFeed(data []byte) ([]collector.TrendingItem, error) {
	var feed rssFeed
	if err := xml.Unmarshal(data, &feed); err != nil {
		return nil, fmt.Errorf("xml unmarshal: %w", err)
	}

	items := make([]collector.TrendingItem, 0, len(feed.Channel.Items))
	for _, rssItem := range feed.Channel.Items {
		pubTime := parseRSSDate(rssItem.PubDate)

		var articleURLs []string
		var articleTitles []string
		for _, news := range rssItem.NewsItems {
			if news.URL != "" {
				articleURLs = append(articleURLs, news.URL)
				articleTitles = append(articleTitles, news.Title)
			}
		}

		items = append(items, collector.TrendingItem{
			Keyword:       rssItem.Title,
			Source:        "google_trends",
			Traffic:       parseTraffic(rssItem.Traffic),
			ArticleURLs:   articleURLs,
			ArticleTitles: articleTitles,
			PublishedAt:   pubTime,
		})
	}

	return items, nil
}

// parseTraffic converts Google Trends approx_traffic string to int.
// Examples: "500+" → 500, "1,000+" → 1000, "200" → 200, "" → 0.
func parseTraffic(s string) int {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "+")
	s = strings.ReplaceAll(s, ",", "")
	n, _ := strconv.Atoi(s)
	return n
}

// parseRSSDate parses RFC1123 / RFC822 date formats used in RSS feeds.
func parseRSSDate(s string) time.Time {
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
