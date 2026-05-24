DROP INDEX IF EXISTS users_pen_name_unique_idx;
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_pen_name_length_check;
ALTER TABLE users DROP COLUMN IF EXISTS pen_name;
