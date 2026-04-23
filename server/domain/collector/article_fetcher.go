package collector

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
)

// ArticleFetcher fetches article bodies from URLs and returns plain text.
// Follows the same function-type pattern as URLDecoder.
type ArticleFetcher func(ctx context.Context, urls []string) []FetchedArticle

const (
	maxArticleChars  = 3000
	fetchConcurrency = 10
	fetchTimeout     = 10 * time.Second
)

// NewHTTPArticleFetcher returns an ArticleFetcher backed by real HTTP calls.
// Uses a safe transport that blocks requests to private/internal IP ranges.
func NewHTTPArticleFetcher() ArticleFetcher {
	client := &http.Client{
		Timeout:       fetchTimeout,
		Transport:     newSafeTransport(),
		CheckRedirect: safeRedirectPolicy,
	}
	return func(ctx context.Context, urls []string) []FetchedArticle {
		return fetchArticles(ctx, client, urls)
	}
}

func fetchArticles(ctx context.Context, client *http.Client, urls []string) []FetchedArticle {
	results := make([]FetchedArticle, len(urls))
	sem := make(chan struct{}, fetchConcurrency)
	var wg sync.WaitGroup

	for i, u := range urls {
		results[i] = FetchedArticle{URL: u}
		wg.Add(1)
		go func(idx int, url string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			body, err := fetchSingleArticle(ctx, client, url)
			results[idx].Body = body
			results[idx].Err = err
		}(i, u)
	}

	wg.Wait()
	return results
}

func fetchSingleArticle(ctx context.Context, client *http.Client, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; OTA-Bot/1.0)")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}

	// Limit read to 1MB to avoid huge pages
	limited := io.LimitReader(resp.Body, 1<<20)
	text, err := htmlToText(limited)
	if err != nil {
		return "", err
	}

	runes := []rune(text)
	if len(runes) > maxArticleChars {
		text = string(runes[:maxArticleChars])
	}
	return text, nil
}

// htmlToText extracts visible text from HTML, stripping <script>, <style>,
// and navigation elements.
func htmlToText(r io.Reader) (string, error) {
	tokenizer := html.NewTokenizer(r)
	var sb strings.Builder
	skipDepth := 0

	for {
		tt := tokenizer.Next()
		switch tt {
		case html.ErrorToken:
			err := tokenizer.Err()
			if err == io.EOF {
				return strings.TrimSpace(sb.String()), nil
			}
			return strings.TrimSpace(sb.String()), err

		case html.StartTagToken:
			tn, _ := tokenizer.TagName()
			tag := string(tn)
			if tag == "script" || tag == "style" || tag == "nav" || tag == "header" || tag == "footer" || tag == "noscript" {
				skipDepth++
			}

		case html.EndTagToken:
			tn, _ := tokenizer.TagName()
			tag := string(tn)
			if tag == "script" || tag == "style" || tag == "nav" || tag == "header" || tag == "footer" || tag == "noscript" {
				if skipDepth > 0 {
					skipDepth--
				}
			}

		case html.TextToken:
			if skipDepth == 0 {
				text := strings.TrimSpace(string(tokenizer.Text()))
				if text != "" {
					if sb.Len() > 0 {
						sb.WriteByte(' ')
					}
					sb.WriteString(text)
				}
			}
		}
	}
}
