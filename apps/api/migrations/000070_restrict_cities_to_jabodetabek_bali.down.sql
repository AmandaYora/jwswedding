-- Down of 000070_restrict_cities_to_jabodetabek_bali: intentional no-op.
--
-- The up migration NULLs city values outside the 23 Jabodetabek+Bali entries
-- (PLAN revisi-vendor-venue-portal §4.1/A7). Those original values are NOT
-- recoverable from the database itself — restore them only from the
-- pre-migration report + mysqldump taken per task A6. Rolling back the code
-- without that backup leaves the NULLs as-is, which is exactly what this
-- empty down does (rather than pretending to restore data it cannot).
SELECT 1;
