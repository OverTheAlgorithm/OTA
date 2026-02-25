# Known Issues & Technical Debt

## [Data Collection] Google Trends RSS 피드 제한된 결과 반환

**발견일**: 2026-02-24
**심각도**: Medium
**상태**: Open

### 문제

Google Trends 웹사이트(trends.google.com/trending?geo=KR&hours=24)에서는 24시간 기준 80~93개의 트렌딩 키워드를 표시하지만, RSS 피드(trends.google.co.kr/trending/rss?geo=KR)는 약 10~15개만 반환한다.

### 영향

- 트렌딩 키워드 커버리지가 전체의 10~15% 수준
- 네이버 API 검색의 seed 키워드가 부족하여 수집 범위 제한

### 조사된 대안

| 방안 | 설명 | 상태 |
|------|------|------|
| Google Trends API (Alpha) | 2025.07 발표, 알파 신청 필요 | 대기 |
| 웹사이트 JS 렌더링 파싱 | Headless browser 필요, 무겁고 불안정 | 보류 |
| Google Trends 내부 API 호출 | 비공식, 안정성 리스크 | 미조사 |
| RSS 결과 + 네이버 API 보완 | 현재 채택한 방식 | 진행 중 |

---

## [Data Collection] Google News RSS 상업적 사용 제한

**발견일**: 2026-02-24
**심각도**: Low (현재 운영에 영향 없음)
**상태**: 인지됨

### 문제

Google News RSS 피드 약관에 "personal, non-commercial use" 전용이라고 명시되어 있음.
현재 코드에 Google News RSS collector(server/platform/googlenews/)가 구현되어 있으나, 상업적 서비스에서 사용 시 약관 위반 소지.

### 결정

- 현재는 Google Trends RSS(키워드 수집) + 네이버 API(기사 검색) 하이브리드 방식으로 전환 중
- Google News RSS collector는 당장 삭제하지 않으나, 프로덕션 파이프라인에서는 사용하지 않는 방향
- 네이버 뉴스 검색 API는 "유사 검색 서비스 운영" 외에는 상업적 제한 없음 확인

### 한국 언론사 RSS 참고

주요 언론사 15곳 중 12곳이 자체 RSS를 제공하나, 전부 "개인 비상업적 사용 전용" 약관.
한국 디지털뉴스 이용규칙상 RSS 콘텐츠의 상업적 재배포는 일괄 금지.
단, 헤드라인으로 트렌드 파악 후 자체 콘텐츠 작성 + 원문 링크 제공 모델(뉴닉/어피티 방식)은 법적 회색지대.
