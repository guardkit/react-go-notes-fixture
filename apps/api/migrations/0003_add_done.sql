-- Adds the completion flag. It is NOT NULL with a false default, so every row,
-- including the ones created before this migration, reads back as false.
ALTER TABLE notes ADD COLUMN done BOOLEAN NOT NULL DEFAULT false;
