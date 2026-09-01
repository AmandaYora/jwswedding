// Allowlist untuk atribut `accept` pada input unggah evidence — PLAN.md
// mom-25082026-item-belum item 7. Ini hanya penyaring dialog file browser,
// bukan penegakan; validasi otoritatif ada di backend
// (EvidenceService.Upload -> domain.IsAllowedUploadMimeType), yang daftarnya
// harus dicerminkan di sini. html/svg/js/executable sengaja tidak termasuk —
// lihat domain.IsAllowedUploadMimeType di evidence.go untuk alasannya
// (stored XSS lewat Content-Disposition: inline evidence).
export const UPLOAD_ACCEPT =
  "image/jpeg,image/png,image/gif,image/webp," +
  "application/pdf," +
  "application/msword,application/vnd.openxmlformats-officedocument.wordprocessingml.document," +
  "application/vnd.ms-excel,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet," +
  "application/vnd.ms-powerpoint,application/vnd.openxmlformats-officedocument.presentationml.presentation," +
  "text/plain,text/csv," +
  "application/zip,application/x-rar-compressed,application/x-7z-compressed";
