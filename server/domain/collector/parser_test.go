package collector

import (
	"testing"

	"github.com/google/uuid"
)

func TestStripMarkdownCodeFence(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "pure json without fences",
			input:    `{"key": "value"}`,
			expected: `{"key": "value"}`,
		},
		{
			name:     "json with language identifier",
			input:    "```json\n{\"key\": \"value\"}\n```",
			expected: `{"key": "value"}`,
		},
		{
			name:     "json without language identifier",
			input:    "```\n{\"key\": \"value\"}\n```",
			expected: `{"key": "value"}`,
		},
		{
			name:     "json with extra whitespace",
			input:    "```json\n\n{\"key\": \"value\"}\n\n```",
			expected: `{"key": "value"}`,
		},
		{
			name:     "json with trailing newlines",
			input:    "```json\n{\"key\": \"value\"}\n```\n\n",
			expected: `{"key": "value"}`,
		},
		{
			name:     "multiline json with fences",
			input:    "```json\n{\n  \"key\": \"value\",\n  \"nested\": {\n    \"field\": 123\n  }\n}\n```",
			expected: "{\n  \"key\": \"value\",\n  \"nested\": {\n    \"field\": 123\n  }\n}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stripMarkdownCodeFence(tt.input)
			if result != tt.expected {
				t.Errorf("stripMarkdownCodeFence() failed\nInput: %q\nExpected: %q\nGot: %q",
					tt.input, tt.expected, result)
			}
		})
	}
}

func TestParseContextItems_BothFormats(t *testing.T) {
	runID := uuid.New()

	validJSON := `{
		"items": [
			{
				"category": "top",
				"rank": 1,
				"topic": "Test Topic",
				"summary": "Test Summary",
				"sources": ["https://example.com"]
			}
		]
	}`

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "pure json",
			input:   validJSON,
			wantErr: false,
		},
		{
			name:    "json with markdown fence and language",
			input:   "```json\n" + validJSON + "\n```",
			wantErr: false,
		},
		{
			name:    "json with markdown fence without language",
			input:   "```\n" + validJSON + "\n```",
			wantErr: false,
		},
		{
			name:    "json with extra whitespace",
			input:   "\n\n```json\n" + validJSON + "\n```\n\n",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			items, err := parseContextItems(tt.input, runID)

			if tt.wantErr {
				if err == nil {
					t.Error("parseContextItems() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("parseContextItems() unexpected error: %v", err)
				return
			}

			if len(items) != 1 {
				t.Errorf("parseContextItems() expected 1 item, got %d", len(items))
				return
			}

			item := items[0]
			if item.Category != "top" {
				t.Errorf("expected category 'top', got %q", item.Category)
			}
			if item.Topic != "Test Topic" {
				t.Errorf("expected topic 'Test Topic', got %q", item.Topic)
			}
			if item.Summary != "Test Summary" {
				t.Errorf("expected summary 'Test Summary', got %q", item.Summary)
			}
		})
	}
}

func TestParseContextItems_InvalidJSON(t *testing.T) {
	runID := uuid.New()

	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{
			name:    "malformed json",
			input:   `{invalid json}`,
			wantErr: "invalid json from ai",
		},
		{
			name:    "empty items array",
			input:   `{"items": []}`,
			wantErr: "ai returned empty items",
		},
		{
			name: "items missing required fields",
			input: `{
				"items": [
					{"category": "top", "rank": 1}
				]
			}`,
			wantErr: "no valid context items after filtering",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseContextItems(tt.input, runID)
			if err == nil {
				t.Error("parseContextItems() expected error, got nil")
				return
			}
			// Just check that we got an error with expected substring
			// (don't check exact message as it may include wrapped errors)
			if tt.wantErr != "" && !contains(err.Error(), tt.wantErr) {
				t.Errorf("parseContextItems() error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
