package collector

import (
	"fmt"
	"strings"
)

// BuildQuizPrompt returns a prompt for generating a single multiple-choice quiz
// question based on the already-written article content from Phase 2.
// Input: the written topic/summary/detail/details (not raw source articles).
func BuildQuizPrompt(topic, summary, detail string, details []DetailItem) string {
	var sb strings.Builder

	sb.WriteString("Based on the following news article, generate one multiple-choice quiz question.\n")
	sb.WriteString("The question should test comprehension of a key fact — something a reader would know if they read the article.\n\n")

	sb.WriteString(fmt.Sprintf("Article title: %s\n", topic))
	sb.WriteString(fmt.Sprintf("Summary: %s\n", summary))
	sb.WriteString(fmt.Sprintf("Detail: %s\n", detail))

	if len(details) > 0 {
		sb.WriteString("Details:\n")
		for _, d := range details {
			sb.WriteString(fmt.Sprintf("- %s: %s\n", d.Title, d.Content))
		}
	}

	sb.WriteString(`
Output ONLY pure JSON (no markdown fences, no explanations):
{
  "question": "기사 내용을 바탕으로 한 퀴즈 질문",
  "options": ["선택지1", "선택지2", "선택지3", "선택지4"],
  "correct_index": 0
}

Rules:
- Question in Korean, testing a specific fact from the article
- Exactly 4 options. Only one correct.
- correct_index: 0-based index of the correct answer
- Wrong options should be plausible but clearly wrong to someone who read the article
- Do NOT ask about opinions or subjective interpretations
`)

	return sb.String()
}
