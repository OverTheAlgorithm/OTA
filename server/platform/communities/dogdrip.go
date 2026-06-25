package communities

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"

	"ota/domain/communitytrend"
)

// Dogdrip adapts the dogdrip.net "dogdrip" (개드립 베스트) board.
// Webzine-based markup: each row is an <li> inside <ul class="list">
// wrapping <a data-document-srl="..."> which itself has class "title-link".
// Comments count is a sibling span with class "text-primary" inside parent h5.
// Upvotes and time are inside div.list-meta.
type Dogdrip struct {
	baseURL string // injectable for tests; defaults to live host
	now     func() time.Time
}

// NewDogdrip builds the adapter pointed at the live site.
func NewDogdrip() *Dogdrip {
	return &Dogdrip{baseURL: "https://www.dogdrip.net", now: time.Now}
}

func (d *Dogdrip) Key() string              { return "dogdrip" }
func (d *Dogdrip) RobotsURL() string        { return d.baseURL + "/robots.txt" }
func (d *Dogdrip) BestBoardPaths() []string { return []string{"/dogdrip"} }

func (d *Dogdrip) FetchRecent(ctx context.Context) ([]communitytrend.TrendItem, error) {
	kst := time.FixedZone("KST", 9*3600)
	now := d.now().In(kst)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, kst)

	var allItems []communitytrend.TrendItem
	page := 1
	maxPages := 25

	client := &http.Client{
		Timeout: 15 * time.Second,
	}

	for {
		url := fmt.Sprintf("%s/dogdrip?page=%d", d.baseURL, page)

		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("dogdrip create request: %w", err)
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("dogdrip fetch page %d: %w", page, err)
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("dogdrip read page %d: %w", page, err)
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("dogdrip status %d on page %d", resp.StatusCode, page)
		}

		items, stop, err := d.parsePage(bytes.NewReader(body), today, now)
		if err != nil {
			return nil, fmt.Errorf("dogdrip parse page %d: %w", page, err)
		}

		if len(items) == 0 {
			break
		}

		allItems = append(allItems, items...)

		if stop || page >= maxPages {
			break
		}

		page++

		// Politeness sleep
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}

	return allItems, nil
}

func (d *Dogdrip) parse(r io.Reader) ([]communitytrend.TrendItem, error) {
	kst := time.FixedZone("KST", 9*3600)
	now := d.now().In(kst)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, kst)

	items, _, err := d.parsePage(r, today, now)
	return items, err
}

func (d *Dogdrip) parsePage(r io.Reader, today time.Time, nowKST time.Time) ([]communitytrend.TrendItem, bool, error) {
	root, err := html.Parse(r)
	if err != nil {
		return nil, false, fmt.Errorf("dogdrip parse html: %w", err)
	}

	listNode := findFirst(root, func(n *html.Node) bool {
		return isElement(n, "ul") && hasClass(n, "list")
	})
	if listNode == nil {
		return nil, false, fmt.Errorf("could not find ul.list")
	}

	var items []communitytrend.TrendItem
	stop := false

	for li := listNode.FirstChild; li != nil; li = li.NextSibling {
		if li.Type != html.ElementNode || li.Data != "li" {
			continue
		}

		if hasClass(li, "notice") {
			continue
		}

		isPopular := hasClass(li, "popular-item")

		anchor := findFirst(li, func(n *html.Node) bool {
			return isElement(n, "a") && attr(n, "data-document-srl") != ""
		})
		if anchor == nil {
			continue
		}

		srl := attr(anchor, "data-document-srl")
		title := strings.TrimSpace(textContent(anchor))
		if title == "" {
			continue
		}

		comments := 0
		h5 := parentWithTag(anchor, "h5")
		if h5 != nil {
			span := findFirst(h5, func(n *html.Node) bool {
				return isElement(n, "span") && hasClass(n, "text-primary")
			})
			if span != nil {
				if val, err := strconv.Atoi(strings.TrimSpace(textContent(span))); err == nil {
					comments = val
				}
			}
		}

		upvotes := 0
		timeText := ""
		metaDiv := findFirst(li, func(n *html.Node) bool {
			return isElement(n, "div") && hasClass(n, "list-meta")
		})
		if metaDiv != nil {
			var findMetaDetails func(*html.Node)
			findMetaDetails = func(n *html.Node) {
				if n.Type == html.ElementNode && n.Data == "span" {
					if hasClass(n, "text-primary") {
						txt := strings.TrimSpace(textContent(n))
						if val, err := strconv.Atoi(txt); err == nil {
							upvotes = val
						}
					}
					if hasClass(n, "text-muted") {
						hasClock := false
						var findClock func(*html.Node)
						findClock = func(node *html.Node) {
							if node.Type == html.ElementNode && node.Data == "i" && hasClass(node, "fa-clock") {
								hasClock = true
								return
							}
							for c := node.FirstChild; c != nil; c = c.NextSibling {
								findClock(c)
							}
						}
						findClock(n)
						if hasClock {
							timeText = strings.TrimSpace(textContent(n))
						}
					}
				}
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					findMetaDetails(c)
				}
			}
			findMetaDetails(metaDiv)
		}

		parsedTime := nowKST
		if timeText != "" {
			pt, ok := parseRelativeTime(timeText, nowKST)
			if ok {
				parsedTime = pt
				if !isPopular {
					parsedTimeKST := parsedTime.In(nowKST.Location())
					isToday := parsedTimeKST.Year() == today.Year() && parsedTimeKST.Month() == today.Month() && parsedTimeKST.Day() == today.Day()
					if !isToday {
						stop = true
					}
				}
			}
		}

		engagement := map[string]int{
			"comments": comments,
			"upvotes":   upvotes,
		}

		items = append(items, communitytrend.TrendItem{
			SourceID:   srl,
			TextUnit:   title,
			Engagement: engagement,
			ObservedAt: parsedTime,
		})
	}

	return items, stop, nil
}

func parseRelativeTime(text string, now time.Time) (time.Time, bool) {
	text = strings.TrimSpace(text)
	if text == "방금 전" {
		return now, true
	}
	if strings.HasSuffix(text, "분 전") {
		valStr := strings.TrimSpace(strings.TrimSuffix(text, "분 전"))
		val, err := strconv.Atoi(valStr)
		if err == nil {
			return now.Add(-time.Duration(val) * time.Minute), true
		}
	}
	if strings.HasSuffix(text, "시간 전") {
		valStr := strings.TrimSpace(strings.TrimSuffix(text, "시간 전"))
		val, err := strconv.Atoi(valStr)
		if err == nil {
			return now.Add(-time.Duration(val) * time.Hour), true
		}
	}
	if strings.HasSuffix(text, "일 전") {
		valStr := strings.TrimSpace(strings.TrimSuffix(text, "일 전"))
		val, err := strconv.Atoi(valStr)
		if err == nil {
			return now.Add(-time.Duration(val) * 24 * time.Hour), true
		}
	}
	parts := strings.Split(text, ".")
	if len(parts) == 3 {
		y, err1 := strconv.Atoi(parts[0])
		m, err2 := strconv.Atoi(parts[1])
		d, err3 := strconv.Atoi(parts[2])
		if err1 == nil && err2 == nil && err3 == nil {
			return time.Date(y, time.Month(m), d, 0, 0, 0, 0, now.Location()), true
		}
	} else if len(parts) == 2 {
		m, err1 := strconv.Atoi(parts[0])
		d, err2 := strconv.Atoi(parts[1])
		if err1 == nil && err2 == nil {
			return time.Date(now.Year(), time.Month(m), d, 0, 0, 0, 0, now.Location()), true
		}
	}
	return time.Time{}, false
}

func parentWithTag(n *html.Node, tag string) *html.Node {
	for p := n.Parent; p != nil; p = p.Parent {
		if p.Type == html.ElementNode && p.Data == tag {
			return p
		}
	}
	return nil
}
