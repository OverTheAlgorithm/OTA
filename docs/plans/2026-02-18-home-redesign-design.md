# Home 화면 리디자인 설계

**날짜**: 2026-02-18
**상태**: 승인됨

---

## 목표

홈 화면에 두 가지 핵심 기능 추가:
1. **받아본 맥락 이력** — 실제 수신한 메시지 기준으로 날짜별 토픽 표시
2. **관심사 설정** — 태그 클릭 방식으로 구독 카테고리 관리

---

## 레이아웃 구조

단일 스크롤 페이지 (탭 없음). 위에서 아래로:

```
┌─────────────────────────────────┐
│ [logo]          [avatar] [logout]│  ← sticky header
├─────────────────────────────────┤
│  안녕하세요, esPark님             │
│  다음 브리핑: 내일 07:00          │
├─────────────────────────────────┤
│  내 관심사                       │
│  [연예/오락] [경제] [스포츠] [+]  │  ← 선택된 태그 (삭제 가능)
│  ── 더 추가하기 ──               │
│  [정치] [IT/기술] [패션] ...      │  ← 추천 태그 목록
│  [직접입력___________] [추가]     │  ← 커스텀 입력
├─────────────────────────────────┤
│  받아본 맥락                     │
│  ┌──────────────────────────┐   │
│  │ 2026.02.18               │   │
│  │ 전체 맥락                 │   │
│  │  • 환승연애 시즌3 ...      │   │
│  │  • 코스피 2400 돌파 ...   │   │
│  │ 내 관심사                 │   │
│  │  • [경제] 원달러 환율 ...  │   │
│  └──────────────────────────┘   │
└─────────────────────────────────┘
```

---

## 컴포넌트 구조

```
home.tsx
├── <HomeHeader />
│     로고, 유저 아바타 (프로필 이미지 없으면 이니셜), 로그아웃
├── <WelcomeBanner />
│     이름 인사, 다음 브리핑 시간 (매일 07:00 KST 고정)
├── <InterestSection />
│   ├── <SelectedTags />       선택된 태그 + X 버튼으로 제거
│   ├── <SuggestedTags />      미리 정의된 태그 중 미선택 항목만 표시
│   └── <CustomTagInput />     직접 입력 + 추가 버튼
└── <HistorySection />
    └── <HistoryCard /> × N    날짜별 카드
          ├── 전체 맥락 토픽 목록 (category = "top" 또는 전체)
          └── 내 관심사 토픽 목록 (사용자 구독 카테고리 매칭)
```

---

## 미리 정의된 관심사 태그

```
연예/오락 | 경제 | 스포츠 | 정치 | IT/기술 | 패션/뷰티 |
음식/맛집 | 여행 | 건강/의학 | 게임 | 사회/이슈 | 문화/예술
```

---

## 백엔드 API 설계

### 새로 추가할 엔드포인트

| Method | Path | 설명 |
|--------|------|------|
| GET | `/api/v1/context/history` | 받아본 맥락 이력 |
| GET | `/api/v1/preferences` | 배달 수신 설정 조회 |
| PUT | `/api/v1/preferences` | 배달 수신 on/off 토글 |
| GET | `/api/v1/subscriptions` | 내 관심사 목록 |
| POST | `/api/v1/subscriptions` | 관심사 추가 |
| DELETE | `/api/v1/subscriptions/:category` | 관심사 삭제 |

### GET /api/v1/context/history

`delivery_logs` → `collection_runs` → `context_items` JOIN.
현재 유저의 수신 이력을 날짜 역순으로 반환.

```json
[
  {
    "date": "2026-02-18",
    "delivered_at": "2026-02-18T22:15:00Z",
    "channel": "email",
    "items": [
      {
        "category": "top",
        "rank": 1,
        "topic": "환승연애 시즌3 출연자 논란",
        "summary": "시즌3 출연자가 전 남자친구 두 명을 동반 출연해 화제."
      }
    ]
  }
]
```

### POST /api/v1/subscriptions

```json
{ "category": "경제" }
```

### GET /api/v1/subscriptions

```json
["경제", "스포츠", "IT/기술"]
```

---

## 데이터 흐름

```
홈 진입
  ├── GET /api/v1/subscriptions      → InterestSection 초기화
  ├── GET /api/v1/context/history    → HistorySection 초기화
  └── GET /api/v1/preferences        → (향후 배달 토글 UI용)

관심사 추가
  └── POST /api/v1/subscriptions → 로컬 상태 즉시 업데이트 (optimistic)

관심사 제거
  └── DELETE /api/v1/subscriptions/:category → 로컬 상태 즉시 업데이트
```

---

## 디자인 톤

- 배경: `#0f0a19`, 텍스트: `#f5f0ff`, 보조 텍스트: `#9b8bb4`
- 카드: `bg-[#1a1229]` + `border-[#2d1f42]`
- 강조: `#e84d3d` (레드) / `#5ba4d9` (블루) / `#7bc67e` (그린)
- 선택된 태그: 채워진 배경, 미선택 태그: 아웃라인 스타일
- 아이콘: Toss 스타일 (심플 라인, 적절한 여백)

---

## 구현 Phase

| Phase | 내용 | 범위 |
|-------|------|------|
| 1 | 백엔드 API 6개 구현 | Go (server/) |
| 2 | 관심사 선택 UI | React (web/) |
| 3 | 맥락 이력 UI | React (web/) |
| 4 | 연결 및 polish | 전체 |
