-- Make user_id nullable to support anonymous push token registration.
-- Token is the device identifier; user_id is linked on login, unlinked on logout.

-- 1. Drop existing unique constraint (user_id, token)
ALTER TABLE push_tokens DROP CONSTRAINT push_tokens_user_id_token_key;

-- 2. Add unique constraint on token only (one device = one token)
ALTER TABLE push_tokens ADD CONSTRAINT push_tokens_token_key UNIQUE (token);

-- 3. Drop existing FK (ON DELETE CASCADE)
ALTER TABLE push_tokens DROP CONSTRAINT push_tokens_user_id_fkey;

-- 4. Make user_id nullable
ALTER TABLE push_tokens ALTER COLUMN user_id DROP NOT NULL;

-- 5. Re-add FK with ON DELETE SET NULL (device keeps token, loses user association)
ALTER TABLE push_tokens ADD CONSTRAINT push_tokens_user_id_fkey
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL;
