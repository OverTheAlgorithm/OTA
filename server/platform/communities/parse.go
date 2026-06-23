// Package communities holds source-specific adapters that normalize community
// boards into communitytrend.TrendItem values. Parsing uses golang.org/x/net/html
// (the project does not depend on goquery).
package communities

import (
	"strings"

	"golang.org/x/net/html"
)

// attr returns the value of the named attribute, or "" if absent.
func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

// hasClass reports whether n's class attribute contains the given class token.
func hasClass(n *html.Node, cls string) bool {
	for _, f := range strings.Fields(attr(n, "class")) {
		if f == cls {
			return true
		}
	}
	return false
}

// walk visits every node in the tree rooted at n.
func walk(n *html.Node, fn func(*html.Node)) {
	fn(n)
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walk(c, fn)
	}
}

// findAll returns all nodes under root for which pred returns true.
func findAll(root *html.Node, pred func(*html.Node) bool) []*html.Node {
	var out []*html.Node
	walk(root, func(n *html.Node) {
		if pred(n) {
			out = append(out, n)
		}
	})
	return out
}

// findFirst returns the first node under root for which pred returns true, or nil.
func findFirst(root *html.Node, pred func(*html.Node) bool) *html.Node {
	var found *html.Node
	walk(root, func(n *html.Node) {
		if found == nil && pred(n) {
			found = n
		}
	})
	return found
}

// textContent concatenates all descendant text, collapsing whitespace.
func textContent(n *html.Node) string {
	var b strings.Builder
	walk(n, func(c *html.Node) {
		if c.Type == html.TextNode {
			b.WriteString(c.Data)
		}
	})
	return strings.Join(strings.Fields(b.String()), " ")
}

// isElement reports whether n is an element node with the given tag.
func isElement(n *html.Node, tag string) bool {
	return n.Type == html.ElementNode && n.Data == tag
}
