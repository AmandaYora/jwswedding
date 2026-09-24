import { useCallback, useEffect, useRef, useState } from "react";
import { Card, CardHeader, CardContent } from "@/shared/components/ui/Card";
import { Button } from "@/shared/components/ui/Button";
import { SignatureField, type SignaturePayload } from "@/modules/users/components/SignatureField";
import { useStaffStore } from "@/modules/users/stores/useStaffStore";
import { getApiErrorMessage } from "@/shared/lib/api-error";

// Panel "Tanda Tangan Saya" — editor TTD milik pemanggil sendiri. Dirender oleh
// UsersPage di rute /pengguna untuk Admin/Staff/Sales: mereka tidak punya akses
// ke manajemen pengguna (Owner-only di backend), tetapi tetap berhak mengurus
// TTD-nya sendiri, dan tempatnya di menu Pengguna — bukan halaman terpisah.
// Owner mengurus TTD-nya lewat modal edit baris dirinya di daftar pengguna.
// Semua panggilan di sini memakai jalur `me`, yang mengambil staffID dari
// klaim JWT, jadi panel ini tidak bisa menyentuh TTD orang lain.
export function MySignaturePanel() {
  const fetchMySignatureImageUrl = useStaffStore((s) => s.fetchMySignatureImageUrl);
  const saveMySignature = useStaffStore((s) => s.saveMySignature);
  const deleteMySignature = useStaffStore((s) => s.deleteMySignature);

  const [existingUrl, setExistingUrl] = useState<string | null>(null);
  const [pending, setPending] = useState<SignaturePayload | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  // Object URL yang sedang dipakai <img>, disimpan untuk di-revoke saat
  // diganti/dilepas — kalau tidak, setiap penyimpanan membocorkan satu blob.
  const urlRef = useRef<string | null>(null);

  const setPreview = useCallback((next: string | null) => {
    if (urlRef.current) URL.revokeObjectURL(urlRef.current);
    urlRef.current = next;
    setExistingUrl(next);
  }, []);

  const reload = useCallback(async () => {
    setPreview(await fetchMySignatureImageUrl());
  }, [fetchMySignatureImageUrl, setPreview]);

  useEffect(() => {
    void reload();
    return () => {
      if (urlRef.current) URL.revokeObjectURL(urlRef.current);
      urlRef.current = null;
    };
  }, [reload]);

  async function handleSave() {
    if (!pending) return;
    setBusy(true);
    setError(null);
    setNotice(null);
    try {
      await saveMySignature(pending);
      setPending(null);
      await reload();
      setNotice("Tanda tangan berhasil disimpan.");
    } catch (err) {
      setError(getApiErrorMessage(err, "Gagal menyimpan tanda tangan"));
    } finally {
      setBusy(false);
    }
  }

  async function handleDelete() {
    setBusy(true);
    setError(null);
    setNotice(null);
    try {
      await deleteMySignature();
      setPreview(null);
      setNotice("Tanda tangan dihapus.");
    } catch (err) {
      setError(getApiErrorMessage(err, "Gagal menghapus tanda tangan"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="w-full max-w-2xl">
      <Card>
        <CardHeader
          title="Tanda Tangan Saya"
          subtitle="Dipakai pada Penawaran, Invoice, dan Kwitansi yang Anda terbitkan sendiri."
        />
        <CardContent className="space-y-4">
          {error && (
            <p className="rounded-md border border-danger/30 bg-danger-soft px-3.5 py-2.5 text-[13px] font-medium text-danger">{error}</p>
          )}
          {notice && (
            <p className="rounded-md border border-success/30 bg-success-soft px-3.5 py-2.5 text-[13px] font-medium text-success">{notice}</p>
          )}

          <SignatureField
            existingImageUrl={existingUrl}
            onChange={setPending}
            onClear={existingUrl ? () => void handleDelete() : undefined}
            disabled={busy}
          />

          <p className="text-[12.5px] text-text-secondary">
            Dokumen yang sudah terbit tetap mencatat Anda sebagai penerbitnya. Mengganti tanda tangan di sini juga mengubah
            tampilannya saat dokumen lama dicetak ulang.
          </p>

          <div className="flex justify-end">
            <Button disabled={!pending || busy} onClick={() => void handleSave()}>
              {busy ? "Menyimpan…" : "Simpan Tanda Tangan"}
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
