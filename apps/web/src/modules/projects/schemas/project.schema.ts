import { z } from "zod";

export const PROJECT_STATUS_OPTIONS = ["Draft", "Preparation", "Ready", "Completed", "Cancelled"] as const;

// EVENT_SESSION_PRESETS backs the project-level Jam Acara dropdown (Blok A,
// item 11). Labels/values follow the revision document's text (08.00–13.00 /
// 16.00–21.00), not the example numbers in gambar 1 (D5). "Belum ditentukan"
// (empty pair) is the default because the field is optional and legacy projects
// have no value. "Custom" is added as a separate option by the form itself.
export const EVENT_SESSION_PRESETS = [
  { label: "Belum ditentukan", start: "", end: "" },
  { label: "Sesi Pagi (08.00 - 13.00)", start: "08:00", end: "13:00" },
  { label: "Sesi Malam (16.00 - 21.00)", start: "16:00", end: "21:00" },
] as const;

export const projectSchema = z.object({
  name: z.string().min(3, "Nama project minimal 3 karakter"),
  brideName: z.string().min(2, "Nama mempelai wanita wajib diisi"),
  groomName: z.string().min(2, "Nama mempelai pria wajib diisi"),
  eventDate: z.string().min(1, "Tanggal acara wajib diisi"),
  // Project-level Jam Acara — "HH:MM" or "" ("Belum ditentukan"). Optional
  // (Blok A). The backend converts "" to NULL.
  eventStartTime: z.string().optional().default(""),
  eventEndTime: z.string().optional().default(""),
  venue: z.string().min(2, "Venue wajib diisi"),
  // Field name stays prepStartDate (display-text-only rename to "Tanggal
  // Booking" — see DOMAIN_GLOSSARY.md and PLAN.md).
  prepStartDate: z.string().min(1, "Tanggal booking wajib diisi"),
  packageName: z.string().min(2, "Paket/layanan wajib diisi"),
  contractValue: z.coerce.number().min(0, "Nilai kontrak tidak valid"),
  status: z.enum(PROJECT_STATUS_OPTIONS),
  // Optional now (PLAN.md revisi-timeline-vendor-role-sales) — a project
  // created by Sales may have no Wedding Planner assigned yet, until
  // handover.
  picStaffId: z.string().optional().default(""),
  // PIC Sales — "" means "belum ditugaskan" (0 on the wire), same sentinel
  // convention as picStaffId now uses.
  picSalesStaffId: z.string().optional().default(""),
  description: z.string().optional().default(""),
});

export type ProjectFormValues = z.infer<typeof projectSchema>;
