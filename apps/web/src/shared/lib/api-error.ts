import axios from "axios";

// Every backend error response body is { success: false, message, errors? }
// (.claude/rules/api-standard.md) — this extracts the human-readable message
// consistently wherever a page/store action needs to surface it.
export function getApiErrorMessage(err: unknown, fallback: string): string {
  if (axios.isAxiosError(err)) {
    const message = (err.response?.data as { message?: string } | undefined)?.message;
    if (message) return message;
  }
  return fallback;
}

// getApiErrorMessageFromBlob mirrors getApiErrorMessage exactly, except for
// requests made with `responseType: "blob"` (every PDF download in this
// app) — axios still hands back a Blob for the response body even on a
// non-2xx status, so err.response.data is a Blob, not the parsed JSON error
// object getApiErrorMessage expects (PLAN.md redesain-pdf-invoice-kwitansi-v2
// §6.3.5/T5). Falls back to getApiErrorMessage's own logic when the body
// isn't a Blob, so callers can use this unconditionally on any PDF-download
// catch block regardless of which branch actually produced the error.
export async function getApiErrorMessageFromBlob(err: unknown, fallback: string): Promise<string> {
  if (axios.isAxiosError(err)) {
    // 1. Server menjawab. Pesannya ada di body — Blob untuk responseType blob.
    if (err.response) {
      if (err.response.data instanceof Blob) {
        try {
          const text = await err.response.data.text();
          const parsed = JSON.parse(text) as { message?: string };
          if (parsed.message) return parsed.message;
        } catch {
          // Body bukan JSON (atau tidak terbaca) — pakai kode statusnya, yang
          // tetap memberi tahu ini penolakan server, bukan kegagalan lain.
        }
        return `${fallback} (server menjawab ${err.response.status})`;
      }
      return getApiErrorMessage(err, fallback);
    }

    // 2. Tidak ada respons sama sekali: permintaannya tidak pernah sampai atau
    // tidak pernah kembali. Ini BUKAN kegagalan membuat PDF, dan menyebutnya
    // begitu mengirim orang membongkar backend yang sebenarnya sehat —
    // penyebab lazimnya ekstensi browser yang memblokir unduhan, API yang
    // sedang mati, atau koneksi yang putus.
    if (err.code === "ECONNABORTED" || err.code === "ETIMEDOUT") {
      return "Server terlalu lama merespons. Coba lagi sebentar lagi.";
    }
    return "Permintaan tidak sampai ke server — periksa koneksi, pastikan API berjalan, dan nonaktifkan ekstensi pemblokir untuk halaman ini.";
  }

  // 3. Bukan galat HTTP sama sekali (mis. browser menolak membuka tab baru).
  // Pesannya disertakan apa adanya: menyembunyikannya di balik fallback persis
  // yang membuat kegagalan seperti ini mustahil didiagnosis.
  if (err instanceof Error && err.message) {
    return `${fallback}: ${err.message}`;
  }
  return fallback;
}
