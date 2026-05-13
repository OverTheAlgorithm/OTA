DROP TABLE IF EXISTS editor_posts;
DROP TABLE IF EXISTS role_change_logs;

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;
