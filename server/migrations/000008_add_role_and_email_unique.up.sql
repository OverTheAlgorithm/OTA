ALTER TABLE users ADD COLUMN role TEXT NOT NULL DEFAULT 'user';
ALTER TABLE users ADD CONSTRAINT users_email_unique UNIQUE (email);
