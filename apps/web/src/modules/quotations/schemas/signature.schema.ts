import { z } from "zod";

// TTD Penawaran: validasi pilihan penanda tangan (D2). Kekosongan goresan
// diperiksa imperatif lewat SignaturePad.isEmpty(), bukan di sini — kanvas
// bukan nilai form.
export const signatureAcceptSchema = z.object({
  role: z.string().min(1, "Pilih atas nama siapa penawaran ini ditandatangani"),
  signerName: z.string().min(1, "Nama penanda tangan wajib diisi"),
});

export type SignatureAcceptFormValues = z.infer<typeof signatureAcceptSchema>;
