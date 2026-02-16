CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    kakao_id BIGINT UNIQUE NOT NULL,
    email VARCHAR(255),
    nickname VARCHAR(100),
    profile_image TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_users_kakao_id ON users(kakao_id);
