// Penawaran sebagai PO pra-deal (PLAN penawaran-client-master, D6/D9).
// Satu dokumen dengan fase; judul cetak tetap "PURCHASE ORDER".

export type QuotationStatus =
  | "Draft"
  | "Ditawarkan"
  | "Diterima"
  | "Ditolak"
  | "Kedaluwarsa"
  | "Dibatalkan";

export interface QuotationBlock {
  id: string;
  category: string;
  body: string;
  qtyText: string;
  bonusNote: string;
  sortOrder: number;
}

export interface QuotationAdjustment {
  id: string;
  description: string;
  /** Bertanda: negatif adalah takeout/cashback. */
  amount: number;
  sortOrder: number;
}

export interface Quotation {
  id: string;
  clientId: string;
  clientName: string;
  /** "" sampai dikirim ke klien (D7), permanen setelahnya. */
  poNumber: string;
  revision: number;
  status: QuotationStatus;
  basePrice: number;
  /**
   * Nama komersial paket ("Silver") — inilah yang menjadi "Paket / Layanan"
   * di project dan baris "Paket" di PDF Invoice maupun PO.
   *
   * Disalin dari Template Paket saat penawaran dibuat, boleh diketik ulang
   * selama Draft, dan WAJIB terisi sebelum penawaran dikirim. "" hanya pada
   * penawaran lama yang lahir sebelum kolomnya ada.
   */
  packageName: string;
  termsText: string;
  bonusNote: string;
  eventDate: string | null;
  pax: number;
  venueId: string | null;
  venueName: string;
  issuedAt: string | null;
  acceptedAt: string | null;
  /** "0"/"" bila belum jadi project. */
  projectId: string;
  blocks: QuotationBlock[];
  adjustments: QuotationAdjustment[];
  totalAdjustments: number;
  total: number;
  /** null bila revisi yang berlaku belum diteken (T9, D13a). */
  signature: QuotationSignature | null;
}

export interface QuotationListItem {
  id: string;
  clientId: string;
  clientName: string;
  poNumber: string;
  revision: number;
  status: QuotationStatus;
  basePrice: number;
  /** basePrice + penyesuaian — nilai kontrak yang benar-benar dibuat saat diterima. */
  total: number;
  eventDate: string | null;
  projectId: string;
  /** ID pembuat penawaran — dasar filter Sales (D3). "0"/"" bila tak diketahui. */
  salesStaffId: string;
  /** ID Wedding Planner dari project hasil Accept — dasar tampilan nama WP di kartu (D7). "0"/"" bila belum ada project. */
  picStaffId: string;
  issuedAt: string | null;
  acceptedAt: string | null;
  /** Revisi yang berlaku sudah bertanda tangan (T9, D13a). */
  signed: boolean;
}

export interface QuotationDeleteImpact {
  quotation: Quotation;
  projectId: string;
  projectName: string;
  paidInvoiceCount: number;
  paidInvoiceTotal: number;
}

export interface AcceptProjectValues {
  projectName: string;
  prepStartDate: string;
  picStaffId: string;
  notes: string;
  /** TTD dari jalur pengelola (D10) — tidak ada bila revisi sudah berTTD. */
  signature?: AcceptSignatureInput;
}

// Pembubuhan TTD pada revisi yang berlaku (TTD Penawaran, §6.5).
export interface QuotationSignature {
  signerName: string;
  signerRole: string;
  signedAt: string;
  /** magic_link | upload | specimen. */
  channel: string;
}

// Satu pilihan "Atas Nama" — dari clients, bukan client_contacts (T1).
export interface SignerOption {
  role: string;
  name: string;
}

// Keterangan specimen tersimpan milik satu client (maksimum satu, D12).
export interface ClientSpecimen {
  role: string;
  signerName: string;
  /** draw | upload. */
  source: string;
  updatedAt: string;
}

export interface SignatureOptions {
  options: SignerOption[];
  specimen: ClientSpecimen | null;
}

// Satu dari dua cara pengelola membubuhkan TTD (D10): foto TTD dari klien
// (mimeType + base64Data) atau specimen tersimpan (useSpecimen — pemiliknya
// wajib sama dengan role, D12a).
export interface AcceptSignatureInput {
  role: string;
  signerName: string;
  mimeType?: string;
  base64Data?: string;
  useSpecimen?: boolean;
}
