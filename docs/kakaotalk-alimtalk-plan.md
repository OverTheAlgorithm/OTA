# KakaoTalk 알림톡(AlimTalk) 구현 계획

## 개요

이 문서는 OTA 서비스에서 매일 아침 7시(KST) 사용자에게 카카오톡 메시지를 발송하기 위한 구현 계획을 담고 있다.

---

## 1. 기술 선택: 알림톡(AlimTalk) via SOLAPI

### 선택 이유

| 방식 | 특징 | OTA 적합성 |
|------|------|------------|
| **카카오톡 메시지 API** | 사용자가 서비스 친구에게 보내는 P2P 방식 | ❌ 서버→사용자 발송 불가 |
| **알림톡(AlimTalk)** | 서비스→사용자 정보성 메시지, 전화번호로 발송 | ✅ 최적 |
| **브랜드 메시지** | 광고/프로모션 메시지, 채널 친구 대상 | ❌ 정보성 콘텐츠에 부적합 |

**결론**: 매일 뉴스 다이제스트를 모든 사용자에게 발송하는 OTA의 요구사항에는 **알림톡 + SOLAPI**가 유일한 선택이다.

### SOLAPI 선택 이유

- 카카오 공식 딜러(Official Dealer). 개인/기업이 알림톡 API를 직접 사용할 수 없으며 반드시 공식 딜러를 통해야 함
- 스타트업 친화적: 월 기본료 없음, 메시지당 과금 (알림톡 13원/건)
- Go 구현에 적합한 REST API + HMAC-SHA256 인증
- 100명 사용자 기준 약 1,300원/일

---

## 2. 사전 준비 (개발 전 필수)

> **주의**: 아래 절차는 개발 코드 작성 전에 완료되어야 한다. 특히 템플릿 심사는 1~3일 소요된다.

### 2-1. 사업자 등록 (하드 선행조건)

- 카카오 비즈니스 채널 인증에 **사업자등록번호**가 필수
- 사업자 등록 없이는 알림톡 발송 불가
- 프리랜서/개인도 간이과세자로 등록 가능

### 2-2. 카카오 채널 생성 및 비즈니스 채널 인증

1. [카카오 채널 관리자센터](https://center-pf.kakao.com/) 접속
2. 채널 생성 시 설정:
   - 채널 공개: **ON**
   - 검색 허용: **ON**
   - 고객 문의 연락처 등록 (전화번호 또는 URL)
3. 비즈니스 채널 인증 신청:
   - 사업자등록번호 입력
   - 심사 기간: 수일 소요 가능

### 2-3. SOLAPI 가입 및 채널 연결

1. [solapi.com](https://solapi.com) 가입
2. 카카오 채널 연결 → `pfId` 발급
3. API Key + API Secret 발급
4. 발신번호 등록 (SMS 폴백 발송용 유선/무선 번호)

### 2-4. 알림톡 템플릿 등록 및 심사

- 심사 기간: **1~3 영업일**
- 승인된 템플릿은 수정 불가 (수정 시 재심사 필요)
- 변수 문법: `#{변수명}`
- 조건: 정보성 내용만 가능, 광고성 문구 포함 시 거절

**제안 템플릿 (초안)**:

```
[오버 더 알고리즘] #{date} 오늘의 맥락

#{digest}

📌 구독 관리: #{settings_url}
```

- `date`: 예) "2026년 2월 18일 (수)"
- `digest`: 오늘의 주제 요약 (1,000자 이내 전체 메시지 기준)
- `settings_url`: 구독 설정 페이지 URL

---

## 3. 데이터베이스 변경

### 3-1. users 테이블 변경

현재 `users` 테이블에 전화번호 컬럼이 없다. 알림톡은 전화번호로 발송하므로 추가가 필요하다.

**Migration: `000002_add_phone_alimtalk_to_users.up.sql`**

```sql
ALTER TABLE users
    ADD COLUMN phone_number VARCHAR(20),
    ADD COLUMN phone_verified BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN alimtalk_opt_in BOOLEAN NOT NULL DEFAULT FALSE;
```

**Migration: `000002_add_phone_alimtalk_to_users.down.sql`**

```sql
ALTER TABLE users
    DROP COLUMN alimtalk_opt_in,
    DROP COLUMN phone_verified,
    DROP COLUMN phone_number;
```

### 3-2. message_deliveries 테이블 (발송 이력)

```sql
CREATE TABLE message_deliveries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    channel VARCHAR(20) NOT NULL,         -- 'alimtalk' | 'email'
    status VARCHAR(20) NOT NULL,          -- 'pending' | 'sent' | 'failed' | 'fallback_sms'
    external_message_id VARCHAR(100),     -- SOLAPI messageId
    error_message TEXT,
    sent_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_deliveries_user_id ON message_deliveries(user_id);
CREATE INDEX idx_deliveries_created_at ON message_deliveries(created_at);
```

---

## 4. 시스템 아키텍처

```
┌─────────────────────────────────────────────────────┐
│                  OTA Backend (Go/Gin)                │
│                                                     │
│  ┌─────────────┐    ┌──────────────┐               │
│  │  Scheduler  │───▶│  Dispatcher  │               │
│  │  (cron      │    │              │               │
│  │   22:00 UTC)│    │ ┌──────────┐ │               │
│  └─────────────┘    │ │ AlimTalk │ │               │
│                     │ │ Service  │─┼──▶ SOLAPI API  │
│  ┌─────────────┐    │ └──────────┘ │               │
│  │  User API   │    │ ┌──────────┐ │               │
│  │  Handlers   │    │ │  Email   │ │               │
│  │             │    │ │ Service  │─┼──▶ SMTP/SES    │
│  └─────────────┘    │ └──────────┘ │               │
│                     └──────────────┘               │
└─────────────────────────────────────────────────────┘
             │
             ▼
      PostgreSQL (Docker)
```

**스케줄**: 매일 22:00 UTC = 07:00 KST (다음날 아침)

---

## 5. 구현 범위 및 파일 구조

```
server/
├── internal/
│   ├── user/
│   │   ├── model.go              # phone_number, phone_verified, alimtalk_opt_in 필드 추가
│   │   └── repository.go         # UpdatePhone, UpdateAlimtalkOptIn 메서드 추가
│   ├── notification/             # [신규 패키지]
│   │   ├── alimtalk/
│   │   │   ├── client.go         # SOLAPI HTTP 클라이언트 (HMAC-SHA256 인증)
│   │   │   ├── sender.go         # 알림톡 발송 로직
│   │   │   └── model.go          # 요청/응답 구조체
│   │   └── dispatcher.go         # 사용자 목록 조회 → 채널별 발송 분기
│   ├── scheduler/                # [신규 패키지]
│   │   └── scheduler.go          # cron 기반 일일 발송 스케줄러
│   └── phone/                   # [신규 패키지]
│       ├── handler.go            # 전화번호 등록/인증 HTTP 핸들러
│       ├── otp.go                # OTP 생성/검증 로직
│       └── sms.go                # OTP 발송 (SOLAPI SMS 사용)
├── migrations/
│   ├── 000002_add_phone_alimtalk_to_users.up.sql
│   ├── 000002_add_phone_alimtalk_to_users.down.sql
│   ├── 000003_create_message_deliveries.up.sql
│   └── 000003_create_message_deliveries.down.sql
```

---

## 6. API 엔드포인트 추가

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/api/v1/phone/send-otp` | Yes | 입력한 번호로 OTP SMS 발송 |
| POST | `/api/v1/phone/verify-otp` | Yes | OTP 검증 및 전화번호 등록 확인 |
| PATCH | `/api/v1/settings/alimtalk` | Yes | 알림톡 수신 설정 ON/OFF |

---

## 7. SOLAPI 연동 상세

### 7-1. 인증 (HMAC-SHA256)

```
date      = ISO8601 타임스탬프 (예: "2026-02-17T22:00:00Z")
salt      = 랜덤 12~64바이트 hex 문자열
signature = HMAC-SHA256(date + salt, API_SECRET)

Authorization: HMAC-SHA256 apiKey={API_KEY}, date={date}, salt={salt}, signature={signature}
```

- 요청은 ±15분 이내에만 유효
- salt는 요청마다 새로 생성

### 7-2. 발송 API

**Endpoint**: `POST https://api.solapi.com/messages/v4/send-many/detail`

**요청 예시**:
```json
{
  "messages": [
    {
      "to": "01012345678",
      "kakaoOptions": {
        "pfId": "PF_CHANNEL_ID",
        "templateId": "APPROVED_TEMPLATE_CODE",
        "variables": {
          "#{date}": "2026년 2월 18일 (수)",
          "#{digest}": "• 환승연애3: 출연자 A가 전 남자친구 2명과 동반 출연, 삼각관계 전개 중\n• 총선: 여야 협상 결렬, 임시국회 소집 논의\n• 코스피: 2,850선 회복, 외국인 순매수 지속",
          "#{settings_url}": "https://overthealgorithm.com/settings"
        },
        "disableSms": false
      },
      "from": "07012345678"
    }
  ]
}
```

**응답 예시**:
```json
{
  "groupId": "G4V202602180900000000000000",
  "to": "01012345678",
  "from": "07012345678",
  "type": "ATA",
  "statusCode": "2000",
  "statusMessage": "정상 접수",
  "messageId": "M4V202602180900000000000001"
}
```

**상태 코드**:
- `2000`: 정상 발송
- `3020`: 수신자 KakaoTalk 미설치 → SMS 폴백 발송
- `4000+`: 오류

### 7-3. 배치 발송

- 1회 요청당 최대 1,000건
- 사용자 수 > 1,000명 시 청크 분할 처리 필요

---

## 8. 환경변수 추가

`server/.env.example`에 추가:

```bash
# SOLAPI (알림톡)
SOLAPI_API_KEY=your_solapi_api_key
SOLAPI_API_SECRET=your_solapi_api_secret
SOLAPI_SENDER_KEY=PFxxxxxxxxxxxxxxxxxxxxxxxx   # pfId (카카오 채널 ID)
SOLAPI_TEMPLATE_ID=APPROVED_TEMPLATE_CODE      # 승인된 템플릿 코드
SOLAPI_FROM_NUMBER=07012345678                 # 등록된 발신번호 (SMS 폴백용)
```

---

## 9. 구현 순서 (Phase)

### Phase 1: 기반 작업 (사전 준비 완료 후)

1. DB 마이그레이션 작성 및 적용
2. User 모델/리포지토리에 phone 관련 필드 추가
3. Config에 SOLAPI 환경변수 추가

### Phase 2: 전화번호 등록

1. SOLAPI SMS OTP 발송 구현 (`phone/otp.go`)
2. OTP 인증 및 전화번호 저장 구현 (`phone/handler.go`)
3. 프론트엔드: 전화번호 입력/인증 UI (홈 페이지 또는 설정 페이지)

### Phase 3: 알림톡 발송

1. SOLAPI 클라이언트 구현 (`notification/alimtalk/client.go`)
2. 알림톡 발송 서비스 구현 (`notification/alimtalk/sender.go`)
3. Dispatcher 구현 (`notification/dispatcher.go`)
4. 메시지 발송 이력 저장

### Phase 4: 스케줄러

1. cron 기반 스케줄러 구현 (`scheduler/scheduler.go`)
2. 매일 22:00 UTC (= 07:00 KST) 발송 트리거
3. 콘텐츠는 하드코딩 테스트용 더미 데이터로 시작

### Phase 5: 설정 UI

1. 알림톡 수신 설정 토글 API
2. 프론트엔드 설정 화면

---

## 10. 미결 사항 및 결정 필요 항목

| 항목 | 현재 상태 | 결정 필요 내용 |
|------|-----------|----------------|
| 사업자 등록 | ❓ 미확인 | 사업자 등록 여부 확인 |
| 카카오 채널 | ❓ 미생성 | 채널명, 프로필 이미지 결정 |
| 알림톡 템플릿 | ❓ 미작성 | 최종 메시지 포맷 확정 후 심사 신청 |
| SMS 폴백 | 권장 | AlimTalk 실패 시 SMS로 재발송할지 여부 |
| OTP 발송 방식 | SOLAPI SMS 활용 예정 | 비용 고려 (SMS 약 8원/건) |
| 콘텐츠 소스 | 미구현 | 알림톡 구현과 별개로 콘텐츠 파이프라인 필요 |

---

## 11. 참고 자료

- [SOLAPI 알림톡 가이드](https://solapi.com/guides/kakao-ata-guide)
- [SOLAPI API 인증](https://developers.solapi.dev/references/authentication/api-key)
- [SOLAPI 발송 API 레퍼런스](https://developers.solapi.dev/references/messages/sendMany)
- [카카오 채널 관리자센터](https://center-pf.kakao.com/)
- [카카오 비즈니스 알림톡 가이드](https://kakaobusiness.gitbook.io/main/ad/infotalk)
- [SOLAPI 가격표](https://solapi.com/pricing)
