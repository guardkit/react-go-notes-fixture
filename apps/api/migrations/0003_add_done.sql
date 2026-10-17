-- Adds the completion flag. Every row - including the ones written before this
-- column existed - takes the not-done default, so the upgrade keeps every row
-- and constraint from 0001 and 0002 and answers each of them with a flag.
ALTER TABLE notes ADD COLUMN done BOOLEAN NOT NULL DEFAULT false;
