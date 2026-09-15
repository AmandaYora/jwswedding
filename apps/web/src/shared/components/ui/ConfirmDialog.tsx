import type { ReactNode } from "react";
import { AlertTriangle } from "lucide-react";
import { Modal } from "@/shared/components/ui/Modal";
import { Button } from "@/shared/components/ui/Button";

// Satu-satunya bentuk konfirmasi tindakan di aplikasi ini.
//
// Sebelumnya ada dua pola yang bersaing: sebagian tindakan memunculkan dialog,
// sebagian lagi menyisipkan pita merah di dalam kartu. Yang kedua bermasalah
// bukan karena kurang cantik — ia tidak memblokir apa pun, muncul di tempat
// yang bisa berada di luar layar saat kartunya panjang, dan tidak punya fokus
// maupun Escape. Akibatnya tindakan paling merusak di aplikasi ini (Hapus
// Permanen sebuah project) justru meminta persetujuan dengan cara paling
// lemah, sementara menghapus satu baris tagihan memakai dialog penuh.
//
// Komponen ini domain-agnostik (aturan shared UI): ia tidak tahu apa pun
// tentang project, tagihan, atau vendor — pemanggil yang menyediakan kalimat,
// rincian, dan aksinya.
export function ConfirmDialog({
  open,
  onClose,
  onConfirm,
  title,
  message,
  details,
  confirmLabel = "Ya, Lanjutkan",
  cancelLabel = "Batal",
  busyLabel,
  busy = false,
  error,
  tone = "danger",
}: {
  open: boolean;
  onClose: () => void;
  onConfirm: () => void;
  /** Nama tindakannya, bukan pertanyaannya. cth. "Hapus Project". */
  title: string;
  /** Pertanyaannya, satu kalimat. */
  message: ReactNode;
  /**
   * Konsekuensi yang perlu dibaca sebelum memutuskan — apa yang ikut hilang,
   * berapa nilainya. Dipisah dari `message` supaya pertanyaannya tetap satu
   * kalimat yang bisa dibaca sekilas, dan rinciannya tidak menyamar sebagai
   * bagian dari pertanyaan.
   */
  details?: ReactNode;
  confirmLabel?: string;
  cancelLabel?: string;
  /** Teks tombol selagi aksinya berjalan. cth. "Menghapus...". */
  busyLabel?: string;
  busy?: boolean;
  /** Galat dari percobaan terakhir; dialognya tetap terbuka agar terbaca. */
  error?: string | null;
  /**
   * "danger" untuk yang menghapus atau membatalkan; "default" untuk tindakan
   * berat yang tidak merusak. Tindakan merusak tidak boleh tampil setenang
   * tindakan biasa — itu satu-satunya isyarat yang dipunyai orang sebelum
   * menekan tombolnya.
   */
  tone?: "danger" | "default";
}) {
  return (
    <Modal
      open={open}
      onClose={onClose}
      title={title}
      size="sm"
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={busy}>
            {cancelLabel}
          </Button>
          <Button variant={tone === "danger" ? "danger" : "primary"} onClick={onConfirm} disabled={busy}>
            {busy && busyLabel ? busyLabel : confirmLabel}
          </Button>
        </>
      }
    >
      <div className="flex flex-col gap-3">
        {error && (
          <p className="rounded-md border border-danger/30 bg-danger-soft px-3.5 py-2.5 text-[13px] font-medium text-danger">
            {error}
          </p>
        )}
        <div className="flex items-start gap-3">
          {tone === "danger" && (
            <span className="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-danger-soft">
              <AlertTriangle className="h-4 w-4 text-danger" />
            </span>
          )}
          <div className="min-w-0 flex-1">
            <p className="text-[13.5px] leading-relaxed text-text-primary">{message}</p>
            {details && <div className="mt-2 text-[12.5px] leading-relaxed text-text-secondary">{details}</div>}
          </div>
        </div>
      </div>
    </Modal>
  );
}
