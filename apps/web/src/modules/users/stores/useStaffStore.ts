import { create } from "zustand";
import { httpClient } from "@/shared/services/http-client";
import { API } from "@/shared/services/api-endpoints";
import type { StaffMember, StaffSummary } from "@/modules/users/types";
import type { UserFormValues, UserCreateFormValues } from "@/modules/users/schemas/user.schema";
import type { SignaturePayload } from "@/modules/users/components/SignatureField";
import { toPaginationMeta, EMPTY_PAGINATION_META, type PaginationMeta, type RawPaginationMeta } from "@/shared/types/pagination";
import { toAffectedProjects, type AffectedProject, type RawDeleteImpact } from "@/shared/types/delete-impact";

interface RawStaffMember {
  id: number;
  name: string;
  title: string;
  initials: string;
  role: StaffMember["role"];
  username: string;
  email: string;
  phone: string;
  isActive: boolean;
  hasSignature: boolean;
}

function toStaffMember(raw: RawStaffMember): StaffMember {
  return { ...raw, id: String(raw.id) };
}

interface RawStaffSummary {
  id: number;
  name: string;
  title: string;
  role: StaffMember["role"];
}

function toStaffSummary(raw: RawStaffSummary): StaffSummary {
  return { id: String(raw.id), name: raw.name, title: raw.title, role: raw.role };
}

export interface CreateStaffResult {
  staff: StaffMember;
  username: string;
  password: string;
}

interface StaffState {
  staff: StaffMember[];
  staffPage: StaffMember[];
  staffPageMeta: PaginationMeta;
  staffSummaries: StaffSummary[];
  fetchStaff: () => Promise<void>;
  fetchStaffPage: (page: number, search: string, role: string) => Promise<void>;
  // Public-safe {id, name, title} list (any staff role) — what every PIC
  // picker/label across the `projects` module should use; `staff` above is
  // Owner-only now (Pengguna management, see staff_handler.go's
  // requireOwnerTenant).
  fetchStaffSummaries: () => Promise<void>;
  createStaff: (values: UserCreateFormValues) => Promise<CreateStaffResult>;
  updateStaff: (id: string, values: UserFormValues) => Promise<void>;
  toggleStaffActive: (id: string) => Promise<void>;
  // Hard delete (PLAN.md) — informed-consent, not a blocking precondition.
  // The tenant's Owner account is rejected server-side unconditionally
  // (403), never reaching a confirmation dialog.
  fetchStaffDeleteImpact: (id: string) => Promise<AffectedProject[]>;
  deleteStaff: (id: string) => Promise<void>;
  // --- TTD pengguna (PLAN tanda-tangan-pengguna) ---
  // Dua jalur: `id` (Owner-only, dari modal Pengguna) dan `me` (semua role,
  // dari halaman "Tanda Tangan Saya"). Keduanya sengaja terpisah alih-alih
  // satu fungsi ber-flag, supaya jalur Owner tidak pernah bisa terpanggil
  // tanpa sengaja dari layar self-service.
  saveStaffSignature: (id: string, payload: SignaturePayload) => Promise<void>;
  deleteStaffSignature: (id: string) => Promise<void>;
  fetchStaffSignatureImageUrl: (id: string) => Promise<string | null>;
  saveMySignature: (payload: SignaturePayload) => Promise<void>;
  deleteMySignature: () => Promise<void>;
  fetchMySignatureImageUrl: () => Promise<string | null>;
  fetchMySignatureExists: () => Promise<boolean>;
}

// Object URL pratinjau TTD. Pemanggil WAJIB URL.revokeObjectURL saat selesai —
// tanpa itu setiap pembukaan modal membocorkan satu blob.
async function fetchSignatureObjectUrl(url: string): Promise<string | null> {
  try {
    const res = await httpClient.get(url, { responseType: "blob" });
    return URL.createObjectURL(res.data as Blob);
  } catch {
    // 404 = belum punya TTD, bukan galat yang perlu dinaikkan ke UI.
    return null;
  }
}

// Backed by the real `staff` module (Fase 3) — tenant-scoped, fetch-then-set
// (ADR-0009).
export const useStaffStore = create<StaffState>((set, get) => ({
  staff: [],
  staffPage: [],
  staffPageMeta: EMPTY_PAGINATION_META,
  staffSummaries: [],

  fetchStaff: async () => {
    const res = await httpClient.get(API.staff.base, { params: { all: true } });
    set({ staff: (res.data.data as RawStaffMember[]).map(toStaffMember) });
  },

  fetchStaffSummaries: async () => {
    const res = await httpClient.get(API.staff.summary);
    set({ staffSummaries: (res.data.data as RawStaffSummary[]).map(toStaffSummary) });
  },

  // Backs UserListPage's table — real server-side pagination + search/role
  // filtering, separate from the `staff` full-roster cache above (which PIC
  // pickers elsewhere still rely on).
  fetchStaffPage: async (page, search, role) => {
    const res = await httpClient.get(API.staff.base, { params: { page, search: search || undefined, role: role || undefined } });
    set({
      staffPage: (res.data.data as RawStaffMember[]).map(toStaffMember),
      staffPageMeta: toPaginationMeta(res.data.meta as RawPaginationMeta),
    });
  },

  createStaff: async (values) => {
    const res = await httpClient.post(API.staff.base, values);
    const member = toStaffMember(res.data.data as RawStaffMember);
    await get().fetchStaff();
    return { staff: member, username: values.username, password: values.password };
  },

  updateStaff: async (id, values) => {
    await httpClient.patch(API.staff.item(id), values);
    await get().fetchStaff();
  },

  toggleStaffActive: async (id) => {
    await httpClient.post(API.staff.toggleActive(id));
    await get().fetchStaff();
  },

  fetchStaffDeleteImpact: async (id) => {
    const res = await httpClient.get(API.staff.deleteImpact(id));
    return toAffectedProjects(res.data.data as RawDeleteImpact);
  },

  deleteStaff: async (id) => {
    await httpClient.delete(API.staff.item(id));
    await get().fetchStaff();
  },

  saveStaffSignature: async (id, payload) => {
    await httpClient.put(API.staff.signature(id), payload);
    await get().fetchStaff();
  },

  deleteStaffSignature: async (id) => {
    await httpClient.delete(API.staff.signature(id));
    await get().fetchStaff();
  },

  fetchStaffSignatureImageUrl: (id) => fetchSignatureObjectUrl(API.staff.signatureImage(id)),

  saveMySignature: async (payload) => {
    await httpClient.put(API.staff.mySignature, payload);
  },

  deleteMySignature: async () => {
    await httpClient.delete(API.staff.mySignature);
  },

  fetchMySignatureImageUrl: () => fetchSignatureObjectUrl(API.staff.mySignatureImage),

  fetchMySignatureExists: async () => {
    const res = await httpClient.get(API.staff.mySignature);
    return Boolean((res.data.data as { hasSignature?: boolean } | null)?.hasSignature);
  },
}));
