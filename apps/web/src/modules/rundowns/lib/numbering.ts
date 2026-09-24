import { NO_NUMBER_MARKER } from "@/modules/rundowns/types";

/**
 * Pratinjau nomor SUSUNAN ACARA seperti yang akan ditetapkan server saat
 * menyimpan — cermin renumberItems di rundown_service.go. Baris bertanda
 * tanpa-nomor mendapat "" dan tidak menaikkan hitungan.
 *
 * Hanya untuk tampilan: server tetap satu-satunya yang menetapkan nomor.
 */
export function previewNumbers(items: { noLabel: string }[]): string[] {
  let n = 0;
  return items.map((item) => {
    if (isNoNumber(item.noLabel)) return "";
    n += 1;
    return String(n);
  });
}

export function isNoNumber(noLabel: string): boolean {
  return noLabel.trim() === NO_NUMBER_MARKER;
}
