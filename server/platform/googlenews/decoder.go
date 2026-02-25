package googlenews

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// scriptName is the script path relative to the Go module root.
const scriptName = "scripts/decode_gnews_urls.py"

// decodeResult represents one decoded URL from the Python script.
type decodeResult struct {
	Original string `json:"original"`
	Decoded  string `json:"decoded"`
	OK       bool   `json:"ok"`
}

// findModuleRoot walks up from dir looking for go.mod to find the module root.
func findModuleRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "."
		}
		dir = parent
	}
}

// resolveScriptPath returns the absolute path to the Python decoder script.
func resolveScriptPath() string {
	return filepath.Join(findModuleRoot(), scriptName)
}

// DecodeGoogleNewsURLs takes Google News redirect URLs and returns original article URLs.
// For any URL that fails to decode, the original Google redirect URL is returned as fallback.
func DecodeGoogleNewsURLs(ctx context.Context, urls []string) map[string]string {
	result := make(map[string]string, len(urls))

	// Pre-fill with fallback (original URLs)
	for _, u := range urls {
		result[u] = u
	}

	if len(urls) == 0 {
		return result
	}

	inputJSON, err := json.Marshal(urls)
	if err != nil {
		log.Printf("[gnews-decoder] failed to marshal urls: %v", err)
		return result
	}

	cmd := exec.CommandContext(ctx, "python3", resolveScriptPath())
	cmd.Stdin = bytes.NewReader(inputJSON)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		log.Printf("[gnews-decoder] python script failed: %v, stderr: %s", err, stderr.String())
		return result
	}

	var decoded []decodeResult
	if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil {
		log.Printf("[gnews-decoder] failed to parse output: %v", err)
		return result
	}

	successCount := 0
	for _, d := range decoded {
		if d.OK && d.Decoded != "" && isValidURL(d.Decoded) {
			result[d.Original] = d.Decoded
			successCount++
		}
	}

	log.Printf("[gnews-decoder] decoded %d/%d URLs successfully", successCount, len(urls))
	return result
}

// isValidURL checks if a decoded URL is a valid HTTP(S) URL.
func isValidURL(u string) bool {
	return strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://")
}

// ReplaceArticleURLs replaces Google News redirect URLs with decoded original URLs
// in the given slices. Returns the number of URLs successfully decoded.
func ReplaceArticleURLs(ctx context.Context, urlSlices ...[]string) int {
	// Collect all unique URLs
	unique := make(map[string]struct{})
	for _, slice := range urlSlices {
		for _, u := range slice {
			if strings.Contains(u, "news.google.com") {
				unique[u] = struct{}{}
			}
		}
	}

	if len(unique) == 0 {
		return 0
	}

	urls := make([]string, 0, len(unique))
	for u := range unique {
		urls = append(urls, u)
	}

	decoded := DecodeGoogleNewsURLs(ctx, urls)

	// Replace in-place
	replaced := 0
	for _, slice := range urlSlices {
		for i, u := range slice {
			if newURL, ok := decoded[u]; ok && newURL != u {
				slice[i] = newURL
				replaced++
			}
		}
	}

	return replaced
}
