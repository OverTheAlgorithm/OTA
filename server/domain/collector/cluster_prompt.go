package collector

import (
	"fmt"
	"strings"
	"time"
)

// formatBrainCategoryTable formats brain categories as a key/emoji/label table.
// Used in Phase 1 prompt where only classification is needed (no writing instructions).
func formatBrainCategoryTable(categories []BrainCategory) string {
	if len(categories) == 0 {
		return "(No brain categories configured)"
	}
	var sb strings.Builder
	sb.WriteString("| key | emoji | label |\n|-----|-------|-------|\n")
	for _, bc := range categories {
		sb.WriteString(fmt.Sprintf("| %s | %s | %s |\n", bc.Key, bc.Emoji, bc.Label))
	}
	return sb.String()
}

// formatCategoryList formats categories as a comma-separated list for the AI prompt.
func formatCategoryList(categories []Category) string {
	if len(categories) == 0 {
		return "general, entertainment, business, sports, technology, science, health"
	}
	keys := make([]string, len(categories))
	for i, c := range categories {
		keys[i] = fmt.Sprintf("%q", c.Key)
	}
	return strings.Join(keys, ", ")
}

// BuildClusterPrompt returns the Phase 1 prompt: cluster topics, assign categories,
// brain_category, priority, buzz_score, and select source URLs. No writing — just analysis.
func BuildClusterPrompt(collectedData string, brainCategories []BrainCategory, categories []Category) string {
	now := time.Now().UTC().Add(9 * time.Hour) // KST
	dateStr := now.Format("2006-01-02")

	return fmt.Sprintf(`You are a Korean trend curator for a daily morning briefing service.

## Your Role
You receive PRE-COLLECTED trending data from multiple structured sources (Google Trends, Google News, etc.).
Your job is to CLUSTER related topics, assign categories, calculate buzz_score, and select source URLs.

**You must NOT invent or discover topics.** Only work with the data provided below.
**You do NOT write summaries or details in this phase.** Only cluster and classify.

## Today's Date: %s (KST)

## Collected Trending Data

%s

## Task

1. **Cluster**: Group related items across sources into unified topics.
   - e.g., "RTX 5090" from Google Trends + "엔비디아 RTX 5090 Ti 출시" from Google News = same topic.
   - Use semantic similarity, not exact string matching.

2. **Category**: Assign each topic a news category from this list:
   %s
   - category represents the DOMAIN of the topic (e.g. entertainment, business, sports).
   - Every topic MUST have a category from the list above.

3. **Priority**: Assign each topic a priority level:
   - "top": Topics people actively discuss at work/school ("Did you hear about...?"). Must be 3-5 items.
   - "brief": Important but not conversational (disasters, diplomacy, law changes). 0-2 items.
   - "none": All other topics.

   **IMPORTANT — Topic Diversity**:
   Do NOT bias toward serious news (politics, economy, disasters).
   This service exists to help users keep up with what OTHER PEOPLE are talking about.
   Casual gossip, entertainment drama, viral moments, and memes are EQUALLY important as serious news — often MORE important for "top" priority.
   Aim for a healthy mix: at least 1-2 "top" items should be light/fun/gossip topics if they exist in the data.

4. **Brain Category**: Assign each topic a "brain_category" key from the table below.
   Brain categories tell users HOW to use each piece of information in daily life.
   Choose the SINGLE most fitting key for each topic.

%s
   - Every topic MUST have a brain_category (do not leave it empty).
   - Multiple topics can share the same brain_category.

5. **buzz_score**: Calculate buzz_score (1-100) based on CONCRETE DATA, not gut feeling:
   - Cross-source presence: +20 if topic appears in both Google Trends AND Google News
   - Google Trends traffic: 100000+ → +30, 10000+ → +20, 1000+ → +15, 100+ → +10
   - Article cluster size: 5+ articles → +20, 3-4 → +15, 2 → +10, 1 → +5
   - Category news prominence: main page of Google News → +10
   - Base score: start at 20
   - Cap at 100. The #1 "top" priority item must score >= 70.

6. **Source Selection**: Pick the most relevant article URLs from the collected data for each topic.
   - Every topic MUST have at least 1 source URL. A topic without sources is invalid and must be dropped.
   - ONLY use URLs that appear EXACTLY in the collected data above. Copy-paste them verbatim.
   - Do NOT modify, shorten, or reconstruct any URL.
   - Do NOT generate, guess, or hallucinate any URLs.
   - Prefer 2-3 sources per topic. Maximum 4.

## topic_hint
For each topic, write a short Korean hint (5-15 words) that captures the essence.
This is NOT the final title — it's a working label for Phase 2 writing.

## Output Format
Output ONLY pure JSON. No markdown code fences, no explanations.

{
  "topics": [
    {
      "topic_hint": "버터떡 열풍과 혈당 우려",
      "category": "general",
      "priority": "top",
      "brain_category": "trend",
      "buzz_score": 85,
      "sources": ["https://example.com/article1", "https://example.com/article2"]
    }
  ]
}

## Critical Rules
1. Only use topics from the collected data. Do NOT add topics you found yourself.
2. Every source URL must come from the collected data. No invented URLs.
3. topic_hint must be in Korean.
4. Pure JSON only. No markdown fences.
5. Every topic MUST have at least 1 source. No empty sources arrays.
6. category must be one of the allowed categories listed above. "top" and "brief" are NOT categories — they are priority levels.`, dateStr, collectedData, formatCategoryList(categories), formatBrainCategoryTable(brainCategories))
}
