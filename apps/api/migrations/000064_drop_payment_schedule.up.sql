-- Hapus total fitur "Tahap Pembayaran (Rencana)".
--
-- Alasannya bisnis, bukan teknis: klien mencicil dengan pola yang beragam dan
-- tidak bisa dipatok di muka, jadi menjadwalkan termin di penawaran hanya
-- melahirkan tagihan yang langsung meleset. Tagihan kini SELALU diterbitkan
-- manual dari tab Pembayaran project, saat nominal dan tanggalnya benar-benar
-- disepakati.
--
-- Yang ikut hilang: preset termin per Template Paket, kolom rencana termin
-- yang dibekukan di penawaran, blok "Tahap Pembayaran (Rencana)" di PDF PO,
-- dan penyemaian tagihan otomatis saat penawaran Diterima.
--
-- TIDAK ikut hilang: tabel `client_invoices` beserta seluruh alurnya (terbit,
-- Tandai Lunas, Kwitansi). Yang dihapus adalah RENCANA-nya, bukan tagihannya.
-- Tagihan yang sudah terlanjur disemai versi lama tetap utuh apa adanya.
--
-- `snapshot_json` sengaja tidak disentuh: dokumen penawaran yang sudah beku
-- masih menyimpan kunci "termsPlan" di dalamnya, dan encoding/json memang
-- mengabaikan kunci yang tidak punya field — jadi PO lama tetap terbaca tanpa
-- perlu menulis ulang riwayat yang sudah ditandatangani.
ALTER TABLE quotations DROP COLUMN terms_plan_json;

DROP TABLE package_template_terms;
