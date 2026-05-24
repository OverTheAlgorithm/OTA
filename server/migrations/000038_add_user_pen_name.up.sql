-- Pen name (필명) for editor+ users. Optional byline shown on published editor
-- posts. Falls back to nickname when empty/NULL/whitespace at read time, so
-- changing the pen name retroactively updates already-published posts.

ALTER TABLE users
    ADD COLUMN pen_name TEXT;

-- The trim-then-length check rejects whitespace-only values (incl. tabs and
-- newlines) and enforces the same 2..32 bounds as user.NormalisePenName.
-- The E'…' string covers ASCII whitespace: space, tab, LF, CR, VT, FF.
ALTER TABLE users
    ADD CONSTRAINT users_pen_name_length_check
    CHECK (
        pen_name IS NULL
        OR char_length(btrim(pen_name, E' \t\n\r\v\f')) BETWEEN 2 AND 32
    );

-- Case-insensitive uniqueness, but only for actually-set values. NULLs and
-- whitespace-only strings are excluded so multiple users without a pen name
-- never collide.
CREATE UNIQUE INDEX users_pen_name_unique_idx
    ON users (LOWER(btrim(pen_name, E' \t\n\r\v\f')))
    WHERE pen_name IS NOT NULL AND btrim(pen_name, E' \t\n\r\v\f') <> '';
