import { z } from "zod";

// Satu skema per seksi, tipe form selalu lewat z.infer (.claude/rules/
// frontend-react.md). Validasinya sengaja longgar: buku acara adalah dokumen
// kerja yang diisi bertahap, dan keputusan produk-nya adalah seksi yang belum
// lengkap tetap boleh dicetak sebagai kerangka. Yang dijaga hanya hal yang
// merusak dokumen atau tidak bisa diperbaiki belakangan.

const text = z.string().trim();

export const createRundownSchema = z.object({
  projectId: z.string().min(1, "Project wajib dipilih"),
  woPicName: text.max(100, "Maksimal 100 karakter").default(""),
  woPicPhone: text.max(100, "Maksimal 100 karakter").default(""),
  eventTimeLabel: text.max(255, "Maksimal 255 karakter").default(""),
});
export type CreateRundownValues = z.infer<typeof createRundownSchema>;

export const coverSchema = z.object({
  groomName: text.max(255).default(""),
  groomBirthOrder: text.max(255).default(""),
  groomParents: text.max(255).default(""),
  brideName: text.max(255).default(""),
  brideBirthOrder: text.max(255).default(""),
  brideParents: text.max(255).default(""),
  eventDateLabel: text.max(255).default(""),
  venueLabel: text.max(255).default(""),
  eventTimeLabel: text.max(255).default(""),
  coupleTitle: text.max(255).default(""),
  woPicName: text.max(100).default(""),
  woPicPhone: text.max(100).default(""),
});
export type CoverValues = z.infer<typeof coverSchema>;

export const vendorRowSchema = z.object({
  categoryLabel: text.max(100).default(""),
  vendorName: text.max(150).default(""),
});
export type VendorRowValues = z.infer<typeof vendorRowSchema>;

export const roleRowSchema = z.object({
  roleLabel: text.max(150).default(""),
  personName: text.max(150).default(""),
  note: text.max(255).default(""),
});
export type RoleRowValues = z.infer<typeof roleRowSchema>;

export const committeeRowSchema = z.object({
  roleLabel: text.max(150).default(""),
  personText: z.string().default(""),
  jobDesc: z.string().default(""),
});
export type CommitteeRowValues = z.infer<typeof committeeRowSchema>;

export const menuItemSchema = z.object({
  groupKey: z.enum(["Stall", "Buffet", "AfterAkad"]),
  style: z.enum(["Heading", "Numbered", "Bullet", "Plain"]),
  content: z.string().default(""),
});
export type MenuItemValues = z.infer<typeof menuItemSchema>;

export const dataLainnyaSchema = z.object({
  siblingsBride: z.string().default(""),
  siblingsGroom: z.string().default(""),
  souvenirNote: text.max(255).default(""),
  tableClothNote: text.max(255).default(""),
});
export type DataLainnyaValues = z.infer<typeof dataLainnyaSchema>;

export const makeupLineSchema = z.object({
  style: z.enum(["Heading", "Numbered", "Dash", "Note"]),
  content: z.string().default(""),
});

export const makeupRoomSchema = z.object({
  roomLabel: text.max(150).default(""),
  lines: z.array(makeupLineSchema).default([]),
});
export type MakeupRoomValues = z.infer<typeof makeupRoomSchema>;

export const acaraItemSchema = z.object({
  // Hanya "-" (tanpa nomor) atau kosong yang bermakna di sini; angkanya
  // ditetapkan server saat menyimpan, jadi apa pun yang dikirim klien akan
  // ditimpa.
  noLabel: z.string().default(""),
  timeLabel: text.max(50).default(""),
  item: z.string().default(""),
  pic: z.string().default(""),
  note: z.string().default(""),
});
export type AcaraItemValues = z.infer<typeof acaraItemSchema>;

export const layoutNoteSchema = z.object({
  kind: z.enum(["Legend", "Rule"]),
  numberLabel: z.string().default(""),
  content: z.string().default(""),
});
export type LayoutNoteValues = z.infer<typeof layoutNoteSchema>;

export const photoGroupSchema = z.object({ groupName: text.max(255).default("") });
export const vipGuestSchema = z.object({
  fullName: text.max(150).default(""),
  position: text.max(150).default(""),
});
export const playlistEntrySchema = z.object({
  title: text.max(255).default(""),
  artist: text.max(150).default(""),
});
export type PlaylistEntryValues = z.infer<typeof playlistEntrySchema>;

// Label tab, dipakai editor dan navigasi. Urutannya adalah urutan halaman di
// dokumen yang dihasilkan.
export const RUNDOWN_TABS = [
  { key: "cover", label: "Sampul" },
  { key: "vendors", label: "Vendor" },
  { key: "roles", label: "List Nama" },
  { key: "committees", label: "Panitia Keluarga" },
  { key: "data-lainnya", label: "Data Lainnya" },
  { key: "makeup", label: "Ruangan Makeup" },
  { key: "acara-akad", label: "Acara Akad" },
  { key: "acara-resepsi", label: "Acara Resepsi" },
  { key: "layout", label: "Layout Akad" },
  { key: "foto-tamu", label: "List Foto Tamu" },
  { key: "tamu-vip", label: "Tamu VIP" },
  { key: "playlist", label: "Playlist" },
] as const;
