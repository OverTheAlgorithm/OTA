package collector

import (
	"fmt"
	"strings"
)

// categoryTone returns category-specific writing tone instructions.
func categoryTone(category string) string {
	switch category {
	case "business":
		return `톤: 분석적, 차분
- "~한 것으로 분석됩니다", "전문가들은 ~에 주목하고 있습니다", "~한 흐름이 이어지고 있어요"
- 수치와 맥락을 강조하되 딱딱하지 않게. 독자가 경제 흐름을 이해할 수 있도록.`
	case "entertainment":
		return `톤: 경쾌, 수다스러운
- "~라니 놀랍죠?", "팬들 사이에서 화제가 되고 있어요", "~해서 난리예요"
- 에너지 있게 쓰되 가십 수준으로 떨어지지 않게. 왜 화제인지 맥락을 담아서.`
	case "sports":
		return `톤: 생생, 열정적
- "짜릿한 역전승!", "~의 활약이 돋보였습니다", "팬들의 환호 속에"
- 경기 현장의 감각을 살리되, 결과와 의미를 명확히.`
	case "technology":
		return `톤: 호기심 자극, 설명적
- "쉽게 말하면 ~인 셈이죠", "이 기술이 중요한 이유는", "~가 바뀔 수 있어요"
- 기술 용어는 쉬운 비유로 풀어주고, 일상에 미치는 영향을 연결.`
	case "science", "health":
		return `톤: 교육적, 신중한
- "연구에 따르면", "다만 아직 ~단계라는 점은 유의해야 합니다", "~할 수 있다고 해요"
- 사실 기반으로 신중하게 쓰되 딱딱한 논문체는 피하기. 독자가 궁금해할 '그래서 나한테 어떤 의미?'에 답하기.`
	default: // general
		return `톤: 담백, 간결
- "~한 가운데", "핵심은 ~입니다", "~인 셈이에요"
- 군더더기 없이 핵심을 전달. 짧고 명확한 문장 위주.`
	}
}

// BuildDetailPrompt returns the Phase 2 prompt for a single topic.
// It includes the topic hint, category, article bodies, and writing instructions.
// The AI writes topic title, summary, detail, and details based on actual article content.
func BuildDetailPrompt(topic Phase1Topic, articles []FetchedArticle, brainCategories []BrainCategory) string {
	var sb strings.Builder

	sb.WriteString("You are a Korean trend curator writing a single topic for a daily morning briefing.\n\n")

	sb.WriteString(fmt.Sprintf("## Topic\n- Hint: %s\n- Category: %s\n- Brain category: %s\n\n",
		topic.TopicHint, topic.Category, topic.BrainCategory))

	// Include brain_category instruction if available.
	for _, bc := range brainCategories {
		if bc.Key == topic.BrainCategory && bc.Instruction != nil && *bc.Instruction != "" {
			sb.WriteString(fmt.Sprintf("## Brain Category Instruction\nApply this instruction when writing: %s\n\n", *bc.Instruction))
			break
		}
	}

	sb.WriteString("## Source Articles\n")
	hasContent := false
	for i, a := range articles {
		if a.Err != nil || a.Body == "" {
			sb.WriteString(fmt.Sprintf("\n### Article %d\nURL: %s\n(Content unavailable)\n", i+1, a.URL))
			continue
		}
		hasContent = true
		sb.WriteString(fmt.Sprintf("\n### Article %d\nURL: %s\n\n%s\n", i+1, a.URL, a.Body))
	}

	// Note: topics with no article content are dropped before reaching this prompt.
	// The hasContent check here is a defensive safeguard only.
	if !hasContent {
		sb.WriteString("\n(WARNING: No article content available. This topic should have been dropped.)\n")
	}

	// Category-specific tone
	sb.WriteString(fmt.Sprintf("\n## 문체 (이 토픽에 적용)\n%s\n", categoryTone(topic.Category)))

	sb.WriteString(`
## Task — Write (Chain-of-Thought — follow this order STRICTLY)
Build the output BOTTOM-UP. Each step uses ONLY the result of the previous step.

**Step A → details**: Based on the article content above, write structured detail entries. Every topic MUST have at least 1 entry.
**Step B → detail**: Summarize ONLY what is covered in the details entries into a cohesive 3-5 sentence paragraph.
**Step C → summary**: Condense ONLY the detail paragraph into 1-3 conversation-starter sentences.
**Step D → topic**: Create a specific, concise title that accurately reflects ONLY what summary covers.
**Step E → poll (ALMOST ALWAYS null)**: Generate a poll ONLY when the topic contains a genuine societal conflict where real opposing camps exist (policy debate, ethical dilemma, institutional change with clear winners/losers). The vast majority of articles — 80~90% — should have "poll": null. If you are not confident that reasonable people would hold fundamentally incompatible positions on this topic, set "poll": null.

This order guarantees consistency: the title never promises content that details cannot deliver.

## Output Format
Output ONLY pure JSON. No markdown code fences, no explanations.

{
  "topic": "구체적이고 간결한 제목",
  "summary": "1-3문장 요약",
  "detail": "3-5문장 상세",
  "details": [{"title": "핵심 포인트 제목", "content": "3-5문장 상세 설명"}],
  "poll": {
    "question": "기사 내용에 대한 중립적 의견 질문",
    "options": ["선택지 A", "선택지 B", "선택지 C"]
  }
}

## Poll Eligibility — 투표 생성 판단 기준 (CRITICAL)
대부분의 기사는 투표가 필요 없다. 아래 조건을 모두 충족할 때만 poll을 생성하라:
1. 사회적으로 실제 진영이 나뉘는 갈등이 존재한다 (정책 찬반, 윤리적 딜레마, 제도 변화의 수혜자/피해자 대립 등)
2. 각 입장이 나름의 논리와 근거를 갖고 있어 어느 쪽이 옳다고 단정할 수 없다
3. 선택지가 각 입장의 핵심 논리를 담은 구체적 주장문으로 표현 가능하다

### poll을 만들면 안 되는 경우 (반드시 "poll": null)
- 단순 사실/사건 보도 (사고, 재해, 부고, 날씨, 통계 발표)
- 인물의 성과/기록/수상 — "본받을만하다" 같은 감상은 투표 선택지가 아니다
- 감동/미담/휴먼스토리
- 신제품/서비스 출시, 기술 발표
- 연예/스포츠 결과 보도
- 갈등 없이 한쪽 방향으로만 의견이 수렴되는 주제
- 확신이 없으면 만들지 마라. 애매하면 null이다.

### poll 선택지 규칙 (poll이 non-null일 때만 적용)
- Korean only. 담백하고 감정 없는 단문. 기사 톤/표현 모방 금지.
- 각 선택지는 해당 입장의 핵심 논리를 담은 평서형 주장문이어야 한다. 단순 "찬성"/"반대" 라벨 금지.
  예) "이란의 핵 위협을 제거하기 전까지 불가피한 희생을 감수해야 한다" (O)
  예) "군사 작전에 찬성한다" (X — 논리가 없는 빈 라벨)
- 2~4개 선택지. 중도가 어색하면 2개. 5개 이상 금지.
- 각 선택지는 self-contained (헤드라인 읽지 않아도 의미 파악 가능).
- 기사 내용 이해/사실 확인 금지 — 그건 퀴즈의 영역이다.

## Writing Rules (Korean Output)
- NO news-speak: "~했다", "~밝혔다", "~것으로 알려졌다"
- NO casual speech: "~했어", "~됐음", "~임"
- Use the tone specified in 문체 section above
- Include: WHO (names), WHAT (specific event), WHY people care (controversy/surprise/record)

### summary (1-3 sentences)
Enough context to start a conversation. Must include names, events, and why it matters.

### detail (5-8 sentences)
Extended context beyond the summary. Background, timeline, causes, and implications.
Write as a coherent paragraph, not bullet points. Aim for depth — explain the context thoroughly.`)

	if topic.Priority == "brief" {
		sb.WriteString("\nFor \"brief\" category: use 1-sentence summary and shorter detail (1-2 sentences).")
	}

	sb.WriteString(`

### details — 포맷 선택 + 작성
아래 5가지 포맷 중 이 토픽의 콘텐츠에 가장 적합한 것을 하나 골라서 작성하세요.
어떤 포맷을 골랐는지 출력할 필요 없습니다. 결과 JSON만 출력하세요.

**포맷 A — 핵심 포인트** (기본, 가장 범용적)
각 entry가 독립적인 사실/관점을 전달:
- title: 핵심 사실을 한 문장으로 ("삼성, 3분기 영업이익 40% 감소")
- content: 그 사실의 배경/맥락/수치 (3-5문장)

**포맷 B — 타임라인** (사건 전개가 있는 토픽에 적합)
시간 순서로 전개:
- title: 시점 + 사건 ("3월 5일 — 첫 공식 발표")
- content: 해당 시점에 무슨 일이 있었는지 (3-5문장)

**포맷 C — 궁금증 해소** (독자가 '왜?'를 궁금해할 토픽에 적합)
질문-답변 구조:
- title: 독자가 궁금해할 질문 ("왜 이게 중요할까?", "앞으로 어떻게 될까?")
- content: 해당 질문에 대한 답변 (3-5문장)

**포맷 D — 핵심 한줄 + 깊이** (임팩트가 강한 토픽에 적합)
첫 entry에 결론, 나머지에 근거:
- entry 1 title: 한 문장으로 핵심 결론
- entry 1 content: 왜 이것이 결론인지 (3-5문장)
- 이후 entries: 이 결론을 뒷받침하는 근거/반론/맥락

**포맷 E — 비교/대립** (찬반 또는 양측이 있는 토픽에 적합)
서로 다른 입장을 대비:
- title: 입장 레이블 ("정부 입장", "업계 반응", "소비자 시각")
- content: 해당 입장의 핵심 주장과 근거 (3-5문장)

### details 공통 규칙
- MUST include at least 1 entry. Empty array [] is NEVER allowed.
- Up to 5 entries.
- MUST NOT repeat summary or detail content.
- Each entry must be an independent, NEW fact not in summary or detail.

## Critical Rules
1. Write in Korean. All topic/summary/detail/details must be Korean.
2. Pure JSON only. No markdown fences.
3. Follow the Chain-of-Thought order (details → detail → summary → topic). The topic title is generated LAST.
4. Every topic MUST have at least 1 details entry. No empty arrays.
5. Ground your writing in the article content provided. Do NOT fabricate facts or add information not found in the articles.
`)

	return sb.String()
}
