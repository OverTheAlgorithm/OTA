ALTER TABLE users ADD COLUMN email_verified BOOLEAN NOT NULL DEFAULT false;

-- Existing users who have an email set are already using the service, so mark them as verified.
UPDATE users SET email_verified = true WHERE email IS NOT NULL AND email != '';
