-- Adds a creation time. Existing rows take the migration time through the
-- default, so the upgrade keeps every row and constraint from 0001.
ALTER TABLE notes ADD COLUMN created_at TIMESTAMPTZ NOT NULL DEFAULT now();
CREATE INDEX notes_owner_created_idx ON notes (owner_sub, created_at DESC);
