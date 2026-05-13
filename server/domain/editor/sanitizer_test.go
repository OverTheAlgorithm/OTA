package editor

import (
	"strings"
	"testing"
)

func TestSanitize_StripsScripts(t *testing.T) {
	in := `<p>hello</p><script>alert(1)</script><img src="x" onerror="alert(2)">`
	out := Sanitize(in)
	if strings.Contains(out, "<script>") {
		t.Errorf("script tag survived: %q", out)
	}
	if strings.Contains(out, "onerror") {
		t.Errorf("event handler survived: %q", out)
	}
}

func TestSanitize_KeepsRichFormatting(t *testing.T) {
	in := `<p><strong>bold</strong> and <em>italic</em></p><ul><li>a</li><li>b</li></ul><h2>head</h2>`
	out := Sanitize(in)
	for _, s := range []string{"<strong>", "<em>", "<ul>", "<li>", "<h2>"} {
		if !strings.Contains(out, s) {
			t.Errorf("expected %q in output, got %q", s, out)
		}
	}
}

func TestSanitize_KeepsImageWithSrcAlt(t *testing.T) {
	in := `<p>before</p><img src="/api/v1/images/editor/2026/05/x.png" alt="caption"><p>after</p>`
	out := Sanitize(in)
	if !strings.Contains(out, `src="/api/v1/images/editor/2026/05/x.png"`) {
		t.Errorf("image src dropped: %q", out)
	}
	if !strings.Contains(out, `alt="caption"`) {
		t.Errorf("alt dropped: %q", out)
	}
}

func TestSanitize_LinkRelIsHardened(t *testing.T) {
	in := `<a href="https://example.com" target="_blank">link</a>`
	out := Sanitize(in)
	if !strings.Contains(out, "nofollow") {
		t.Errorf("nofollow not enforced: %q", out)
	}
}

func TestSanitize_RejectsJavascriptURL(t *testing.T) {
	in := `<a href="javascript:alert(1)">x</a>`
	out := Sanitize(in)
	if strings.Contains(out, "javascript:") {
		t.Errorf("javascript URL survived: %q", out)
	}
}

func TestExcerpt(t *testing.T) {
	tests := []struct {
		name string
		in   string
		n    int
		want string
	}{
		{"empty", "", 50, ""},
		{"plain short", "<p>hello world</p>", 50, "hello world"},
		{"truncated", "<p>" + strings.Repeat("a", 60) + "</p>", 10, "aaaaaaaaaa…"},
		{"collapses whitespace", "<p>a</p>   <p>b</p>", 10, "a b"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Excerpt(tc.in, tc.n)
			if got != tc.want {
				t.Errorf("Excerpt(%q, %d) = %q, want %q", tc.in, tc.n, got, tc.want)
			}
		})
	}
}

func TestFirstImageURL(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"no image", "<p>hello</p>", ""},
		{"single image", `<p>a</p><img src="/x.png" alt="x"><p>b</p>`, "/x.png"},
		{"multiple", `<img src="/a.png"><img src="/b.png">`, "/a.png"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := FirstImageURL(tc.in)
			if got != tc.want {
				t.Errorf("FirstImageURL(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
