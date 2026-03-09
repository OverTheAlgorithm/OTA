CREATE TABLE terms (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title VARCHAR(200) NOT NULL,
    description TEXT,
    url TEXT NOT NULL,
    active BOOLEAN NOT NULL,
    required BOOLEAN NOT NULL,
    version VARCHAR(20) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (title, version)
);

CREATE INDEX idx_terms_active ON terms(active);

CREATE TABLE user_term_consents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    term_id UUID NOT NULL REFERENCES terms(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, term_id)
);

CREATE INDEX idx_user_term_consents_user_id ON user_term_consents(user_id);
