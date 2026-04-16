-- Remove the unused domains column from agents
ALTER TABLE agents DROP COLUMN IF EXISTS domains;
