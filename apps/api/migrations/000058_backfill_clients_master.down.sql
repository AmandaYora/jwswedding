-- Rollback backfill: putuskan tautan, buang master hasil backfill. Baris
-- client_contacts dan projects sendiri tidak dihapus -- hanya tautannya.
UPDATE projects SET client_id = NULL WHERE client_id IS NOT NULL;
UPDATE client_contacts SET client_id = NULL WHERE client_id IS NOT NULL;
DELETE FROM clients;
