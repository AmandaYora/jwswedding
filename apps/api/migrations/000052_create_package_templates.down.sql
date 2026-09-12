-- Urutan terbalik dari .up.sql: anak lebih dulu, walau ON DELETE CASCADE
-- sudah menanganinya -- DROP eksplisit membuat down-migration tidak
-- bergantung pada perilaku cascade.
DROP TABLE package_template_terms;
DROP TABLE package_template_blocks;
DROP TABLE package_templates;
