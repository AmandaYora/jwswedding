import { create } from "zustand";
import { httpClient } from "@/shared/services/http-client";
import { API } from "@/shared/services/api-endpoints";
import type {
  PackageAdjustment,
  PackageBlock,
  PackageOrder,
  PackageOrderTerm,
} from "@/modules/package-templates/types";

interface RawPackageOrder extends Omit<PackageOrder, "blocks" | "adjustments" | "termsPlan"> {
  blocks: (Omit<PackageBlock, "id"> & { id: number })[];
  adjustments: (Omit<PackageAdjustment, "id"> & { id: number })[];
  termsPlan: (Omit<PackageOrderTerm, "id"> & { id?: number })[];
}

function toPackageOrder(raw: RawPackageOrder | null): PackageOrder | null {
  if (!raw) return null;
  return {
    ...raw,
    blocks: (raw.blocks ?? []).map((b) => ({ ...b, id: String(b.id) })),
    adjustments: (raw.adjustments ?? []).map((a) => ({ ...a, id: String(a.id) })),
    termsPlan: (raw.termsPlan ?? []).map((t, i) => ({ ...t, id: String(t.id ?? i) })),
  };
}

export type BlockInput = Pick<PackageBlock, "category" | "body" | "qtyText" | "bonusNote">;
export type AdjustmentInput = Pick<PackageAdjustment, "description" | "amount">;

interface PackageOrderState {
  /** null means the project has no PO yet — the empty state (D23), not an error. */
  order: PackageOrder | null;
  loading: boolean;
  fetchOrder: (projectId: string) => Promise<void>;
  applyTemplate: (projectId: string, templateId: string) => Promise<void>;
  startBlank: (projectId: string) => Promise<void>;
  saveHeader: (
    projectId: string,
    values: { basePrice: number; termsText: string; bonusNote: string },
  ) => Promise<void>;
  saveBlocks: (projectId: string, blocks: BlockInput[]) => Promise<void>;
  saveAdjustments: (projectId: string, adjustments: AdjustmentInput[]) => Promise<void>;
  issue: (projectId: string) => Promise<void>;
  revise: (projectId: string) => Promise<void>;
  cancel: (projectId: string) => Promise<void>;
  reset: () => void;
}

// Every mutation returns the whole PO, so the store simply replaces its state
// from the response instead of refetching — the backend already recomputed the
// totals and the schedule amounts it had to write anyway.
export const usePackageOrderStore = create<PackageOrderState>((set) => {
  const replace = (raw: RawPackageOrder | null) => set({ order: toPackageOrder(raw) });

  return {
    order: null,
    loading: false,

    fetchOrder: async (projectId) => {
      set({ loading: true });
      try {
        const res = await httpClient.get(API.projects.packageOrder(projectId));
        replace(res.data.data as RawPackageOrder | null);
      } finally {
        set({ loading: false });
      }
    },

    applyTemplate: async (projectId, templateId) => {
      const res = await httpClient.post(API.projects.packageOrderApplyTemplate(projectId), {
        templateId: Number(templateId),
      });
      replace(res.data.data as RawPackageOrder);
    },

    startBlank: async (projectId) => {
      const res = await httpClient.post(API.projects.packageOrderStartBlank(projectId));
      replace(res.data.data as RawPackageOrder);
    },

    saveHeader: async (projectId, values) => {
      const res = await httpClient.put(API.projects.packageOrderHeader(projectId), values);
      replace(res.data.data as RawPackageOrder);
    },

    saveBlocks: async (projectId, blocks) => {
      const res = await httpClient.put(API.projects.packageOrderBlocks(projectId), blocks);
      replace(res.data.data as RawPackageOrder);
    },

    saveAdjustments: async (projectId, adjustments) => {
      const res = await httpClient.put(API.projects.packageOrderAdjustments(projectId), adjustments);
      replace(res.data.data as RawPackageOrder);
    },

    issue: async (projectId) => {
      const res = await httpClient.post(API.projects.packageOrderIssue(projectId));
      replace(res.data.data as RawPackageOrder);
    },

    revise: async (projectId) => {
      const res = await httpClient.post(API.projects.packageOrderRevise(projectId));
      replace(res.data.data as RawPackageOrder);
    },

    cancel: async (projectId) => {
      const res = await httpClient.post(API.projects.packageOrderCancel(projectId));
      replace(res.data.data as RawPackageOrder);
    },

    reset: () => set({ order: null }),
  };
});
