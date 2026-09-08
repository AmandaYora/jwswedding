-- Jam Acara project-level (PLAN.md revisi-putri-mom-25082026, Blok A / item 11):
-- 2 nullable TIME columns on the project itself -- preset (Sesi Pagi/Malam) or
-- custom, optional. Mengikuti pola 000041 (jam acara per vendor). Field ini
-- berdiri sendiri, tidak menggantikan project_vendors.event_start_time (D4).
ALTER TABLE projects
  ADD COLUMN event_start_time TIME NULL AFTER event_date,
  ADD COLUMN event_end_time   TIME NULL AFTER event_start_time;
