CREATE TABLE email_verification_codes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    email VARCHAR(255) NOT NULL,
    code VARCHAR(6) NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    used BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Fast lookup by user_id + code for verification
CREATE INDEX idx_email_verification_user_code ON email_verification_codes(user_id, code);

-- Cleanup expired codes periodically
CREATE INDEX idx_email_verification_expires ON email_verification_codes(expires_at);

-- Rate limiting: count recent codes per user
CREATE INDEX idx_email_verification_user_created ON email_verification_codes(user_id, created_at);

COMMENT ON TABLE email_verification_codes IS 'Stores 6-digit verification codes for email address verification';
COMMENT ON COLUMN email_verification_codes.code IS '6-digit numeric verification code';
COMMENT ON COLUMN email_verification_codes.expires_at IS 'Code expires 5 minutes after creation';
COMMENT ON COLUMN email_verification_codes.used IS 'Whether the code has been successfully used';
