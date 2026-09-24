import type { ZodIssue } from "zod";
import { sectionSchemaFor, type RundownTabKey } from "@/modules/rundowns/schemas/rundown.schema";

/**
 * Galat per sel, dikunci dengan path bertitik: "items.3.timeLabel",
 * "cover.brideName", "makeupRooms.0.lines.1.content". Bentuk yang sama untuk
 * galat dari Zod (sebelum kirim) dan dari server (422), supaya editor cukup
 * punya satu cara menandai sel.
 */
export type SectionErrors = Record<string, string>;

function messageFor(issue: ZodIssue): string {
  if (issue.code === "too_big" && issue.type === "string") return `Maksimal ${issue.maximum} karakter`;
  if (issue.code === "invalid_enum_value") return "Pilihan tidak dikenal";
  return issue.message;
}

/**
 * Memeriksa payload seksi dengan skema Zod-nya. Hasilnya HANYA untuk
 * validasi — payload yang dikirim tetap apa adanya (skema memangkas spasi,
 * dan itu keputusan server, bukan editor).
 */
export function validateSectionPayload(tab: RundownTabKey, payload: unknown): SectionErrors {
  const result = sectionSchemaFor(tab).safeParse(payload);
  if (result.success) return {};
  const errors: SectionErrors = {};
  for (const issue of result.error.issues) {
    const key = issue.path.join(".");
    if (!(key in errors)) errors[key] = messageFor(issue);
  }
  return errors;
}

/**
 * Menerjemahkan `errors` dari respons 422 server ("items[3].timeLabel":
 * ["Maksimal 50 karakter"]) ke bentuk SectionErrors.
 */
export function fieldErrorsFromServer(errors: unknown): SectionErrors {
  if (!errors || typeof errors !== "object") return {};
  const out: SectionErrors = {};
  for (const [key, value] of Object.entries(errors as Record<string, unknown>)) {
    const message = Array.isArray(value) ? String(value[0] ?? "") : String(value ?? "");
    out[key.replace(/\[(\d+)\]/g, ".$1")] = message;
  }
  return out;
}
