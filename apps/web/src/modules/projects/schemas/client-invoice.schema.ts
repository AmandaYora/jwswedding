import { z } from "zod";
import { PAYMENT_METHOD_OPTIONS } from "@/modules/projects/schemas/payment.schema";

// No "Refund" -- an Invoice is a bill, never a refund (rejected server-side
// too, see PLAN.md invoice-kwitansi-client §4.2).
export const CLIENT_INVOICE_TYPE_OPTIONS = ["DP", "Termin", "Pelunasan", "Tambahan"] as const;

export const clientInvoiceSchema = z.object({
  type: z.enum(CLIENT_INVOICE_TYPE_OPTIONS),
  description: z.string().optional().default(""),
  amount: z.coerce.number().positive("Nominal harus lebih dari 0"),
  dueDate: z.string().min(1, "Jatuh tempo wajib diisi"),
});

export type ClientInvoiceFormValues = z.infer<typeof clientInvoiceSchema>;

// "Tandai Lunas" -- no type/amount (both inherited from the Invoice itself,
// forced server-side regardless of what's sent), matching a normal
// ClientPayment's remaining fields.
export const markInvoicePaidSchema = z.object({
  paymentDate: z.string().min(1, "Tanggal wajib diisi"),
  method: z.enum(PAYMENT_METHOD_OPTIONS),
  referenceNumber: z.string().optional().default(""),
  notes: z.string().optional().default(""),
  proofFile: z.instanceof(File).optional(),
});

export type MarkInvoicePaidFormValues = z.infer<typeof markInvoicePaidSchema>;
