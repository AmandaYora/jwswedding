import { create } from "zustand";
import { httpClient } from "@/shared/services/http-client";
import { API } from "@/shared/services/api-endpoints";
import type {
  RundownArchiveStatus,
  RundownCoverPrefill,
  RundownDetail,
  RundownSummary,
  RundownTemplate,
} from "@/modules/rundowns/types";
import type { CreateRundownValues } from "@/modules/rundowns/schemas/rundown.schema";

interface PageMeta {
  page: number;
  limit: number;
  total: number;
  total_pages: number;
}

/** Payload satu PUT seksi — bentuknya sama untuk semua seksi; backend hanya
 *  membaca field yang relevan dengan seksi yang dituju. */
export type SectionPayload = Partial<{
  cover: RundownDetail["cover"];
  dataLainnya: RundownDetail["dataLainnya"];
  vendors: RundownDetail["vendors"];
  roles: RundownDetail["roles"];
  committees: RundownDetail["committees"];
  menuItems: RundownDetail["menuItems"];
  makeupRooms: RundownDetail["makeupRooms"];
  items: RundownDetail["itemsAkad"];
  layoutNotes: RundownDetail["layoutNotes"];
  photoGroups: RundownDetail["photoGroups"];
  vipGuests: RundownDetail["vipGuests"];
  playlist: RundownDetail["playlist"];
  playlistNotes: string;
}>;

/**
 * Batas waktu generate. Lebih longgar dari default axios (30 dtk) karena satu
 * konversi PDF boleh sampai 60 dtk ditambah antrean satu-per-satu di server
 * (generateBudget 110 dtk di rundown_handler.go).
 */
const GENERATE_TIMEOUT_MS = 150_000;

const EMPTY_TEMPLATE: RundownTemplate = {
  roles: [],
  committees: [],
  makeupRooms: [],
  itemsAkad: [],
  itemsResepsi: [],
  layoutNotes: [],
  updatedAt: null,
};

interface RundownState {
  list: RundownSummary[];
  listMeta: PageMeta | null;
  usedProjectIds: string[];
  detail: RundownDetail | null;
  template: RundownTemplate | null;

  fetchList: (page: number, search?: string) => Promise<void>;
  fetchUsedProjectIds: () => Promise<void>;
  fetchDetail: (id: string) => Promise<void>;
  create: (
    values: CreateRundownValues,
    vendors: RundownDetail["vendors"],
    useTemplate: boolean
  ) => Promise<RundownDetail>;
  saveSection: (id: string, section: string, payload: SectionPayload) => Promise<void>;
  uploadLayout: (id: string, base64Data: string) => Promise<void>;
  remove: (id: string) => Promise<void>;
  generate: (id: string, format: "docx" | "pdf") => Promise<{ archive: RundownArchiveStatus }>;
  fetchProjectPrefill: (id: string) => Promise<RundownCoverPrefill>;
  fetchTemplate: () => Promise<RundownTemplate>;
  saveTemplateSection: (section: string, payload: SectionPayload) => Promise<void>;
  saveAsTemplate: (id: string) => Promise<void>;
}

/**
 * Mengunduh Blob lalu melepas object URL-nya.
 *
 * Nama berkas diambil dari Content-Disposition kalau ada: server yang tahu
 * nama project terbaru, dan menyusunnya ulang di sini hanya akan menyimpang.
 */
function saveBlob(blob: Blob, disposition: string | undefined, fallback: string) {
  const match = disposition?.match(/filename="?([^"]+)"?/);
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = match?.[1] ?? fallback;
  document.body.appendChild(a);
  a.click();
  a.remove();
  // Pencabutan ditunda satu putaran event loop. Di Chrome unduhan memang sudah
  // dimulai secara sinkron saat click(), tetapi di sebagian versi Firefox dan
  // Safari mencabut object URL pada tik yang sama membatalkan unduhan yang
  // baru saja dimulai — dan gejalanya "kadang berkasnya tidak keluar", yang
  // nyaris mustahil ditelusuri.
  setTimeout(() => URL.revokeObjectURL(url), 0);
}

function toArchiveStatus(header: unknown): RundownArchiveStatus {
  return header === "shared" || header === "private" || header === "failed" ? header : "unknown";
}

export const useRundownStore = create<RundownState>((set, get) => ({
  list: [],
  listMeta: null,
  usedProjectIds: [],
  detail: null,
  template: null,

  fetchList: async (page, search) => {
    const res = await httpClient.get(API.rundowns.base, { params: { page, search } });
    set({ list: res.data.data as RundownSummary[], listMeta: res.data.meta as PageMeta });
  },

  fetchUsedProjectIds: async () => {
    const res = await httpClient.get(API.rundowns.usedProjectIds);
    const ids = (res.data.data ?? []) as number[];
    set({ usedProjectIds: ids.map(String) });
  },

  fetchDetail: async (id) => {
    const res = await httpClient.get(API.rundowns.item(id));
    set({ detail: res.data.data as RundownDetail });
  },

  create: async (values, vendors, useTemplate) => {
    const res = await httpClient.post(API.rundowns.base, {
      projectId: Number(values.projectId),
      woPicName: values.woPicName,
      woPicPhone: values.woPicPhone,
      eventTimeLabel: values.eventTimeLabel,
      vendors,
      useTemplate,
    });
    const detail = res.data.data as RundownDetail;
    set({ detail });
    return detail;
  },

  saveSection: async (id, section, payload) => {
    const res = await httpClient.put(API.rundowns.section(id, section), payload);
    set({ detail: res.data.data as RundownDetail });
  },

  uploadLayout: async (id, base64Data) => {
    const res = await httpClient.post(API.rundowns.layoutImage(id), { base64Data });
    set({ detail: res.data.data as RundownDetail });
  },

  remove: async (id) => {
    await httpClient.delete(API.rundowns.item(id));
    set({ detail: null });
    await get().fetchList(1);
  },

  generate: async (id, format) => {
    const res = await httpClient.get(API.rundowns.generate(id, format), {
      responseType: "blob",
      timeout: GENERATE_TIMEOUT_MS,
    });
    saveBlob(
      res.data as Blob,
      res.headers?.["content-disposition"] as string | undefined,
      `Rundown.${format}`
    );
    return { archive: toArchiveStatus(res.headers?.["x-rundown-archive"]) };
  },

  fetchProjectPrefill: async (id) => {
    const res = await httpClient.get(API.rundowns.projectPrefill(id));
    return (res.data.data as { cover: RundownCoverPrefill }).cover;
  },

  fetchTemplate: async () => {
    const res = await httpClient.get(API.rundowns.template);
    const template = { ...EMPTY_TEMPLATE, ...(res.data.data as RundownTemplate) };
    set({ template });
    return template;
  },

  saveTemplateSection: async (section, payload) => {
    const res = await httpClient.put(API.rundowns.templateSection(section), payload);
    set({ template: res.data.data as RundownTemplate });
  },

  saveAsTemplate: async (id) => {
    const res = await httpClient.post(API.rundowns.saveAsTemplate(id));
    set({ template: res.data.data as RundownTemplate });
  },
}));
