-- User bank account (프로필에 미리 등록)
CREATE TABLE user_bank_accounts (
    user_id        UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    bank_name      VARCHAR(50)  NOT NULL,
    account_number VARCHAR(50)  NOT NULL,
    account_holder VARCHAR(50)  NOT NULL,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- Withdrawal request (부모 레코드 — 공통 정보)
CREATE TABLE withdrawals (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    amount          INT  NOT NULL CHECK (amount > 0),
    bank_name       VARCHAR(50) NOT NULL,
    account_number  VARCHAR(50) NOT NULL,
    account_holder  VARCHAR(50) NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_withdrawals_user_id    ON withdrawals(user_id);
CREATE INDEX idx_withdrawals_created_at ON withdrawals(created_at DESC);

-- Withdrawal state transitions (이벤트 레코드 — 상태 전이)
CREATE TABLE withdrawal_transitions (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    withdrawal_id  UUID        NOT NULL REFERENCES withdrawals(id) ON DELETE CASCADE,
    status         VARCHAR(20) NOT NULL CHECK (status IN ('pending','approved','rejected','cancelled')),
    note           TEXT        NOT NULL DEFAULT '',
    actor_id       UUID        REFERENCES users(id),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_wt_withdrawal_id ON withdrawal_transitions(withdrawal_id);
CREATE INDEX idx_wt_status        ON withdrawal_transitions(status);
