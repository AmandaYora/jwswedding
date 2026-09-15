export type ClientRole = "Bride" | "Groom" | "Family Representative";

// Master pasangan (satu baris per pasangan, lepas dari project).
export interface Client {
  id: string;
  brideName: string;
  groomName: string;
  displayName: string;
  phone: string;
  email: string;
  notes: string;
  contactCount: number;
  projectCount: number;
}

// Kontak/akun portal milik satu Client.
export interface ClientContact {
  id: string;
  clientId: string;
  role: ClientRole;
  username: string;
  relationNote: string;
  name: string;
  phone: string;
  email: string;
  isActive: boolean;
  lastCredentialResetAt: string | null;
}

// Specimen TTD tersimpan milik satu client (maksimum satu, D12): pratinjau
// + milik siapa + kapan + dari jalur mana. Gambarnya diambil terpisah.
export interface ClientSignature {
  role: ClientRole;
  signerName: string;
  source: "draw" | "upload";
  updatedAt: string;
}

// Bahan dialog konfirmasi hapus Client (D14) — dialog menolak tampil kalau
// ini gagal dibaca.
export interface ClientDeleteImpact {
  client: Client;
  contactCount: number;
  projectCount: number;
  projectNames: string[];
  paidInvoiceCount: number;
  paidInvoiceTotal: number;
  quotationCount: number;
  quotationNumbers: string[];
}
