-- 0002_password.sql - adds password-based sign-in alongside magic links.
-- Empty string means "no password set" (a magic-link-only account).

ALTER TABLE users ADD COLUMN password_hash TEXT NOT NULL DEFAULT '';
