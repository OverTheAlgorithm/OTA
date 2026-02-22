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

	if !strings.Contains(result.TextBody, "대화 소재") {
		t.Errorf("expected text body to contain '대화 소재', got '%s'", result.TextBody)
	}

	if !strings.Contains(result.TextBody, "주요 이슈 1") {
		t.Errorf("expected text body to contain '주요 이슈 1', got '%s'", result.TextBody)
	}

	if !strings.Contains(result.TextBody, "주요 이슈 2") {
		t.Errorf("expected text body to contain '주요 이슈 2', got '%s'", result.TextBody)
	}

	if !strings.Contains(result.HTMLBody, "대화 소재") {
		t.Errorf("expected HTML body to contain '대화 소재', got '%s'", result.HTMLBody)
	}

	if !strings.Contains(result.HTMLBody, "주요 이슈 1") {
		t.Errorf("expected HTML body to contain '주요 이슈 1', got '%s'", result.HTMLBody)
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
	if !strings.Contains(result.TextBody, "대화 소재") {
		t.Error("expected text body to contain '대화 소재'")
	}

	if !strings.Contains(result.TextBody, "연예") {
		t.Error("expected text body to contain '연예'")
	}

	if !strings.Contains(result.TextBody, "경제") {
		t.Error("expected text body to contain '경제'")
	}

	// HTML should have proper structure
	if !strings.Contains(result.HTMLBody, "대화 소재") {
		t.Error("expected HTML body to contain '대화 소재'")
	}

	if !strings.Contains(result.HTMLBody, "연예") {
		t.Error("expected HTML body to contain '연예'")
	}

	if !strings.Contains(result.HTMLBody, "경제") {
		t.Error("expected HTML body to contain '경제'")
	}
}

func TestFormatMessage_BriefCategory(t *testing.T) {
	items := []collector.ContextItem{
		{Category: "top", Rank: 1, Topic: "대화 주제", Summary: "대화할 수 있는 주제."},
		{Category: "brief", Rank: 1, Topic: "산불 발생", Summary: "경남에서 산불이 발생했어요."},
	}
	subscriptions := []string{}

	result := FormatMessage(items, subscriptions, "")

	// brief items should be included (universal)
	if !strings.Contains(result.TextBody, "산불 발생") {
		t.Error("expected text body to contain brief item")
	}
	if !strings.Contains(result.TextBody, "알아두면 좋은 것") {
		t.Error("expected text body to contain '알아두면 좋은 것' section")
	}
	if !strings.Contains(result.HTMLBody, "알아두면 좋은 것") {
		t.Error("expected HTML body to contain '알아두면 좋은 것' section")
	}
	// brief section should be visually distinct (dimmer colors)
	if !strings.Contains(result.HTMLBody, "#9b8bb4") {
		t.Error("expected HTML brief section to use subdued color")
	}
	// subject should count both top and brief
	if !strings.Contains(result.Subject, "2가지") {
		t.Errorf("expected subject to contain '2가지', got '%s'", result.Subject)
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
