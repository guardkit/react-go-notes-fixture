-- Adds the completion flag. Rows already in the table take false through the
-- column default, so the upgrade keeps every row and constraint from 0001 and
-- 0002 and no pre-existing note is ever read back as done (ASSUM-001).
ALTER TABLE notes ADD COLUMN done BOOLEAN NOT NULL DEFAULT false;
