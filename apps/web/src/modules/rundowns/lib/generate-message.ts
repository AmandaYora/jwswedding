import type { RundownArchiveStatus } from "@/modules/rundowns/types";

export type GenerateFormat = "docx" | "pdf";

export interface GenerateMessage {
  tone: "success" | "danger";
  text: string;
  /** Berkas tersimpan di tab Dokumen project — tampilkan tautan ke sana. */
  documentsLink: boolean;
}

/**
 * Kalimat hasil unduhan rundown, dari status arsip di header X-Rundown-Archive.
 * Dipakai halaman detail dan halaman daftar supaya kalimatnya sama persis.
 */
export function generateMessage(format: GenerateFormat, archive: RundownArchiveStatus): GenerateMessage {
  const name = format.toUpperCase();
  switch (archive) {
    case "shared":
      return {
        tone: "success",
        text: `${name} terunduh dan tersimpan di Dokumen project. Klien tetap melihat versi terbaru ini.`,
        documentsLink: true,
      };
    case "private":
      return {
        tone: "success",
        text: `${name} terunduh dan tersimpan di Dokumen project (belum dibagikan ke klien).`,
        documentsLink: true,
      };
    case "failed":
      return {
        tone: "danger",
        text: `${name} terunduh, tetapi gagal disimpan ke Dokumen project. Coba unduh lagi.`,
        documentsLink: false,
      };
    default:
      return { tone: "success", text: `${name} terunduh.`, documentsLink: false };
  }
}
