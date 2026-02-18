package collector

import "fmt"

func BuildCollectionPrompt() string {
	return fmt.Sprintf(`당신은 한국의 실시간 사회 트렌드를 분석하는 전문가입니다.

## 목표
오늘(2026년 2월 현재) 한국에서 가장 화제가 되고 있는 주제들을 웹에서 검색하여 찾아주세요.
네이버 실시간 검색어, 트위터(X) 트렌드, 유튜브 인기 동영상, 주요 뉴스 사이트를 참고하세요.

## 출력 형식
**매우 중요**: 반드시 순수 JSON만 출력하세요. 마크다운 코드 블록(` + "```json" + `)이나 추가 설명 없이, JSON 객체만 출력해주세요.

%s

## 카테고리별 규칙

### "top" 카테고리 (필수, 3~5개)
- **정의**: 분야를 막론하고 현재 한국에서 **가장 많이 언급되고 화제**가 되는 주제
- **판단 기준**: 검색량, 댓글 수, 뉴스 기사 수, SNS 언급 빈도
- rank는 1부터 시작하며, 가장 뜨거운 주제가 1번
- 정치, 연예, 경제, 스포츠 등 모든 분야를 아우름

### 그 외 카테고리 (선택, 각 1~3개)
- **사용 가능 카테고리**: "entertainment", "politics", "economy", "sports", "technology", "society"
- 영문 소문자로 작성
- 해당 분야에서 화제인 주제만 포함 (억지로 채우지 않아도 됨)

## 작성 가이드

### ✅ 좋은 예시
` + "```json" + `
{
  "items": [
    {
      "category": "top",
      "rank": 1,
      "topic": "환승연애3 김혜선 논란",
      "summary": "출연자 김혜선이 전 남자친구 2명과 함께 출연해 양다리 논란에 휩싸였으며, SNS에서 해명글을 올렸으나 비난이 계속되고 있다.",
      "sources": ["https://naver.com/news/123"]
    },
    {
      "category": "top",
      "rank": 2,
      "topic": "비트코인 8000만원 돌파",
      "summary": "비트코인이 사상 최고가인 8000만원을 돌파하며 개인 투자자들의 관심이 폭발적으로 증가하고 있다.",
      "sources": ["https://naver.com/economy/456"]
    },
    {
      "category": "entertainment",
      "rank": 1,
      "topic": "아이유 콘서트 티켓팅 대란",
      "summary": "3월 서울 콘서트 예매가 시작 1분 만에 전석 매진되며 암표가 10배 가격에 거래되고 있다.",
      "sources": ["https://naver.com/enter/789"]
    }
  ]
}
` + "```" + `

### ❌ 나쁜 예시
- topic: "연예계 소식" → 너무 추상적
- topic: "정치" → 구체적인 사건/인물 없음
- summary: "이슈가 있다." → 맥락 부족, 무슨 이슈인지 불명확
- summary: "논란이 되고 있다. 많은 사람들이 관심을 갖고 있다. 앞으로 어떻게 될지 주목된다." → 3문장 (규칙 위반)

## 작성 규칙 요약
1. **구체성**: 사람 이름, 사건명, 숫자 등 구체적 정보 포함
2. **간결성**: summary는 1문장 (예외적으로 최대 2문장)
3. **맥락**: "왜 화제인가"를 명확히 설명
4. **순수 JSON**: 마크다운 코드 블록 없이 JSON만 출력
5. **실시간성**: 오늘/최근 며칠 이내의 화제만 포함 (과거 사건 제외)

이제 웹 검색을 통해 한국의 실시간 트렌드를 수집하고 위 형식으로 응답해주세요.`, jsonFormatExample())
}

func jsonFormatExample() string {
	return `{
  "items": [
    {
      "category": "top",
      "rank": 1,
      "topic": "구체적 주제명 (예: 사건명, 인물명 포함)",
      "summary": "한 문장으로 된 구체적 맥락 요약 (누가, 무엇을, 왜 화제인지)",
      "sources": ["https://example.com/article1"]
    },
    {
      "category": "top",
      "rank": 2,
      "topic": "두 번째 화제 주제",
      "summary": "이 주제가 왜 화제인지 구체적으로 설명하는 한 문장",
      "sources": ["https://example.com/article2"]
    },
    {
      "category": "entertainment",
      "rank": 1,
      "topic": "연예 분야 화제",
      "summary": "해당 분야에서의 구체적 맥락",
      "sources": ["https://example.com/article3"]
    }
  ]
}`
}
