import { create } from "zustand";
import { httpClient } from "@/shared/services/http-client";
import { API } from "@/shared/services/api-endpoints";
import type { PackageBlock, PackageTemplate, PackageTemplateTerm } from "@/modules/package-templates/types";

interface RawPackageTemplate extends Omit<PackageTemplate, "id" | "blocks" | "terms"> {
  id: number;
  blocks: (Omit<PackageBlock, "id"> & { id: number })[];
  terms: (Omit<PackageTemplateTerm, "id"> & { id: number })[];
}

function toPackageTemplate(raw: RawPackageTemplate): PackageTemplate {
  return {
    ...raw,
    id: String(raw.id),
    blocks: (raw.blocks ?? []).map((b) => ({ ...b, id: String(b.id) })),
    terms: (raw.terms ?? []).map((t) => ({ ...t, id: String(t.id) })),
  };
}

export interface PackageTemplateSubmitValues {
  name: string;
  basePrice: number;
  defaultTerms: string;
  defaultBonusNote: string;
  isActive: boolean;
}

/** Blocks and terms are submitted whole, never row by row — the backend replaces the list. */
export type BlockDraft = Omit<PackageBlock, "id" | "sortOrder">;
export type TermDraft = Omit<PackageTemplateTerm, "id" | "sequence">;

interface PackageTemplateState {
  templates: PackageTemplate[];
  loading: boolean;
  fetchTemplates: (activeOnly?: boolean) => Promise<void>;
  getTemplate: (id: string) => Promise<PackageTemplate>;
  createTemplate: (values: PackageTemplateSubmitValues) => Promise<void>;
  updateTemplate: (id: string, values: PackageTemplateSubmitValues) => Promise<void>;
  deleteTemplate: (id: string) => Promise<void>;
  saveBlocks: (id: string, blocks: BlockDraft[]) => Promise<PackageTemplate>;
  saveTerms: (id: string, terms: TermDraft[]) => Promise<PackageTemplate>;
}

// Tenant-scoped master data, Owner/Admin to write. Un-paginated on purpose: a
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
      set({ templates: (res.data.data as RawPackageTemplate[]).map(toPackageTemplate) });
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

  saveBlocks: async (id, blocks) => {
    const res = await httpClient.put(API.packageTemplates.blocks(id), blocks);
    await get().fetchTemplates();
    return toPackageTemplate(res.data.data as RawPackageTemplate);
  },

  saveTerms: async (id, terms) => {
    const res = await httpClient.put(API.packageTemplates.terms(id), terms);
    await get().fetchTemplates();
    return toPackageTemplate(res.data.data as RawPackageTemplate);
  },
}));
