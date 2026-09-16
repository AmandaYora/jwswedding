import { create } from "zustand";
import { httpClient } from "@/shared/services/http-client";
import { API } from "@/shared/services/api-endpoints";
import type {
  AcceptProjectValues,
  ClientSpecimen,
  Quotation,
  QuotationAdjustment,
  QuotationBlock,
  QuotationDeleteImpact,
  QuotationListItem,
  QuotationSignature,
  QuotationStatus,
  SignatureOptions,
  SignerOption,
} from "@/modules/quotations/types";
import { toPaginationMeta, EMPTY_PAGINATION_META, type PaginationMeta, type RawPaginationMeta } from "@/shared/types/pagination";

interface RawQuotationBlock {
  id: number;
  category: string;
  body: string;
  qtyText: string;
  bonusNote: string;
  sortOrder: number;
}

interface RawQuotationAdjustment {
  id: number;
  description: string;
  amount: number;
  sortOrder: number;
}

interface RawQuotationSignature {
  signerName: string;
  signerRole: string;
  signedAt: string;
  channel: string;
}

function toSignature(raw: RawQuotationSignature | null | undefined): QuotationSignature | null {
  if (!raw) return null;
  return { signerName: raw.signerName, signerRole: raw.signerRole, signedAt: raw.signedAt, channel: raw.channel };
}

// Diekspor untuk halaman publik tanda tangan (tanpa login) — responsnya
// memakai bentuk dokumen yang sama.
export interface RawQuotation {
  id: number;
  clientId: number;
  clientName: string;
  poNumber: string;
  revision: number;
  status: QuotationStatus;
  basePrice: number;
  packageName: string;
  termsText: string;
  bonusNote: string;
  eventDate: string | null;
  pax: number;
  venueId: number | null;
  venueName: string;
  issuedAt: string | null;
  acceptedAt: string | null;
  projectId: number;
  blocks: RawQuotationBlock[];
  adjustments: RawQuotationAdjustment[];
  totalAdjustments: number;
  total: number;
  signature: RawQuotationSignature | null;
}

interface RawQuotationListItem {
  id: number;
  clientId: number;
  clientName: string;
  poNumber: string;
  revision: number;
  status: QuotationStatus;
  basePrice: number;
  total: number;
  eventDate: string | null;
  projectId: number;
  salesStaffId: number;
  picStaffId: number;
  issuedAt: string | null;
  acceptedAt: string | null;
  signed: boolean;
}

export function toQuotation(raw: RawQuotation): Quotation {
  return {
    id: String(raw.id),
    clientId: String(raw.clientId),
    clientName: raw.clientName,
    poNumber: raw.poNumber,
    revision: raw.revision,
    status: raw.status,
    basePrice: raw.basePrice,
    packageName: raw.packageName ?? "",
    termsText: raw.termsText,
    bonusNote: raw.bonusNote,
    eventDate: raw.eventDate,
    pax: raw.pax ?? 0,
    venueId: raw.venueId !== null ? String(raw.venueId) : null,
    venueName: raw.venueName,
    issuedAt: raw.issuedAt,
    acceptedAt: raw.acceptedAt,
    projectId: raw.projectId ? String(raw.projectId) : "",
    blocks: (raw.blocks ?? []).map((b) => ({ ...b, id: String(b.id) })),
    adjustments: (raw.adjustments ?? []).map((a) => ({ ...a, id: String(a.id) })),
    totalAdjustments: raw.totalAdjustments,
    total: raw.total,
    signature: toSignature(raw.signature),
  };
}

function toListItem(raw: RawQuotationListItem): QuotationListItem {
  return {
    id: String(raw.id),
    clientId: String(raw.clientId),
    clientName: raw.clientName,
    poNumber: raw.poNumber,
    revision: raw.revision,
    status: raw.status,
    basePrice: raw.basePrice,
    total: raw.total ?? raw.basePrice,
    eventDate: raw.eventDate,
    projectId: raw.projectId ? String(raw.projectId) : "",
    salesStaffId: raw.salesStaffId ? String(raw.salesStaffId) : "",
    picStaffId: raw.picStaffId ? String(raw.picStaffId) : "",
    issuedAt: raw.issuedAt,
    acceptedAt: raw.acceptedAt,
    signed: raw.signed ?? false,
  };
}

export type QuotationBlockInput = Pick<QuotationBlock, "category" | "body" | "qtyText" | "bonusNote">;
export type QuotationAdjustmentInput = Pick<QuotationAdjustment, "description" | "amount">;

export interface QuotationHeaderInput {
  basePrice: number;
  packageName: string;
  termsText: string;
  bonusNote: string;
  eventDate: string | null;
  pax: number;
  venueId: string | null;
  clientId: string;
}

export interface QuotationListFilters {
  status?: string;
  search?: string;
  clientId?: string;
  salesStaffId?: string;
  picStaffId?: string;
  /** Untuk pemilih (dropdown Tambah Project): ambil sampai 100 baris. */
  limit?: number;
}

interface QuotationState {
  quotationPage: QuotationListItem[];
  quotationPageMeta: PaginationMeta;
  currentQuotation: Quotation | null;
  loading: boolean;

  categories: string[];
  fetchCategories: () => Promise<void>;
  fetchQuotationPage: (page: number, filters: QuotationListFilters) => Promise<void>;
  fetchQuotation: (id: string) => Promise<void>;
  fetchAcceptCandidates: () => Promise<void>;
  createQuotation: (values: {
    clientId: string;
    templateId: string;
    packageName: string;
    eventDate: string;
    pax: number;
    venueId: string | null;
  }) => Promise<Quotation>;
  saveHeader: (id: string, values: QuotationHeaderInput) => Promise<void>;
  saveBlocks: (id: string, blocks: QuotationBlockInput[]) => Promise<void>;
  saveAdjustments: (id: string, adjustments: QuotationAdjustmentInput[]) => Promise<void>;
  issue: (id: string) => Promise<void>;
  withdraw: (id: string) => Promise<void>;
  revise: (id: string) => Promise<void>;
  accept: (id: string, values: AcceptProjectValues, venueSnapshot?: { rentalPrice: number | null; charge: number | null }) => Promise<string>;
  reject: (id: string) => Promise<void>;
  expire: (id: string) => Promise<void>;
  cancel: (id: string) => Promise<void>;
  duplicate: (id: string) => Promise<Quotation>;
  fetchDeleteImpact: (id: string) => Promise<QuotationDeleteImpact>;
  deleteQuotation: (id: string) => Promise<void>;
  downloadPdf: (id: string) => Promise<Blob>;
  createSignatureLink: (id: string) => Promise<{ token: string; expiresAt: string }>;
  fetchSignatureOptions: (id: string) => Promise<SignatureOptions>;
  reset: () => void;
}

// Setiap mutasi mengembalikan seluruh dokumen, jadi store mengganti state dari
// respons — backend sudah menghitung ulang total yang memang harus ditulis.
export const useQuotationStore = create<QuotationState>((set, get) => {
  const replace = (raw: RawQuotation) => set({ currentQuotation: toQuotation(raw) });

  return {
    quotationPage: [],
    quotationPageMeta: EMPTY_PAGINATION_META,
    currentQuotation: null,
    loading: false,
    categories: [],

    // Kategori yang sudah pernah dipakai tenant — datalist editor. Dimuat
    // sekali saat editor dibuka; kategori dokumen yang sedang disusun
    // digabungkan di komponen supaya yang baru diketik ikut muncul.
    fetchCategories: async () => {
      const res = await httpClient.get(API.quotations.categories);
      set({ categories: (res.data.data as string[]) ?? [] });
    },

    fetchQuotationPage: async (page, filters) => {
      const res = await httpClient.get(API.quotations.base, {
        params: {
          page,
          status: filters.status || undefined,
          search: filters.search || undefined,
          clientId: filters.clientId || undefined,
          salesStaffId: filters.salesStaffId || undefined,
          picStaffId: filters.picStaffId || undefined,
          limit: filters.limit,
        },
      });
      set({
        quotationPage: (res.data.data as RawQuotationListItem[]).map(toListItem),
        quotationPageMeta: toPaginationMeta(res.data.meta as RawPaginationMeta),
      });
    },

    fetchQuotation: async (id) => {
      set({ loading: true });
      try {
        const res = await httpClient.get(API.quotations.item(id));
        replace(res.data.data as RawQuotation);
      } finally {
        set({ loading: false });
      }
    },

    // Kandidat dialog Tambah Project: Ditawarkan + Diterima-yang-belum-jadi
    // project (T2). Dua permintaan digabung di sini — bukan dua
    // fetchQuotationPage berurutan, yang akan saling menimpa state halaman.
    fetchAcceptCandidates: async () => {
      const [offered, accepted] = await Promise.all([
        httpClient.get(API.quotations.base, { params: { page: 1, status: "Ditawarkan", limit: 100 } }),
        httpClient.get(API.quotations.base, { params: { page: 1, status: "Diterima", limit: 100 } }),
      ]);
      const items = [
        ...((offered.data.data as RawQuotationListItem[]) ?? []),
        ...((accepted.data.data as RawQuotationListItem[]) ?? []),
      ]
        .map(toListItem)
        .filter((item) => item.status === "Ditawarkan" || (item.status === "Diterima" && !item.projectId));
      set({ quotationPage: items });
    },

    createQuotation: async (values) => {
      const res = await httpClient.post(API.quotations.base, {
        clientId: Number(values.clientId),
        templateId: values.templateId ? Number(values.templateId) : 0,
        packageName: values.packageName,
        eventDate: values.eventDate || null,
        pax: values.pax,
        venueId: values.venueId ? Number(values.venueId) : null,
      });
      const quotation = toQuotation(res.data.data as RawQuotation);
      set({ currentQuotation: quotation });
      return quotation;
    },

    saveHeader: async (id, values) => {
      const res = await httpClient.patch(API.quotations.item(id), {
        basePrice: values.basePrice,
        packageName: values.packageName,
        termsText: values.termsText,
        bonusNote: values.bonusNote,
        eventDate: values.eventDate,
        pax: values.pax,
        venueId: values.venueId ? Number(values.venueId) : null,
        clientId: values.clientId ? Number(values.clientId) : 0,
      });
      replace(res.data.data as RawQuotation);
    },

    saveBlocks: async (id, blocks) => {
      const res = await httpClient.put(API.quotations.blocks(id), blocks);
      replace(res.data.data as RawQuotation);
    },

    saveAdjustments: async (id, adjustments) => {
      const res = await httpClient.put(API.quotations.adjustments(id), adjustments);
      replace(res.data.data as RawQuotation);
    },

    issue: async (id) => {
      const res = await httpClient.post(API.quotations.issue(id));
      replace(res.data.data as RawQuotation);
    },

    withdraw: async (id) => {
      const res = await httpClient.post(API.quotations.withdraw(id));
      replace(res.data.data as RawQuotation);
    },

    revise: async (id) => {
      const res = await httpClient.post(API.quotations.revise(id));
      replace(res.data.data as RawQuotation);
    },

    accept: async (id, values, venueSnapshot) => {
      const signature = values.signature
        ? {
            role: values.signature.role,
            signerName: values.signature.signerName,
            mimeType: values.signature.mimeType ?? null,
            base64Data: values.signature.base64Data ?? null,
            useSpecimen: values.signature.useSpecimen ?? false,
          }
        : null;
      const res = await httpClient.post(API.quotations.accept(id), {
        projectName: values.projectName,
        prepStartDate: values.prepStartDate,
        picStaffId: values.picStaffId ? Number(values.picStaffId) : 0,
        notes: values.notes,
        venueRentalPrice: venueSnapshot?.rentalPrice ?? null,
        venueCharge: venueSnapshot?.charge ?? null,
        signature,
      });
      const data = res.data.data as { quotation: RawQuotation; projectId: number };
      replace(data.quotation);
      // Project baru lahir — daftar project yang mungkin terbuka ikut segar
      // saat kembali ke sana (fetchProjectPage di halamannya sendiri).
      return String(data.projectId);
    },

    reject: async (id) => {
      const res = await httpClient.post(API.quotations.reject(id));
      replace(res.data.data as RawQuotation);
    },

    expire: async (id) => {
      const res = await httpClient.post(API.quotations.expire(id));
      replace(res.data.data as RawQuotation);
    },

    cancel: async (id) => {
      const res = await httpClient.post(API.quotations.cancel(id));
      replace(res.data.data as RawQuotation);
    },

    duplicate: async (id) => {
      const res = await httpClient.post(API.quotations.duplicate(id));
      const quotation = toQuotation(res.data.data as RawQuotation);
      set({ currentQuotation: quotation });
      return quotation;
    },

    fetchDeleteImpact: async (id) => {
      const res = await httpClient.get(API.quotations.deleteImpact(id));
      const raw = res.data.data as {
        quotation: RawQuotation;
        projectId: number;
        projectName: string;
        paidInvoiceCount: number;
        paidInvoiceTotal: number;
      };
      return {
        quotation: toQuotation(raw.quotation),
        projectId: raw.projectId ? String(raw.projectId) : "",
        projectName: raw.projectName,
        paidInvoiceCount: raw.paidInvoiceCount,
        paidInvoiceTotal: raw.paidInvoiceTotal,
      };
    },

    deleteQuotation: async (id) => {
      await httpClient.delete(API.quotations.item(id));
      if (get().currentQuotation?.id === id) set({ currentQuotation: null });
    },

    downloadPdf: async (id) => {
      const res = await httpClient.get(API.quotations.pdf(id), { responseType: "blob" });
      return res.data as Blob;
    },

    createSignatureLink: async (id) => {
      const res = await httpClient.post(API.quotations.signatureLink(id));
      const data = res.data.data as { token: string; expiresAt: string };
      return { token: data.token, expiresAt: data.expiresAt };
    },

    fetchSignatureOptions: async (id) => {
      const res = await httpClient.get(API.quotations.signatureOptions(id));
      const data = res.data.data as {
        options: SignerOption[];
        specimen: ClientSpecimen | null;
      };
      return { options: data.options ?? [], specimen: data.specimen ?? null };
    },

    reset: () => set({ currentQuotation: null }),
  };
});
