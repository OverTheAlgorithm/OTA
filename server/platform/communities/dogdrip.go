package communities

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strconv"
	"time"

	"golang.org/x/net/html"

	"ota/domain/communitytrend"
)

// Dogdrip adapts the dogdrip.net "boomupbest" (추천 베스트) board.
// Rhymix-based markup: each row is an <a data-document-srl="..."> wrapping
// <span class="title-link">TITLE</span> and <span class="text-primary">N</span> (comments).
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
func (d *Dogdrip) BestBoardPaths() []string { return []string{"/boomupbest"} }

func (d *Dogdrip) FetchRecent(ctx context.Context) ([]communitytrend.TrendItem, error) {
	body, err := fetchHTML(ctx, d.baseURL+"/boomupbest")
	if err != nil {
		return nil, fmt.Errorf("dogdrip fetch: %w", err)
	}
	return d.parse(bytes.NewReader(body))
}

// parse is separated from FetchRecent so fixtures can be tested without network.
func (d *Dogdrip) parse(r io.Reader) ([]communitytrend.TrendItem, error) {
	now := time.Now
	if d.now != nil {
		now = d.now
	}
	root, err := html.Parse(r)
	if err != nil {
		return nil, fmt.Errorf("dogdrip parse html: %w", err)
	}

	anchors := findAll(root, func(n *html.Node) bool {
		return isElement(n, "a") && attr(n, "data-document-srl") != ""
	})

	seen := make(map[string]bool)
	var items []communitytrend.TrendItem
	for _, a := range anchors {
		srl := attr(a, "data-document-srl")
		if seen[srl] {
			continue
		}
		titleNode := findFirst(a, func(n *html.Node) bool {
			return isElement(n, "span") && hasClass(n, "title-link")
		})
		if titleNode == nil {
			continue
		}
		title := textContent(titleNode)
		if title == "" {
			continue
		}
		seen[srl] = true

		engagement := map[string]int{}
		if cn := findFirst(a, func(n *html.Node) bool {
			return isElement(n, "span") && hasClass(n, "text-primary")
		}); cn != nil {
			if c, err := strconv.Atoi(textContent(cn)); err == nil {
				engagement["comments"] = c
			}
		}

		items = append(items, communitytrend.TrendItem{
			SourceID:   srl,
			TextUnit:   title,
			Engagement: engagement,
			ObservedAt: now(),
		})
	}
	return items, nil
}
