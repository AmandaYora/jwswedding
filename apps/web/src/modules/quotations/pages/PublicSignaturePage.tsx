import { useEffect, useRef, useState } from "react";
import { useParams } from "react-router-dom";
import { Card, CardContent, CardHeader } from "@/shared/components/ui/Card";
import { Button } from "@/shared/components/ui/Button";
import { Select, Field } from "@/shared/components/ui/Input";
import { ConfirmDialog } from "@/shared/components/ui/ConfirmDialog";
import { SignaturePad, type SignaturePadHandle } from "@/shared/components/ui/SignaturePad";
import { usePublicSignatureStore } from "@/modules/quotations/stores/usePublicSignatureStore";
import { formatCurrency, formatDate } from "@/shared/lib/formatters";

// Halaman publik tanda tangan (jalur C): seluruh isi PURCHASE ORDER tanpa
// login, dengan Terima (Atas Nama + goresan TTD) dan Tolak. Tanpa sumber
// daya eksternal apa pun supaya token tidak bocor lewat Referer (§9).
export default function PublicSignaturePage() {
  const { token } = useParams<{ token: string }>();
  const status = usePublicSignatureStore((s) => s.status);
  const quotation = usePublicSignatureStore((s) => s.quotation);
  const options = usePublicSignatureStore((s) => s.options);
  const error = usePublicSignatureStore((s) => s.error);
  const load = usePublicSignatureStore((s) => s.load);
  const accept = usePublicSignatureStore((s) => s.accept);
  const reject = usePublicSignatureStore((s) => s.reject);
  const reset = usePublicSignatureStore((s) => s.reset);

  const [role, setRole] = useState("");
  const [padEmpty, setPadEmpty] = useState(true);
  const [busy, setBusy] = useState(false);
  const [confirmReject, setConfirmReject] = useState(false);
  const padHandle = useRef<SignaturePadHandle | null>(null);

  useEffect(() => {
    reset();
    // Ganti token = sesi baru: buang pilihan dan goresan sebelumnya (kanvas
    // tidak re-mount saat hanya param URL yang berubah).
    setRole("");
    setPadEmpty(true);
    padHandle.current?.clear();
    if (token) void load(token);
    return () => reset();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [token]);

  useEffect(() => {
    if (options.length > 0 && !role) setRole(options[0].role);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [options]);

  async function handleAccept() {
    if (!token || !role) return;
    const dataUrl = padHandle.current?.toPNGDataURL();
    if (!dataUrl) return;
    setBusy(true);
    try {
      await accept(token, role, dataUrl);
    } finally {
      setBusy(false);
    }
  }

  async function handleReject() {
    if (!token) return;
    setBusy(true);
    try {
      await reject(token);
      setConfirmReject(false);
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="mx-auto flex min-h-screen w-full max-w-3xl flex-col gap-4 bg-background px-4 py-8">
      <header className="text-center">
        <h1 className="text-xl font-bold text-text-primary">PURCHASE ORDER</h1>
        <p className="mt-1 text-[13px] text-text-secondary">Tanda tangani penawaran Anda di bawah ini.</p>
      </header>

      {status === "loading" && (
        <p className="py-16 text-center text-[13px] text-text-secondary">Memuat penawaran…</p>
      )}

      {(status === "invalid" || (!token && status !== "loading")) && (
        <Card>
          <CardContent className="py-10 text-center">
            <p className="text-[15px] font-semibold text-text-primary">Link tidak berlaku</p>
            <p className="mx-auto mt-2 max-w-md text-[13px] leading-relaxed text-text-secondary">
              Tautan ini salah, sudah dipakai, sudah kedaluwarsa (24 jam), atau sudah digantikan tautan yang lebih
              baru. Hubungi wedding organizer Anda untuk meminta tautan yang baru.
            </p>
          </CardContent>
        </Card>
      )}

      {status === "accepted" && quotation && (
        <Card>
          <CardContent className="py-10 text-center">
            <p className="text-[15px] font-semibold text-text-primary">
              {quotation.status === "Diterima" && quotation.projectId
                ? "Revisi PO telah ditandatangani"
                : "Terima kasih, penawaran telah disetujui"}
            </p>
            <p className="mx-auto mt-2 max-w-md text-[13px] leading-relaxed text-text-secondary">
              {quotation.status === "Diterima" && quotation.projectId
                ? "Perubahan pada penawaran ini sudah sah. Wedding organizer Anda akan menindaklanjutinya."
                : "Kesepakatan Anda sudah tercatat. Wedding organizer Anda akan segera menghubungi Anda."}
            </p>
          </CardContent>
        </Card>
      )}

      {status === "rejected" && (
        <Card>
          <CardContent className="py-10 text-center">
            <p className="text-[15px] font-semibold text-text-primary">Penawaran ditolak</p>
            <p className="mx-auto mt-2 max-w-md text-[13px] leading-relaxed text-text-secondary">
              Keputusan Anda sudah tercatat. Terima kasih atas waktunya.
            </p>
          </CardContent>
        </Card>
      )}

      {status === "ready" && quotation && (
        <>
          <Card>
              <CardHeader
                title={quotation.poNumber || "Penawaran"}
                subtitle={
                  quotation.revision > 0 ? `Revisi ${quotation.revision} · ${quotation.packageName}` : quotation.packageName
                }
              />
              <CardContent className="flex flex-col gap-3 text-[13px]">
                <div className="grid gap-1 sm:grid-cols-2">
                  <span className="text-text-secondary">Client</span>
                  <span className="font-medium text-text-primary">{quotation.clientName}</span>
                  <span className="text-text-secondary">Tanggal Acara</span>
                  <span className="font-medium text-text-primary">
                    {quotation.eventDate ? formatDate(quotation.eventDate) : "Menyusul"}
                  </span>
                  <span className="text-text-secondary">Tempat</span>
                  <span className="font-medium text-text-primary">{quotation.venueName || "—"}</span>
                  <span className="text-text-secondary">Jumlah Pax</span>
                  <span className="font-medium text-text-primary">{quotation.pax > 0 ? `${quotation.pax} Pax` : "—"}</span>
                </div>

                {quotation.blocks.length > 0 && (
                  <div className="overflow-x-auto">
                    <table className="w-full border-collapse text-[12.5px]">
                      <thead>
                        <tr className="bg-surface-muted text-left">
                          <th className="border border-border px-2 py-1.5 font-semibold">Kategori</th>
                          <th className="border border-border px-2 py-1.5 font-semibold">Produk</th>
                          <th className="border border-border px-2 py-1.5 font-semibold">Qty</th>
                          <th className="border border-border px-2 py-1.5 font-semibold">Bonus</th>
                        </tr>
                      </thead>
                      <tbody>
                        {quotation.blocks.map((b) => (
                          <tr key={b.id}>
                            <td className="border border-border px-2 py-1.5 align-top font-medium">{b.category}</td>
                            <td className="whitespace-pre-wrap border border-border px-2 py-1.5 align-top">{b.body}</td>
                            <td className="whitespace-pre-wrap border border-border px-2 py-1.5 align-top">{b.qtyText}</td>
                            <td className="whitespace-pre-wrap border border-border px-2 py-1.5 align-top">{b.bonusNote}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                )}

                {quotation.adjustments.length > 0 && (
                  <div className="flex flex-col gap-1">
                    {quotation.adjustments.map((a) => (
                      <div key={a.id} className="flex items-center justify-between gap-3">
                        <span className="text-text-secondary">{a.description}</span>
                        <span className="font-medium tabular-nums text-text-primary">{formatCurrency(a.amount)}</span>
                      </div>
                    ))}
                  </div>
                )}

                <div className="flex items-center justify-between border-t border-border pt-2">
                  <span className="font-semibold text-text-primary">Total Pembayaran</span>
                  <span className="text-[16px] font-bold tabular-nums text-text-primary">{formatCurrency(quotation.total)}</span>
                </div>

                {quotation.termsText.trim() && (
                  <div>
                    <p className="text-[12px] font-semibold uppercase tracking-wide text-text-secondary">Syarat & Ketentuan</p>
                    <pre className="mt-1 whitespace-pre-wrap break-words font-sans text-text-primary">{quotation.termsText}</pre>
                  </div>
                )}
                {quotation.bonusNote.trim() && (
                  <div>
                    <p className="text-[12px] font-semibold uppercase tracking-wide text-text-secondary">Bonus Tambahan</p>
                    <pre className="mt-1 whitespace-pre-wrap break-words font-sans text-text-primary">{quotation.bonusNote}</pre>
                  </div>
                )}
              </CardContent>
            </Card>

            <Card>
              <CardHeader title="Tanda tangan" subtitle="Pilih atas nama siapa, lalu bubuhkan tanda tangan di area gambar." />
              <CardContent className="flex flex-col gap-3">
                <Field label="Atas Nama" htmlFor="public-signer-role" required>
                  <Select value={role} onChange={(e) => setRole(e.target.value)}>
                    {options.map((o) => (
                      <option key={o.role} value={o.role}>
                        {o.name} ({o.role === "Bride" ? "Mempelai Wanita" : o.role === "Groom" ? "Mempelai Pria" : o.role})
                      </option>
                    ))}
                  </Select>
                </Field>

                <div>
                  <SignaturePad handleRef={padHandle} onChange={(empty) => setPadEmpty(empty)} />
                  <div className="mt-1.5 flex justify-end">
                    <button
                      type="button"
                      className="text-[12.5px] font-semibold text-text-secondary underline underline-offset-2"
                      onClick={() => padHandle.current?.clear()}
                    >
                      Hapus goresan
                    </button>
                  </div>
                </div>

                {error && <p className="text-[13px] text-danger">{error}</p>}

                <div className="flex flex-col gap-2 sm:flex-row sm:justify-end">
                  <Button variant="secondary" disabled={busy} onClick={() => setConfirmReject(true)}>
                    Tolak
                  </Button>
                  <Button disabled={busy || !role || padEmpty} onClick={() => void handleAccept()}>
                    {busy ? "Menyimpan…" : "Terima & Tanda Tangani"}
                  </Button>
                </div>
                {padEmpty && (
                  <p className="text-[12px] text-text-secondary">
                    Tombol Terima aktif setelah ada goresan di area tanda tangan — penawaran tidak bisa diterima tanpa
                    tanda tangan.
                  </p>
                )}
              </CardContent>
            </Card>

            <ConfirmDialog
              open={confirmReject}
              onClose={() => setConfirmReject(false)}
              onConfirm={() => void handleReject()}
              title="Tolak Penawaran"
              message="Yakin menolak penawaran ini?"
              details="Penawaran berstatus Ditolak dan tautan ini tidak bisa dipakai lagi."
              confirmLabel="Ya, Tolak"
              busyLabel="Menolak…"
              busy={busy}
              error={error}
            />
          </>
        )}
    </div>
  );
}
