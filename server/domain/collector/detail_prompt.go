package collector

import (
	"fmt"
	"strings"
)

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

	sb.WriteString(`
## Task — Write (Chain-of-Thought — follow this order STRICTLY)
Build the output BOTTOM-UP. Each step uses ONLY the result of the previous step.

**Step A → details**: Based on the article content above, write structured detail entries. Every topic MUST have at least 1 entry.
**Step B → detail**: Summarize ONLY what is covered in the details entries into a cohesive 3-5 sentence paragraph.
**Step C → summary**: Condense ONLY the detail paragraph into 1-3 conversation-starter sentences.
**Step D → topic**: Create a specific, concise title that accurately reflects ONLY what summary covers.

This order guarantees consistency: the title never promises content that details cannot deliver.

## Output Format
Output ONLY pure JSON. No markdown code fences, no explanations.

{
  "topic": "[인물/사건] 최종 제목",
  "summary": "1-3문장 요약",
  "detail": "3-5문장 상세",
  "details": [{"title": "핵심 포인트 제목", "content": "2-3문장 상세 설명"}]
}

## Writing Style (Korean Output)
- NO news-speak: "~했다", "~밝혔다", "~것으로 알려졌다"
- NO casual speech: "~했어", "~됐음", "~임"
- YES friendly polite tone: "~했는데요", "~라서 난리예요", "~해서 화제예요", "~했대요"
- Include: WHO (names), WHAT (specific event), WHY people care (controversy/surprise/record)

### summary (1-3 sentences)
Enough context to start a conversation. Must include names, events, and why it matters.

### detail (3-5 sentences)
Extended context beyond the summary. Background, timeline, causes, and implications.
Write as a coherent paragraph, not bullet points.`)

	if topic.Priority == "brief" {
		sb.WriteString("\nFor \"brief\" category: use 1-sentence summary and shorter detail (1-2 sentences).")
	}

	sb.WriteString(`

### details (up to 5 structured detail entries)
Each entry is a JSON object with "title" and "content" fields:
- "title": A concise one-sentence heading that captures the key point (works as a scannable subtitle)
- "content": 2-3 sentences expanding on the title with specific details, quotes, numbers, or context

The title should be instantly understandable on its own — users scan titles first, then tap to read content.
Each entry must be an independent, NEW fact not in summary or detail.

Types of content for each entry:
- Direct quotes from people involved + context of when/where they said it
- Community memes or reactions + why they went viral
- Related statistics or numbers + what they mean in context
- Connection to previous events + how this changes things
- Expected next developments + what experts/insiders are saying
MUST include at least 1 entry. Empty array [] is NEVER allowed.
MUST NOT repeat summary or detail content.

## Critical Rules
1. Write in Korean. All topic/summary/detail/details must be Korean.
2. Pure JSON only. No markdown fences.
3. Follow the Chain-of-Thought order (details → detail → summary → topic). The topic title is generated LAST.
4. Every topic MUST have at least 1 details entry. No empty arrays.
5. Ground your writing in the article content provided. Do NOT fabricate facts or add information not found in the articles.
`)

	return sb.String()
}
