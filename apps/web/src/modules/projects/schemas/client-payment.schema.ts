import { z } from "zod";
import { PAYMENT_METHOD_OPTIONS } from "@/modules/projects/schemas/payment.schema";

export const CLIENT_PAYMENT_TYPE_OPTIONS = ["DP", "Termin", "Pelunasan", "Tambahan", "Refund"] as const;

// Simpler than payment.schema.ts's paymentSchema — no projectVendorId (a
// client payment isn't tied to any vendor). proofFile is validated only
// client-side and never sent as part of the JSON body itself — the store
// action uploads it as a separate evidence call after the payment is
// created (see useProjectStore.createClientPayment).
export const clientPaymentSchema = z.object({
  // Tagihan yang dilunasi pembayaran ini — "" berarti uang masuk yang memang
  // tidak punya tagihan (DP sebelum tagihan terbit, pembayaran di luar
  // termin, Refund).
  //
  // Terisi berarti submit-nya BUKAN createClientPayment melainkan
  // markClientInvoicePaid: endpoint itulah yang selain mencatat uangnya juga
  // menautkan `client_payment_id` dan mengubah status Tagihan jadi Lunas.
  // Sekadar menyalin jenis/nominal dari Tagihan lalu tetap memanggil
  // createClientPayment akan mencatat uangnya sambil meninggalkan Tagihan
  // berstatus "Belum Dibayar" — dan seseorang akan melunasinya lagi nanti.
  //
  // `type`/`amount` di bawah tetap ada dan tetap divalidasi saat invoiceId
  // terisi: keduanya diisi dari Tagihan yang dipilih supaya form menampilkan
  // angka yang sebenarnya. Backend memaksanya sekali lagi dari Tagihan
  // (ClientInvoiceService.MarkPaid), jadi UI tidak pernah jadi otoritas.
  invoiceId: z.string().optional().default(""),
  type: z.enum(CLIENT_PAYMENT_TYPE_OPTIONS),
  amount: z.coerce.number().positive("Nominal harus lebih dari 0"),
  paymentDate: z.string().min(1, "Tanggal wajib diisi"),
  method: z.enum(PAYMENT_METHOD_OPTIONS),
  referenceNumber: z.string().optional().default(""),
  notes: z.string().optional().default(""),
  proofFile: z.instanceof(File).optional(),
});

export type ClientPaymentFormValues = z.infer<typeof clientPaymentSchema>;

// Edit scope is the ledger fields only — no file re-upload, see PLAN.md §2.5.
export const clientPaymentUpdateSchema = z.object({
  type: z.enum(CLIENT_PAYMENT_TYPE_OPTIONS),
  amount: z.coerce.number().positive("Nominal harus lebih dari 0"),
  paymentDate: z.string().min(1, "Tanggal wajib diisi"),
  method: z.enum(PAYMENT_METHOD_OPTIONS),
  referenceNumber: z.string().optional().default(""),
  notes: z.string().optional().default(""),
});

export type ClientPaymentUpdateFormValues = z.infer<typeof clientPaymentUpdateSchema>;
