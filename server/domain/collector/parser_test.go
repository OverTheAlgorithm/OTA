package collector

import (
	"testing"
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

func TestParsePhase1Response(t *testing.T) {
	validJSON := `{
		"topics": [
			{
				"topic_hint": "테스트 토픽",
				"category": "top",
				"brain_category": "trend",
				"buzz_score": 85,
				"sources": ["https://example.com/1"]
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
			name:    "json with markdown fence",
			input:   "```json\n" + validJSON + "\n```",
			wantErr: false,
		},
		{
			name:    "malformed json",
			input:   `{invalid json}`,
			wantErr: true,
		},
		{
			name:    "empty topics array",
			input:   `{"topics": []}`,
			wantErr: true,
		},
		{
			name: "topic missing sources",
			input: `{"topics": [
				{"topic_hint": "test", "category": "top", "brain_category": "trend", "buzz_score": 80, "sources": []}
			]}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			topics, err := parsePhase1Response(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if len(topics) != 1 {
				t.Errorf("expected 1 topic, got %d", len(topics))
				return
			}
			if topics[0].Category != "top" {
				t.Errorf("expected category 'top', got %q", topics[0].Category)
			}
			if topics[0].BuzzScore != 85 {
				t.Errorf("expected buzz_score 85, got %d", topics[0].BuzzScore)
			}
		})
	}
}

func TestParsePhase2Response(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name: "valid response",
			input: `{
				"topic": "테스트 제목",
				"summary": "요약입니다",
				"detail": "상세 내용입니다",
				"details": [{"title": "포인트", "content": "내용"}]
			}`,
			wantErr: false,
		},
		{
			name:    "empty topic",
			input:   `{"topic": "", "summary": "요약", "detail": "상세", "details": []}`,
			wantErr: true,
		},
		{
			name:    "malformed json",
			input:   `{bad}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parsePhase2Response(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if result.Topic != "테스트 제목" {
				t.Errorf("expected topic '테스트 제목', got %q", result.Topic)
			}
		})
	}
}

func TestRankForTopic(t *testing.T) {
	topics := []Phase1Topic{
		{TopicHint: "low", Category: "top", BuzzScore: 50},
		{TopicHint: "high", Category: "top", BuzzScore: 90},
		{TopicHint: "mid", Category: "top", BuzzScore: 70},
		{TopicHint: "ent1", Category: "entertainment", BuzzScore: 60},
	}

	// Within "top" category: 90 → rank 1, 70 → rank 2, 50 → rank 3
	if r := rankForTopic(topics[1], topics); r != 1 {
		t.Errorf("expected rank 1 for buzz_score 90, got %d", r)
	}
	if r := rankForTopic(topics[2], topics); r != 2 {
		t.Errorf("expected rank 2 for buzz_score 70, got %d", r)
	}
	if r := rankForTopic(topics[0], topics); r != 3 {
		t.Errorf("expected rank 3 for buzz_score 50, got %d", r)
	}
	// "entertainment" has only one topic → rank 1
	if r := rankForTopic(topics[3], topics); r != 1 {
		t.Errorf("expected rank 1 for entertainment buzz_score 60, got %d", r)
	}
}
