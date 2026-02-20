package collector

import (
	"encoding/json"
	"fmt"
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
구글 한국어 검색어 순위, 네이버 실시간 검색어, 트위터(X) 트렌드, 유튜브 인기 동영상, 주요 뉴스 사이트, 커뮤니티(에펨코리아, 디시인사이드, 더쿠)를 참고하세요.

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
- 반드시 구체적 사건·인물·수치·프로그램명 포함: "오픈AI o3 한국 출시 논란", "[가수명] 콘서트 취소", "코스피 X%% 급락"
- 오늘 또는 최근 2~3일 이내의 화제만 포함
- 연예, 경제, 스포츠, 정치, 기술, 사회 등 다양한 분야에서 고르게 추출
- 마크다운 코드 블록 없이 JSON만 출력`, dateStr)
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

## 오늘 화제 키워드 목록 (%s 기준)
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

- **좋은 summary**: "[실제 인물명]이 [구체적 사건]을 했다. [왜 논란인지/사람들 반응]. [대화를 이어갈 수 있는 포인트]."
- **좋은 detail**: 위 summary의 배경, 경위, 당사자 입장, 찬반 여론, 이 사건의 의미. 3~5문장.
- **sources**: 실제로 참고한 뉴스/기사 URL. 없으면 빈 배열 [].

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
3. **detail 길이**: 3~5문장. summary의 배경, 경위, 핵심 인물의 발언, 대립하는 시각, 왜 이게 중요한지를 담아야 함. "이걸 읽으면 그 주제에 대해 5분은 대화할 수 있다"는 기준
4. **흥미 요소 필수**: 논란/갈등/반전/기록/충격 중 최소 하나가 summary에 포함
5. **대화 연결점**: 가능하면 "사람들이 갈리는 의견"이나 "밈/유행어"를 언급
6. **순수 JSON**: 마크다운 코드 블록 없이 JSON만 출력

이제 위 키워드 목록을 각각 웹 검색으로 확인하고, 위 형식으로 응답해주세요.`, dateStr, keywordList, jsonFormatExample())
}

// jsonFormatExample returns the JSON schema example used in Stage 2 prompt.
func jsonFormatExample() string {
	return `{
  "items": [
    {
      "category": "top",
      "rank": 1,
      "topic": "[인물/사건 특정] 구체적 주제명",
      "summary": "누가 무엇을 했고, 왜 사람들이 이야기하는지, 어떤 논란/반응이 있는지까지 포함. 이 정보만으로 대화를 시작할 수 있어야 함.",
      "detail": "summary보다 깊은 배경 설명. 사건의 경위, 핵심 인물의 발언 인용, 찬반 양측의 시각, 이 사건이 왜 중요한지, 앞으로 어떻게 될 것 같은지까지. 3~5문장으로 충분히 읽을 만한 분량.",
      "sources": ["https://example.com/article1"]
    },
    {
      "category": "top",
      "rank": 2,
      "topic": "[인물/사건 특정] 두 번째 주제",
      "summary": "구체적 수치, 인물 발언, 커뮤니티 반응 등 대화 소재가 되는 디테일 포함.",
      "detail": "이 주제의 배경, 왜 지금 화제가 됐는지, 관련 인물들의 입장, 커뮤니티에서 어떤 논쟁이 벌어지고 있는지 상세히.",
      "sources": ["https://example.com/article2"]
    },
    {
      "category": "entertainment",
      "rank": 1,
      "topic": "[프로그램명+인물명] 구체적 사건",
      "summary": "무슨 일이 있었고 사람들이 어떻게 반응하는지까지.",
      "detail": "사건의 전말, 당사자 반응, 제작진 입장, 시청자 여론이 어떻게 나뉘는지, 관련 밈이나 유행어까지 상세히.",
      "sources": ["https://example.com/article3"]
    }
  ]
}`
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
