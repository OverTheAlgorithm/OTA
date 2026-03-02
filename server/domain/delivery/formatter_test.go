package delivery

import (
	"strings"
	"testing"

	"ota/domain/collector"
)

// testBrainCategories provides brain categories for tests.
var testBrainCategories = []collector.BrainCategory{
	{Key: "must_know", Emoji: "🔥", Label: "모르면 나만 모르는 이야기예요", AccentColor: "#e84d3d", DisplayOrder: 1},
	{Key: "conversation", Emoji: "💬", Label: "대화할 때 꺼내보세요", AccentColor: "#9b8bb4", DisplayOrder: 3},
	{Key: "result", Emoji: "🏆", Label: "결과만 알면 충분해요", AccentColor: "#7bc67e", DisplayOrder: 5},
	{Key: "over_the_algorithm", Emoji: "🌈", Label: "Over the Algorithm", AccentColor: "#5ba4d9", DisplayOrder: 10},
}

func TestFormatMessage_EmptyItems(t *testing.T) {
	items := []collector.ContextItem{}
	subscriptions := []string{}

	result := FormatMessage(items, subscriptions, nil, "", nil, nil)

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

func TestFormatMessage_BrainCategoryGrouping(t *testing.T) {
	items := []collector.ContextItem{
		{
			Category:      "top",
			BrainCategory: "must_know",
			Rank:          1,
			Topic:         "주요 이슈 1",
			Summary:       "첫 번째 주요 이슈입니다.",
		},
		{
			Category:      "top",
			BrainCategory: "must_know",
			Rank:          2,
			Topic:         "주요 이슈 2",
			Summary:       "두 번째 주요 이슈입니다.",
		},
	}
	subscriptions := []string{}

	result := FormatMessage(items, subscriptions, testBrainCategories, "", nil, nil)

	if !strings.Contains(result.Subject, "오늘의 맥락 2가지") {
		t.Errorf("expected subject to contain '오늘의 맥락 2가지', got '%s'", result.Subject)
	}

	// Brain category label should appear
	if !strings.Contains(result.TextBody, "모르면 나만 모르는 이야기예요") {
		t.Errorf("expected text body to contain brain category label, got '%s'", result.TextBody)
	}

	if !strings.Contains(result.TextBody, "주요 이슈 1") {
		t.Errorf("expected text body to contain '주요 이슈 1', got '%s'", result.TextBody)
	}

	if !strings.Contains(result.HTMLBody, "모르면 나만 모르는 이야기예요") {
		t.Errorf("expected HTML body to contain brain category label")
	}

	if !strings.Contains(result.HTMLBody, "#e84d3d") {
		t.Error("expected HTML body to use brain category accent color")
	}
}

func TestFormatMessage_WithSubscriptions(t *testing.T) {
	items := []collector.ContextItem{
		{
			Category:      "top",
			BrainCategory: "must_know",
			Rank:          1,
			Topic:         "주요 이슈",
			Summary:       "주요 이슈입니다.",
		},
		{
			Category:      "entertainment",
			BrainCategory: "conversation",
			Rank:          1,
			Topic:         "연예 소식",
			Summary:       "연예 관련 소식입니다.",
		},
		{
			Category:      "sports",
			BrainCategory: "result",
			Rank:          1,
			Topic:         "스포츠 소식",
			Summary:       "스포츠 관련 소식입니다.",
		},
	}
	subscriptions := []string{"entertainment"}

	result := FormatMessage(items, subscriptions, testBrainCategories, "", nil, nil)

	// All items are included (preferred + non-preferred sections)
	if !strings.Contains(result.TextBody, "주요 이슈") {
		t.Error("expected text body to contain '주요 이슈'")
	}
	if !strings.Contains(result.TextBody, "연예 소식") {
		t.Error("expected text body to contain '연예 소식'")
	}
	// Sports is in non-preferred section but still present
	if !strings.Contains(result.TextBody, "스포츠 소식") {
		t.Error("expected text body to contain '스포츠 소식' in non-preferred section")
	}

	// Non-preferred divider should appear
	if !strings.Contains(result.TextBody, "시야를 넓힐 기회에요") {
		t.Error("expected non-preferred divider '시야를 넓힐 기회에요'")
	}
}

func TestFormatMessage_NonPreferredItems(t *testing.T) {
	// Item with no matching subscription → should appear in non-preferred section
	items := []collector.ContextItem{
		{
			Category:      "top",
			BrainCategory: "must_know",
			Rank:          1,
			Topic:         "주요 이슈",
			Summary:       "주요 이슈입니다.",
		},
		{
			Category:      "entertainment",
			BrainCategory: "conversation",
			Rank:          1,
			Topic:         "연예 소식",
			Summary:       "연예 관련 소식입니다.",
		},
	}
	subscriptions := []string{"sports"} // doesn't match entertainment

	result := FormatMessage(items, subscriptions, testBrainCategories, "", nil, nil)

	// Both items should be present
	if !strings.Contains(result.TextBody, "주요 이슈") {
		t.Error("expected preferred item in text body")
	}
	if !strings.Contains(result.TextBody, "연예 소식") {
		t.Error("expected non-preferred item in text body")
	}
	// Divider should appear since there are non-preferred items
	if !strings.Contains(result.TextBody, "시야를 넓힐 기회에요") {
		t.Error("expected non-preferred divider when there are non-preferred items")
	}
}

func TestFormatMessage_MultipleBrainCategories(t *testing.T) {
	items := []collector.ContextItem{
		{
			Category:      "top",
			BrainCategory: "must_know",
			Rank:          1,
			Topic:         "주요 이슈",
			Summary:       "주요 이슈입니다.",
		},
		{
			Category:      "entertainment",
			BrainCategory: "conversation",
			Rank:          1,
			Topic:         "연예 소식",
			Summary:       "연예 관련 소식입니다.",
		},
		{
			Category:      "economy",
			BrainCategory: "result",
			Rank:          1,
			Topic:         "경제 소식",
			Summary:       "경제 관련 소식입니다.",
		},
	}
	subscriptions := []string{"entertainment", "economy"}

	result := FormatMessage(items, subscriptions, testBrainCategories, "", nil, nil)

	// All three brain category labels should appear
	if !strings.Contains(result.TextBody, "모르면 나만 모르는 이야기예요") {
		t.Error("expected text body to contain must_know label")
	}
	if !strings.Contains(result.TextBody, "대화할 때 꺼내보세요") {
		t.Error("expected text body to contain conversation label")
	}
	if !strings.Contains(result.TextBody, "결과만 알면 충분해요") {
		t.Error("expected text body to contain result label")
	}

	// HTML should have all three sections
	if !strings.Contains(result.HTMLBody, "모르면 나만 모르는 이야기예요") {
		t.Error("expected HTML body to contain must_know label")
	}
	if !strings.Contains(result.HTMLBody, "대화할 때 꺼내보세요") {
		t.Error("expected HTML body to contain conversation label")
	}
	if !strings.Contains(result.HTMLBody, "결과만 알면 충분해요") {
		t.Error("expected HTML body to contain result label")
	}
}

func TestFormatMessage_UngroupedFallback(t *testing.T) {
	// Items without brain_category go to "기타" section
	items := []collector.ContextItem{
		{Category: "top", Rank: 1, Topic: "기타 주제", Summary: "분류 없는 주제입니다."},
	}
	subscriptions := []string{}

	result := FormatMessage(items, subscriptions, testBrainCategories, "", nil, nil)

	if !strings.Contains(result.TextBody, "기타") {
		t.Error("expected text body to contain '기타' fallback section")
	}
	if !strings.Contains(result.TextBody, "기타 주제") {
		t.Error("expected text body to contain the item topic")
	}
}

// TestFormatMessage_AllItemsIncluded verifies that all items appear in the email
// regardless of subscription status (no filtering).
func TestFormatMessage_AllItemsIncluded(t *testing.T) {
	items := []collector.ContextItem{
		{
			Category:      "entertainment",
			BrainCategory: "conversation",
			Rank:          1,
			Topic:         "OTA 특별 토픽",
			Summary:       "알고리즘 너머의 이야기예요.",
			BuzzScore:     92,
		},
		{
			Category:      "sports",
			BrainCategory: "result",
			Rank:          1,
			Topic:         "스포츠 소식",
			Summary:       "스포츠 소식입니다.",
		},
	}
	subscriptions := []string{} // 아무것도 구독 안 함

	result := FormatMessage(items, subscriptions, testBrainCategories, "", nil, nil)

	// Both items should be present (all items included as preferred since no preferred items exist)
	if !strings.Contains(result.TextBody, "OTA 특별 토픽") {
		t.Error("모든 아이템이 포함되어야 합니다 (OTA 특별 토픽)")
	}
	if !strings.Contains(result.TextBody, "스포츠 소식") {
		t.Error("모든 아이템이 포함되어야 합니다 (스포츠 소식)")
	}
}

// TestFormatMessage_OTASection verifies the OTA section header
// and buzz_score are rendered correctly in the email body.
func TestFormatMessage_OTASection(t *testing.T) {
	items := []collector.ContextItem{
		{
			Category:      "top",
			BrainCategory: "must_know",
			Rank:          1,
			Topic:         "일반 주요 이슈",
			Summary:       "일반 주요 이슈 요약입니다.",
			BuzzScore:     75,
		},
		{
			Category:      "entertainment",
			BrainCategory: "over_the_algorithm",
			Rank:          1,
			Topic:         "OTA 토픽",
			Summary:       "OTA 토픽 요약입니다.",
			BuzzScore:     92,
		},
	}
	subscriptions := []string{}

	result := FormatMessage(items, subscriptions, testBrainCategories, "", nil, nil)

	// OTA 섹션 헤더가 있어야 함
	if !strings.Contains(result.HTMLBody, "Over the Algorithm") {
		t.Error("OTA 섹션 헤더가 HTML에 포함되어야 합니다")
	}
	// OTA 아이템 내용이 있어야 함
	if !strings.Contains(result.HTMLBody, "OTA 토픽") {
		t.Error("OTA 아이템이 HTML에 포함되어야 합니다")
	}
	// buzz_score가 렌더링되어야 함
	if !strings.Contains(result.HTMLBody, "92") {
		t.Error("buzz_score 92가 HTML에 표시되어야 합니다")
	}
	if !strings.Contains(result.HTMLBody, "75") {
		t.Error("buzz_score 75가 HTML에 표시되어야 합니다")
	}
}

// TestFormatMessage_BuzzScoreZeroHidden verifies that buzz_score=0 is not rendered.
func TestFormatMessage_BuzzScoreZeroHidden(t *testing.T) {
	items := []collector.ContextItem{
		{
			Category:      "top",
			BrainCategory: "must_know",
			Rank:          1,
			Topic:         "buzz 없는 주제",
			Summary:       "buzz_score가 0인 주제입니다.",
			BuzzScore:     0,
		},
	}
	subscriptions := []string{}

	result := FormatMessage(items, subscriptions, testBrainCategories, "", nil, nil)

	if strings.Contains(result.HTMLBody, "화제도") {
		t.Error("buzz_score가 0이면 화제도 표시가 없어야 합니다")
	}
}

func TestFormatMessage_HTMLEscaping(t *testing.T) {
	items := []collector.ContextItem{
		{
			Category:      "top",
			BrainCategory: "must_know",
			Rank:          1,
			Topic:         "테스트 <script>alert('xss')</script>",
			Summary:       "요약입니다.",
		},
	}
	subscriptions := []string{}

	result := FormatMessage(items, subscriptions, testBrainCategories, "", nil, nil)

	if !strings.Contains(result.HTMLBody, "테스트") {
		t.Error("expected HTML body to contain topic")
	}
}

func TestFormatMessage_PointsLabel(t *testing.T) {
	items := []collector.ContextItem{
		{
			Category:      "top",
			BrainCategory: "must_know",
			Rank:          1,
			Topic:         "주요 이슈",
			Summary:       "요약",
			Details:       []collector.DetailItem{{Title: "detail1"}},
		},
		{
			Category:      "sports",
			BrainCategory: "result",
			Rank:          1,
			Topic:         "스포츠 소식",
			Summary:       "요약",
			Details:       []collector.DetailItem{{Title: "detail1"}},
		},
	}
	subscriptions := []string{} // top is preferred, sports is non-preferred
	msgCtx := &MessageContext{UserID: "uid1", RunID: "rid1"}

	result := FormatMessage(items, subscriptions, testBrainCategories, "https://example.com", nil, msgCtx)

	// Preferred item: +5pt
	if !strings.Contains(result.HTMLBody, "+5pt") {
		t.Errorf("expected preferred item to show +5pt, HTML: %s", result.HTMLBody)
	}
	// Non-preferred item: +10pt
	if !strings.Contains(result.HTMLBody, "+10pt") {
		t.Errorf("expected non-preferred item to show +10pt, HTML: %s", result.HTMLBody)
	}
	// uid/rid tracking params in links
	if !strings.Contains(result.HTMLBody, "uid=uid1") {
		t.Error("expected uid tracking param in HTML link")
	}
	if !strings.Contains(result.HTMLBody, "rid=rid1") {
		t.Error("expected rid tracking param in HTML link")
	}
}
