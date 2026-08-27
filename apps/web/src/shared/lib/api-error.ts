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
  if (axios.isAxiosError(err) && err.response?.data instanceof Blob) {
    try {
      const text = await err.response.data.text();
      const parsed = JSON.parse(text) as { message?: string };
      if (parsed.message) return parsed.message;
    } catch {
      // Body wasn't JSON (or wasn't readable) — fall through to the
      // fallback below rather than surfacing a parse error to the user.
    }
    return fallback;
  }
  return getApiErrorMessage(err, fallback);
}
