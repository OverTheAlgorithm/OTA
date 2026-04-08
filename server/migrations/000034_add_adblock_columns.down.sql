ALTER TABLE users
    DROP COLUMN IF EXISTS adblock_detected_at,
    DROP COLUMN IF EXISTS adblock_not_detected_at;
