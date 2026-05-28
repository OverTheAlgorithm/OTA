-- Tracks how a user's nickname came to be set, as a single linear state
-- machine. The three states encode the only meaningful distinctions we
-- need for both the Kakao-login refresh decision and the first-time
-- comment-warning UX:
--
--   'default'      — nickname comes from Kakao and the user has not yet
--                    acknowledged that it is shown publicly. Kakao logins
--                    overwrite this nickname; comment composers show the
--                    warning modal.
--   'acknowledged' — user saw the warning and chose to keep the Kakao
--                    nickname. Kakao logins may still refresh it (consent
--                    given); the warning modal is suppressed.
--   'custom'       — user explicitly set their own nickname. Kakao logins
--                    do NOT overwrite it; the warning modal is suppressed.
--
-- Transitions are forward-only:
--   default      -> acknowledged  (POST /user/nickname-warning/dismiss)
--   default      -> custom        (PUT  /user/nickname)
--   acknowledged -> custom        (PUT  /user/nickname)
--
-- A single column avoids the "flag combination" bugs that come from
-- representing the same state as a boolean pair.

ALTER TABLE users
    ADD COLUMN nickname_state TEXT NOT NULL DEFAULT 'default'
        CHECK (nickname_state IN ('default', 'acknowledged', 'custom'));
