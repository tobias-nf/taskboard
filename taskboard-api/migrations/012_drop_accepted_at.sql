-- Remove the accepted_at column — the accepted status was removed
ALTER TABLE tasks DROP COLUMN IF EXISTS accepted_at;
