ALTER TABLE users
    ADD COLUMN adblock_detected_at     TIMESTAMPTZ,
    ADD COLUMN adblock_not_detected_at TIMESTAMPTZ;
