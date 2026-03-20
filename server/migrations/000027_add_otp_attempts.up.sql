-- Add attempts column to email_verification_codes for brute-force protection.
-- Each failed verify-code attempt increments this counter.
-- When attempts >= MaxVerifyAttempts (5), the code is rejected without checking.

ALTER TABLE email_verification_codes
  ADD COLUMN IF NOT EXISTS attempts INT NOT NULL DEFAULT 0;
