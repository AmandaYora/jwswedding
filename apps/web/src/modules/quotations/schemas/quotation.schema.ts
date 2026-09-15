import { z } from "zod";

export const QUOTATION_STATUS_OPTIONS = [
  "Draft",
  "Ditawarkan",
  "Diterima",
  "Ditolak",
  "Kedaluwarsa",
  "Dibatalkan",
] as const;

// Dialog Tambah Penawaran: client + template opsional + tanggal.
// Semua field ber-master dipilih lewat Select (D19); kategori blok paket
// tetap teks bebas (datalist), karena bukan master data.
export const quotationCreateSchema = z.object({
  clientId: z.string().min(1, "Pilih client"),
  templateId: z.string().default(""),
  eventDate: z.string().default(""),
  pax: z.coerce.number().min(0).default(0),
  venueId: z.string().default(""),
});

export type QuotationCreateFormValues = z.infer<typeof quotationCreateSchema>;

// Dialog Tambah Project (T3.7): dari penawaran Ditawarkan (pilih di daftar
// atau ketik nomor PO) + ringkasan read-only + Nama Project + Tanggal
// Booking + PIC (opsional) + Catatan (opsional). Field lain turunan.
export const acceptProjectSchema = z.object({
  quotationId: z.string().min(1, "Pilih penawaran"),
  projectName: z.string().min(3, "Nama project minimal 3 karakter"),
  prepStartDate: z.string().min(1, "Tanggal Booking wajib diisi"),
  picStaffId: z.string().default(""),
  notes: z.string().default(""),
});

export type AcceptProjectFormValues = z.infer<typeof acceptProjectSchema>;
