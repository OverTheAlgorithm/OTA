# 커뮤니티 트렌드 — 구현 계획 #02: 수집 파이프라인

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development 또는 executing-plans.
> 선행: 플랜 #01 (스키마·커뮤니티/태그 CRUD) 완료.

**Goal:** 소스 중립 어댑터로 커뮤니티 베스트판 관측 항목을 수집하고, robots 게이트·중복검정·AI 1패스 태깅·사람 확정을 거쳐 `ct_tag_daily`에 집계를 쌓는 end-to-end 파이프라인.

**Architecture:** `platform/communities`에 정중한 HTTP + 사이트별 어댑터(`golang.org/x/net/html` 사용, goquery 미도입). `domain/communitytrend`에 `TrendItem`/`SourceAdapter`/`AdapterRegistry`, robots 판정, 지문/dedup, `Tagger` 인터페이스, 워크시트 상태기계, 일일 런 서비스.

**Tech Stack:** Go 1.25, `golang.org/x/net/html`, pgx, testcontainers, gemini(`platform/gemini`) 래핑.

## Global Constraints

- 원문(제목/본문) DB 미저장. `TrendItem.TextUnit`은 메모리 휘발성.
- 정중한 수집: 정직한 User-Agent(봇 명시), 사이트당 동시성 1, 타임아웃, robots Disallow 시 자동 접근 0.
- **어댑터는 실제 작동을 반드시 검증** (사용자 요구): ① fixture 기반 결정적 단위 테스트(CI), ② `CT_LIVE_SMOKE=1` 가드 라이브 스모크 테스트(수동 실행으로 실제 사이트 파싱 확인).
- 카운트=신규유입형, 임계값 `CT_MIN_TAG_COUNT`(#04에서 사용).

## File Structure

| 파일 | 책임 |
|------|------|
| `domain/communitytrend/adapter.go` | `TrendItem`, `SourceAdapter`, `AdapterRegistry` |
| `domain/communitytrend/robots.go` | robots.txt 파싱 + 경로 허용 판정 |
| `domain/communitytrend/robots_test.go` | robots 파서 단위 테스트 |
| `domain/communitytrend/dedup.go` | 지문 생성(sha256) + 신규 필터 |
| `domain/communitytrend/dedup_test.go` | 지문 단위 테스트 |
| `platform/communities/fetch.go` | 정중한 HTTP client |
| `platform/communities/dogdrip.go` | dogdrip 어댑터 (boomupbest) |
| `platform/communities/clien.go` | clien 어댑터 |
| `platform/communities/parse.go` | x/net/html 공통 노드 워크 헬퍼 |
| `platform/communities/fixtures/*.html` | 저장 HTML (CI 결정적 테스트) |
| `platform/communities/dogdrip_test.go` | fixture 파싱 + 라이브 스모크 |
| `domain/communitytrend/tagger.go` | `Tagger` 인터페이스 + 입출력 타입 |
| `domain/communitytrend/tag_prompt.go` | 프롬프트 빌더(분류체계 주입, 보수적 규칙, 1패스) |
| `platform/gemini/ct_tagger.go` | `Tagger` gemini 구현 |
| `domain/communitytrend/worksheet.go` | 워크시트 상태기계 + repo 인터페이스 |
| `storage/ct_worksheet_repo.go`, `storage/ct_tag_daily_repo.go`, `storage/ct_robots_repo.go`, `storage/ct_seen_repo.go` | repo 구현 |
| `domain/communitytrend/pipeline.go` | 일일 런 오케스트레이션 |
| `scheduler/community_trend_job.go` | 일일 잡 |
| `api/handler/community_trend_handler.go` | 워크시트 조회/확정 엔드포인트 추가 |

## Tasks (요약 — 각 Task는 TDD: 실패테스트→구현→통과→커밋)

1. **어댑터 인터페이스 + 레지스트리** (`adapter.go`) — `TrendItem{SourceID,TextUnit,Engagement,ObservedAt}`, `SourceAdapter{Key,RobotsURL,BestBoardPaths,FetchRecent}`, `AdapterRegistry`(Register/Get/Keys). 단위 테스트.
2. **정중한 HTTP + dogdrip 어댑터** (`fetch.go`,`parse.go`,`dogdrip.go`) — fixture 파싱 단위 테스트 + **라이브 스모크 테스트**. ← 사용자 요구 핵심.
3. **clien 어댑터** — fixture + 라이브 스모크.
4. **robots 게이트** (`robots.go`) — User-agent `*` 그룹 파싱, 경로 prefix 매칭, 404=허용/403·429=거부. 단위 테스트(표 기반).
5. **지문/dedup** (`dedup.go`) — `Fingerprint(communityKey, sourceID)=sha256hex`, seen 필터. 단위 테스트.
6. **repo 구현** — worksheet/tag_daily/robots/seen. 통합 테스트.
7. **Tagger 인터페이스 + 프롬프트** + gemini 구현 — 프롬프트 빌더 단위 테스트(분류체계 주입 확인). 실제 호출은 `CT_LIVE_SMOKE` 가드.
8. **워크시트 상태기계 + 파이프라인** — pending→suggested→confirmed, 확정 시 원자적 기록. 통합 테스트.
9. **확정 API + 워크시트 조회 API** — 핸들러 확장 + 통합 테스트.
10. **일일 스케줄러 잡** — robots→fetch→suggest 자동, 확정은 사람. 와이어링.

> Task 2,3,7은 `CT_LIVE_SMOKE=1`로 실제 사이트/AI 호출을 검증한다. CI 기본 실행에서는 fixture/모의로 결정적.

## 어댑터 검증 전략 (사용자 요구 반영)

- **fixture 단위 테스트**: `fixtures/dogdrip_boomupbest.html`(실제 저장본) 파싱 → 항목 수>0, SourceID·TextUnit 비어있지 않음, 댓글수 파싱. 네트워크 없음 → CI 안정.
- **라이브 스모크 테스트**: `CT_LIVE_SMOKE=1` 일 때만 실행. 실제 dogdrip/clien에 1회 요청 → 항목 수>0 확인. 구현 시 1회 수동 실행해 실제 작동 증명, 결과를 커밋 메시지/PR에 기록.
