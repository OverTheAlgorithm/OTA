-- Revert: restore user_id NOT NULL, UNIQUE(user_id, token), ON DELETE CASCADE.

-- 1. Remove anonymous tokens (can't enforce NOT NULL with NULL rows)
DELETE FROM push_tokens WHERE user_id IS NULL;

-- 2. Drop FK (ON DELETE SET NULL)
ALTER TABLE push_tokens DROP CONSTRAINT push_tokens_user_id_fkey;

-- 3. Make user_id NOT NULL again
ALTER TABLE push_tokens ALTER COLUMN user_id SET NOT NULL;

-- 4. Re-add FK with ON DELETE CASCADE
ALTER TABLE push_tokens ADD CONSTRAINT push_tokens_user_id_fkey
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

-- 5. Drop token-only unique constraint
ALTER TABLE push_tokens DROP CONSTRAINT push_tokens_token_key;

-- 6. Restore original unique constraint
ALTER TABLE push_tokens ADD CONSTRAINT push_tokens_user_id_token_key UNIQUE (user_id, token);
