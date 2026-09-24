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
// dokumen yang dihasilkan. Labelnya sengaja sama dengan judul halaman di
// dokumen cetak — itu yang dikenali WO dari kertasnya; penjelasannya ada di
// RUNDOWN_TAB_HINTS.
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

export type RundownTabKey = (typeof RUNDOWN_TABS)[number]["key"];

// Satu kalimat per tab: apa yang diisi di sini, dengan kosakata WO.
export const RUNDOWN_TAB_HINTS: Record<RundownTabKey, string> = {
  cover: "Nama pengantin, orang tua, tanggal, jam, venue, dan kontak PIC WO di halaman depan.",
  vendors: "Daftar vendor per kategori. Bisa ditarik ulang dari vendor yang terpasang di project.",
  roles: "Petugas akad: wali nikah, saksi, penghulu, pembawa acara, dan peran lain beserta namanya.",
  committees: "Panitia dari pihak keluarga beserta kontak dan tugasnya.",
  "data-lainnya": "Saudara kandung pengantin, menu catering, souvenir, dan table cloth.",
  makeup: "Ruangan makeup dan siapa atau apa saja yang disiapkan di tiap ruangan.",
  "acara-akad": "Urutan acara akad dari persiapan sampai selesai, lengkap dengan jam dan PIC.",
  "acara-resepsi": "Urutan acara resepsi dari persiapan sampai selesai, lengkap dengan jam dan PIC.",
  layout: "Denah akad beserta keterangan nomornya dan aturan untuk tamu.",
  "foto-tamu": "Urutan grup yang dipanggil untuk foto bersama pengantin.",
  "tamu-vip": "Tamu VIP yang perlu disambut khusus, dengan jabatannya.",
  playlist: "Lagu yang diputar selama acara, dan catatan untuk tim musik.",
};

// Enam seksi Template Rundown (PLAN rundown-ux-ideal D2) — yang isinya memang
// sama dari satu acara ke acara lain. Urutannya mengikuti RUNDOWN_TABS.
export const TEMPLATE_TABS = RUNDOWN_TABS.filter((t) =>
  (["roles", "committees", "makeup", "acara-akad", "acara-resepsi", "layout"] as string[]).includes(t.key)
);

// Skema payload PUT per seksi — bentuknya sama dengan payloadFor di
// RundownEditor. Dipakai untuk menandai sel yang salah SEBELUM dikirim; server
// tetap memeriksa ulang dengan batas yang sama.
const SECTION_SCHEMAS: Record<RundownTabKey, z.ZodTypeAny> = {
  cover: z.object({ cover: coverSchema }),
  vendors: z.object({ vendors: z.array(vendorRowSchema) }),
  roles: z.object({ roles: z.array(roleRowSchema) }),
  committees: z.object({ committees: z.array(committeeRowSchema) }),
  "data-lainnya": z.object({ dataLainnya: dataLainnyaSchema, menuItems: z.array(menuItemSchema) }),
  makeup: z.object({ makeupRooms: z.array(makeupRoomSchema) }),
  "acara-akad": z.object({ items: z.array(acaraItemSchema) }),
  "acara-resepsi": z.object({ items: z.array(acaraItemSchema) }),
  layout: z.object({ layoutNotes: z.array(layoutNoteSchema) }),
  "foto-tamu": z.object({ photoGroups: z.array(photoGroupSchema) }),
  "tamu-vip": z.object({ vipGuests: z.array(vipGuestSchema) }),
  playlist: z.object({ playlist: z.array(playlistEntrySchema), playlistNotes: z.string() }),
};

export function sectionSchemaFor(tab: RundownTabKey): z.ZodTypeAny {
  return SECTION_SCHEMAS[tab];
}
