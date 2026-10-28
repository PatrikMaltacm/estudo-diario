-- Execute once in PostgreSQL before deploying the unsubscribe endpoint.
-- Existing accounts remain eligible for the reminders they already receive.
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS email_reminders_enabled BOOLEAN NOT NULL DEFAULT TRUE;
