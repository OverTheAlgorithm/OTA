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

	result := FormatMessage(items, subscriptions, nil, "", nil)

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

	result := FormatMessage(items, subscriptions, testBrainCategories, "", nil)

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

	result := FormatMessage(items, subscriptions, testBrainCategories, "", nil)

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
			Category:      "entertainment",
			BrainCategory: "conversation",
			Rank:          1,
			Topic:         "연예 소식",
			Summary:       "연예 관련 소식입니다.",
		},
	}
	subscriptions := []string{"sports"}

	result := FormatMessage(items, subscriptions, testBrainCategories, "", nil)

	// No "top" items and subscription doesn't match
	if !strings.Contains(result.TextBody, "구독하신 주제에 대한 맥락이 없습니다") {
		t.Errorf("expected text body to contain '구독하신 주제에 대한 맥락이 없습니다', got '%s'", result.TextBody)
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

	result := FormatMessage(items, subscriptions, testBrainCategories, "", nil)

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

	result := FormatMessage(items, subscriptions, testBrainCategories, "", nil)

	if !strings.Contains(result.TextBody, "기타") {
		t.Error("expected text body to contain '기타' fallback section")
	}
	if !strings.Contains(result.TextBody, "기타 주제") {
		t.Error("expected text body to contain the item topic")
	}
}

// TestFormatMessage_OverTheAlgorithmAlwaysIncluded verifies that items with
// brain_category="over_the_algorithm" appear in the email regardless of their
// category and regardless of user subscriptions.
func TestFormatMessage_OverTheAlgorithmAlwaysIncluded(t *testing.T) {
	items := []collector.ContextItem{
		{
			Category:      "entertainment", // 구독 안 한 카테고리
			BrainCategory: "over_the_algorithm",
			Rank:          1,
			Topic:         "OTA 특별 토픽",
			Summary:       "알고리즘 너머의 이야기예요.",
			BuzzScore:     92,
		},
		{
			Category:      "sports", // 구독 안 한 카테고리
			BrainCategory: "result",
			Rank:          1,
			Topic:         "스포츠 소식",
			Summary:       "구독하지 않은 스포츠 소식입니다.",
		},
	}
	subscriptions := []string{} // 아무것도 구독 안 함

	result := FormatMessage(items, subscriptions, testBrainCategories, "", nil)

	if !strings.Contains(result.TextBody, "OTA 특별 토픽") {
		t.Error("over_the_algorithm 아이템은 구독 여부와 무관하게 항상 포함되어야 합니다")
	}
	if strings.Contains(result.TextBody, "스포츠 소식") {
		t.Error("구독하지 않은 일반 카테고리 아이템은 포함되면 안 됩니다")
	}
}

// TestFormatMessage_OverTheAlgorithmSection verifies the OTA section header
// and buzz_score are rendered correctly in the email body.
func TestFormatMessage_OverTheAlgorithmSection(t *testing.T) {
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

	result := FormatMessage(items, subscriptions, testBrainCategories, "", nil)

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

	result := FormatMessage(items, subscriptions, testBrainCategories, "", nil)

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

	result := FormatMessage(items, subscriptions, testBrainCategories, "", nil)

	if !strings.Contains(result.HTMLBody, "테스트") {
		t.Error("expected HTML body to contain topic")
	}
}
