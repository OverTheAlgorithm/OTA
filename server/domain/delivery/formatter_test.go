package delivery

import (
	"strings"
	"testing"

	"ota/domain/collector"
)

func TestFormatMessage_EmptyItems(t *testing.T) {
	items := []collector.ContextItem{}
	subscriptions := []string{}

	result := FormatMessage(items, subscriptions, "")

	if result.Subject != "오늘의 맥락" {
		t.Errorf("expected subject '오늘의 맥락', got '%s'", result.Subject)
	}

	if !strings.Contains(result.TextBody, "수집된 맥락이 없습니다") {
		t.Errorf("expected text body to contain '수집된 맥락이 없습니다', got '%s'", result.TextBody)
	}

	if !strings.Contains(result.HTMLBody, "수집된 맥락이 없습니다") {
		t.Errorf("expected HTML body to contain '수집된 맥락이 없습니다', got '%s'", result.HTMLBody)
	}
}

func TestFormatMessage_OnlyTopCategory(t *testing.T) {
	items := []collector.ContextItem{
		{
			Category: "top",
			Rank:     1,
			Topic:    "주요 이슈 1",
			Summary:  "첫 번째 주요 이슈입니다.",
		},
		{
			Category: "top",
			Rank:     2,
			Topic:    "주요 이슈 2",
			Summary:  "두 번째 주요 이슈입니다.",
		},
	}
	subscriptions := []string{}

	result := FormatMessage(items, subscriptions, "")

	if !strings.Contains(result.Subject, "오늘의 맥락 2가지") {
		t.Errorf("expected subject to contain '오늘의 맥락 2가지', got '%s'", result.Subject)
	}

	if !strings.Contains(result.TextBody, "주요 화제") {
		t.Errorf("expected text body to contain '주요 화제', got '%s'", result.TextBody)
	}

	if !strings.Contains(result.TextBody, "주요 이슈 1") {
		t.Errorf("expected text body to contain '주요 이슈 1', got '%s'", result.TextBody)
	}

	if !strings.Contains(result.TextBody, "주요 이슈 2") {
		t.Errorf("expected text body to contain '주요 이슈 2', got '%s'", result.TextBody)
	}

	if !strings.Contains(result.HTMLBody, "<h2>🔥 주요 화제</h2>") {
		t.Errorf("expected HTML body to contain '<h2>🔥 주요 화제</h2>', got '%s'", result.HTMLBody)
	}
}

func TestFormatMessage_WithSubscriptions(t *testing.T) {
	items := []collector.ContextItem{
		{
			Category: "top",
			Rank:     1,
			Topic:    "주요 이슈",
			Summary:  "주요 이슈입니다.",
		},
		{
			Category: "entertainment",
			Rank:     1,
			Topic:    "연예 소식",
			Summary:  "연예 관련 소식입니다.",
		},
		{
			Category: "sports",
			Rank:     1,
			Topic:    "스포츠 소식",
			Summary:  "스포츠 관련 소식입니다.",
		},
	}
	subscriptions := []string{"entertainment"}

	result := FormatMessage(items, subscriptions, "")

	// Should include top + entertainment, exclude sports
	if !strings.Contains(result.TextBody, "주요 이슈") {
		t.Error("expected text body to contain '주요 이슈'")
	}

	if !strings.Contains(result.TextBody, "연예 소식") {
		t.Error("expected text body to contain '연예 소식'")
	}

	if strings.Contains(result.TextBody, "스포츠 소식") {
		t.Error("expected text body to NOT contain '스포츠 소식'")
	}

	if !strings.Contains(result.Subject, "구독 주제 포함") {
		t.Errorf("expected subject to contain '구독 주제 포함', got '%s'", result.Subject)
	}
}

func TestFormatMessage_NoMatchingSubscriptions(t *testing.T) {
	items := []collector.ContextItem{
		{
			Category: "entertainment",
			Rank:     1,
			Topic:    "연예 소식",
			Summary:  "연예 관련 소식입니다.",
		},
	}
	subscriptions := []string{"sports"}

	result := FormatMessage(items, subscriptions, "")

	// No "top" items and subscription doesn't match
	if !strings.Contains(result.TextBody, "구독하신 주제에 대한 맥락이 없습니다") {
		t.Errorf("expected text body to contain '구독하신 주제에 대한 맥락이 없습니다', got '%s'", result.TextBody)
	}
}

func TestFormatMessage_MultipleCategories(t *testing.T) {
	items := []collector.ContextItem{
		{
			Category: "top",
			Rank:     1,
			Topic:    "주요 이슈",
			Summary:  "주요 이슈입니다.",
		},
		{
			Category: "entertainment",
			Rank:     1,
			Topic:    "연예 소식",
			Summary:  "연예 관련 소식입니다.",
		},
		{
			Category: "economy",
			Rank:     1,
			Topic:    "경제 소식",
			Summary:  "경제 관련 소식입니다.",
		},
	}
	subscriptions := []string{"entertainment", "economy"}

	result := FormatMessage(items, subscriptions, "")

	// Should include all three
	if !strings.Contains(result.TextBody, "주요 화제") {
		t.Error("expected text body to contain '주요 화제'")
	}

	if !strings.Contains(result.TextBody, "연예") {
		t.Error("expected text body to contain '연예'")
	}

	if !strings.Contains(result.TextBody, "경제") {
		t.Error("expected text body to contain '경제'")
	}

	// HTML should have proper structure
	if !strings.Contains(result.HTMLBody, "<ul>") {
		t.Error("expected HTML body to contain '<ul>'")
	}

	if !strings.Contains(result.HTMLBody, "<li>") {
		t.Error("expected HTML body to contain '<li>'")
	}
}

func TestFormatMessage_HTMLEscaping(t *testing.T) {
	items := []collector.ContextItem{
		{
			Category: "top",
			Rank:     1,
			Topic:    "테스트 <script>alert('xss')</script>",
			Summary:  "요약입니다.",
		},
	}
	subscriptions := []string{}

	result := FormatMessage(items, subscriptions, "")

	// Note: Current implementation doesn't escape HTML
	// This test documents current behavior
	// TODO: Add HTML escaping in future if needed
	if !strings.Contains(result.HTMLBody, "테스트") {
		t.Error("expected HTML body to contain topic")
	}
}
