// Bentuk data buku acara sebagaimana dikirim API. Cerminan langsung
// viewDTO/summaryDTO di apps/api/internal/modules/rundowns/presentation/dto.go
// -- kalau salah satunya berubah, ubah keduanya.

export type MenuGroup = "Stall" | "Buffet" | "AfterAkad";
export type MenuStyle = "Heading" | "Numbered" | "Bullet" | "Plain";
export type MakeupStyle = "Heading" | "Numbered" | "Dash" | "Note";
export type LayoutNoteKind = "Legend" | "Rule";

export interface RundownSummary {
  id: string;
  projectId: string;
  projectName: string;
  brideName: string;
  groomName: string;
  eventDateLabel: string;
  venueLabel: string;
  hasLayoutImage: boolean;
  updatedAt: string;
}

export interface RundownCover {
  groomName: string;
  groomBirthOrder: string;
  groomParents: string;
  brideName: string;
  brideBirthOrder: string;
  brideParents: string;
  eventDateLabel: string;
  venueLabel: string;
  eventTimeLabel: string;
  coupleTitle: string;
  woPicName: string;
  woPicPhone: string;
}

export interface RundownDataLainnya {
  siblingsBride: string;
  siblingsGroom: string;
  souvenirNote: string;
  tableClothNote: string;
}

export interface RundownVendor {
  categoryLabel: string;
  vendorName: string;
}

export interface RundownRole {
  roleLabel: string;
  personName: string;
  note: string;
}

export interface RundownCommittee {
  roleLabel: string;
  personText: string;
  jobDesc: string;
}

export interface RundownMenuItem {
  groupKey: MenuGroup;
  style: MenuStyle;
  content: string;
}

export interface RundownMakeupLine {
  style: MakeupStyle;
  content: string;
}

export interface RundownMakeupRoom {
  roomLabel: string;
  lines: RundownMakeupLine[];
}

/**
 * Satu baris SUSUNAN ACARA.
 *
 * `noLabel` TIDAK diketik WO: server menomori ulang tiap kali seksi disimpan.
 * Nilai "-" berarti "baris tanpa nomor" (mis. baris pembuka
 * "04.00 - 14.00 Checking Dekor"), dan server mengosongkannya.
 */
export interface RundownAcaraItem {
  noLabel: string;
  timeLabel: string;
  item: string;
  pic: string;
  note: string;
}

export interface RundownLayoutNote {
  kind: LayoutNoteKind;
  numberLabel: string;
  content: string;
}

export interface RundownPhotoGroup {
  groupName: string;
}

export interface RundownVIPGuest {
  fullName: string;
  position: string;
}

export interface RundownPlaylistEntry {
  title: string;
  artist: string;
}

export interface RundownDetail {
  id: string;
  projectId: string;
  projectName: string;
  cover: RundownCover;
  dataLainnya: RundownDataLainnya;
  vendors: RundownVendor[];
  roles: RundownRole[];
  committees: RundownCommittee[];
  menuItems: RundownMenuItem[];
  makeupRooms: RundownMakeupRoom[];
  itemsAkad: RundownAcaraItem[];
  itemsResepsi: RundownAcaraItem[];
  layoutNotes: RundownLayoutNote[];
  photoGroups: RundownPhotoGroup[];
  vipGuests: RundownVIPGuest[];
  playlist: RundownPlaylistEntry[];
  playlistNotes: string;
  hasLayoutImage: boolean;
  updatedAt: string;
}

export const NO_NUMBER_MARKER = "-";
