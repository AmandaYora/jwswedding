-- Fase 2 penawaran-client-master (T2.2): salin PO lama menjadi penawaran.
-- Setiap project_package_orders mendapat satu quotations padanan; statusnya
-- menjadi Diterima apapun status lamanya (project-nya memang sudah ada),
-- kecuali Dibatalkan yang tetap Dibatalkan. PO lama yang belum bernomor
-- tetap tanpa nomor. event_date/pax/venue_id disalin dari project, client_id
-- dari projects.client_id yang sudah di-backfill di Fase 1.
INSERT INTO quotations (
  tenant_id, client_id, po_number, number_period, number_seq, revision,
  status, base_price, terms_text, bonus_note, terms_plan_json,
  event_date, pax, venue_id, snapshot_json, issued_at, accepted_at,
  created_by_staff_id, legacy_project_id
)
SELECT
  p.tenant_id,
  COALESCE(p.client_id, 0),
  o.po_number, o.number_period, o.number_seq, o.revision,
  CASE WHEN o.status = 'Dibatalkan' THEN 'Dibatalkan' ELSE 'Diterima' END,
  o.base_price, o.terms_text, o.bonus_note, o.terms_plan_json,
  p.event_date, p.pax, p.venue_id, o.snapshot_json, o.issued_at,
  CASE WHEN o.status = 'Dibatalkan' THEN NULL ELSE o.issued_at END,
  o.created_by_staff_id, o.project_id
FROM project_package_orders o
JOIN projects p ON p.id = o.project_id;

INSERT INTO quotation_blocks (quotation_id, category, body, qty_text, bonus_note, sort_order)
SELECT q.id, b.category, b.body, b.qty_text, b.bonus_note, b.sort_order
FROM project_package_blocks b
JOIN quotations q ON q.legacy_project_id = b.project_id;

INSERT INTO quotation_adjustments (quotation_id, description, amount, sort_order)
SELECT q.id, a.description, a.amount, a.sort_order
FROM project_package_adjustments a
JOIN quotations q ON q.legacy_project_id = a.project_id;

UPDATE projects p
JOIN quotations q ON q.legacy_project_id = p.id
SET p.quotation_id = q.id;
