-- Revert: remove attempts column from email_verification_codes

ALTER TABLE email_verification_codes
  DROP COLUMN IF EXISTS attempts;
