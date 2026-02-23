package collector

import (
	"fmt"
	"strings"
	"time"
)

// BuildSourceBasedPrompt returns the prompt for the structured source pipeline.
// It takes pre-collected trending data (from Google Trends, Google News, etc.)
// and asks the AI to cluster, rank, and summarize — NOT to discover topics.
// The prompt is in English for optimal model performance; output is Korean.
// formatBrainCategoryList formats brain categories for inclusion in the AI prompt.
func formatBrainCategoryList(categories []BrainCategory) string {
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

func BuildSourceBasedPrompt(collectedData string, brainCategories []BrainCategory) string {
	now := time.Now().UTC().Add(9 * time.Hour) // KST
	dateStr := now.Format("2006-01-02")

	return fmt.Sprintf(`You are a Korean trend curator for a daily morning briefing service.

## Your Role
You receive PRE-COLLECTED trending data from multiple structured sources (Google Trends, Google News, etc.).
Your job is to CLUSTER related topics, RANK them by importance, and WRITE engaging Korean summaries.

**You must NOT invent or discover topics.** Only work with the data provided below.

## Today's Date: %s (KST)

## Collected Trending Data

%s

## Article URLs as References
The collected data above includes verified article URLs from real RSS feeds.
These URLs point to actual news articles and content pages.

When writing summaries and details for each topic:
- If the article headlines/titles provide enough context, proceed without reading them.
- If you need deeper information (exact quotes, statistics, timeline, controversy details),
  visit the relevant article URLs using web search to read their content.
- You do NOT need to read every URL. Use your judgment — prioritize:
  * "top" category items that need rich, conversation-worthy detail
  * Topics where headlines alone are ambiguous or insufficient
  * Items where you need specific numbers, quotes, or facts
- This approach ensures your summaries are grounded in real reporting, not generated from memory.

## Task

1. **Cluster**: Group related items across sources into unified topics.
   - e.g., "RTX 5090" from Google Trends + "엔비디아 RTX 5090 Ti 출시" from Google News = same topic.
   - Use semantic similarity, not exact string matching.

2. **Rank**: Assign importance based on:
   - Google Trends traffic value (higher = more searched)
   - Number of related articles (more coverage = bigger story)
   - Cross-source presence (appears in both Trends AND News = very important)

3. **Categorize**: Assign each topic to one of these categories:
   - "top": Topics people actively discuss at work/school ("Did you hear about...?"). Must be 3-5 items.
   - "brief": Important but not conversational (disasters, diplomacy, law changes). 0-2 items, 1-sentence summary, empty details [].
   - Domain categories (optional, 1-3 each): "entertainment", "politics", "economy", "sports", "technology", "society"

   **IMPORTANT — Topic Diversity**:
   Do NOT bias toward serious news (politics, economy, disasters).
   This service exists to help users keep up with what OTHER PEOPLE are talking about.
   Casual gossip, entertainment drama, viral moments, and memes are EQUALLY important as serious news — often MORE important for "top" category.
   Example: A dating show contestant's scandal may be more "top"-worthy than a GDP report, because more people actually talk about it at work/school.
   Aim for a healthy mix: at least 1-2 "top" items should be light/fun/gossip topics if they exist in the data.

4. **Brain Category**: Assign each topic a "brain_category" key from the table below.
   Brain categories tell users HOW to use each piece of information in daily life.
   Choose the SINGLE most fitting key for each topic.

%s
   - Every topic MUST have a brain_category (do not leave it empty).
   - Multiple topics can share the same brain_category.

5. **Write**: For each topic, produce a Korean summary in a friendly, polite tone.
   - Read article URLs as needed to get specific details (quotes, numbers, context).
   - For "top" items, always try to read at least one article URL to ensure accuracy.

## buzz_score Calculation Rules
Calculate buzz_score (1-100) based on CONCRETE DATA, not gut feeling:
- Cross-source presence: +20 if topic appears in both Google Trends AND Google News
- Google Trends traffic: 100000+ → +30, 10000+ → +20, 1000+ → +15, 100+ → +10
- Article cluster size: 5+ articles → +20, 3-4 → +15, 2 → +10, 1 → +5
- Category news prominence: main page of Google News → +10
- Base score: start at 20
- Cap at 100. The #1 "top" item must score >= 70.

## sources Rules
- ONLY use URLs from the collected data above. These are verified, real URLs.
- Do NOT generate, guess, or hallucinate any URLs.
- If a topic has no article URLs in the collected data, use an empty array [].
- Google News redirect URLs (news.google.com/rss/articles/...) are acceptable.

## Output Format
Output ONLY pure JSON. No markdown code fences, no explanations.

%s

## Writing Style (Korean Output)
- NO news-speak: "~했다", "~밝혔다", "~것으로 알려졌다"
- NO casual speech: "~했어", "~됐음", "~임"
- YES friendly polite tone: "~했는데요", "~라서 난리예요", "~해서 화제예요", "~했대요"
- Include: WHO (names), WHAT (specific event), WHY people care (controversy/surprise/record)

### summary (1-3 sentences)
Enough context to start a conversation. Must include names, events, and why it matters.

### detail (3-5 sentences)
Extended context beyond the summary. Background, timeline, causes, and implications.
Write as a coherent paragraph, not bullet points. Skip for "brief" category (use empty string "").

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
Empty array [] if no additional facts available. MUST NOT repeat summary or detail content.

## Critical Rules
1. Only use topics from the collected data. Do NOT add topics you found yourself.
2. Every source URL must come from the collected data. No invented URLs.
3. Write in Korean. Instructions are in English for precision, but all topic/summary/detail/details must be Korean.
4. Pure JSON only. No markdown fences.`, dateStr, collectedData, formatBrainCategoryList(brainCategories), jsonFormatExample())
}

// BuildSourceReviewPrompt asks the AI to find replacement URLs for invalid sources.
// The AI searches the web for actual source pages matching each topic.
func BuildSourceReviewPrompt(items []ContextItem, invalid []InvalidSource) string {
	if len(invalid) == 0 {
		return ""
	}

	type entry struct {
		Topic   string
		Summary string
		URL     string
		Reason  string
	}
	var entries []entry
	for _, inv := range invalid {
		var topic, summary string
		if inv.ItemIndex < len(items) {
			topic = items[inv.ItemIndex].Topic
			summary = items[inv.ItemIndex].Summary
		}
		entries = append(entries, entry{Topic: topic, Summary: summary, URL: inv.URL, Reason: inv.Reason})
	}

	var list strings.Builder
	for i, e := range entries {
		list.WriteString(fmt.Sprintf("%d. Topic: %q / Summary: %q / Failed URL: %s / Reason: %s\n", i+1, e.Topic, e.Summary, e.URL, e.Reason))
	}

	return fmt.Sprintf(`The following source URLs were found to be invalid when accessed via HTTP (404, page not found, etc.).
For each item, search the web to find the actual source URL for the given topic.

## Invalid URL List
%s
## Output Format
Output ONLY pure JSON. No markdown code fences.

{
  "corrections": [
    {"old_url": "failed URL", "new_url": "replacement URL or empty string"}
  ]
}

## Rules
- If no replacement can be found, set new_url to an empty string ""
- Replacement URLs must point to actually existing pages
- Only provide URLs relevant to the topic
- When in doubt, an empty string is better than a wrong URL`, list.String())
}

// jsonFormatExample returns the JSON schema example used in the AI prompt.
func jsonFormatExample() string {
	return `{
  "items": [
    {
      "category": "top",
      "brain_category": "must_know",
      "rank": 1,
      "topic": "[인물/사건 특정] 구체적 주제명",
      "summary": "누가 무엇을 했는데, 왜 사람들이 난리인지, 어떤 논란/반응이 있는지까지. 이 정보만으로 대화를 시작할 수 있어야 합니다.",
      "detail": "이 사건의 상세 맥락을 설명합니다. summary에서 다루지 못한 배경, 경위, 전개 과정을 3~5문장 정도로 서술해요. 구체적인 수치, 당사자 발언, 사건의 타임라인 등을 포함하면 좋습니다.",
      "details": [
        {"title": "당사자가 인스타그램에 '이런 논란은 처음'이라는 글을 올렸대요", "content": "해당 게시물은 올린 지 2시간 만에 댓글 5천 개가 달렸어요. 이후 소속사에서 공식 입장을 내겠다고 밝혔지만 아직 발표는 없는 상황이에요."},
        {"title": "온라인 커뮤니티에서 관련 밈이 빠르게 퍼지고 있어요", "content": "특히 '이건 좀...' 짤이 실시간 트렌드에 올랐고, X(트위터)에서도 관련 해시태그가 트렌딩 1위를 차지했대요."},
        {"title": "비슷한 사례가 지난달에도 있었는데 3일 만에 사과문이 나왔어요", "content": "이번에는 상황이 더 복잡해서 어떻게 전개될지 관심이 쏠리고 있어요. 업계에서는 최소 일주일은 지켜봐야 한다는 반응이에요."}
      ],
      "buzz_score": 92,
      "sources": ["https://www.yna.co.kr/view/AKR20260222012345"]
    },
    {
      "category": "top",
      "brain_category": "conversation",
      "rank": 2,
      "topic": "[인물/사건 특정] 두 번째 주제",
      "summary": "구체적 수치, 인물 발언, 커뮤니티 반응 등 대화 소재가 되는 디테일 포함.",
      "detail": "이 사건이 왜 중요한지 배경을 설명합니다. 관련된 이전 사건과의 연결, 각 측의 입장 차이, 향후 전개 예상 등을 포함해요.",
      "details": [
        {"title": "어제 새로운 증거가 공개되면서 상황이 완전히 바뀌었어요", "content": "지난주 처음 보도됐을 때는 큰 관심을 받지 못했는데, 내부 관계자의 폭로가 결정적이었대요. 기존 보도와 완전히 다른 내용이라 충격이 컸어요."},
        {"title": "찬성 측과 반대 측의 의견이 크게 갈리고 있어요", "content": "찬성 측에서는 '당연한 조치'라는 입장인 반면, 반대 측은 '절차가 잘못됐다'며 강하게 반발하고 있어요. SNS 댓글창이 뜨겁게 달아오르고 있대요."}
      ],
      "buzz_score": 78,
      "sources": ["https://namu.wiki/w/관련주제"]
    },
    {
      "category": "brief",
      "brain_category": "result",
      "rank": 1,
      "topic": "[사건/주제] 간단한 제목",
      "summary": "한 문장으로 요약. 대화 소재는 아니지만 알아두면 좋은 정보.",
      "detail": "",
      "details": [],
      "buzz_score": 55,
      "sources": []
    },
    {
      "category": "entertainment",
      "brain_category": "fun",
      "rank": 1,
      "topic": "[프로그램명+인물명] 구체적 사건",
      "summary": "무슨 일이 있었고 사람들이 어떻게 반응하는지까지.",
      "detail": "이 이슈의 맥락을 설명합니다. 어떤 프로그램에서 어떤 장면이 있었고, SNS에서 어떤 반응이 나왔는지 등.",
      "details": [],
      "buzz_score": 65,
      "sources": []
    }
  ]
}`
}
