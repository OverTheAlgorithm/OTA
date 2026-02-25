package googlenews

import (
	"context"
	"testing"
)

func TestDecodeGoogleNewsURLs_EmptyInput(t *testing.T) {
	result := DecodeGoogleNewsURLs(context.Background(), []string{})
	if len(result) != 0 {
		t.Errorf("expected empty map, got %d entries", len(result))
	}
}

func TestDecodeGoogleNewsURLs_NonGoogleURL(t *testing.T) {
	urls := []string{"https://www.example.com/article/123"}
	result := DecodeGoogleNewsURLs(context.Background(), urls)

	// Non-Google URL should be returned as-is (fallback)
	if got := result[urls[0]]; got != urls[0] {
		t.Errorf("expected fallback to original URL, got %s", got)
	}
}

func TestDecodeGoogleNewsURLs_RealURLs(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	urls := []string{
		"https://news.google.com/rss/articles/CBMiW0FVX3lxTE5YTGtkMGt2WnZZQUUwcV9pR055TVBZbkZnM3k2SzZQUm0wWmI5T08zVmhzOV9oc2szdmtCUVhzTDhqSjByTDVXTEhSWlh5WmhUMk1NZTBUM1AxZlk?oc=5",
	}

	result := DecodeGoogleNewsURLs(context.Background(), urls)
	decoded := result[urls[0]]

	if decoded == urls[0] {
		t.Error("expected decoded URL to differ from original Google URL")
	}
	if !isValidURL(decoded) {
		t.Errorf("decoded URL is not valid: %s", decoded)
	}

	t.Logf("decoded: %s", decoded)
}

func TestIsValidURL(t *testing.T) {
	tests := []struct {
		url  string
		want bool
	}{
		{"https://www.example.com/article", true},
		{"http://example.com", true},
		{"ftp://files.example.com", false},
		{"not-a-url", false},
		{"", false},
	}

	for _, tt := range tests {
		if got := isValidURL(tt.url); got != tt.want {
			t.Errorf("isValidURL(%q) = %v, want %v", tt.url, got, tt.want)
		}
	}
}

func TestReplaceArticleURLs_NoGoogleURLs(t *testing.T) {
	slice := []string{"https://example.com/1", "https://example.com/2"}
	replaced := ReplaceArticleURLs(context.Background(), slice)

	if replaced != 0 {
		t.Errorf("expected 0 replacements for non-Google URLs, got %d", replaced)
	}
}
