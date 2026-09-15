import { useEffect, useMemo, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { Modal } from "@/shared/components/ui/Modal";
import { Button } from "@/shared/components/ui/Button";
import { Input, Textarea, Select, Field } from "@/shared/components/ui/Input";
import { useQuotationStore } from "@/modules/quotations/stores/useQuotationStore";
import { useStaffStore } from "@/modules/users/stores/useStaffStore";
import { useVenueStore } from "@/modules/venues/stores/useVenueStore";
import type { ClientSpecimen, SignerOption } from "@/modules/quotations/types";
import { getApiErrorMessage } from "@/shared/lib/api-error";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { formatCurrency, formatDate } from "@/shared/lib/formatters";
import { httpClient } from "@/shared/services/http-client";
import { API } from "@/shared/services/api-endpoints";
import { compressFileForUpload } from "@/shared/lib/image-compression";

interface AcceptProjectDialogProps {
  /** Bila diisi, penawaran sudah dipilih (dari detail/editor). Bila kosong,
   * dialog menampilkan pemilih penawaran Ditawarkan + ketik nomor PO (T3.7). */
  quotationId?: string;
  /** signOnly (D13a): hanya membubuhkan TTD pada revisi yang sudah punya
   * project — field project disembunyikan, tombol menjadi "Simpan TTD". */
  signOnly?: boolean;
  onClose: () => void;
  onAccepted: (projectId: string) => void;
}

// Dialog Tambah Project (T3.7): pilih penawaran (Ditawarkan, atau Diterima
// yang belum jadi project — T2) → ringkasan read-only → blok TTD klien
// (D9/D10, disembunyikan bila sudah diteken lewat magic link) → Nama Project
// (prefill nama pasangan) + Tanggal Booking + PIC Wedding Planner (opsional)
// + Catatan (opsional). Semua field lain turunan (D16).
export function AcceptProjectDialog({ quotationId: presetId, signOnly, onClose, onAccepted }: AcceptProjectDialogProps) {
  const quotationPage = useQuotationStore((s) => s.quotationPage);
  const fetchAcceptCandidates = useQuotationStore((s) => s.fetchAcceptCandidates);
  const currentQuotation = useQuotationStore((s) => s.currentQuotation);
  const fetchQuotation = useQuotationStore((s) => s.fetchQuotation);
  const accept = useQuotationStore((s) => s.accept);
  const fetchSignatureOptions = useQuotationStore((s) => s.fetchSignatureOptions);
  const staffSummaries = useStaffStore((s) => s.staffSummaries);
  const fetchStaffSummaries = useStaffStore((s) => s.fetchStaffSummaries);
  const venues = useVenueStore((s) => s.venues);
  const fetchVenues = useVenueStore((s) => s.fetchVenues);

  const [selectedId, setSelectedId] = useState(presetId ?? "");
  const [poSearch, setPoSearch] = useState("");
  const [projectName, setProjectName] = useState("");
  const [prepStartDate, setPrepStartDate] = useState("");
  const [picStaffId, setPicStaffId] = useState("");
  const [notes, setNotes] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // --- Blok TTD (D9/D10/D12a) ---
  const [signerOptions, setSignerOptions] = useState<SignerOption[]>([]);
  const [specimen, setSpecimen] = useState<ClientSpecimen | null>(null);
  const [selectedRole, setSelectedRole] = useState("");
  const [uploaded, setUploaded] = useState<{ fileName: string; mimeType: string; base64Data: string } | null>(null);
  const [useSpecimen, setUseSpecimen] = useState(false);
  const [specimenPreview, setSpecimenPreview] = useState<string | null>(null);
  const [uploadBusy, setUploadBusy] = useState(false);
  const fileInputRef = useRef<HTMLInputElement | null>(null);

  useEffect(() => {
    // Dropdown butuh penawaran yang bisa menjadi project (T2); mode
    // signOnly tidak butuh dropdown sama sekali (presetId selalu ada).
    if (!presetId) void fetchAcceptCandidates();
    void fetchStaffSummaries();
    void fetchVenues();
  }, [presetId, fetchAcceptCandidates, fetchStaffSummaries, fetchVenues]);

  useEffect(() => {
    if (selectedId) void fetchQuotation(selectedId);
  }, [selectedId, fetchQuotation]);

  // Prefill nama project dari nama pasangan begitu penawaran termuat; ganti
  // pilihan = mulai lagi dari nama pasangan yang baru.
  useEffect(() => {
    if (currentQuotation && currentQuotation.id === selectedId) {
      setProjectName(currentQuotation.clientName || "");
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [currentQuotation?.id, selectedId]);

  const summary = currentQuotation && currentQuotation.id === selectedId ? currentQuotation : null;
  // Sudah diteken = blok TTD disembunyikan, Accept langsung ke pembuatan
  // project (TTD dari jalur mana pun sudah mendarat di snapshot).
  const alreadySigned = !!summary?.signature;

  // Opsi Atas Nama + specimen dimuat begitu ringkasan termuat dan belum
  // diteken. Ganti penawaran = mulai lagi dari awal (tanpa carry-over).
  useEffect(() => {
    setSignerOptions([]);
    setSpecimen(null);
    setSelectedRole("");
    setUploaded(null);
    setUseSpecimen(false);
    if (!summary || alreadySigned) return;
    let cancelled = false;
    void fetchSignatureOptions(summary.id)
      .then((res) => {
        if (cancelled) return;
        setSignerOptions(res.options);
        setSpecimen(res.specimen);
        if (res.options.length > 0) setSelectedRole(res.options[0].role);
      })
      .catch(() => {
        if (!cancelled) setSignerOptions([]);
      });
    return () => {
      cancelled = true;
    };
  }, [summary?.id, alreadySigned, fetchSignatureOptions]);

  // Pratinjau specimen hanya bila pemiliknya SAMA dengan Atas Nama terpilih
  // (D12a) dan tidak sedang memakai unggahan.
  const specimenUsable = !!specimen && !!selectedRole && specimen.role === selectedRole && !uploaded;
  useEffect(() => {
    setUseSpecimen(false);
    if (!summary || !specimenUsable) {
      setSpecimenPreview(null);
      return;
    }
    let cancelled = false;
    let objectUrl: string | null = null;
    void httpClient
      .get(API.clients.signatureImage(summary.clientId), { responseType: "blob" })
      .then((res) => {
        if (cancelled) return;
        objectUrl = URL.createObjectURL(res.data as Blob);
        setSpecimenPreview(objectUrl);
      })
      .catch(() => {
        if (!cancelled) setSpecimenPreview(null);
      });
    return () => {
      cancelled = true;
      if (objectUrl) URL.revokeObjectURL(objectUrl);
      setSpecimenPreview(null);
    };
  }, [summary, specimenUsable]);

  async function handleFileChange(file: File | undefined) {
    if (!file) return;
    setUploadBusy(true);
    setError(null);
    try {
      const payload = await compressFileForUpload(file);
      setUploaded({ fileName: payload.fileName, mimeType: payload.mimeType, base64Data: payload.base64Data });
      setUseSpecimen(false);
    } catch {
      setError("Gagal membaca berkas gambar");
    } finally {
      setUploadBusy(false);
    }
  }

  const searched = useMemo(() => {
    const q = poSearch.trim().toLowerCase();
    const acceptCandidates = quotationPage.filter(
      (item) => item.status === "Ditawarkan" || (item.status === "Diterima" && !item.projectId)
    );
    if (!q) return acceptCandidates;
    return acceptCandidates.filter((item) => item.poNumber.toLowerCase().includes(q));
  }, [quotationPage, poSearch]);

  const venue = venues.find((v) => summary && summary.venueId !== null && v.id === summary.venueId);
  const selectedOption = signerOptions.find((o) => o.role === selectedRole);

  async function handleAccept() {
    if (!selectedId || !summary) return;
    setBusy(true);
    setError(null);
    try {
      const projectId = await accept(
        selectedId,
        {
          projectName: projectName.trim(),
          prepStartDate,
          picStaffId,
          notes: notes.trim(),
          // Bila revisi sudah berTTD (magic link), TTD tidak dikirim lagi —
          // gerbang Accept memeriksanya di snapshot (D9).
          signature: alreadySigned
            ? undefined
            : useSpecimen
              ? { role: selectedRole, signerName: selectedOption?.name ?? "", useSpecimen: true }
              : uploaded
                ? {
                    role: selectedRole,
                    signerName: selectedOption?.name ?? "",
                    mimeType: uploaded.mimeType,
                    base64Data: uploaded.base64Data,
                  }
                : undefined,
        },
        venue ? { rentalPrice: venue.rentalPrice, charge: venue.charge } : undefined
      );
      onAccepted(projectId);
    } catch (err) {
      setError(getApiErrorMessage(err, "Gagal menerima penawaran"));
    } finally {
      setBusy(false);
    }
  }

  // Syarat yang tidak bisa dipenuhi dari dialog ini. Keduanya sudah lama
  // ditegakkan backend saat Terima, dan sejak penawaran hanya bisa dikirim
  // setelah keduanya terisi, ini praktis hanya menjaring penawaran lama —
  // tetapi justru penawaran lama itulah yang dulu berakhir buntu.
  const blockingReason = !summary
    ? null
    : !summary.eventDate
      ? "Penawaran ini belum punya Tanggal Acara, sehingga belum bisa menjadi project."
      : summary.total <= 0
        ? "Total penawaran ini masih nol, sehingga belum bisa menjadi project."
        : null;

  const acceptAllowedStatus =
    !!summary &&
    (summary.status === "Ditawarkan" ||
      (summary.status === "Diterima" && !summary.projectId) ||
      (!!signOnly && summary.status === "Diterima" && !!summary.projectId));

  // TTD WAJIB (D9): tombol mati sampai TTD ada — kecuali revisi ini memang
  // sudah diteken klien lewat magic link.
  const signatureMissing =
    !!summary && !alreadySigned && !blockingReason && acceptAllowedStatus && !uploaded && !useSpecimen;
  const signatureReason = signatureMissing
    ? "Penawaran ini belum ditandatangani klien. Unggah foto TTD-nya, pakai TTD tersimpan, atau kirim link tanda tangan."
    : null;

  const canSubmit = signOnly
    ? !!selectedId && !!summary && acceptAllowedStatus && !blockingReason && !signatureMissing && !busy
    : !!selectedId &&
      !!summary &&
      acceptAllowedStatus &&
      !blockingReason &&
      !signatureMissing &&
      projectName.trim().length >= 3 &&
      !!prepStartDate &&
      !busy;

  const dialogTitle = signOnly ? "Tanda Tangani Revisi" : "Tambah Project dari Penawaran";
  const submitLabel = busy ? (signOnly ? "Menyimpan..." : "Membuat...") : signOnly ? "Simpan Tanda Tangan" : "Terima & Buat Project";

  return (
    <Modal open onClose={onClose} title={dialogTitle} size="lg">
      <div className="flex flex-col gap-4">
        {!presetId && (
          <div className="grid gap-3 sm:grid-cols-2">
            <Field label="Penawaran" htmlFor="accept-quotation" required hint="Ditawarkan, atau Diterima yang belum jadi project.">
              <Select value={selectedId} onChange={(e) => setSelectedId(e.target.value)}>
                <option value="">Pilih penawaran</option>
                {searched.map((item) => (
                  <option key={item.id} value={item.id}>
                    {item.poNumber} — {item.clientName} ({formatCurrency(item.total)})
                  </option>
                ))}
              </Select>
            </Field>
            <Field label="Atau ketik nomor PO" htmlFor="accept-po-search" hint="Menyaring daftar di samping.">
              <Input
                id="accept-po-search"
                value={poSearch}
                placeholder="PO/202609/0001"
                onChange={(e) => setPoSearch(e.target.value)}
              />
            </Field>
          </div>
        )}

        {summary && (
          <div className="rounded-lg border border-border bg-surface px-4 py-3 text-[13px]">
            <div className="grid gap-1 sm:grid-cols-2">
              <span className="text-text-secondary">Nomor PO</span>
              <span className="font-semibold text-text-primary">{summary.poNumber}</span>
              <span className="text-text-secondary">Client</span>
              <span className="font-medium text-text-primary">{summary.clientName}</span>
              <span className="text-text-secondary">Tanggal Acara</span>
              <span className={summary.eventDate ? "font-medium text-text-primary" : "font-semibold text-danger"}>
                {summary.eventDate ? formatDate(summary.eventDate) : "Belum ditentukan"}
              </span>
              <span className="text-text-secondary">Total Penawaran</span>
              <span className="font-semibold tabular-nums text-text-primary">{formatCurrency(summary.total)}</span>
            </div>
          </div>
        )}

        {/* Tanggal Acara TIDAK punya kotak isian di sini, dan memang tidak
            boleh punya: ia milik penawaran, bukan project (D16). Dulu dialog
            ini tetap mengizinkan Terima ditekan, lalu backend menjawab
            "Tanggal acara wajib diisi" — pesan yang menyuruh mengisi sesuatu
            yang tidak ada kotaknya di layar ini. Sekarang penghalangnya
            disebutkan di muka, lengkap dengan cara melewatinya. */}
        {blockingReason && (
          <p className="rounded-md border border-danger/30 bg-danger-soft px-3.5 py-2.5 text-[13px] leading-relaxed text-danger">
            {blockingReason}{" "}
            {summary && (
              <Link
                to={ROUTE_PATHS.quotationDetail(summary.id)}
                className="font-semibold underline underline-offset-2"
                onClick={onClose}
              >
                Buka penawarannya
              </Link>
            )}
            , tarik kembali ke Draft, lengkapi, lalu kirim ulang.
          </p>
        )}

        {/* --- Blok TTD klien (D9/D10). Disembunyikan bila revisi ini sudah
            diteken — ditandai agar pengelola tahu dari mana TTD-nya. */}
        {summary && !blockingReason && acceptAllowedStatus && (
          alreadySigned && summary.signature ? (
            <p className="rounded-md border border-success/30 bg-success-soft px-3.5 py-2.5 text-[13px] leading-relaxed text-success">
              Sudah ditandatangani {summary.signature.signerName} ({summary.signature.signerRole}) pada{" "}
              {formatDate(summary.signature.signedAt)} —{" "}
              {signOnly ? "tidak ada yang perlu disimpan." : "terima untuk membuat project-nya."}
            </p>
          ) : (
            <div className="flex flex-col gap-3 rounded-lg border border-border px-4 py-3">
              <p className="text-[13px] font-semibold text-text-primary">Tanda tangan klien</p>
              <Field label="Atas Nama" htmlFor="accept-signer-role" required>
                <Select
                  value={selectedRole}
                  onChange={(e) => {
                    setSelectedRole(e.target.value);
                    setUseSpecimen(false);
                  }}
                >
                  {signerOptions.map((o) => (
                    <option key={o.role} value={o.role}>
                      {o.name} ({o.role === "Bride" ? "Mempelai Wanita" : o.role === "Groom" ? "Mempelai Pria" : o.role})
                    </option>
                  ))}
                </Select>
              </Field>

              {/* Pakai ulang specimen — hanya bila pemiliknya SAMA dengan Atas
                  Nama terpilih (D12a). Bila berbeda, sebut pemiliknya dan
                  tawarkan unggah saja. */}
              {specimen && selectedRole && specimen.role !== selectedRole && (
                <p className="rounded-md border border-warning/30 bg-warning-soft px-3 py-2 text-[12.5px] leading-relaxed text-warning-strong">
                  TTD tersimpan milik {specimen.signerName} ({specimen.role}), bukan {selectedOption?.name ?? selectedRole}.
                  Unggah foto TTD yang baru untuk melanjutkan.
                </p>
              )}
              {specimenUsable && specimen && (
                <div className="flex flex-wrap items-center gap-3">
                  {specimenPreview ? (
                    <img
                      src={specimenPreview}
                      alt={`TTD tersimpan milik ${specimen.signerName}`}
                      className="h-16 max-w-44 rounded border border-border-light bg-white object-contain px-2 py-1"
                    />
                  ) : (
                    <span className="text-[12.5px] text-text-secondary">Memuat TTD tersimpan…</span>
                  )}
                  <Button
                    size="sm"
                    variant={useSpecimen ? "primary" : "secondary"}
                    onClick={() => setUseSpecimen((v) => !v)}
                  >
                    {useSpecimen ? "Memakai TTD tersimpan ✓" : "Pakai TTD tersimpan"}
                  </Button>
                </div>
              )}

              {!useSpecimen && (
                <div className="flex flex-col gap-2">
                  <input
                    ref={fileInputRef}
                    type="file"
                    accept="image/png,image/jpeg"
                    className="hidden"
                    onChange={(e) => {
                      void handleFileChange(e.target.files?.[0]);
                      e.target.value = "";
                    }}
                  />
                  <div className="flex flex-wrap items-center gap-2">
                    <Button size="sm" variant="secondary" disabled={uploadBusy} onClick={() => fileInputRef.current?.click()}>
                      {uploadBusy ? "Mengompres…" : uploaded ? "Ganti foto TTD" : "Unggah foto TTD"}
                    </Button>
                    {uploaded && (
                      <>
                        <span className="max-w-56 truncate text-[12.5px] text-text-secondary">{uploaded.fileName}</span>
                        <button
                          type="button"
                          className="text-[12.5px] font-semibold text-danger underline underline-offset-2"
                          onClick={() => setUploaded(null)}
                        >
                          Hapus
                        </button>
                      </>
                    )}
                  </div>
                  <p className="text-[12px] text-text-secondary">
                    Foto TTD yang dikirim klien (PNG/JPG, maks 2 MB). Dikompres otomatis seperti evidence.
                  </p>
                </div>
              )}

              {signatureReason && (
                <p className="rounded-md border border-danger/30 bg-danger-soft px-3 py-2 text-[12.5px] leading-relaxed text-danger">
                  {signatureReason}
                </p>
              )}
            </div>
          )
        )}

        {!signOnly && (
          <>
            <Field label="Nama Project" htmlFor="accept-name" required>
              <Input
                id="accept-name"
                value={projectName}
                placeholder="Akad Rara & Dafa"
                onChange={(e) => setProjectName(e.target.value)}
              />
            </Field>
            <div className="grid gap-3 sm:grid-cols-2">
              <Field label="Tanggal Booking" htmlFor="accept-booking" required>
                <Input id="accept-booking" type="date" value={prepStartDate} onChange={(e) => setPrepStartDate(e.target.value)} />
              </Field>
              <Field label="PIC Wedding Planner" htmlFor="accept-pic" hint="Opsional">
                <Select value={picStaffId} onChange={(e) => setPicStaffId(e.target.value)}>
                  <option value="">Belum ditugaskan</option>
                  {staffSummaries.map((s) => (
                    <option key={s.id} value={s.id}>
                      {s.name}
                    </option>
                  ))}
                </Select>
              </Field>
            </div>
            <Field label="Catatan" htmlFor="accept-notes" hint="Opsional">
              <Textarea id="accept-notes" rows={3} value={notes} onChange={(e) => setNotes(e.target.value)} />
            </Field>
          </>
        )}

        {error && <p className="text-[13px] text-danger">{error}</p>}

        <div className="flex justify-end gap-2">
          <Button variant="ghost" onClick={onClose}>
            Batal
          </Button>
          <Button disabled={!canSubmit} onClick={() => void handleAccept()}>
            {submitLabel}
          </Button>
        </div>
      </div>
    </Modal>
  );
}
