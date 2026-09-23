import { useEffect, useRef, useState } from "react";
import { Signature, Trash2, Upload } from "lucide-react";
import { Button } from "@/shared/components/ui/Button";
import { FieldLabel } from "@/shared/components/ui/Input";
import { SignaturePad, type SignaturePadHandle } from "@/shared/components/ui/SignaturePad";
import { compressFileForUpload, type CompressedFilePayload } from "@/shared/lib/image-compression";

export type SignaturePayload = CompressedFilePayload;

interface SignatureFieldProps {
  /** URL object pratinjau TTD yang SUDAH tersimpan — null bila belum ada. */
  existingImageUrl: string | null;
  /**
   * Dipanggil setiap kali masukan berubah. null berarti "tidak ada TTD baru
   * untuk dikirim" — induk lalu membiarkan TTD tersimpan apa adanya.
   */
  onChange: (payload: SignaturePayload | null) => void;
  /** Permintaan menghapus TTD yang sudah tersimpan. */
  onClear?: () => void;
  disabled?: boolean;
}

// Dua cara mengisi TTD, dan keduanya SALING MENIADAKAN: memilih salah satu
// mengosongkan yang lain, supaya tidak pernah ambigu mana yang akan dikirim
// (PLAN tanda-tangan-pengguna K8/T24).
type Mode = "draw" | "upload";

export function SignatureField({ existingImageUrl, onChange, onClear, disabled }: SignatureFieldProps) {
  const [mode, setMode] = useState<Mode>("draw");
  const [uploaded, setUploaded] = useState<SignaturePayload | null>(null);
  const [uploadBusy, setUploadBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const padRef = useRef<SignaturePadHandle | null>(null);
  const fileInputRef = useRef<HTMLInputElement | null>(null);

  // Induk memegang sumber kebenarannya; komponen ini hanya melapor.
  const onChangeRef = useRef(onChange);
  onChangeRef.current = onChange;

  useEffect(() => {
    // Berpindah mode selalu membuang masukan mode sebelumnya, termasuk
    // laporannya ke induk — kalau tidak, goresan yang sudah tak terlihat di
    // layar masih ikut terkirim.
    padRef.current?.clear();
    setUploaded(null);
    setError(null);
    onChangeRef.current(null);
  }, [mode]);

  function handleStrokeEnd(empty: boolean) {
    if (empty) {
      onChangeRef.current(null);
      return;
    }
    const dataUrl = padRef.current?.toPNGDataURL();
    if (!dataUrl) {
      onChangeRef.current(null);
      return;
    }
    // Buang awalan `data:image/png;base64,` — API menerima base64 mentah,
    // idiom yang sama dengan compressFileForUpload (ADR-0010).
    onChangeRef.current({
      fileName: "tanda-tangan.png",
      mimeType: "image/png",
      base64Data: dataUrl.slice(dataUrl.indexOf(",") + 1),
    });
  }

  async function handleFile(file: File | undefined) {
    if (!file) return;
    setError(null);
    if (!["image/png", "image/jpeg", "image/jpg"].includes(file.type)) {
      setError("Berkas harus gambar PNG atau JPG.");
      return;
    }
    setUploadBusy(true);
    try {
      const payload = await compressFileForUpload(file);
      setUploaded(payload);
      onChangeRef.current(payload);
    } catch {
      setError("Gagal membaca berkas gambar. Coba berkas lain.");
    } finally {
      setUploadBusy(false);
      // Reset supaya memilih berkas yang sama dua kali tetap memicu onChange.
      if (fileInputRef.current) fileInputRef.current.value = "";
    }
  }

  function handleClearCanvas() {
    padRef.current?.clear();
    onChangeRef.current(null);
  }

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <FieldLabel>Tanda Tangan</FieldLabel>
        <div className="flex rounded-lg border border-border p-0.5" role="tablist" aria-label="Cara mengisi tanda tangan">
          {(["draw", "upload"] as const).map((m) => (
            <button
              key={m}
              type="button"
              role="tab"
              aria-selected={mode === m}
              disabled={disabled}
              onClick={() => setMode(m)}
              className={`rounded-md px-3 py-1 text-[12.5px] font-medium transition ${
                mode === m ? "bg-primary text-white" : "text-text-secondary hover:text-text-primary"
              }`}
            >
              {m === "draw" ? "Gambar" : "Unggah"}
            </button>
          ))}
        </div>
      </div>

      {existingImageUrl && (
        <div className="flex items-center gap-3 rounded-lg border border-border bg-surface-muted px-3 py-2">
          <div className="h-12 w-28 shrink-0 overflow-hidden rounded bg-white">
            <img src={existingImageUrl} alt="Tanda tangan tersimpan" className="h-full w-full object-contain" />
          </div>
          <p className="flex-1 text-[12.5px] text-text-secondary">
            Tanda tangan tersimpan. Mengisi ulang di bawah akan menggantikannya.
          </p>
          {onClear && (
            <Button type="button" variant="ghost" size="sm" icon={<Trash2 className="h-3.5 w-3.5" />} disabled={disabled} onClick={onClear}>
              Hapus
            </Button>
          )}
        </div>
      )}

      {mode === "draw" ? (
        <div className="space-y-2">
          <SignaturePad handleRef={padRef} onChange={handleStrokeEnd} />
          <div className="flex items-center justify-between">
            <p className="text-[12px] text-text-secondary">Gambar tanda tangan di dalam kotak.</p>
            <Button type="button" variant="ghost" size="sm" disabled={disabled} onClick={handleClearCanvas}>
              Bersihkan
            </Button>
          </div>
        </div>
      ) : (
        <div className="flex items-center gap-3 rounded-lg border border-dashed border-border px-3 py-4">
          <Signature className="h-5 w-5 shrink-0 text-text-secondary" />
          <div className="flex-1 text-[12.5px] text-text-secondary">
            {uploaded ? <span className="font-medium text-text-primary">{uploaded.fileName}</span> : "PNG atau JPG, maksimal 2 MB."}
          </div>
          <input
            ref={fileInputRef}
            type="file"
            accept="image/png,image/jpeg"
            className="hidden"
            onChange={(e) => void handleFile(e.target.files?.[0])}
          />
          <Button
            type="button"
            variant="secondary"
            size="sm"
            icon={<Upload className="h-3.5 w-3.5" />}
            disabled={disabled || uploadBusy}
            onClick={() => fileInputRef.current?.click()}
          >
            {uploadBusy ? "Memproses…" : uploaded ? "Ganti" : "Pilih berkas"}
          </Button>
        </div>
      )}

      {error && (
        <p className="rounded-md border border-danger/30 bg-danger-soft px-3 py-2 text-[12.5px] font-medium text-danger">{error}</p>
      )}
    </div>
  );
}
