-- Mengoreksi tenants.plan_id agar menunjuk paket privat JWS di ElProof
-- (id=2, dipetakan ke app_id='app_02ef90c52704' di subscription_plan_apps),
-- bukan lagi paket publik/demo ElProof (id=1) yang jadi warisan migrasi data
-- lama (docs/plan/migrasi-data-jws/PLAN.md D9). HANYA plan_id yang disentuh --
-- subscription_status dan subscription_expires_at TIDAK diubah (R2): JWS
-- sudah pernah aktivasi sebagai tenant SaaS dan langganannya masih berjalan
-- (aktif sampai 2027-07-26), migrasi ini tidak boleh memicu pembayaran ulang.
UPDATE tenants SET plan_id = 2 WHERE id = 2;
