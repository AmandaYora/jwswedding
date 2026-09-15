import { create } from "zustand";
import { httpClient } from "@/shared/services/http-client";
import { API } from "@/shared/services/api-endpoints";
import { toQuotation, type RawQuotation } from "@/modules/quotations/stores/useQuotationStore";
import type { Quotation, SignerOption } from "@/modules/quotations/types";
import { getApiErrorMessage } from "@/shared/lib/api-error";

export type PublicSignatureStatus = "idle" | "loading" | "invalid" | "ready" | "accepted" | "rejected";

interface PublicSignatureState {
  status: PublicSignatureStatus;
  quotation: Quotation | null;
  options: SignerOption[];
  error: string | null;

  load: (token: string) => Promise<void>;
  accept: (token: string, role: string, pngDataUrl: string) => Promise<void>;
  reject: (token: string) => Promise<void>;
  reset: () => void;
}

function stripDataUrlPrefix(dataUrl: string): string {
  const comma = dataUrl.indexOf(",");
  return comma >= 0 ? dataUrl.slice(comma + 1) : dataUrl;
}

// Store halaman publik tanda tangan (tanpa token auth): satu modul-store
// sendiri mengikuti ADR-0009 — bukan bagian useQuotationStore yang beredar
// di sesi staff.
export const usePublicSignatureStore = create<PublicSignatureState>((set) => ({
  status: "idle",
  quotation: null,
  options: [],
  error: null,

  load: async (token) => {
    set({ status: "loading", error: null });
    try {
      const res = await httpClient.get(API.publicSignature.resolve(token));
      const data = res.data.data as { quotation: RawQuotation; options: SignerOption[] };
      set({ status: "ready", quotation: toQuotation(data.quotation), options: data.options ?? [] });
    } catch (err) {
      set({ status: "invalid", error: getApiErrorMessage(err, "Link tidak berlaku") });
    }
  },

  accept: async (token, role, pngDataUrl) => {
    set({ error: null });
    try {
      await httpClient.post(API.publicSignature.accept(token), {
        role,
        mimeType: "image/png",
        base64Data: stripDataUrlPrefix(pngDataUrl),
      });
      set({ status: "accepted" });
    } catch (err) {
      set({ error: getApiErrorMessage(err, "Gagal menyimpan tanda tangan") });
      throw err;
    }
  },

  reject: async (token) => {
    set({ error: null });
    try {
      await httpClient.post(API.publicSignature.reject(token));
      set({ status: "rejected" });
    } catch (err) {
      set({ error: getApiErrorMessage(err, "Gagal menolak penawaran") });
      throw err;
    }
  },

  reset: () => set({ status: "idle", quotation: null, options: [], error: null }),
}));
