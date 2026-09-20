-- Urutan terbalik dari .up.sql: anak lebih dulu, root terakhir.
-- rundown_makeup_lines punya FK ke rundown_makeup_rooms, jadi ia paling depan.
DROP TABLE IF EXISTS rundown_playlist;
DROP TABLE IF EXISTS rundown_vip_guests;
DROP TABLE IF EXISTS rundown_photo_groups;
DROP TABLE IF EXISTS rundown_layout_notes;
DROP TABLE IF EXISTS rundown_items;
DROP TABLE IF EXISTS rundown_makeup_lines;
DROP TABLE IF EXISTS rundown_makeup_rooms;
DROP TABLE IF EXISTS rundown_menu_items;
DROP TABLE IF EXISTS rundown_committees;
DROP TABLE IF EXISTS rundown_roles;
DROP TABLE IF EXISTS rundown_vendors;
DROP TABLE IF EXISTS rundowns;
