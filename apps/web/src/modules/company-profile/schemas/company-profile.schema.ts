import { z } from "zod";

// Backs the self-service "Profil Usaha" page (PATCH /tenants/me, Owner-only)
// -- PLAN.md invoice-kwitansi-client §1.7. Every field optional/blankable
// (all are TEXT/VARCHAR NULL-backed strings server-side, no field is
// required to save) except businessName/ownerName, which the tenant record
// already always carries a real value for.
export const companyProfileSchema = z.object({
  businessName: z.string().min(1, "Nama usaha wajib diisi"),
  ownerName: z.string().min(1, "Nama pemilik wajib diisi"),
  email: z.string().optional().default(""),
  phone: z.string().optional().default(""),
  city: z.string().optional().default(""),
  address: z.string().optional().default(""),
  bankName: z.string().optional().default(""),
  bankAccountNumber: z.string().optional().default(""),
  bankAccountHolderName: z.string().optional().default(""),
});

export type CompanyProfileFormValues = z.infer<typeof companyProfileSchema>;
