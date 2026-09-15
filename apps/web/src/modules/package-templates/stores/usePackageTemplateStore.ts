import { create } from "zustand";
import { httpClient } from "@/shared/services/http-client";
import { API } from "@/shared/services/api-endpoints";
import type {
  PackageBlock,
  PackageTemplate,
  PackageTemplateSummary,
} from "@/modules/package-templates/types";

// The API numbers its ids; the rest of the frontend addresses entities by
// string. These Raw* shapes are the only place that difference is handled.
interface RawPackageTemplateSummary extends Omit<PackageTemplateSummary, "id"> {
  id: number;
}

interface RawPackageTemplate extends Omit<PackageTemplate, "id" | "blocks"> {
  id: number;
  blocks: (Omit<PackageBlock, "id"> & { id: number })[];
}

function toSummary(raw: RawPackageTemplateSummary): PackageTemplateSummary {
  return { ...raw, id: String(raw.id) };
}

function toPackageTemplate(raw: RawPackageTemplate): PackageTemplate {
  return {
    ...raw,
    id: String(raw.id),
    blocks: (raw.blocks ?? []).map((b) => ({ ...b, id: String(b.id) })),
  };
}

export interface PackageTemplateSubmitValues {
  name: string;
  basePrice: number;
  defaultTerms: string;
  defaultBonusNote: string;
  isActive: boolean;
}

/** Blocks are submitted whole, never row by row — the backend replaces the list. */
export type BlockDraft = Omit<PackageBlock, "id" | "sortOrder">;

interface PackageTemplateState {
  /**
   * Summary rows only. Whatever needs a template's blocks calls
   * `getTemplate` — by type there is nothing here to seed an editor from.
   */
  templates: PackageTemplateSummary[];
  loading: boolean;
  fetchTemplates: (activeOnly?: boolean) => Promise<void>;
  getTemplate: (id: string) => Promise<PackageTemplate>;
  createTemplate: (values: PackageTemplateSubmitValues) => Promise<void>;
  updateTemplate: (id: string, values: PackageTemplateSubmitValues) => Promise<void>;
  deleteTemplate: (id: string) => Promise<void>;
  saveBlocks: (id: string, blocks: BlockDraft[]) => Promise<PackageTemplate>;
}

// Tenant-scoped master data, Owner-only to write. Un-paginated on purpose: a
// WO sells a handful of package tiers, the same shape as Timeline Default.
export const usePackageTemplateStore = create<PackageTemplateState>((set, get) => ({
  templates: [],
  loading: false,

  fetchTemplates: async (activeOnly = false) => {
    set({ loading: true });
    try {
      const res = await httpClient.get(API.packageTemplates.base, {
        params: activeOnly ? { activeOnly: "true" } : undefined,
      });
      set({ templates: (res.data.data as RawPackageTemplateSummary[]).map(toSummary) });
    } finally {
      set({ loading: false });
    }
  },

  getTemplate: async (id) => {
    const res = await httpClient.get(API.packageTemplates.item(id));
    return toPackageTemplate(res.data.data as RawPackageTemplate);
  },

  createTemplate: async (values) => {
    await httpClient.post(API.packageTemplates.base, values);
    await get().fetchTemplates();
  },

  updateTemplate: async (id, values) => {
    await httpClient.patch(API.packageTemplates.item(id), values);
    await get().fetchTemplates();
  },

  deleteTemplate: async (id) => {
    await httpClient.delete(API.packageTemplates.item(id));
    await get().fetchTemplates();
  },

  // Both PUTs replace the whole child list and answer with the saved
  // template, so the caller can show exactly what was stored; the list is
  // refreshed too because its counts just changed.
  saveBlocks: async (id, blocks) => {
    const res = await httpClient.put(API.packageTemplates.blocks(id), blocks);
    await get().fetchTemplates();
    return toPackageTemplate(res.data.data as RawPackageTemplate);
  },
}));
