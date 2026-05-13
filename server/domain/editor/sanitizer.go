package editor

import (
	"regexp"
	"strings"
	"sync"

	"github.com/microcosm-cc/bluemonday"
)

// policy is built lazily because bluemonday.UGCPolicy() allocates a sizeable
// rule set that we don't want to construct at every save.
var (
	policyOnce sync.Once
	policy     *bluemonday.Policy
)

func sanitizerPolicy() *bluemonday.Policy {
	policyOnce.Do(func() {
		p := bluemonday.UGCPolicy()

		// Allow text-align styling on block elements (TipTap text-align extension).
		p.AllowAttrs("style").Matching(regexp.MustCompile(`^text-align:\s*(left|right|center|justify);?$`)).OnElements("p", "h1", "h2", "h3", "h4", "h5", "h6", "li", "blockquote")

		// Images: keep <img> with src/alt/title only. Restrict URL schemes.
		p.AllowImages()
		p.AllowAttrs("alt", "title").OnElements("img")

		// Code blocks
		p.AllowElements("pre")
		p.AllowAttrs("class").Matching(regexp.MustCompile(`^language-[a-zA-Z0-9_+-]+$`)).OnElements("code")

		// External links get rel hardened; target=_blank is fine.
		p.RequireNoFollowOnLinks(true)
		p.AllowAttrs("target").Matching(regexp.MustCompile(`^_blank$`)).OnElements("a")
		p.AllowAttrs("rel").Matching(regexp.MustCompile(`^(?:nofollow|noopener|noreferrer|ugc)(?:\s+(?:nofollow|noopener|noreferrer|ugc))*$`)).OnElements("a")

		policy = p
	})
	return policy
}

// Sanitize runs the HTML through a strict UGC policy. Output is safe to render
// directly into the DOM.
func Sanitize(html string) string {
	return sanitizerPolicy().Sanitize(html)
}

// stripTagsPattern strips all HTML tags, leaving only text content.
var stripTagsPattern = regexp.MustCompile(`<[^>]*>`)

// whitespacePattern collapses runs of whitespace (including newlines) into a single space.
var whitespacePattern = regexp.MustCompile(`\s+`)

// Excerpt produces a short plain-text summary suitable for list previews.
// It strips tags, collapses whitespace, and truncates to n runes. Returns an
// empty string for empty input.
func Excerpt(html string, n int) string {
	if html == "" || n <= 0 {
		return ""
	}
	text := stripTagsPattern.ReplaceAllString(html, " ")
	text = strings.ReplaceAll(text, " ", " ")
	text = whitespacePattern.ReplaceAllString(text, " ")
	text = strings.TrimSpace(text)

	runes := []rune(text)
	if len(runes) <= n {
		return text
	}
	return strings.TrimSpace(string(runes[:n])) + "…"
}

// firstImageSrcPattern matches the src attribute of the first <img> tag.
var firstImageSrcPattern = regexp.MustCompile(`(?i)<img\b[^>]*\bsrc\s*=\s*"([^"]+)"`)

// FirstImageURL returns the src of the first <img> tag in the HTML, or "" if
// none is present. Used to cache a thumbnail URL on the post row.
func FirstImageURL(html string) string {
	m := firstImageSrcPattern.FindStringSubmatch(html)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}
