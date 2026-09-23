import { useEffect, useRef } from "react";
import SignaturePadLib from "signature_pad";

// Pegangan kanvas TTD: hapus, periksa kosong, ekspor PNG transparan.
//
// Domain-agnostic, karena itu tinggal di shared/ui — dipakai halaman TTD publik
// milik klien maupun pengisian TTD pengguna internal (PLAN
// tanda-tangan-pengguna T21).
export interface SignaturePadHandle {
  clear: () => void;
  isEmpty: () => boolean;
  /** Data URL PNG — null bila kanvas masih kosong. */
  toPNGDataURL: () => string | null;
}

interface SignaturePadProps {
  handleRef: React.MutableRefObject<SignaturePadHandle | null>;
  /** Dipanggil setiap goresan selesai — induk memperbarui status tombol. */
  onChange?: (empty: boolean) => void;
}

export function SignaturePad({ handleRef, onChange }: SignaturePadProps) {
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const padRef = useRef<SignaturePadLib | null>(null);
  const onChangeRef = useRef(onChange);
  onChangeRef.current = onChange;

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;

    // Kanvas tajam di layar retina: resolusi mengikuti devicePixelRatio,
    // lalu diskalakan balik supaya koordinat goresan tetap 1:1.
    const ratio = Math.max(window.devicePixelRatio || 1, 1);
    const width = canvas.offsetWidth;
    const height = canvas.offsetHeight;
    canvas.width = width * ratio;
    canvas.height = height * ratio;
    const ctx = canvas.getContext("2d");
    if (ctx) ctx.scale(ratio, ratio);

    const pad = new SignaturePadLib(canvas, {
      penColor: "#1e293b",
      minWidth: 1,
      maxWidth: 2.5,
    });
    padRef.current = pad;

    const notify = () => onChangeRef.current?.(pad.isEmpty());
    pad.addEventListener("endStroke", notify);
    // clear() tidak memicu endStroke — beri tahu manual lewat pegangannya.

    handleRef.current = {
      clear: () => {
        pad.clear();
        onChangeRef.current?.(true);
      },
      isEmpty: () => pad.isEmpty(),
      toPNGDataURL: () => (pad.isEmpty() ? null : pad.toDataURL("image/png")),
    };

    return () => {
      pad.removeEventListener("endStroke", notify);
      pad.off();
      padRef.current = null;
      handleRef.current = null;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <canvas
      ref={canvasRef}
      className="h-44 w-full touch-none rounded-lg border border-dashed border-border bg-white"
      aria-label="Area tanda tangan"
    />
  );
}
