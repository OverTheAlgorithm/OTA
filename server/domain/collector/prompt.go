package collector

import "fmt"

func BuildCollectionPrompt() string {
	return fmt.Sprintf(`당신은 한국의 실시간 사회 트렌드를 분석하는 전문가입니다.

오늘 한국에서 가장 화제가 되고 있는 주제들을 웹에서 검색하여 찾아주세요.
네이버 실시간 검색어, 트위터(X) 트렌드, 유튜브 인기 동영상, 주요 뉴스 등을 참고하세요.

반드시 아래 JSON 형식으로만 응답해주세요. JSON 외의 텍스트는 포함하지 마세요.

%s

규칙:
- "top" 카테고리: 분야를 막론하고 가장 뜨거운 주제 3~5개. rank는 1부터 시작.
- 그 외 카테고리: "entertainment", "politics", "economy", "sports", "technology" 등 해당하는 분야명을 영문 소문자로 작성. 각 카테고리별 1~3개.
- 각 summary는 반드시 한 문장. 어쩔 수 없는 경우에만 최대 두 문장.
- "정치", "경제" 같은 추상적 분류가 아닌, 구체적인 맥락을 topic과 summary에 담아주세요.
- 좋은 예: topic "환승연애 시즌3", summary "출연자가 전 남자친구를 두 명이나 데리고 출연하며 화제를 모으고 있다."
- 나쁜 예: topic "연예", summary "연예계에 이슈가 있다."
- sources에는 참고한 URL을 포함해주세요.`, jsonFormatExample())
}

func jsonFormatExample() string {
	return `{
  "items": [
    {
      "category": "top",
      "rank": 1,
      "topic": "구체적 주제명",
      "summary": "한 문장으로 된 구체적 맥락 요약",
      "sources": ["https://example.com/article1"]
    },
    {
      "category": "entertainment",
      "rank": 1,
      "topic": "구체적 주제명",
      "summary": "한 문장으로 된 구체적 맥락 요약",
      "sources": ["https://example.com/article2"]
    }
  ]
}`
}
