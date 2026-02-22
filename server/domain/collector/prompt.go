package collector

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// BuildKeywordExtractionPrompt returns the Stage 1 prompt.
// Goal: ground the pipeline on real trending keywords from actual web searches.
// The model returns a simple JSON list of specific, concrete keywords/events.
func BuildKeywordExtractionPrompt() string {
	now := time.Now().UTC().Add(9 * time.Hour) // KST
	dateStr := now.Format("2006년 1월 2일")

	return fmt.Sprintf(`당신은 한국 트렌드 모니터입니다.

오늘(%s) 한국에서 실제로 가장 많이 검색되고 언급되는 키워드를 웹에서 직접 검색해서 확인하세요.

## 참고할 소스 (우선순위 순)
1. **구글 트렌드 한국** (https://trends.google.co.kr/trending?geo=KR) — 최우선 확인
2. **X(트위터) 한국 트렌드** — 실시간 화제 키워드
3. **네이버/다음 실시간 검색어** — 포털 검색 순위
4. **유튜브 인기 동영상** — 영상 콘텐츠 화제
5. **커뮤니티** (에펨코리아, 디시인사이드, 더쿠, 레딧 코리아) — 사람들이 실제로 대화하는 주제
6. 뉴스 사이트 — 참고용 (뉴스에만 의존하지 말 것)

⚠️ **뉴스 기사는 보조 소스입니다.** 실시간 검색어, SNS 트렌드, 커뮤니티 화제가 더 중요합니다.

## 출력 형식
반드시 순수 JSON만 출력하세요. 마크다운 코드 블록이나 추가 설명 없이.

{
  "keywords": [
    "구체적인 키워드나 사건 1",
    "구체적인 키워드나 사건 2"
  ]
}

## 규칙
- 15~20개 추출
- 막연한 분야명 금지: "AI 기술", "주식 시장", "정치 이슈" → 이런 단어는 절대 사용 금지
- 반드시 구체적 사건·인물·수치·프로그램명 포함: "오픈AI o3 한국 출시 논란", "[가수명] 콘서트 취소", "코스피 X%%%% 급락"
- **오늘(%s) 화제만 포함**. "며칠 전" 이슈는 오늘도 여전히 화제인 경우에만 포함
- 연예, 경제, 스포츠, 정치, 기술, 사회 등 다양한 분야에서 고르게 추출
- 마크다운 코드 블록 없이 JSON만 출력`, dateStr, dateStr)
}

// BuildEnrichmentPrompt returns the Stage 2 prompt.
// Keywords from Stage 1 are passed in as anchors, preventing topic hallucination.
// The model researches ONLY these specific keywords and writes full structured summaries.
func BuildEnrichmentPrompt(keywords []string) string {
	now := time.Now().UTC().Add(9 * time.Hour) // KST
	dateStr := now.Format("2006년 1월 2일")

	keywordList := ""
	for i, kw := range keywords {
		keywordList += fmt.Sprintf("%d. %s\n", i+1, kw)
	}

	return fmt.Sprintf(`당신은 한국 직장인·대학생들이 "요즘 이 얘기 들었어?"라며 대화를 시작할 수 있게 해주는 전문 큐레이터입니다.

## 핵심 목적
이 서비스의 유저는 관심 없는 분야의 화제에도 대화에 참여하고 싶어하는 사람들입니다.
따라서 당신이 제공하는 정보는 **읽는 것만으로 그 주제에 대해 대화할 수 있을 정도로 구체적**이어야 합니다.

## ⚠️ 최신성 기준 — 절대 규칙
**오늘 날짜: %s**

이 서비스는 **매일 아침** 유저에게 전달되는 브리핑입니다.
모든 정보는 반드시 **오늘(%s) 기준으로 최신**이어야 합니다.

- 수치(주가, 환율, 기록 등)는 반드시 **오늘 또는 어제** 기준 최신 값을 웹 검색으로 확인
- "며칠 전", "지난주" 수준의 정보는 **stale(오래된 정보)**입니다 → 절대 포함 금지
- 키워드가 며칠 전 이슈라면, **오늘 시점의 최신 전개**를 반영해야 함 (예: "이후 어떻게 됐는지")
- 웹 검색 시 반드시 오늘 날짜(%s) 전후의 결과를 우선 확인

## 오늘 화제 키워드 목록
아래 키워드들은 오늘 실제로 트렌딩 중인 주제입니다. **이 키워드들에 대해서만** 웹 검색으로 구체적인 맥락을 수집하세요.
목록에 없는 주제를 추가하거나 내용을 임의로 만들어내지 마세요.

%s

## 출력 형식
**매우 중요**: 반드시 순수 JSON만 출력하세요. 마크다운 코드 블록이나 추가 설명 없이, JSON 객체만 출력해주세요.

%s

## 카테고리 분류 규칙

### "top" 카테고리 (필수, 3~5개)
- 위 키워드 중 분야를 막론하고 **가장 많이 대화되는** 주제 선정
- rank는 1부터 시작, 가장 뜨거운 주제가 1번
- 한 가지 큰 사건 안에 여러 포인트가 있으면 **가장 대화가 되는 세부 포인트**를 topic으로 잡아주세요

### 그 외 카테고리 (선택, 각 1~3개)
- **사용 가능**: "entertainment", "politics", "economy", "sports", "technology", "society"
- 영문 소문자로 작성
- 화제가 되는 주제가 있을 때만 포함 (억지로 채우지 않음)
- 각 키워드를 적절한 카테고리에 분류하세요:
  - 연예(entertainment): 드라마, 영화, 아이돌, 예능, 방송사고, SNS 논란 등
  - 경제(economy): 주식, 부동산, 환율, 기업 이슈, 취업/채용 화제 등
  - 스포츠(sports): 경기 결과, 선수 이슈, 이적, 기록 등
  - 정치(politics): 정치권 이슈, 법안, 여론 화제 등
  - 기술(technology): 신제품, AI, 앱, 스타트업 이슈 등
  - 사회(society): 사건사고, 사회 현상, 트렌드, 세대 갈등 등

## ⚠️ "대화 가능 테스트" — 가장 중요한 기준

작성한 summary를 읽고 아래 질문에 답해보세요:
> "이 정보만으로 직장 동료에게 '야 이거 알아?' 하고 이야기를 꺼낼 수 있는가?"

**YES → 통과**, **NO → 다시 써야 함**

### 구체적으로 이것들이 반드시 포함되어야 합니다:
1. **이름**: 누가 관련됐는가 (인물명, 프로그램명, 기업명 등)
2. **사건**: 정확히 무슨 일이 있었는가
3. **흥미 포인트**: 왜 사람들이 이걸 이야기하는가 (논란, 반전, 갈등, 기록, 충격 등)
4. **대화 포인트**: 사람들이 갈리는 의견, 밈, 반응 등 대화를 이어갈 수 있는 요소

## ✅ 좋은 summary의 조건 (형식 예시 — 아래 내용을 그대로 쓰지 말 것)

⚠️ **아래는 형식과 수준을 보여주기 위한 예시입니다. 내용을 절대 복사하지 마세요.**
실제 출력에는 위 키워드 목록에서 찾은 실제 내용만 담아야 합니다.

- **좋은 summary**: "[실제 인물명]이 [구체적 사건]을 했는데, [왜 난리인지/사람들 반응]. [대화 포인트]."
- **좋은 details**: summary에 없는 새로운 추가 정보들. 당사자 발언, 커뮤니티 밈/반응, 관련 수치, 이전 사건과의 연결점 등.

## details 작성 규칙
- 최대 5개, 각각 독립적으로 읽을 수 있는 한 문장 추가 정보
- summary를 반복하지 말 것. summary에 없는 새로운 정보만
- 예: 당사자 발언 인용, 커뮤니티 밈/반응, 관련 통계/수치, 이전 사건과의 연결, 예상 전개
- 추가 정보가 없으면 빈 배열 []

## buzz_score (화제도 점수)
- 1~100 정수. 지금 사람들이 이 주제로 얼마나 대화하고 있는지의 체감 지표
- 90~100: 전 국민이 아는 수준 (대통령 탄핵, 월드컵 4강 등)
- 70~89: 직장/학교에서 한번쯤 들어봤을 정도
- 40~69: 관심 있는 사람들 사이에서 화제
- 1~39: 특정 커뮤니티에서만 화제
- top 카테고리의 1위는 반드시 70 이상이어야 함

## sources 작성 규칙
- 실제로 웹 검색에서 참고한 URL만 포함. 없으면 빈 배열 []
- ⚠️ **반드시 해당 토픽의 구체적인 페이지 URL만 포함!**
  * ✅ 좋은 예: 특정 기사 URL, 특정 커뮤니티 글, 특정 유튜브 영상, 나무위키 특정 문서
  * ❌ **절대 포함 금지**: 포털 메인, 검색 결과 페이지, 트렌드 집계 페이지
    - ❌ https://naver.com, https://finance.naver.com, https://search.naver.com/...
    - ❌ https://trends.google.co.kr/..., https://google.com
    - ❌ https://daum.net
  * 이런 URL은 "토픽의 출처"가 아니라 "검색 도구"입니다. 출처는 구체적인 콘텐츠 페이지여야 합니다
- **확실하지 않은 URL은 절대 넣지 마세요.** 존재하지 않는 URL을 만들어내느니 빈 배열이 낫습니다

## 톤 & 스타일
- ❌ 뉴스 기사체 금지: "~했다", "~밝혔다", "~것으로 알려졌다"
- ❌ **반말 절대 금지**: "~했어", "~됐음", "~임", "~했는데" (반말) → 절대 사용 금지
- ✅ 자연스러운 존댓말: "~했는데요", "~라서 난리예요", "~해서 화제예요", "~했대요"
- ✅ 편안하지만 예의 바른 톤: ~요, ~습니다, ~예요 등 존댓말 어미 사용
- 재미있는 주제는 재미있게, 심각한 주제는 적절한 톤으로
- 딱딱하고 건조한 뉴스 요약이 아니라, 읽는 사람이 "오 진짜요?" 하고 반응할 수 있는 글

## ❌ 나쁜 예시 (절대 이렇게 쓰면 안 됨)

| 나쁜 예시 | 왜 나쁜가 | 어떻게 고쳐야 하는가 |
|-----------|-----------|---------------------|
| topic: "2026 동계 올림픽 관련 소식" | 어떤 종목? 어떤 선수? 뭐가 화제? | → 키워드 목록에 있는 구체적 선수명과 사건을 그대로 사용 |
| topic: "주식 시장 동향" | 당연히 매일 오르내리니까 이건 뉴스가 아님 | → 키워드 목록에 있는 구체적 수치와 사건 |
| summary: "XX 관련 뉴스들이 속보로 나오고 있으며 주목받고 있습니다" | 무슨 뉴스? 뭐가 속보? 주목받는 이유는? | → 구체적 사건 + 왜 화제인지 |

### 요약: 절대 하면 안 되는 패턴
- **"XX 관련 소식"**: "소식"이라는 단어 자체가 구체성이 없다는 증거
- **"주목받고 있다" / "관심사로 떠오르고 있다"**: 왜 주목받는지를 써야지, "주목받고 있다"는 아무 정보도 없음
- **"등락" / "동향" / "변동"**: 이런 단어는 매일 해당되므로 화제가 아님
- **키워드 목록에 없는 주제 추가**: 위 목록에 없는 내용을 스스로 추가하지 마세요

## 작성 규칙
1. **구체성 최우선**: 인물명, 프로그램명, 수치, 발언 인용 등 대화에 쓸 수 있는 디테일
2. **summary 길이**: 1~3문장. 대화에 필요한 맥락이 충분히 담겨야 함. 핵심 정보를 줄이느라 모호해지면 안 됨
3. **details**: summary에 없는 추가 정보 최대 5개. 각각 한 문장
4. **흥미 요소 필수**: 논란/갈등/반전/기록/충격 중 최소 하나가 summary에 포함
5. **대화 연결점**: 가능하면 "사람들이 갈리는 의견"이나 "밈/유행어"를 언급
6. **순수 JSON**: 마크다운 코드 블록 없이 JSON만 출력

이제 위 키워드 목록을 각각 웹 검색으로 확인하고, 위 형식으로 응답해주세요.`, dateStr, dateStr, dateStr, keywordList, jsonFormatExample())
}

// jsonFormatExample returns the JSON schema example used in Stage 2 prompt.
// Source URLs intentionally show diverse types (trends, SNS, community, wiki)
// to prevent the model from only citing news articles.
func jsonFormatExample() string {
	return `{
  "items": [
    {
      "category": "top",
      "rank": 1,
      "topic": "[인물/사건 특정] 구체적 주제명",
      "summary": "누가 무엇을 했는데, 왜 사람들이 난리인지, 어떤 논란/반응이 있는지까지. 이 정보만으로 대화를 시작할 수 있어야 함.",
      "details": [
        "당사자가 직접 한 발언이나 SNS 게시물 인용",
        "커뮤니티에서 퍼진 밈이나 유행어",
        "관련 통계나 수치 (조회수, 검색량 등)"
      ],
      "buzz_score": 92,
      "sources": ["https://www.yna.co.kr/view/AKR20260222012345"]
    },
    {
      "category": "top",
      "rank": 2,
      "topic": "[인물/사건 특정] 두 번째 주제",
      "summary": "구체적 수치, 인물 발언, 커뮤니티 반응 등 대화 소재가 되는 디테일 포함.",
      "details": [
        "이 사건의 배경과 경위",
        "찬반 양측의 시각 요약"
      ],
      "buzz_score": 78,
      "sources": ["https://namu.wiki/w/관련주제"]
    },
    {
      "category": "entertainment",
      "rank": 1,
      "topic": "[프로그램명+인물명] 구체적 사건",
      "summary": "무슨 일이 있었고 사람들이 어떻게 반응하는지까지.",
      "details": [],
      "buzz_score": 65,
      "sources": []
    }
  ]
}`
}

// BuildSourceReviewPrompt returns a Stage 3 prompt that asks the AI to find
// replacement URLs for source links that failed HTTP validation.
// The AI is given the topic context and the broken URLs so it can search for
// correct alternatives. Pass empty items to get an empty prompt.
func BuildSourceReviewPrompt(items []ContextItem, invalid []InvalidSource) string {
	if len(invalid) == 0 {
		return ""
	}

	// Group invalid sources by item index for context.
	type entry struct {
		Topic  string
		URL    string
		Reason string
	}
	var entries []entry
	for _, inv := range invalid {
		topic := ""
		if inv.ItemIndex < len(items) {
			topic = items[inv.ItemIndex].Topic
		}
		entries = append(entries, entry{Topic: topic, URL: inv.URL, Reason: inv.Reason})
	}

	var list strings.Builder
	for i, e := range entries {
		list.WriteString(fmt.Sprintf("%d. 주제: %q / 실패한 URL: %s / 사유: %s\n", i+1, e.Topic, e.URL, e.Reason))
	}

	return fmt.Sprintf(`아래 출처 URL들이 실제로 접속했을 때 유효하지 않았습니다 (404, 페이지 없음 등).
각 항목에 대해 웹 검색으로 해당 주제의 실제 출처 URL을 찾아주세요.

## 실패한 URL 목록
%s
## 출력 형식
반드시 순수 JSON만 출력하세요. 마크다운 코드 블록 없이.

{
  "corrections": [
    {"old_url": "실패한 URL", "new_url": "대체 URL 또는 빈 문자열"}
  ]
}

## 규칙
- 대체 URL을 찾을 수 없으면 new_url을 빈 문자열 ""로 설정
- 대체 URL은 반드시 실제 존재하는 페이지여야 함
- 주제와 관련 있는 URL만 제공
- 확실하지 않으면 빈 문자열이 낫습니다`, list.String())
}

// parseKeywords extracts the keyword list from Stage 1 JSON response.
func parseKeywords(outputText string) ([]string, error) {
	clean := stripMarkdownCodeFence(outputText)

	var payload struct {
		Keywords []string `json:"keywords"`
	}
	if err := json.Unmarshal([]byte(clean), &payload); err != nil {
		return nil, fmt.Errorf("invalid json from keyword extraction: %w", err)
	}
	if len(payload.Keywords) == 0 {
		return nil, fmt.Errorf("no keywords returned from stage 1")
	}
	return payload.Keywords, nil
}
