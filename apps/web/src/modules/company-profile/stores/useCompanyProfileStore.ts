import { create } from "zustand";
import { httpClient } from "@/shared/services/http-client";
import { API } from "@/shared/services/api-endpoints";
import { compressFileForUpload } from "@/shared/lib/image-compression";
import { useTenantBrandingStore } from "@/shared/stores/useTenantBrandingStore";
import type { CompanyProfileFormValues } from "@/modules/company-profile/schemas/company-profile.schema";

// Narrow projection of the full tenant record (GET/PATCH /tenants/me) — same
// "define exactly what this page reads/writes, not the whole Tenant"
// convention as useSubscriptionStore.MyTenant. PLAN.md invoice-kwitansi-client
// §1.7/§4.8, then redesain-pdf-invoice-kwitansi §D4/§D7 for hasSignature,
// then redesain-pdf-invoice-kwitansi-v2 §6.3.7 for profileComplete/
// missingProfileFields (the same gate GET /tenants/me/branding surfaces to
// useTenantBrandingStore — reused here to drive this page's own banner
// rather than re-deriving the rule client-side).
export interface CompanyProfile extends CompanyProfileFormValues {
  hasLogo: boolean;
  hasSignature: boolean;
  profileComplete: boolean;
  missingProfileFields: string[];
}

interface RawTenant extends CompanyProfileFormValues {
  hasLogo: boolean;
  hasSignature: boolean;
  profileComplete: boolean;
  missingProfileFields: string[];
}

function toProfile(raw: RawTenant): CompanyProfile {
  return { ...raw };
}

interface CompanyProfileState {
  profile: CompanyProfile | null;
  logoUrl: string | null;
  signatureUrl: string | null;

  fetchProfile: () => Promise<void>;
  updateProfile: (values: CompanyProfileFormValues) => Promise<void>;
  uploadLogo: (file: File) => Promise<void>;
  uploadSignature: (file: File) => Promise<void>;
}

// Backs CompanyProfilePage only — the WO Console's self-service "Profil
// Usaha" page. Fetch-then-set, no client cache (ADR-0009), same convention
// as useSubscriptionStore.
export const useCompanyProfileStore = create<CompanyProfileState>((set, get) => ({
  profile: null,
  logoUrl: null,
  signatureUrl: null,

  fetchProfile: async () => {
    const res = await httpClient.get(API.platform.tenantMe);
    const raw = res.data.data as RawTenant;
    const profile = toProfile(raw);

    let logoUrl: string | null = null;
    if (profile.hasLogo) {
      const fileRes = await httpClient.get(API.platform.tenantMeLogo, { responseType: "blob" });
      logoUrl = URL.createObjectURL(fileRes.data as Blob);
    }
    let signatureUrl: string | null = null;
    if (profile.hasSignature) {
      const fileRes = await httpClient.get(API.platform.tenantMeSignature, { responseType: "blob" });
      signatureUrl = URL.createObjectURL(fileRes.data as Blob);
    }
    const previousLogoUrl = get().logoUrl;
    const previousSignatureUrl = get().signatureUrl;
    set({ profile, logoUrl, signatureUrl });
    if (previousLogoUrl) URL.revokeObjectURL(previousLogoUrl);
    if (previousSignatureUrl) URL.revokeObjectURL(previousSignatureUrl);
  },

  updateProfile: async (values) => {
    await httpClient.patch(API.platform.tenantMe, values);
    // fetchProfile (this page's own data) and hydrate (the tenant-wide
    // branding store, so the "profileComplete" gate on Cetak PDF/Cetak
    // Kwitansi/Client Portal's "Unduh Kwitansi" unlocks immediately without
    // a page reload — PLAN.md redesain-pdf-invoice-kwitansi-v2 §6.3.6) are
    // two independent reads of the same just-saved tenant record, so they
    // run concurrently rather than one awaiting the other. Label MUST be
    // passed to hydrate() — it also rewrites the browser tab title's suffix
    // (AppLayout uses "WO Console"); omitting it would strip that suffix as
    // a side effect of saving a profile.
    await Promise.all([get().fetchProfile(), useTenantBrandingStore.getState().hydrate("WO Console")]);
  },

  // Uploads immediately on file selection (not bundled into the text-field
  // submit) — PATCH .../me and PUT .../me/logo are 2 separate endpoints, so
  // there's nothing to gain by batching them into one user action, and the
  // logo widget doubles as its own instant-feedback preview once saved.
  uploadLogo: async (file) => {
    const compressed = await compressFileForUpload(file);
    await httpClient.put(API.platform.tenantMeLogo, compressed);
    await get().fetchProfile();
  },

  // Mirrors uploadLogo exactly — PLAN.md redesain-pdf-invoice-kwitansi
  // §D4/§D7. compressFileForUpload passes PNG through its lossless
  // re-encode path (image-compression.ts), preserving alpha end to end.
  uploadSignature: async (file) => {
    const compressed = await compressFileForUpload(file);
    await httpClient.put(API.platform.tenantMeSignature, compressed);
    await get().fetchProfile();
  },
}));
