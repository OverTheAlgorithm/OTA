# 퀴즈 UX 재설계 계획 — Phase 1 (버그 수정) + Phase 2 (Paced Reveal)

**작성일**: 2026-04-10
**범위**: `/topic/:id` 페이지의 보너스 퀴즈 기능
**상태**: 계획 (미승인)
**스코프 제외**: `mobile/` (React Native, 미배포)

---

## 0. Executive Summary

### 해결하려는 문제
1. **버그**: 10초 카운트다운 후 포인트 획득에 성공해도 퀴즈 카드가 활성화되지 않고 **사라짐**. 새로고침하면 나타남.
2. **UX 문제**:
   - 로그인 유저는 earn 완료 즉시 "띡" 하고 결과가 튀어나와 긴장감이 없음
   - 비로그인 유저는 퀴즈의 존재조차 모름 → 참여 유도 실패
   - 퀴즈를 푼 유저가 새로고침하면 카드가 사라져 과거 답변 확인 불가

### 채택 방향
- **Phase 1**: READ 경로의 earn-gate 제거. 제출 시점의 authoritative earn-gate는 유지.
- **Phase 2**: 상태 머신 기반 **Paced Reveal** (백엔드 + 프론트 동시 변경)
  - 비로그인 유저에게도 퀴즈 노출하고 클릭 시 로그인 유도
  - 답은 localStorage에 보존해 OAuth 복귀 후 자동 진행
  - 백엔드 `QuizForUser`에 `past_attempt` 필드 추가, 완료한 퀴즈를 정적 비활성화 카드로 hydration
  - submit 실패 시 최대 5회 재시도 허용

### 원칙
1. **Authoritative gate는 제출 시점에만 둔다** — 읽기 시점 gate는 UX 부작용만 크고 실효 없음
2. **페이싱은 고정 sleep이 아니라 최소 표시 시간(min display duration)** — 느린 네트워크에서 누적 지연 방지
3. **각 비동기 작업 + 타이머는 반드시 cleanup** — unmount, navigate-away, 탭 전환 모두 안전해야 함
4. **오답 피드백은 성공보다 짧고 담백하게** — 조롱감 없이, 시각 효과 비대칭
5. **prefers-reduced-motion 존중** — 애니메이션만 끄고 텍스트 스테이지 전환은 유지 (유저가 상태 변화를 인지해야 함)

---

## 1. 루트 원인 분석

### 버그 체인
```
GetTopicByID 호출 (최초 mount)
  ↓
GetQuizForUser(userID, itemID) 호출
  ↓
levelRepo.GetEarnedItemIDs(userID, [itemID]) — coin_logs 조회
  ↓
len(earned) == 0  → return nil, ErrNotEarned
  ↓
handler가 swallow → resp.Quiz = nil
  ↓
프론트 topic.quiz = null 상태로 state 진입
  ↓
[유저가 10초 카운트다운 기다림, earn 성공]
  ↓
handleCountdownComplete → setCoinTag({kind:"success"})
  ↓
하지만 topic.quiz는 여전히 stale null
  ↓
QuizCard 렌더 (earnDone=true)
  ↓
quiz-card.tsx:108  if (!quiz) return null  ← ❌ 카드 소멸
```

### 핵심 인사이트
- `SubmitAnswer`는 별도로 `GetEarnedItemIDs` 체크를 수행함 (service.go:88-95). 이것이 실제 치팅 방어선.
- 따라서 `GetQuizForUser`의 earn-gate는 **UX 중복 체크**이며, 오히려 위 버그 체인의 원인.
- 퀴즈 보너스는 `coin_events` 경로로 지급 → daily limit 우회하지만 `COIN_CAP`이 상한. 보너스 크기는 `1 ~ QUIZ_MAX_BONUS_COINS` 랜덤.
- **오답 시 API가 정답 인덱스를 의도적으로 반환하지 않음** (`server/domain/quiz/model.go:29` 주석: "CorrectIndex is intentionally absent"). 유저가 퀴즈를 다시 풀 수 있어야 하는 기획과 정합. 오답 UI는 정답 reveal 불가.

---

## 2. Phase 1 — 버그 수정 (독립 머지)

목표: 현재 "카드가 사라지는" 버그만 제거. UX는 기존과 동일 (잠금 → 해제). 독립적으로 프로덕션 머지 가능.

### 2.1 백엔드 변경

#### `server/domain/quiz/service.go`
- `GetQuizForUser` (47-82행)에서 earn-gate 블록 제거:
  - **삭제할 부분**: 48-56행의 `GetEarnedItemIDs` 호출과 `ErrNotEarned` 반환
  - **유지**: 이후의 `GetByContextItemID`, `HasAttempted` 체크, `ErrAlreadyAttempted` 반환
  - **유지**: `ErrNotEarned` 상수(16행) — `SubmitAnswer`(94행)와 `quiz_handler.go:56`이 여전히 사용
- 함수 Doc comment(39-46행) 업데이트:
  - 이전 주석 1번 "User has earned coins for the article" 제거
  - "Earn-gate is intentionally NOT checked here — it is enforced authoritatively in SubmitAnswer. Exposing quiz data before earn is safe because submission is the gated operation." 추가

#### `server/api/handler/context_history.go`
- 91-97행의 에러 필터 조건 단순화:
  - 현재: `if err != nil && !errors.Is(err, quiz.ErrNotEarned) && !errors.Is(err, quiz.ErrAlreadyAttempted)`
  - 변경 후: `if err != nil && !errors.Is(err, quiz.ErrAlreadyAttempted)` (ErrNotEarned는 이 경로에서 더 이상 발생하지 않음)
- `errors` 패키지 import 유지 (`ErrAlreadyAttempted` 때문)

### 2.2 백엔드 테스트 변경

#### `server/integration/quiz_test.go`
- **TestQuiz_EarnGateBlocks** (132-154행) **뒤집기**:
  - 새 이름: `TestQuiz_ReadPathOpenSubmitGated`
  - 검증 1: `GetQuizForUser`가 coin_logs 없이도 퀴즈를 **성공 반환** (새 동작)
  - 검증 2: 같은 유저/아이템에 대해 `SubmitAnswer`는 여전히 `ErrNotEarned` 반환 (기존 gate 유지 검증)
  - 이 단일 테스트로 "READ는 열림 / SUBMIT은 여전히 닫힘" invariant 보장
- **TestQuiz_EarnGateAllows** (157-190행) 그대로 유지 — coin_logs 존재 시 정상 반환 계속 검증
- 그 외 submit 관련 테스트들(CorrectAnswer, WrongAnswer, DuplicateSubmission, BonusExempt, BatchStatus) 변경 없음

### 2.3 프론트엔드 변경 (Phase 1만)

Phase 1에서는 **버그만** 고치므로 프론트 변경은 사실상 없음. 백엔드가 `topic.quiz`를 최초 fetch에 포함해서 내려주므로 기존 `quiz-card.tsx` 로직이 자동으로 동작:
- earn 전: `earnDone=false` → 잠금 UI (기존과 동일)
- earn 후: `earnDone=true` + `quiz` 존재 → 활성 UI (**버그 수정**)

⚠️ 단, Phase 1 배포 직후 확인 필요:
- 로그인 유저가 토픽 페이지 최초 진입 시 네트워크 응답에 `quiz: {...}`가 실려 오는지
- earn 전 상태에서 "잠금" UI가 그대로 표시되는지 (QuizCard는 `earnDone` prop만 보고 잠금 여부 결정하므로 영향 없음)
- earn 후 활성 카드에서 답 제출이 정상 동작하는지

### 2.4 Phase 1 Acceptance Criteria
- [ ] 로그인 유저가 `/topic/:id` 최초 로드 시 `/api/v1/context/topic/:id` 응답에 `quiz` 필드가 `{id, question, options}` 형태로 포함
- [ ] 카운트다운 10초 경과 후 earn 성공 시 퀴즈 카드가 잠금 → 활성으로 전환 (페이지 새로고침 없이)
- [ ] 비로그인 유저에게는 여전히 `quiz: null`은 아니지만 프론트가 렌더하지 않음 (`user &&` 가드 유지, 스코프 밖)
- [ ] `server/integration/quiz_test.go`의 재작성된 테스트 통과
- [ ] Submit 경로의 earn-gate가 여전히 작동 (테스트에서 검증)

### 2.5 Phase 1 Rollback
- 백엔드 revert: 두 파일(`service.go`, `context_history.go`)의 이전 버전으로 복원 → 즉시 복구 가능 (dependent DB 변경 없음)
- 테스트: 이전 버전으로 복원 (integration test만 영향)
- 프론트엔드: Phase 1은 프론트 변경 없음 → rollback 불필요

---

## 3. Phase 2 — Paced Reveal UX 재설계

**전제**: Phase 1이 이미 머지되어 `topic.quiz`가 비-null로 내려오는 상태.

### 3.1 상태 머신 정의

```
             ┌────────────┐
             │    IDLE    │ ← 옵션만 표시, 유저 클릭 대기
             └─────┬──────┘
                   │ 옵션 클릭
                   │ (로그인 유저)
                   ▼
         ┌──────────────────────┐
         │ SELECTED_WAITING_EARN│ "포인트 획득 대기중..."
         │                      │ 최소 표시 0.6s + earn 완료까지 대기
         │                      │ 다른 옵션 재선택 허용 (선택만 교체)
         └─────────┬────────────┘
                   │ earn 완료 + 최소 시간 경과
                   ▼
         ┌──────────────────────┐
         │   EARN_CONFIRMED     │ "포인트 획득 완료!"
         │                      │ 고정 1.0s
         │                      │ 연출: 코인 아이콘 + 숫자 count-up
         └─────────┬────────────┘
                   ▼
         ┌──────────────────────┐
         │     SUBMITTING       │ "퀴즈 정답 제출 중..."
         │                      │ min 0.8s + submit 왕복 (longer wins)
         │                      │ 연출: 선택 카드 pulse
         └─────────┬────────────┘
                   │ submit 성공
                   ▼
         ┌──────────────────────┐
         │     EVALUATING       │ "정답 검사 중..."
         │                      │ 고정 0.8s (순수 연출)
         │                      │ 연출: 카드 미세 wobble
         └──┬───────────────┬───┘
            │               │
    result.correct        !result.correct
            │               │
            ▼               ▼
  ┌─────────────────┐  ┌─────────────────┐
  │ RESULT_CORRECT  │  │  RESULT_WRONG   │
  │ 3s+ (persist)   │  │ 1.5s (담백)     │
  │ 초록 accent     │  │ 빨강 accent     │
  │ +N count-up     │  │ shake 1회       │
  │ 최종 상태       │  │ "아쉽지만 틀렸  │
  │                 │  │ 어요"           │
  │                 │  │ (정답 reveal X) │
  └─────────────────┘  └─────────────────┘

  // Submit 실패 경로
  SUBMITTING
     │ submit 에러
     ▼
  ┌─────────────────┐
  │  SUBMIT_FAILED  │ "제출에 실패했어요"
  │                 │ [다시 시도] 버튼
  │                 │ → 클릭 시 SUBMITTING으로 복귀
  └─────────────────┘
```

### 3.2 중요 페이싱 원칙: Min Display Duration

각 단계마다 **최소 표시 시간**과 **비동기 작업**이 `AND`로 묶임. 둘 다 충족되어야 다음 단계로 전이.

| Stage | Min Display | Async Work | 전이 조건 |
|-------|-------------|------------|----------|
| A (SELECTED_WAITING_EARN) | 0.6s | earn 완료 (`earnCommitted` prop true) | 둘 다 만족 |
| B (EARN_CONFIRMED) | 1.0s | 없음 | 시간만 |
| C (SUBMITTING) | 0.8s | `submitQuizAnswer` 응답 | 둘 다 만족 |
| D (EVALUATING) | 0.8s | 없음 | 시간만 |
| E (RESULT_*) | persist (correct 3s+, wrong 1.5s) | 없음 | 최종 상태 |

이유: 네트워크 빠른 유저에겐 리듬 보장, 느린 유저에겐 누적 지연 없음.

### 3.3 비로그인 유저 플로우

```
1. 페이지 진입 → QuizCard 렌더 (IDLE)
   상단 배지: "로그인하고 도전해보세요"
   옵션 4개 클릭 가능

2. 유저가 옵션 클릭
   → onRequestLogin(selectedIndex) 콜백
   → topic.tsx가 quiz-pending.ts helper로 선택 저장
     localStorage["wl_quiz_pending_answer_v1"] = {
       topic_id, selected_index, saved_at
     }
   → LoginPromptModal(variant="quiz_submit") 표시
     카피: "로그인하고 정답을 제출해 포인트를 받아보세요"

3. 유저가 카카오 로그인 클릭
   → KakaoLoginButton이 LOGIN_REDIRECT_KEY에 "/topic/:id" 저장
   → 카카오 OAuth 리다이렉트

4. OAuth 완료 → 서버 callback → 프론트 landing.tsx 복귀
   → landing.tsx가 LOGIN_REDIRECT_KEY 읽고 /topic/:id로 navigate
   → topic.tsx mount

5. topic.tsx mount에서 user 로딩 완료 감지
   → useEffect가 initEarn 호출 (기존 동작) + 카운트다운 시작
   → QuizCard mount에서 quiz-pending helper가 이 topic에 대한 pending selection 조회
   → 찾으면 selectedIndex를 복원하고 SELECTED_WAITING_EARN 상태로 진입
   → 카드에는 "포인트 획득 대기중..." 표시

6. 10초 카운트다운 완료 → earn 성공
   → earnCommitted prop true
   → 상태 머신이 자동으로 B → C → D → E 진행

7. Submit 성공/실패 시점에 quiz-pending helper 정리 (clearPendingAnswer)
```

### 3.4 엣지 케이스 처리 명세

#### 3.4.1 Stale pending selection
- **케이스 1**: OAuth 복귀 후 topic detail fetch가 `quiz: null` 반환 (퀴즈 자체가 삭제된 등 비정상 케이스)
  - **처리**: QuizCard mount 시 `topic.quiz === null`이면 pending 존재해도 무시 + `clearPendingAnswer` 호출
  - **UI**: 퀴즈 카드 자체를 렌더하지 않음 (`topic.has_quiz`도 false거나 quiz null)
- **케이스 2**: OAuth 복귀 후 quiz는 있지만 `quiz.past_attempt`도 함께 옴 (다른 디바이스/탭에서 이미 풀었음)
  - **처리**: 3.4.8 hydration이 우선. mount useEffect에서 past_attempt 체크 → 즉시 result_correct/wrong로 진입 + `clearPendingAnswer`
  - **UI**: hydration UI (정적 결과 카드)
- **케이스 3**: pending이 다른 topic_id이거나 TTL 만료
  - **처리**: `loadPendingAnswer(topicId)` 자체에서 null 반환 + 자동 clear (localStorage helper 책임)
  - **UI**: IDLE 상태로 시작

#### 3.4.2 타이머 누수
- 모든 setTimeout은 `useRef<NodeJS.Timeout | null>(null)` 배열로 관리
- `useEffect` cleanup에서 전부 `clearTimeout`
- unmount / navigate-away / 상태 머신 재시작 시 기존 타이머 취소 후 새로 설정
- 구체: `stageTimersRef.current.forEach(clearTimeout); stageTimersRef.current = []`

#### 3.4.3 Submit 더블파이어
- `submitCalledRef = useRef(false)` 가드
- Stage C 진입 시점에 `if (submitCalledRef.current) return; submitCalledRef.current = true;`
- 실패 후 재시도 시에만 false로 리셋
- 기존 `earnCalledRef` 패턴과 동일 (topic.tsx:347,353)

#### 3.4.4 탭 전환 중 Stage A
- 유저가 옵션 선택 후 탭 전환 → CountdownTag rAF 일시정지 (기존 visibilitychange 로직, topic.tsx:83-94)
- 탭 복귀 시 카운트다운 재개 → 정상 earn → Stage A min 시간 경과 여부 체크 후 B로 전이
- **추가 안전장치**: Stage A에서 `earnCommitted` prop이 true로 바뀌고 min 시간 경과했다면, `visibilitychange`나 mount 사이클에 의존하지 않고 effect로 자동 전이
- QuizCard의 상태 머신은 `earnCommitted` prop + min display 타이머만 본다. 탭 전환과는 무관.

#### 3.4.5 prefers-reduced-motion
- CSS 레벨에서 전부 제어:
  ```css
  @media (prefers-reduced-motion: reduce) {
    .wl-anim-shake, .wl-anim-pulse-soft, .wl-anim-wobble, .wl-anim-count-up {
      animation: none !important;
    }
  }
  ```
- 각 단계의 **최소 표시 시간은 유지** (유저가 텍스트 전환을 인지해야 하므로)
- 단, Stage 전환의 감도를 위해 min display를 축소하지 않음 (동일 0.6/1.0/0.8/0.8)

#### 3.4.6 Submit 네트워크 실패
- `submitQuizAnswer` catch 블록에서 `SUBMIT_FAILED` 상태로 전이
- UI: 빨간 테두리 카드 + "제출에 실패했어요. 다시 시도해주세요." + [다시 시도] 버튼
- 재시도 클릭 시 `submitCalledRef.current = false` → SUBMITTING 상태 재진입
- **재시도 횟수 제한**: **최대 5회** (옵션 개수 4 + 1 = generous). `retryCountRef = useRef(0)` 으로 관리, 5회 초과 시 재시도 버튼 비활성화 + "잠시 후 다시 방문해주세요 🙏" 메시지로 종료
- Rate limit 고려: 재시도 버튼에 간단한 debounce (double-click 방지용 onClick disable, 재시도 중 disabled 상태)
- `retryCountRef`는 unmount 시 초기화 불필요 (컴포넌트 언마운트되면 자동 GC)

#### 3.4.7 Coin tag와 Stage B 동기화
- `handleCountdownComplete`의 success 분기가 **같은 React update cycle에서**:
  1. `setCoinTag({kind: "success", ...})` — 페이지 상단 코인 태그 flash
  2. `setEarnCommitted(true)` (신규 state) — QuizCard prop으로 전달
- `setEarnCommitted`를 state로 승격시키면 QuizCard가 prop 변경 감지 → Stage A 조건 체크 → min 시간 경과 시 즉시 Stage B 진입
- 두 setState가 같은 batch에 들어가므로 React 18+ auto-batching으로 1 frame 내 정렬

#### 3.4.8 이미 완료된 퀴즈의 hydration (PO 요청 — Phase 2에 포함)
- **PO 결정**: 카드 숨김이 아니라 **답이 선택된 상태 + 비활성화된 정적 결과 카드**를 표시. 새로고침 / 며칠 후 재방문해도 유저의 과거 답변과 정답 여부가 그대로 복원되어야 함.
- **백엔드 변경 필요**:
  - `server/domain/quiz/model.go`에 신규 struct `PastAttempt { SelectedIndex int, IsCorrect bool, CoinsEarned int, AttemptedAt time.Time }` 추가
  - `QuizForUser`에 optional 필드 `PastAttempt *PastAttempt` (json: `past_attempt,omitempty`) 추가
  - `server/domain/quiz/repository.go` Repository 인터페이스에 `GetUserAttempt(ctx, userID, quizID) (*PastAttempt, error)` 메서드 추가. 미attempted면 `nil, nil`
  - `server/storage/quiz_repository.go`(실제 구현 파일)에서 해당 메서드 구현 — `SELECT answered_index, is_correct, coins_earned, created_at FROM quiz_results WHERE user_id=$1 AND quiz_id=$2 LIMIT 1`. 스키마 변경 불필요 (integration test `quiz_test.go:236`에서 컬럼 존재 확인됨)
  - `server/domain/quiz/service.go`의 `GetQuizForUser`:
    - `HasAttempted` 호출 제거, 대신 `GetUserAttempt` 호출
    - attempt 있으면 `QuizForUser.PastAttempt` 필드 채워서 반환
    - `ErrAlreadyAttempted` 반환 로직 완전 제거 (하지만 상수는 `SubmitAnswer`와 `quiz_handler.go:56`이 여전히 사용하므로 유지)
  - `server/api/handler/context_history.go`의 에러 필터에서 `ErrAlreadyAttempted` 체크 제거 — 이 경로로 더 이상 발생 안 함
- **Shared types 변경** (`packages/shared/src/types.ts`):
  - 신규 interface: `PastQuizAttempt { selected_index: number; is_correct: boolean; coins_earned: number; attempted_at: string }`
  - `QuizForUser` interface에 optional 필드: `past_attempt?: PastQuizAttempt | null`
  - 기존 필드는 변경 없음 (backward compat)
- **프론트엔드 hydration 로직** (`web/src/components/quiz-card.tsx`):
  - Mount useEffect에서 `quiz?.past_attempt`가 존재하면:
    1. `setSelectedIndex(quiz.past_attempt.selected_index)`
    2. `setSubmitResult({ correct: past_attempt.is_correct, coins_earned: past_attempt.coins_earned, total_coins: 0 })` — total_coins는 UI에서 사용 안 함
    3. `setStage(past_attempt.is_correct ? "result_correct" : "result_wrong")`
    4. `setIsHydrated(true)` — 신규 state, 애니메이션/타이머 skip 플래그
    5. `clearPendingAnswer()` — 혹시 있을 pending 제거 (3.4.1과의 상호작용)
    6. `return` — 이후 pending 복원 로직 실행 금지
  - `isHydrated === true`일 때 렌더 분기:
    - shake, pulse-soft, wobble, count-up 등 모든 애니메이션 클래스 **생략** (static)
    - 상단 메시지: "이미 완료했어요" (정답/오답 구분 배지만 작게 표시)
    - `+N 보너스!` 같은 신규 획득 표시 **숨김** — 과거 이벤트라 "지금 받았다"는 인상 부적절
    - 선택 옵션 카드: 정답이면 초록 flat, 오답이면 빨강 flat (border + bg only, 움직임 없음)
    - 다른 옵션은 회색 flat
    - 옵션 클릭 핸들러 비활성화 (`disabled` + `pointer-events-none`)
- **중요 불변**: hydration 경로에서도 **정답 인덱스는 절대 반환/표시 안 함**. `PastAttempt`에 `selected_index`와 `is_correct`만 들어가고, `correct_index`는 포함하지 않음. 기존 `SubmitResult` 설계 원칙과 동일.
- **Stale pending 처리 (3.4.1과의 상호작용)**: `quiz.past_attempt`가 있으면 localStorage의 pending selection은 **무시**하고 `clearPendingAnswer()`. PO 요청으로 hydration이 포함되면서 3.4.1의 "quiz null 시 pending 폐기" 케이스는 여전히 유효 (예: quiz 자체가 삭제된 경우).
- **Acceptance**: 퀴즈 푼 유저가 새로고침하거나 며칠 후 재방문해도 과거 답변이 정적 카드로 즉시 복원되어야 함. 애니메이션 없이 정답/오답 표시.

#### 3.4.9 모바일 탭 타겟
- 현재 옵션 버튼: `py-3 text-sm` → ~44px 높이
- 변경: `py-3.5` (약 48px) + `min-h-[48px]` 명시적 보장
- 옵션 간 간격: `gap-2` 유지 (모바일 오조작 방지)

#### 3.4.10 Navigation 차단 effect와의 상호작용
- 현재 `topic.tsx:292`: `const isEarning = showCountdown !== null || coinTag?.kind === "loading"`
- Phase 2에서 Stage B/C/D/E는 carousel이 아니라 매우 짧은 (~3초) 연출이므로 navigation 차단이 **필요 없음**
- 하지만 Stage C (submit 왕복) 동안은 **차단이 필요** — 유저가 뒤로가기 누르면 결과 못 봄
- **개선**: `isEarning` 정의에 `quizStage === "submitting" || quizStage === "evaluating"` 추가
- 그러나 이건 `QuizCard` 내부 상태이므로 topic.tsx로 lift해야 함 → 복잡도 증가
- **대안**: QuizCard가 `onStageChange(stage)` 콜백 prop으로 현재 단계를 topic.tsx로 emit → topic.tsx가 별도 state `quizBusy`로 보관 → `isEarning = showCountdown !== null || coinTag?.kind === "loading" || quizBusy`
- Stage E(결과) 진입 후에는 `quizBusy = false`로 해제 → 유저가 결과 보고 자유롭게 이동 가능

### 3.5 파일별 변경 상세

#### 3.5.1 `server/domain/quiz/service.go` (Phase 2에서 추가 변경)
Phase 1의 earn-gate 제거에 더해 Phase 2에서:
- `GetQuizForUser` 본문 변경:
  - `HasAttempted` 호출 제거
  - 대신 `s.repo.GetUserAttempt(ctx, userID, quiz.ID)` 호출
  - 결과가 nil이 아니면 `QuizForUser.PastAttempt` 필드에 채워서 반환
  - `ErrAlreadyAttempted` 반환 분기 삭제 (더 이상 에러 아님, 정상 응답의 일부)
- Doc 주석 재차 업데이트: "returns quiz with past_attempt filled in if user has already attempted; ErrAlreadyAttempted is no longer returned from this function (SubmitAnswer still uses it)"
- `ErrAlreadyAttempted` 상수(17행)는 유지 — SubmitAnswer, quiz_handler.go:56에서 사용

#### 3.5.2 `server/api/handler/context_history.go` (Phase 2에서 추가 변경)
Phase 1의 에러 필터 단순화에 더해:
- `ErrAlreadyAttempted` 조건 제거 (이제 이 에러는 GetQuizForUser에서 발생하지 않음)
- 최종 형태:
  ```
  if userID != "" && h.quizSvc != nil {
      quizForUser, err := h.quizSvc.GetQuizForUser(ctx, userID, id)
      if err != nil {
          slog.Warn("get quiz for user error", ...)
      }
      resp.Quiz = quizForUser
  }
  ```
- `errors` 패키지 import가 다른 곳에서 안 쓰이면 제거. 쓰이면 유지.

#### 3.5.3 `server/integration/quiz_test.go` (Phase 2에서 추가 변경)
Phase 1의 `TestQuiz_ReadPathOpenSubmitGated` 재작성에 더해:
- **TestQuiz_PastAttemptHydration_Correct** (신규):
  - Setup: user + context_item + quiz + coin_logs + quiz_results(is_correct=true, answered_index=2, coins_earned=7)
  - `GetQuizForUser` 호출
  - 검증: 반환된 `QuizForUser.PastAttempt != nil`, `SelectedIndex=2`, `IsCorrect=true`, `CoinsEarned=7`, `AttemptedAt`이 제로 시각 아님
- **TestQuiz_PastAttemptHydration_Wrong** (신규):
  - Setup: quiz_results(is_correct=false, answered_index=0, coins_earned=0)
  - 검증: `PastAttempt.IsCorrect=false`, `CoinsEarned=0`
- **TestQuiz_PastAttemptHydration_NoAttempt** (신규):
  - Setup: quiz 있지만 quiz_results 없음
  - 검증: `PastAttempt == nil`, 퀴즈 question/options는 정상 반환
- 기존 `TestQuiz_DuplicateSubmissionBlocked`는 그대로 유지 (SubmitAnswer의 UNIQUE constraint 중복 체크 검증)
- 기존 `TestQuiz_CorrectAnswerAwardsCoins`, `TestQuiz_WrongAnswerNoCoins`, `TestQuiz_BonusExemptFromDailyLimit`, `TestQuiz_BatchQuizStatus`는 변경 없음

#### 3.5.3a `server/domain/quiz/model.go` (신규 struct 추가 — Phase 2)
- 신규 struct:
  ```go
  type PastAttempt struct {
      SelectedIndex int       `json:"selected_index"`
      IsCorrect     bool      `json:"is_correct"`
      CoinsEarned   int       `json:"coins_earned"`
      AttemptedAt   time.Time `json:"attempted_at"`
  }
  ```
- `QuizForUser`에 필드 추가:
  ```go
  type QuizForUser struct {
      ID            uuid.UUID    `json:"id"`
      ContextItemID uuid.UUID    `json:"context_item_id"`
      Question      string       `json:"question"`
      Options       []string     `json:"options"`
      PastAttempt   *PastAttempt `json:"past_attempt,omitempty"`  // ← 신규
  }
  ```
- `SubmitResult` (27-33행) **변경 없음** — 정답 인덱스 미반환 원칙 유지

#### 3.5.3b `server/domain/quiz/repository.go` (인터페이스 메서드 추가 — Phase 2)
- 신규 메서드 signature:
  ```go
  // GetUserAttempt returns the user's past attempt for the given quiz, or nil if none.
  GetUserAttempt(ctx context.Context, userID string, quizID uuid.UUID) (*PastAttempt, error)
  ```
- 기존 `HasAttempted` 메서드: **유지** 또는 **제거** 중 선택
  - **권장: 유지** — `SubmitAnswer` 경로에서 명시적 pre-check로 쓰일 여지 있음 (현재는 UNIQUE constraint에 의존). 유지 시 구현만 남기고 `service.go`에서 호출 제거 → 미사용 warning은 없음 (인터페이스 메서드라 파라미터 미사용 경고 없음)
  - **제거 옵션**: 현재 `HasAttempted`의 유일한 호출자가 `GetQuizForUser`였고 교체됨. 제거해도 빌드 가능. 단 mock 구현체(`*_test.go`의 mock repos)도 함께 수정 필요.
  - **최종 결정**: 제거. 사용처가 없는 인터페이스 메서드는 삭제하는 게 idiomatic Go. Mock 수정 포함해서 일관되게 처리.

#### 3.5.3c `server/storage/quiz_repository.go` (구현 추가 — Phase 2)
- 파일명 확인 필요 (`server/storage/` 디렉토리 탐색). 일반적으로 `quiz_repo.go` 또는 `quiz_repository.go`. Glob으로 확인 후 구현 추가.
- `GetUserAttempt` 구현:
  ```go
  func (r *QuizRepository) GetUserAttempt(ctx context.Context, userID string, quizID uuid.UUID) (*quiz.PastAttempt, error) {
      var pa quiz.PastAttempt
      err := r.pool.QueryRow(ctx, `
          SELECT answered_index, is_correct, coins_earned, created_at
          FROM quiz_results
          WHERE user_id = $1 AND quiz_id = $2
          LIMIT 1
      `, userID, quizID).Scan(&pa.SelectedIndex, &pa.IsCorrect, &pa.CoinsEarned, &pa.AttemptedAt)
      if errors.Is(err, pgx.ErrNoRows) {
          return nil, nil
      }
      if err != nil {
          return nil, fmt.Errorf("get user attempt: %w", err)
      }
      return &pa, nil
  }
  ```
- `HasAttempted` 구현체 제거 (인터페이스에서 제거했으므로)
- 기존 `SaveResultAndAwardCoins`, `SaveQuiz`, `SaveQuizBatch`, `GetByContextItemID`, `GetQuizExistenceMap`, `GetQuizCompletionMap` 등은 변경 없음

#### 3.5.3d `packages/shared/src/types.ts` (QuizForUser 확장 — Phase 2)
- 신규 interface:
  ```ts
  export interface PastQuizAttempt {
    selected_index: number;
    is_correct: boolean;
    coins_earned: number;
    attempted_at: string;
  }
  ```
- `QuizForUser` interface에 optional 필드 추가:
  ```ts
  export interface QuizForUser {
    id: string;
    question: string;
    options: string[];
    past_attempt?: PastQuizAttempt | null;  // ← 신규
  }
  ```
- `QuizSubmitResult` 변경 없음
- `TopicDetail.quiz` 타입은 그대로 `QuizForUser | null` 유지 (QuizForUser 자체가 확장되므로 자동으로 past_attempt 포함)
- ⚠️ 기존 web 프론트의 `QuizForUser`는 `context_item_id`를 포함하지 않음 (backend는 포함) — 이 불일치는 기존부터 있었고 Phase 2 범위 밖이므로 그대로 둠. 필요 시 별도 cleanup PR.

#### 3.5.3e 기존 test mocks 수정 — Phase 2
- `server/domain/quiz/`에서 Repository mock이 있다면 `GetUserAttempt` 추가, `HasAttempted` 제거
- `server/api/handler/`의 context_history_test.go 등에서 mock 사용하는 곳 확인
- 검색: `Grep(pattern="HasAttempted|mockQuizRepo|QuizRepoMock")` 로 mock 위치 식별 필요

#### 3.5.4 `web/src/lib/quiz-pending.ts` (**신규 파일**)

```
역할: 비로그인 유저의 pending 퀴즈 답을 localStorage에 임시 저장/복원

상수:
- STORAGE_KEY = "wl_quiz_pending_answer_v1"
- TTL_MS = 10 * 60 * 1000  (10분)

타입:
- PendingAnswer = { topic_id: string, selected_index: number, saved_at: number }

함수:
- savePendingAnswer(topicId: string, selectedIndex: number): void
  → JSON.stringify 후 localStorage.setItem
  → try/catch로 storage quota 예외 swallow (모바일 Safari private mode 대응)

- loadPendingAnswer(topicId: string): number | null
  → localStorage.getItem → JSON.parse 시도
  → parse 실패 / 형식 불일치 → clear + null
  → topic_id 불일치 → null (다른 토픽 pending은 건드리지 않음)
  → Date.now() - saved_at > TTL_MS → clear + null
  → 유효하면 selected_index 반환

- clearPendingAnswer(): void
  → localStorage.removeItem
  → try/catch로 swallow
```

#### 3.5.5 `web/src/components/quiz-card.tsx` (**거의 전면 재작성**)

```
Props 인터페이스 변경:
  interface QuizCardProps {
    quiz: QuizForUser | null
    hasQuiz: boolean
    isLoggedIn: boolean            ← 신규
    earnCommitted: boolean         ← 신규 (기존 earnDone 대체)
    contextItemId: string
    topicId: string                ← 신규 (pending 저장 키)
    onCoinsEarned?: (newTotal: number) => void
    onRequestLogin?: (selectedIndex: number) => void  ← 신규
    onStageChange?: (stage: QuizStage) => void        ← 신규 (topic.tsx와 nav-block 동기화)
  }

상태:
  type QuizStage =
    | "idle"
    | "selected_waiting_earn"
    | "earn_confirmed"
    | "submitting"
    | "evaluating"
    | "result_correct"
    | "result_wrong"
    | "submit_failed"

  const [stage, setStage] = useState<QuizStage>("idle")
  const [selectedIndex, setSelectedIndex] = useState<number | null>(null)
  const [submitResult, setSubmitResult] = useState<QuizSubmitResult | null>(null)
  const [stageEnteredAt, setStageEnteredAt] = useState(0)
  const [isHydrated, setIsHydrated] = useState(false)  // ← 신규: past attempt 복원 여부

Refs (타이머/가드):
  const stageTimersRef = useRef<number[]>([])
  const submitCalledRef = useRef(false)
  const retryCountRef = useRef(0)  // ← 신규: submit 재시도 카운터 (최대 5회)
  const mountedRef = useRef(true)

Mount 시 (우선순위 순):
  1. quiz?.past_attempt 체크 (hydration — PO 요청)
     - 존재하면:
       - setSelectedIndex(quiz.past_attempt.selected_index)
       - setSubmitResult({
           correct: quiz.past_attempt.is_correct,
           coins_earned: quiz.past_attempt.coins_earned,
           total_coins: 0,  // UI에서 사용 안 함
         })
       - setStage(quiz.past_attempt.is_correct ? "result_correct" : "result_wrong")
       - setIsHydrated(true)
       - clearPendingAnswer()  // stale pending 청소
       - return  // 이후 로직 실행 금지
  2. pending answer 복원 (비로그인 → 로그인 복귀)
     - loadPendingAnswer(topicId)가 있고 quiz가 null이 아니면
     - selectedIndex 복원 + stage="selected_waiting_earn"
  3. 둘 다 아니면 stage="idle" 유지

Mount cleanup:
  - mountedRef.current = false
  - stageTimersRef.current.forEach(clearTimeout); stageTimersRef.current = []

onStageChange 콜백:
  - useEffect(() => { onStageChange?.(stage) }, [stage, onStageChange])

earnCommitted prop 변경 감지:
  - useEffect(() => {
      if (stage === "selected_waiting_earn" && earnCommitted && selectedIndex !== null) {
        const elapsed = Date.now() - stageEnteredAt
        const wait = Math.max(0, 600 - elapsed)
        const t = setTimeout(() => {
          if (!mountedRef.current) return
          transitionTo("earn_confirmed")
        }, wait)
        stageTimersRef.current.push(t)
      }
    }, [earnCommitted, stage, selectedIndex, stageEnteredAt])

transitionTo(next) 헬퍼:
  - setStage(next); setStageEnteredAt(Date.now())
  - next에 따라 후속 타이머 예약:
    - "earn_confirmed": 1000ms 후 transitionTo("submitting")
    - "submitting": submit API 호출 + min 800ms (Promise.all with delay)
    - "evaluating": 800ms 후 transitionTo(result_correct/wrong)
    - "result_wrong": 1500ms 후 아무것도 안 함 (최종 상태)
    - "result_correct": persist

handleSelect(index):
  - if (!isLoggedIn) { savePendingAnswer(topicId, index); onRequestLogin?.(index); return; }
  - if (stage === "idle") {
      setSelectedIndex(index)
      transitionTo("selected_waiting_earn")
      return
    }
  - if (stage === "selected_waiting_earn") {
      // 재선택 허용
      setSelectedIndex(index)
      return
    }
  - 그 외 stage에서는 무시

doSubmit() (Stage C 진입 시 호출):
  - if (submitCalledRef.current) return
  - submitCalledRef.current = true
  - const startedAt = Date.now()
  - try {
      const [result] = await Promise.all([
        submitQuizAnswer(contextItemId, selectedIndex!),
        delay(800),  // min display
      ])
      if (!mountedRef.current) return
      setSubmitResult(result)
      clearPendingAnswer()
      onCoinsEarned?.(result.total_coins)
      transitionTo("evaluating")
    } catch (e) {
      if (!mountedRef.current) return
      transitionTo("submit_failed")
    }

handleRetry() (submit_failed 상태의 재시도 버튼):
  - submitCalledRef.current = false
  - transitionTo("submitting")  // useEffect가 doSubmit 재실행

렌더 분기 (isHydrated 여부를 최상단에서 분기):
  - stage="idle": 기존 active 카드 UI (옵션 클릭 가능)
    - isLoggedIn === false 시 상단 배지 텍스트 변경: "로그인하고 도전해보세요"
    - isLoggedIn === true 시: "정답 맞히면 보너스 포인트!"

  - stage="selected_waiting_earn":
    - 선택한 옵션만 주황 테두리 강조
    - 나머지 옵션은 흐리게 (선택 가능)
    - 상단 메시지 영역: 작은 pulse 점 + "포인트 획득 대기중..."

  - stage="earn_confirmed":
    - 선택 옵션 + 코인 아이콘 + count-up "+N"
    - 상단 메시지: "포인트 획득 완료!"

  - stage="submitting":
    - 선택 옵션 카드가 pulse 애니메이션
    - 상단 메시지: "퀴즈 정답 제출 중..."

  - stage="evaluating":
    - 선택 옵션 카드가 wobble
    - 상단 메시지: "정답 검사 중..."

  - stage="result_correct" (isHydrated=false, 신규 성공):
    - 카드 전체 초록 accent
    - 선택 옵션에 체크 아이콘
    - "+{coins} 보너스!" 메시지 + count-up 애니메이션
    - 다른 옵션은 flat 회색

  - stage="result_correct" (isHydrated=true, 복원):
    - 카드 전체 초록 accent (flat, 애니메이션 없음)
    - 선택 옵션에 체크 아이콘 (static)
    - 상단 메시지: "이미 완료했어요" + 작은 초록 배지 "정답"
    - "+N 보너스!" 메시지 **숨김** (과거 이벤트)
    - 옵션 클릭 핸들러 비활성화 (disabled + pointer-events-none)
    - 다른 옵션은 flat 회색

  - stage="result_wrong" (isHydrated=false, 신규 오답):
    - 카드 전체 빨간 accent
    - 선택 옵션에 X 아이콘 + shake 애니메이션 1회
    - "아쉽지만 틀렸어요" 메시지 (정답 reveal 없음)
    - 다른 옵션은 flat 회색

  - stage="result_wrong" (isHydrated=true, 복원):
    - 카드 전체 빨간 accent (flat, shake 없음)
    - 선택 옵션에 X 아이콘 (static)
    - 상단 메시지: "이미 완료했어요" + 작은 빨간 배지 "오답"
    - 옵션 클릭 핸들러 비활성화
    - 다른 옵션은 flat 회색

  - stage="submit_failed":
    - 카드 전체 회색 + 빨간 border
    - "제출에 실패했어요" + [다시 시도] 버튼
    - retryCountRef.current >= 5 시: 재시도 버튼 disabled + "잠시 후 다시 방문해주세요 🙏" 문구

접근성:
  - 상단 메시지 컨테이너에 aria-live="polite" 적용해 스테이지 전환 시 screen reader가 announce
  - 각 옵션 버튼에 aria-pressed 또는 aria-checked로 선택 상태 전달
  - disabled 상태에서도 aria-disabled="true" 명시
```

#### 3.5.6 `web/src/pages/topic.tsx` (수정)

```
변경 1: 신규 state
  const [earnCommitted, setEarnCommitted] = useState(false)
  const [quizBusy, setQuizBusy] = useState(false)  // Stage C/D 동안 true
  const [pendingLoginFromQuiz, setPendingLoginFromQuiz] = useState(false)

변경 2: handleCountdownComplete success 분기
  // 기존: setCoinTag({kind:"success"})만 호출
  // 추가: setEarnCommitted(true)

변경 3: QuizCard 렌더 가드 (637-647행)
  - 기존: `{user && topic.has_quiz && coinTag?.kind !== "daily_limit" && coinTag?.kind !== "expired" && (...)}`
  - 변경: `{topic.has_quiz && coinTag?.kind !== "daily_limit" && coinTag?.kind !== "expired" && (...)}`
  - (user && 제거)

변경 4: QuizCard props
  <QuizCard
    quiz={topic.quiz}
    hasQuiz={topic.has_quiz}
    isLoggedIn={!!user}
    earnCommitted={earnCommitted}
    contextItemId={topic.id}
    topicId={topic.id}
    onCoinsEarned={() => setLevelRefreshKey((k) => k + 1)}
    onRequestLogin={(idx) => {
      setPendingLoginFromQuiz(true)
      setLoginOpen(true)  // 기존 loginOpen 재사용 또는 전용 modal
    }}
    onStageChange={(stage) => {
      setQuizBusy(stage === "submitting" || stage === "evaluating")
    }}
  />

변경 5: isEarning 정의 확장
  const isEarning = showCountdown !== null || coinTag?.kind === "loading" || quizBusy

변경 6: 로그인 모달 카피
  - pendingLoginFromQuiz === true 시 로그인 모달 제목/설명을 퀴즈 제출 유도 카피로 변경
  - 또는 LoginPromptModal에 variant prop 추가해서 별도 모달로 분리

변경 7: 기존 LoginPromptModal의 onClose에서 pendingLoginFromQuiz 리셋
```

#### 3.5.7 `web/src/components/login-prompt-modal.tsx` (수정)

```
Props 확장:
  interface LoginPromptModalProps {
    redirectPath: string
    onClose: () => void
    variant?: "default" | "quiz_submit"  ← 신규
  }

variant="quiz_submit" 시:
  - 헤드라인: "로그인하고 정답을 제출해\n포인트를 받아보세요"
  - 기존 dismiss 버튼 "하루 동안 보지 않기"는 숨김 (flow 중단감 방지)
  - 기존 카피 "로그인하고 기사를 읽으면..."는 default에서만 사용
```

**설계 노트**: 별도 컴포넌트 `QuizLoginPromptModal` 신설 대신 variant prop으로 통합하는 이유 — 레이아웃이 거의 동일하고, 중복 파일 증가 방지. 만약 디자인이 크게 달라지면 이후 분리.

#### 3.5.8 `web/src/index.css` (신규 keyframes 추가)

Tailwind 4의 `@import "tailwindcss"` 뒤에 `@layer utilities` 블록 또는 plain CSS로 추가:

```
@keyframes wl-shake {
  0%, 100% { transform: translateX(0); }
  20% { transform: translateX(-6px); }
  40% { transform: translateX(6px); }
  60% { transform: translateX(-3px); }
  80% { transform: translateX(3px); }
}

@keyframes wl-pulse-soft {
  0%, 100% { transform: scale(1); box-shadow: 0 0 0 0 rgba(245, 166, 35, 0.4); }
  50% { transform: scale(1.015); box-shadow: 0 0 0 6px rgba(245, 166, 35, 0); }
}

@keyframes wl-wobble {
  0%, 100% { transform: rotate(0deg); }
  25% { transform: rotate(-1.5deg); }
  75% { transform: rotate(1.5deg); }
}

@keyframes wl-count-up {
  /* CSS-only count-up은 불가. 컴포넌트 내 requestAnimationFrame으로 처리 */
  /* 대신 "펑" 효과용 scale 애니메이션 */
  0% { transform: scale(0.6); opacity: 0; }
  60% { transform: scale(1.15); opacity: 1; }
  100% { transform: scale(1); opacity: 1; }
}

.wl-anim-shake { animation: wl-shake 0.42s ease-in-out 1; }
.wl-anim-pulse-soft { animation: wl-pulse-soft 1.2s ease-in-out infinite; }
.wl-anim-wobble { animation: wl-wobble 0.8s ease-in-out infinite; }
.wl-anim-coin-pop { animation: wl-count-up 0.6s ease-out 1; }

@media (prefers-reduced-motion: reduce) {
  .wl-anim-shake,
  .wl-anim-pulse-soft,
  .wl-anim-wobble,
  .wl-anim-coin-pop {
    animation: none !important;
  }
}
```

숫자 count-up은 컴포넌트 내 `useEffect` + `requestAnimationFrame`으로 처리 (CSS로는 내용 변경 불가). 별도 helper 함수 또는 인라인 구현.

### 3.6 Phase 2 Acceptance Criteria

**로그인 유저**:
- [ ] 카운트다운 중에 옵션을 클릭하면 "포인트 획득 대기중..." 표시
- [ ] 카운트다운 중에 다른 옵션을 재클릭하면 선택만 교체됨 (상태 유지)
- [ ] earn 완료 후 자동으로 Stage B → C → D → E 진행
- [ ] 정답 시 초록 accent + "+N 보너스!" 표시
- [ ] 오답 시 빨간 accent + 선택 카드 shake + "아쉽지만 틀렸어요" 표시 (정답 인덱스 노출 X)
- [ ] Submit 네트워크 실패 시 SUBMIT_FAILED 상태 + 재시도 버튼
- [ ] 재시도 버튼 클릭 시 다시 정상 SUBMITTING 진행

**비로그인 유저**:
- [ ] 퀴즈 카드가 IDLE 상태로 보임 (배지: "로그인하고 도전해보세요")
- [ ] 옵션 클릭 시 로그인 모달 표시 (카피: 퀴즈 제출 유도)
- [ ] 선택한 인덱스가 localStorage에 저장됨
- [ ] 카카오 로그인 후 `/topic/:id`로 복귀 시 선택이 복원되고 자동으로 SELECTED_WAITING_EARN
- [ ] 10초 카운트다운 → earn 완료 → 자동 Stage B~E 진행
- [ ] localStorage TTL (10분) 초과 후 복귀 시 pending 무시

**퀴즈 완료 유저 (Hydration)**:
- [ ] 이미 퀴즈를 푼 유저가 `/topic/:id`에 재방문하면 과거 선택이 비활성화 상태로 복원됨
- [ ] 정답자: 초록 accent + 체크 아이콘 + "이미 완료했어요" 메시지 (애니메이션 없음)
- [ ] 오답자: 빨간 accent + X 아이콘 + "이미 완료했어요" 메시지 (shake 없음)
- [ ] 옵션 클릭 핸들러 비활성화 (재제출 불가)
- [ ] "+N 보너스!" 같은 신규 획득 문구 **표시되지 않음** (과거 이벤트라 부적절)
- [ ] 정답 인덱스는 여전히 노출 안 됨 (API가 반환하지 않음)
- [ ] pending answer가 localStorage에 남아있어도 hydration이 우선하여 무시됨

**모바일**:
- [ ] iOS Safari / Android Chrome / Samsung Internet에서 카드 레이아웃 정상
- [ ] 옵션 버튼 탭 타겟 48px+
- [ ] 탭 전환 → 복귀 시 상태 머신 stuck 없음
- [ ] 애니메이션이 30fps 이상으로 부드럽게 재생 (저사양 기기 허용 범위)

**Accessibility**:
- [ ] `prefers-reduced-motion: reduce` 설정 시 shake/pulse/wobble 제거, 텍스트 전환은 정상 작동
- [ ] 각 stage의 상단 메시지가 screen reader에 적절히 announce (aria-live="polite")

**에러 없음**:
- [ ] 페이지 navigate-away 시 setState-after-unmount 경고 없음
- [ ] 브라우저 개발자 도구 콘솔에 에러 없음
- [ ] stale pending (다른 topic, TTL 초과, quiz=null)이 유령 카드를 만들지 않음

### 3.7 Phase 2 Rollback
- **단일 revert 커밋** 가능한 구조 유지:
  - **프론트엔드**:
    - `quiz-card.tsx` 전체 교체 (원본 백업은 git history)
    - `topic.tsx` 변경사항은 명확한 hunk로 분리
    - `quiz-pending.ts` 삭제
    - `index.css` 신규 keyframes 제거
    - `login-prompt-modal.tsx` variant prop 제거
    - `packages/shared/src/types.ts`의 `PastQuizAttempt` + `QuizForUser.past_attempt` 제거
  - **백엔드** (Phase 2에서 추가):
    - `server/domain/quiz/model.go`의 `PastAttempt` struct + `QuizForUser.PastAttempt` 필드 제거
    - `server/domain/quiz/repository.go`의 `GetUserAttempt` 메서드 제거 + `HasAttempted` 복원
    - `server/storage/quiz_repository.go`의 `GetUserAttempt` 구현 제거 + `HasAttempted` 구현 복원
    - `server/domain/quiz/service.go`의 `GetQuizForUser`에서 `HasAttempted` 다시 호출 + `ErrAlreadyAttempted` 반환 분기 복원
    - `server/api/handler/context_history.go`의 에러 필터에 `ErrAlreadyAttempted` 다시 추가
    - `server/integration/quiz_test.go`의 신규 테스트 3개 제거
- revert 후 Phase 1만 남아 버그는 수정된 상태. 완료된 퀴즈는 다시 숨김 (그 이전 Phase 2 PO 요구사항 미반영 상태)
- **마이그레이션 고려**: Phase 2 배포 → 롤백 시점 사이에 퀴즈를 푼 유저의 quiz_results 데이터는 그대로 남음 (DB 스키마 변경 없으므로 무손실)
- **localStorage 청소**: rollback 시 남은 `wl_quiz_pending_answer_v1` 키는 신경 쓰지 않음 — TTL 10분이라 자연 만료
- **배포 순서**: 백엔드 먼저 배포 후 프론트 배포 (프론트가 새 응답 형태를 기대하므로). 롤백 시 역순 — 프론트 먼저 revert 후 백엔드 revert

### 3.8 Observability (GTM Events)

Phase 2 동작 검증을 위한 신규 GTM 이벤트:

| 이벤트 | 언제 | 파라미터 |
|--------|------|---------|
| `quiz_preselect` | 유저가 옵션 클릭 (IDLE → SELECTED_WAITING_EARN) | `topic_id`, `selected_index`, `is_logged_in` |
| `quiz_preselect_change` | Stage A 동안 다른 옵션 재선택 | `topic_id`, `new_index` |
| `quiz_stage_enter` | 각 Stage 진입 | `topic_id`, `stage` |
| `quiz_submit_success` | SUBMITTING → EVALUATING | `topic_id`, `correct`, `coins_earned`, `duration_ms` |
| `quiz_submit_fail` | SUBMITTING → SUBMIT_FAILED | `topic_id`, `error_type` |
| `quiz_retry_click` | SUBMIT_FAILED 재시도 | `topic_id`, `attempt_count` |
| `quiz_login_gate_open` | 비로그인 유저가 옵션 클릭 → 로그인 모달 표시 | `topic_id`, `selected_index` |
| `quiz_pending_restored` | OAuth 복귀 후 pending 복원 성공 | `topic_id`, `selected_index`, `age_seconds` |
| `quiz_pending_expired` | pending TTL 만료로 폐기 | `topic_id`, `age_seconds` |

이 이벤트들은 Phase 2 배포 후 "유저가 실제로 이 flow를 타는가"를 모니터링하는 데 사용. 기존 `gtmPush` helper (topic.tsx:37) 재사용.

---

## 4. Pre-Mortem — 실패 시나리오 3가지

### Scenario 1: "OAuth 복귀 후 pending이 복원되지만 earn countdown이 돌지 않음"
- **원인 가설**: topic.tsx의 `useEffect([id, user])`가 실행되는 시점과 QuizCard mount의 `loadPendingAnswer` 시점 사이의 race. 또는 `initEarn`이 실패해서 `showCountdown`이 세팅되지 않음 (예: adblock 감지 실패 fallback, 서버 500).
- **탐지 방법**:
  - E2E 수동 테스트: 비로그인 → 퀴즈 클릭 → 로그인 → 복귀 → Stage A에서 stuck 되는지
  - GTM 이벤트 비율: `quiz_pending_restored` vs `quiz_stage_enter(earn_confirmed)` 비율이 크게 차이나면 flow drop-off
- **완화**:
  - QuizCard Stage A 진입 후 15초 이상 `earnCommitted`가 false로 유지되면 `SUBMIT_FAILED` 유사 상태로 전환 (또는 "잠시만 기다려주세요" 메시지)
  - 15초 timeout fallback으로 최소한 유령 카드는 방지
  - GTM `quiz_stage_stuck` 이벤트 추가해서 모니터링

### Scenario 2: "iOS Safari에서 keyframes 애니메이션이 버벅이거나 잘못 렌더링됨"
- **원인 가설**: Tailwind 4의 `@layer utilities` 또는 plain CSS 주입 방식과 iOS Safari의 CSS 엔진 호환 문제. 또는 `transform`에 `translateX` + `scale` 동시 적용 시 jank.
- **탐지 방법**:
  - 실 디바이스 테스트 (iPhone Safari, Samsung 중저가 기기)
  - Chrome DevTools Performance 탭으로 애니메이션 FPS 측정
- **완화**:
  - `will-change: transform` 명시 (shake/pulse 대상 요소)
  - `transform` 속성만 사용 (width/height/margin 변경 금지)
  - `animation-fill-mode: forwards` 명시
  - Worst case: 문제되는 애니메이션만 `@media (max-width: 767px) { ... animation: none; }`로 끄고 텍스트 전환만 남김

### Scenario 3: "SUBMITTING Stage에서 submit API가 영원히 응답 안 함 (무한 로딩)"
- **원인 가설**: 서버 장애, 네트워크 drop, Turnstile 토큰 만료 등
- **탐지 방법**:
  - 서버 로그 에러 비율
  - GTM `quiz_submit_fail` 이벤트 비율
  - 서버 응답 시간 P99 메트릭 (있다면)
- **완화**:
  - `submitQuizAnswer` fetch에 AbortController + timeout (예: 20초)
  - timeout 시 `SUBMIT_FAILED` 상태로 전이
  - shared/api.ts의 fetch wrapper에 timeout 공통 처리 없는지 확인 필요 (없다면 이 flow에서만 한정적으로 추가)

---

## 5. 실행 순서 (PR 분리)

### PR #1: Phase 1 (버그 수정)
- **포함**:
  - `server/domain/quiz/service.go`
  - `server/api/handler/context_history.go`
  - `server/integration/quiz_test.go`
- **배포**: 백엔드만 단독 배포 가능 (프론트 변경 없음)
- **검증**: 프로덕션에서 earn 완료 후 카드 활성화되는지 수동 확인
- **롤백**: `git revert` 단일 커밋

### PR #2: Phase 2 (UX 재설계 — 백엔드 + 프론트)
- **포함 (백엔드)**:
  - `server/domain/quiz/model.go` (PastAttempt struct + QuizForUser 필드 추가)
  - `server/domain/quiz/repository.go` (GetUserAttempt 메서드 추가, HasAttempted 제거)
  - `server/storage/quiz_repository.go` (구현 파일 — 정확한 경로 구현 시 확인)
  - `server/domain/quiz/service.go` (GetQuizForUser 로직 수정)
  - `server/api/handler/context_history.go` (에러 필터 정리)
  - `server/integration/quiz_test.go` (hydration 테스트 3개 추가, quiz mock 업데이트)
  - (필요 시) 기타 quiz mock을 사용하는 테스트 파일들
- **포함 (프론트)**:
  - `packages/shared/src/types.ts` (PastQuizAttempt + QuizForUser.past_attempt 추가)
  - `web/src/lib/quiz-pending.ts` (신규)
  - `web/src/components/quiz-card.tsx` (전면 재작성, hydration 로직 포함)
  - `web/src/pages/topic.tsx` (수정)
  - `web/src/components/login-prompt-modal.tsx` (variant prop)
  - `web/src/index.css` (keyframes)
- **배포 순서**: **백엔드 먼저 배포** (새 응답 형태 live) → **그 다음 프론트 배포** (새 응답을 소비). 역순으로 하면 프론트가 past_attempt 필드 없는 응답을 받아 hydration이 작동 안 함 (기존 동작으로 fallback되어 치명적이진 않지만 배포 중 과도기 동안 hydration 실패)
- **하위호환성 체크**: 백엔드가 past_attempt를 추가해도 기존 프론트는 해당 필드 무시 → 백엔드만 먼저 배포해도 안전
- **검증**:
  - 로컬에서 Docker Compose로 백엔드 + 프론트 동시 구동
  - 데스크톱 + 모바일 디바이스 에뮬레이션으로 flow 완주
  - 비로그인 → 퀴즈 클릭 → 로그인 → 복귀 → 자동 submit flow
  - 오답/정답 각각 애니메이션 확인
  - 퀴즈 푼 후 새로고침 → hydration UI 정상 표시
  - `prefers-reduced-motion` 설정으로 동일 flow 재확인
  - 저사양 모바일에서 FPS 확인
  - Backend integration tests 전부 통과 (`go test ./server/integration/...` with testcontainers)
- **롤백**: `git revert` 단일 커밋. 프론트 먼저 revert → 백엔드 revert 순

### PR #2 이후 검토할 Phase 3 후보 (스코프 밖)
- 완료한 퀴즈의 past result hydration (백엔드 신규 엔드포인트 필요)
- 오답 시 정답 reveal (기획상 퀴즈 재풀이 허용하므로 보류, 요구사항 명확해질 때 재검토)
- 퀴즈 confetti 애니메이션 (정답 celebration 강화)
- 서버사이드 submit timeout 및 retry 전략 개선

---

## 6. ADR (Architecture Decision Record)

### Decision
`GetQuizForUser`의 earn-gate를 제거하고, 치팅 방지는 `SubmitAnswer` 경로의 earn-gate에만 의존한다. 프론트는 비로그인 유저에게도 퀴즈를 노출하고, 상태 머신 기반 paced reveal UX를 적용한다.

### Drivers
1. **버그 해결이 우선** — 현재 "earn 완료 후 카드 소멸" 버그는 유저 경험상 심각. 가장 직접적인 수정은 READ 경로 gate 제거.
2. **비로그인 유저 참여 유도** — AdSense 심사 진행 중이므로 체류시간/상호작용 지표 개선이 전략적 가치 높음.
3. **치팅 방어는 SubmitAnswer에 이미 존재** — 중복 방어는 가치 없고 버그 원인.

### Alternatives Considered
- **A**: Phase 1만 하고 Phase 2는 하지 않음 → 버그는 고치지만 UX 개선 기회 상실.
- **B**: Phase 1 + Phase 2를 하나의 큰 PR로 머지 → rollback granularity 상실, 검증 복잡도 증가. **기각**.
- **C**: READ 경로 gate 유지하고 earn 직후 프론트가 `fetchTopicDetail`을 재호출해서 quiz 채우기 → 불필요한 API 왕복, earn 직후 DB replication lag 가능성, 복잡도 증가. **기각**.
- **D**: 백엔드에 quiz 전용 엔드포인트 신설 (`GET /api/v1/quiz/:contextItemId`) → 엔드포인트 하나 더 추가, 라우팅/테스트/문서화 부담. READ gate 제거보다 이득 없음. **기각**.
- **E**: pending answer를 sessionStorage로 보존 → OAuth 리다이렉트가 외부 도메인(카카오)을 거치므로 탭 세션은 유지되지만, localStorage가 더 견고 (기존 `LOGIN_REDIRECT_KEY`와 동일 패턴). **localStorage 채택**.
- **F**: `SUBMIT_FAILED` 상태 생략하고 단순 alert/toast → flow 끊기고 재시도 포인트 모호. **기각**.
- **G (Hydration 전략)**: 완료된 퀴즈 카드 처리 3가지 대안:
  - **G-1**: 완료 후 카드 완전 숨김 (기존 동작 유지, 백엔드 변경 없음) — 스코프 작지만 PO 요구사항 미충족
  - **G-2**: "퀴즈 완료됨" 정적 플레이스홀더만 표시 (quiz: null 유지, 프론트에서 has_quiz=true & quiz=null 감지) — 과거 답변 표시 불가
  - **G-3**: 백엔드 `QuizForUser`에 `past_attempt` 필드 추가, 프론트에서 hydration (정답 인덱스는 여전히 미반환) — **채택**
  - 채택 근거: PO가 명시적으로 "답이 선택된 상태 + 비활성화된 카드" UX 요구. G-1/G-2로는 충족 불가. `quiz_results` 테이블에 이미 필요 컬럼 존재해 스키마 변경 불필요. 백엔드 변경 범위는 제한적 (model/repo/storage/service/test).
- **H (SUBMIT_FAILED 재시도 횟수)**: 무제한 vs 5회 제한 → **5회 채택** (PO 결정: 옵션 개수 4 + 1 generous).
- **I (Confetti 애니메이션)**: 톤앤매너 우려로 보류 vs 가벼운 버전 포함 → **제외 채택** (PO 결정). 톤앤매너 + 모바일 성능 우려 회피. 초록 accent + count-up + 코인 아이콘 pop만으로 충분한 celebration. 구현 단계에서 confetti 관련 코드 작성 금지.
- **J (로그인 모달 variant)**: 전용 `QuizLoginPromptModal` 신설 vs 기존 `LoginPromptModal`에 `variant` prop 추가 → **variant prop 채택** (PO 결정: 중복 파일 방지, 레이아웃 동일).

### Why Chosen
- 가장 작은 변경으로 버그를 고치고 (Phase 1), 그 위에 UX 개선을 빌드 (Phase 2).
- Phase 1 독립 머지 가능 → 배포 리스크 분산.
- 치팅 벡터 분석:
  - 보너스 크기 `1 ~ QUIZ_MAX_BONUS_COINS` 랜덤, `COIN_CAP`이 상한
  - 제출 시 여전히 coin_logs 확인 → earn 안 한 유저 제출 불가
  - earn 후에는 어차피 답 검색 가능 → READ gate는 10초 미리보기 차이만 만들뿐 실효 없음
- Paced reveal로 유저 몰입감 상승 + 기획 의도 부합 ("기다렸다가 터지는 경험").

### Consequences
**긍정적**:
- 버그 해결
- 비로그인 유저 참여 유도 → 전환율 개선 가능성
- 퀴즈 완료 유저가 재방문 시 과거 답변 확인 가능 (UX 연속성)
- 상태 머신으로 리팩토링해 향후 UX 튜닝 쉬워짐
- 명확한 observability (GTM 이벤트)

**부정적**:
- `quiz-card.tsx` 복잡도 증가 (상태 머신 + 타이머 + hydration 분기 관리)
- 테스트 부담: 상태 전이 + 타이머 + localStorage helper + hydration
- 모바일 Safari 애니메이션 호환성 리스크 (완화: will-change, transform-only, reduced-motion fallback)
- `TestQuiz_EarnGateBlocks` 삭제 → 기존 테스트 의도 상실 → 대체 테스트로 커버
- **Phase 2 백엔드 변경 포함** → 단순 프론트 배포보다 배포 순서 신경 필요 (백엔드 먼저 → 프론트)
- `GetUserAttempt` 신규 쿼리 → 토픽 페이지 로드 시 DB 한 번 더 호출 (가벼운 PK lookup이라 성능 영향 미미, 하지만 모니터링 필요)

### Follow-ups
- Phase 2 배포 후 GTM 이벤트 대시보드 구축해 flow drop-off 모니터링
- 2주 후 리뷰: submit_failed 발생률, pending_restored 성공률, hydration 케이스 노출 빈도 분석
- 모바일 실 디바이스 FPS 측정 (iPhone SE 수준 저사양 포함)
- Confetti 최종 결정 (시각 확인 후 제거 또는 유지)
- `GetUserAttempt` 쿼리 성능 모니터링 (P99 latency)
- 프론트와 백엔드의 `QuizForUser` 타입 불일치 정리 (web 쪽 `context_item_id` 누락) — 별도 cleanup PR
- Phase 3 후보: 퀴즈 재풀이 허용 기능, 정답 인덱스 노출 재검토 (기획 확정 시), submit 타임아웃 튜닝

---

## 7. Expanded Test Plan

### Unit Tests (Frontend)

#### `web/src/lib/quiz-pending.test.ts` (신규)
- `savePendingAnswer → loadPendingAnswer` 정상 복원
- 다른 `topicId`로 load 시 null 반환
- TTL 초과 (`saved_at`을 과거로 조작) 시 null + 자동 clear
- JSON parse 실패 (localStorage에 garbage) 시 null + clear
- quota exceeded 예외 swallow (Safari private mode 모킹)
- `clearPendingAnswer` 동작 확인

#### `web/src/components/quiz-card.test.tsx` (신규 또는 확장)
- IDLE → SELECTED_WAITING_EARN 전이 (옵션 클릭)
- SELECTED_WAITING_EARN에서 재클릭 시 선택만 변경, 상태 유지
- `earnCommitted` prop false → true로 바뀌고 min 시간 경과 후 EARN_CONFIRMED 전이
- EARN_CONFIRMED → SUBMITTING → EVALUATING → RESULT_* 자동 진행 (fake timers)
- submit 성공 시 RESULT_CORRECT/WRONG 분기 검증
- submit 실패 시 SUBMIT_FAILED, 재시도 버튼으로 복구
- submit 5회 연속 실패 시 재시도 버튼 disabled + 한계 메시지
- mount 시 pending 복원 (loadPendingAnswer 모킹)
- `topic.quiz === null`이면 pending 있어도 IDLE로 시작
- unmount 시 setState-after-unmount 경고 없음 (타이머 cleanup)
- 비로그인 유저 클릭 시 `onRequestLogin` 콜백 호출 + pending 저장
- `onStageChange` 콜백이 각 전이마다 호출됨

**Hydration 테스트**:
- `quiz.past_attempt` (correct) 있을 때 mount 즉시 RESULT_CORRECT 정적 상태 (shake/count-up 없음)
- `quiz.past_attempt` (wrong) 있을 때 mount 즉시 RESULT_WRONG 정적 상태 (shake 없음)
- hydration 상태에서 옵션 클릭 해도 onRequestLogin/transitionTo 호출 안 됨 (비활성화)
- hydration 상태에서 "+N 보너스!" 문구 렌더 안 됨
- hydration 상태에서 pending answer가 localStorage에 있어도 무시 + clearPendingAnswer 호출됨
- hydration + isHydrated=true 일 때 애니메이션 클래스 부재 확인

#### `web/src/pages/topic.test.tsx` (기존 테스트 확장)
- `handleCountdownComplete` 성공 분기에서 `earnCommitted = true`가 설정되는지
- `quizBusy` state에 따라 `isEarning`이 올바르게 계산되는지
- LoginPromptModal variant이 퀴즈 flow에서 "quiz_submit"으로 전달되는지

### Integration Tests (Backend Go)

#### `server/integration/quiz_test.go`
- **TestQuiz_ReadPathOpenSubmitGated** (Phase 1에서 재작성):
  - Setup: user + context_item + quiz 있음, coin_logs는 **없음**
  - `GetQuizForUser` → 퀴즈 정상 반환 (earn-gate 제거 검증)
  - `SubmitAnswer` → `ErrNotEarned` 반환 (기존 authoritative gate 유지 검증)
- **TestQuiz_EarnGateAllows**: 변경 없음 (coin_logs 있을 때 GetQuizForUser 성공)
- **TestQuiz_PastAttemptHydration_Correct** (Phase 2 신규):
  - Setup: quiz_results에 `is_correct=true, answered_index=2, coins_earned=7` 삽입
  - `GetQuizForUser` 호출
  - 검증: `PastAttempt != nil`, `SelectedIndex=2`, `IsCorrect=true`, `CoinsEarned=7`, `AttemptedAt` non-zero
  - 검증: 퀴즈 question/options는 여전히 정상 반환됨
- **TestQuiz_PastAttemptHydration_Wrong** (Phase 2 신규):
  - Setup: quiz_results에 `is_correct=false, answered_index=0, coins_earned=0` 삽입
  - 검증: `PastAttempt.IsCorrect=false`, `CoinsEarned=0`, `SelectedIndex=0`
- **TestQuiz_PastAttemptHydration_NoAttempt** (Phase 2 신규):
  - Setup: quiz 있지만 quiz_results 없음
  - 검증: `PastAttempt == nil`, 퀴즈 데이터는 정상 반환
- **TestQuiz_CorrectAnswerAwardsCoins**: 변경 없음
- **TestQuiz_WrongAnswerNoCoins**: 변경 없음
- **TestQuiz_DuplicateSubmissionBlocked**: 변경 없음 (SubmitAnswer UNIQUE constraint 검증)
- **TestQuiz_BonusExemptFromDailyLimit**: 변경 없음
- **TestQuiz_BatchQuizStatus**: 변경 없음

### Manual E2E Checklist

**Desktop Chrome**:
1. 비로그인 상태에서 `/topic/:id` 진입 → 퀴즈 카드 보임, 배지 "로그인하고 도전해보세요"
2. 옵션 클릭 → 로그인 모달 뜸, 카피 "로그인하고 정답을 제출해..."
3. 카카오 로그인 → 복귀 → 퀴즈 카드 "포인트 획득 대기중..." 표시, 선택이 복원됨
4. 10초 대기 → 자동 Stage B (포인트 획득 완료) → C → D → E
5. 정답 시 초록 + count-up 확인
6. 오답 경로: 다른 토픽에서 오답 → 빨간 + shake + "아쉽지만 틀렸어요"
7. 재시도 테스트: 네트워크 offline 상태에서 submit 시도 → SUBMIT_FAILED → online 복구 → 재시도 버튼 → 성공
8. 재시도 한도: 연속 5회 실패 시 재시도 버튼 disabled + "잠시 후 다시 방문해주세요 🙏" 문구
9. **Hydration flow**: 정답자 경로 완료 → 같은 토픽 페이지 새로고침 → 초록 flat + 체크 아이콘 + "이미 완료했어요" 정적 표시, 옵션 비활성화
10. **Hydration flow (오답)**: 오답 경로 완료 → 새로고침 → 빨간 flat + X 아이콘 + "이미 완료했어요", shake 없음, 옵션 비활성화
11. Hydration 상태에서 options 클릭해도 아무 반응 없음 확인
12. Stale pending 케이스: 퀴즈 완료 후 localStorage에 수동으로 pending 주입 → 새로고침 → hydration 우선, pending 무시 + 자동 clearPendingAnswer

**Desktop Chrome + prefers-reduced-motion**:
- DevTools Rendering 탭에서 `prefers-reduced-motion: reduce` 에뮬레이션
- 위 flow 다시 수행 → 애니메이션 없음, 단계 전환 텍스트 정상

**Mobile iOS Safari** (실기기):
- 레이아웃 깨짐 없음
- 옵션 탭 영역 충분히 큼
- 애니메이션 FPS 체감 부드러움
- 탭 전환 → 복귀 후 상태 유지

**Mobile Android Chrome** (실기기):
- 위와 동일

**Samsung Internet** (실기기, 가능하면):
- 위와 동일

### Observability Verification
- GTM Preview 모드에서 각 이벤트 trigger 확인
- `quiz_preselect`, `quiz_stage_enter`, `quiz_submit_success/fail`, `quiz_pending_restored` 모두 emit되는지

---

## 8. Open Questions & Assumptions

### 가정 (확정됨)
- API가 오답 시 정답 인덱스를 반환하지 않음 → 오답 UI는 정답 reveal 없이 담백하게만 (API 설계 유지)
- 퀴즈는 한 번만 풀 수 있음 (UNIQUE constraint + HasAttempted 체크). 재풀이 기획 없음 (PO 확인)
- Phase 1 + Phase 2 분리 머지
- localStorage TTL 10분 (로그인 flow 여유)
- Stage timing 0.6s / 1.0s / 0.8s / 0.8s (PO 원안)
- **퀴즈 완료 유저 hydration — Phase 2에 포함** (PO 요청): 백엔드 `past_attempt` 필드 추가 + 프론트 정적 복원 UI

### PO 결정 완료
- ✅ **SUBMIT_FAILED 재시도 횟수**: **5회 제한** (옵션 4 + 1 generous)
- ✅ **로그인 모달 variant**: `LoginPromptModal`에 `variant` prop 추가해 통합 (전용 컴포넌트 불필요)
- ✅ **완료 퀴즈 표시**: 카드 숨김 대신 **답이 선택된 정적 비활성화 카드** 표시

### PO 결정 완료 (추가)
- ✅ **Confetti 애니메이션**: **제외** (PO 결정). 초록 accent + count-up + 코인 아이콘 pop만으로 celebration 충분. 톤앤매너 + 모바일 성능 우려 둘 다 회피. 구현 시 confetti 관련 코드 (wl-confetti 클래스, ConfettiBurst 컴포넌트 등) 작성 금지.

### Open (구현 후속)
- **Hydration 쿼리 성능**: `GetUserAttempt`가 토픽 페이지 로드 시 매번 호출됨. `quiz_results`에 `(user_id, quiz_id)` 인덱스 유무 확인 필요 (UNIQUE constraint가 있다면 자동 인덱스 존재). 필요 시 explicit index 추가.

---

## 9. 구현 시 주의사항 요약

### 프론트엔드
1. **모든 setTimeout은 ref로 관리 + cleanup 필수**
2. **submit 더블파이어 방지 ref 가드** (`submitCalledRef`)
3. **submit 재시도 카운터 ref** (`retryCountRef`, 5회 제한)
4. **`mountedRef`로 setState-after-unmount 방지**
5. **`earnCommitted` prop 변경 감지 useEffect의 dependency 배열 정확히 설정**
6. **React strict mode double-invoke 안전성** (개발 중 effect 2번 실행에도 멱등)
7. **localStorage try/catch swallow** (iOS Safari private mode)
8. **CSS 애니메이션은 transform 속성만 사용** (layout thrashing 방지)
9. **`will-change` 명시** (모바일 성능)
10. **reduced-motion media query 정확히 작동하는지 실측**
11. **aria-live="polite"로 stage 전환 메시지 screen reader announce**
12. **GTM 이벤트 key 네이밍 일관성** (snake_case)
13. **TypeScript 엄격 타입** — `QuizStage` union으로 exhaustive check
14. **테스트에서 fake timers 사용** (`vi.useFakeTimers` 또는 `jest.useFakeTimers`)
15. **quiz-pending helper의 저장 format 버저닝** — key에 `_v1` 접미사, 미래 schema 변경 대비
16. **Hydration 분기 우선순위**: mount 시 `quiz?.past_attempt` 체크가 pending 복원보다 먼저 실행되어야 함
17. **isHydrated 플래그로 모든 애니메이션 클래스 conditional**
18. **Hydration 상태에서 옵션 클릭 핸들러 완전 비활성화** (`disabled` + `pointer-events-none` 둘 다)

### 백엔드 (Phase 2)
19. **Phase 1과 Phase 2 커밋 완전 분리** — 섞어서 push 금지
20. **`PastAttempt` struct json 태그 정확히** — frontend types와 1:1 매칭 (snake_case)
21. **`HasAttempted` 제거 시 mock 구현체도 함께 수정** — 빌드 깨짐 방지
22. **`GetUserAttempt`에서 `pgx.ErrNoRows`는 정상 케이스** — error wrap 금지, `nil, nil` 반환
23. **`time.Time` JSON 직렬화 확인** — Go 기본 RFC 3339, 프론트 `attempted_at: string`과 호환
24. **Backend integration test는 testcontainers + Postgres 16 필요** — Docker 환경 보장
25. **`errors` 패키지 import** — context_history.go에서 필요 시 유지/제거 결정

### 배포
26. **백엔드 먼저, 프론트 나중**: Phase 2 배포 시 응답 필드 추가 → 기존 프론트는 무시 (forward compat) → 새 프론트 배포로 hydration 활성화
27. **롤백은 역순**: 프론트 먼저 revert → 백엔드 revert
28. **DB 스키마 변경 없음** — 마이그레이션 불필요, `quiz_results` 기존 컬럼 활용
