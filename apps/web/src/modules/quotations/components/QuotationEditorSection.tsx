import { useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { FileText, Pencil, Plus, Trash2, Send, RotateCcw, Undo2, CheckCircle2, XCircle, Copy, Link2, PenLine } from "lucide-react";
import { Card, CardHeader, CardContent } from "@/shared/components/ui/Card";
import { Badge, type BadgeTone } from "@/shared/components/ui/Badge";
import { Button } from "@/shared/components/ui/Button";
import { ConfirmDialog } from "@/shared/components/ui/ConfirmDialog";
import { Modal } from "@/shared/components/ui/Modal";
import { Input, Textarea, Select, Field } from "@/shared/components/ui/Input";
import { CurrencyInput } from "@/shared/components/ui/CurrencyInput";
import { IconActionButton } from "@/shared/components/ui/IconActionButton";
import { useAuthStore } from "@/shared/stores/useAuthStore";
import { useTenantBrandingStore } from "@/shared/stores/useTenantBrandingStore";
import { IncompleteProfileDialog } from "@/shared/components/IncompleteProfileDialog";
import {
  useQuotationStore,
  type QuotationAdjustmentInput,
  type QuotationBlockInput,
} from "@/modules/quotations/stores/useQuotationStore";
import { useClientStore } from "@/modules/clients/stores/useClientStore";
import { useVenueStore } from "@/modules/venues/stores/useVenueStore";
import type { QuotationBlock, QuotationStatus } from "@/modules/quotations/types";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { getApiErrorMessage, getApiErrorMessageFromBlob } from "@/shared/lib/api-error";
import { openPdfInNewTab } from "@/shared/lib/open-pdf";
import { formatCurrency, formatDate } from "@/shared/lib/formatters";
import { httpClient } from "@/shared/services/http-client";
import { API } from "@/shared/services/api-endpoints";
import { AcceptProjectDialog } from "@/modules/quotations/components/AcceptProjectDialog";

const STATUS_TONE: Record<QuotationStatus, BadgeTone> = {
  Draft: "neutral",
  Ditawarkan: "info",
  Diterima: "success",
  Ditolak: "danger",
  Kedaluwarsa: "warning",
  Dibatalkan: "danger",
};

/**
 * Editor Penawaran (PLAN penawaran-client-master §8 T3.6) — pindahan
 * PackageOrderSection: komposisi, penyesuaian, S&K, ringkasan, dan alur fase
 * (Kirim/Tarik/Revisi/Terima/Tolak/Kedaluwarsa/Batal/Duplikat/Hapus) dalam
 * satu scroll, ditambah identitas penawaran (client, tanggal, venue — semua
 * Select dari masternya, D19).
 */
// CloseReason adalah tiga status akhir sebuah penawaran. Nilainya SENGAJA
// sama persis dengan nama statusnya, supaya pemetaan alasan → endpoint bisa
// dibaca tanpa tabel terjemahan di kepala.
type CloseReason = "Ditolak" | "Kedaluwarsa" | "Dibatalkan";

interface CloseReasonOption {
  value: CloseReason;
  /** Nama status yang akan tercatat — dipakai apa adanya sebagai judul pilihan. */
  label: string;
  /** Satu kalimat definisi: kapan alasan ini yang tepat. */
  hint: string;
}

// Alasan yang boleh dipilih IKUT STATUS, bukan daftar tetap. Backend menolak
// sebagiannya: terminal() (Ditolak/Kedaluwarsa) hanya menerima penawaran
// Ditawarkan, sedangkan Cancel juga menerima Draft. Menawarkan "Ditolak" pada
// sebuah Draft hanya akan berakhir 422 setelah dialognya terlanjur diisi.
function closeReasonsFor(status: QuotationStatus): CloseReasonOption[] {
  const cancelled: CloseReasonOption = {
    value: "Dibatalkan",
    label: "Dibatalkan",
    hint: "Penawaran ditarik atas keputusan internal.",
  };
  if (status === "Ditawarkan") {
    return [
      { value: "Ditolak", label: "Ditolak", hint: "Klien memutuskan tidak melanjutkan penawaran ini." },
      {
        value: "Kedaluwarsa",
        label: "Kedaluwarsa",
        hint: "Masa berlaku penawaran berakhir tanpa keputusan dari klien.",
      },
      cancelled,
    ];
  }
  if (status === "Draft") {
    return [cancelled];
  }
  // Diterima dan ketiga status akhir tidak punya jalan tutup: yang Diterima
  // sudah melahirkan project (hapus project-nya kalau memang harus), yang
  // sudah tertutup tidak bisa ditutup dua kali.
  return [];
}

export function QuotationEditorSection({ quotationId }: { quotationId: string }) {
  const navigate = useNavigate();
  const quotation = useQuotationStore((s) => s.currentQuotation);
  const loading = useQuotationStore((s) => s.loading);
  const fetchQuotation = useQuotationStore((s) => s.fetchQuotation);
  const tenantCategories = useQuotationStore((s) => s.categories);
  const fetchCategories = useQuotationStore((s) => s.fetchCategories);
  const saveHeader = useQuotationStore((s) => s.saveHeader);
  const saveBlocks = useQuotationStore((s) => s.saveBlocks);
  const saveAdjustments = useQuotationStore((s) => s.saveAdjustments);
  const issue = useQuotationStore((s) => s.issue);
  const withdraw = useQuotationStore((s) => s.withdraw);
  const revise = useQuotationStore((s) => s.revise);
  const reject = useQuotationStore((s) => s.reject);
  const expire = useQuotationStore((s) => s.expire);
  const cancelQuotation = useQuotationStore((s) => s.cancel);
  const duplicate = useQuotationStore((s) => s.duplicate);
  const deleteQuotation = useQuotationStore((s) => s.deleteQuotation);
  const fetchDeleteImpact = useQuotationStore((s) => s.fetchDeleteImpact);
  const createSignatureLink = useQuotationStore((s) => s.createSignatureLink);
  const resetQuotation = useQuotationStore((s) => s.reset);

  const clients = useClientStore((s) => s.clients);
  const fetchClients = useClientStore((s) => s.fetchClients);
  const venues = useVenueStore((s) => s.venues);
  const fetchVenues = useVenueStore((s) => s.fetchVenues);

  // Sales/Admin/Owner mengelola penawaran dan menimpa harga (T4.4).
  // Wedding Planner tidak membuka menu ini (gerbang rute + backend).
  const role = useAuthStore((s) => s.session?.role);
  const canManage = role === "Owner" || role === "Admin" || role === "Sales";
  const profileComplete = useTenantBrandingStore((s) => s.profileComplete);
  const missingProfileFields = useTenantBrandingStore((s) => s.missingProfileFields);

  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [profileGateOpen, setProfileGateOpen] = useState(false);
  const [blockDraft, setBlockDraft] = useState<{ index: number | null; value: QuotationBlockInput } | null>(null);
  const [adjustDraft, setAdjustDraft] = useState<{ index: number | null; description: string; amount: number; isTakeout: boolean } | null>(null);
  const [headerDraft, setHeaderDraft] = useState<{
    basePrice: number;
    packageName: string;
    termsText: string;
    bonusNote: string;
    eventDate: string;
    pax: number;
    venueId: string;
    clientId: string;
  } | null>(null);
  const [confirmIssue, setConfirmIssue] = useState(false);
  const [closeOpen, setCloseOpen] = useState(false);
  const [closeReason, setCloseReason] = useState<CloseReason | "">("");
  const [acceptOpen, setAcceptOpen] = useState(false);
  const [signRevisionOpen, setSignRevisionOpen] = useState(false);
  const [confirmLinkOpen, setConfirmLinkOpen] = useState(false);
  const [linkFeedback, setLinkFeedback] = useState<string | null>(null);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [deleteImpact, setDeleteImpact] = useState<Awaited<ReturnType<typeof fetchDeleteImpact>> | null>(null);

  useEffect(() => {
    // Clear before fetching: the store is module-level, so navigating from
    // one quotation to another would otherwise show the previous document for
    // the duration of the request.
    resetQuotation();
    void fetchQuotation(quotationId);
  }, [quotationId, fetchQuotation, resetQuotation]);

  useEffect(() => {
    if (canManage) {
      void fetchClients(1, "", 100);
      void fetchVenues();
      void fetchCategories();
    }
  }, [canManage, fetchClients, fetchVenues, fetchCategories]);

  const editable = quotation?.status === "Draft";
  const closeReasons = quotation ? closeReasonsFor(quotation.status) : [];
  const categories = useMemo(
    // D19: kategori blok tetap teks bebas — bukan master data — tapi dibantu
    // datalist. Sumbernya riwayat tenant DIGABUNG dengan kategori dokumen ini;
    // kalau hanya dokumen ini, daftarnya justru kosong tepat saat penawaran
    // baru disusun, yaitu saat bantuannya paling dibutuhkan.
    () =>
      Array.from(
        new Set([...tenantCategories, ...(quotation?.blocks ?? []).map((b) => b.category)].filter(Boolean)),
      ).sort((a, b) => a.localeCompare(b)),
    [tenantCategories, quotation],
  );

  function closeDeleteConfirm() {
    setConfirmDelete(false);
    setDeleteImpact(null);
  }

  async function run(action: () => Promise<void>) {
    setBusy(true);
    setError(null);
    try {
      await action();
    } catch (err) {
      setError(getApiErrorMessage(err, "Gagal menyimpan perubahan"));
    } finally {
      setBusy(false);
    }
  }

  function toBlockInputs(blocks: QuotationBlock[]): QuotationBlockInput[] {
    return blocks.map(({ category, body, qtyText, bonusNote }) => ({ category, body, qtyText, bonusNote }));
  }

  // Must stay synchronous up to openPdfInNewTab's own first line: it claims the
  // tab while the click is still a valid user gesture. Awaiting the request
  // first and only then calling window.open is what made this button do
  // nothing at all -- see openPdfInNewTab's doc comment.
  async function handleDownloadPDF() {
    if (!profileComplete) {
      setProfileGateOpen(true);
      return;
    }
    setError(null);
    try {
      await openPdfInNewTab(async () => {
        const res = await httpClient.get(API.quotations.pdf(quotationId), { responseType: "blob" });
        return res.data as Blob;
      }, quotation?.poNumber ? quotation.poNumber.replace(/\//g, "-") : "Penawaran-Draft");
    } catch (err) {
      setError(await getApiErrorMessageFromBlob(err, "Gagal membuat PDF penawaran"));
    }
  }

  async function openDeleteConfirm() {
    setError(null);
    setBusy(true);
    try {
      // D14: dialog menolak tampil kalau dampaknya gagal dibaca — tidak
      // pernah ada konfirmasi yang berbohong.
      const impact = await fetchDeleteImpact(quotationId);
      setDeleteImpact(impact);
      setConfirmDelete(true);
    } catch (err) {
      setError(getApiErrorMessage(err, "Gagal membaca dampak penghapusan"));
    } finally {
      setBusy(false);
    }
  }

  if (loading && !quotation) {
    return <p className="py-10 text-center text-[13px] text-text-secondary">Memuat penawaran...</p>;
  }

  if (!quotation) {
    return <p className="py-10 text-center text-[13px] text-text-secondary">Penawaran tidak ditemukan.</p>;
  }

  // Satu pintu ke modal header, dipakai kartu Detail Kontrak DAN kartu Syarat
  // & Ketentuan: backend menyimpan seluruh header dalam satu SetHeader, jadi
  // memecahnya jadi dua modal hanya akan memecah satu penyimpanan jadi dua.
  function openHeaderEditor() {
    setHeaderDraft({
      basePrice: quotation!.basePrice,
      packageName: quotation!.packageName,
      termsText: quotation!.termsText,
      bonusNote: quotation!.bonusNote,
      eventDate: quotation!.eventDate ?? "",
      pax: quotation!.pax,
      venueId: quotation!.venueId ?? "",
      clientId: quotation!.clientId,
    });
  }

  // Syarat yang ditegakkan Issue di backend. Didaftar di sini supaya layar
  // menyebutkannya SEBELUM tombol ditekan, bukan menunggu 422 menyebut field
  // yang penggunanya tidak tahu ada di mana.
  const missingForIssue = [
    !quotation.packageName.trim() ? "Nama Paket" : null,
    !quotation.eventDate ? "Tanggal Acara" : null,
    quotation.total <= 0 ? "Harga Paket" : null,
  ].filter((v): v is string => v !== null);

  const statusLabel =
    quotation.status === "Diterima" && !quotation.projectId
      ? "Diterima · Perlu dibuatkan project"
      : (quotation.status === "Ditawarkan" || quotation.status === "Diterima") && quotation.revision > 0
        ? `${quotation.status} · Revisi ${quotation.revision}`
        : quotation.status;

  // Diterima punya dua arti (T7): sudah jadi project ATAU menunggu pengelola
  // melengkapi project. Fakta yang bisa diturunkan tidak disimpan sebagai
  // status baru — cukup diturunkan di tempat ia ditampilkan.
  const needsProject = quotation.status === "Diterima" && !quotation.projectId;
  const statusTone: BadgeTone = needsProject ? "warning" : STATUS_TONE[quotation.status];
  // D13a: revisi pasca-project yang belum ditandatangani ulang.
  const revisionUnsigned = quotation.status === "Diterima" && !!quotation.projectId && !quotation.signature;

  return (
    <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_320px]">
      <div className="flex flex-col gap-4">
        {/* --- Detail kontrak ---
            Kartu ini dulu TIDAK ADA. Nama Paket dan Jumlah Pax tidak tampil di
            mana pun, sementara Client/Tanggal Acara/Venue hanya terbaca di
            Ringkasan sebagai teks mati — dan satu-satunya pintu untuk
            mengubahnya adalah tombol "Ubah" yang menempel di kartu SYARAT &
            KETENTUAN. Akibatnya Kirim ditolak dengan "Tanggal acara wajib
            diisi" sementara di layar memang tidak ada kotak isiannya yang bisa
            ditemukan orang.
            Yang dibutuhkan penawaran untuk bisa dikirim ada di sini semua,
            terbaca, dengan pintu ubahnya sendiri. */}
        <Card>
          <CardHeader
            title="Detail Kontrak"
            subtitle="Identitas dan pokok kesepakatan yang tercetak di kop PO."
            action={
              editable && canManage ? (
                <Button size="sm" variant="secondary" icon={<Pencil className="h-3.5 w-3.5" />} onClick={openHeaderEditor}>
                  Ubah
                </Button>
              ) : undefined
            }
          />
          <CardContent className="grid gap-x-6 gap-y-3 sm:grid-cols-2">
            <DetailField label="Client" value={quotation.clientName} />
            <DetailField label="Nama Paket" value={quotation.packageName} required />
            <DetailField
              label="Tanggal Acara"
              value={quotation.eventDate ? formatDate(quotation.eventDate) : ""}
              required
            />
            <DetailField label="Venue" value={quotation.venueName} />
            <DetailField label="Jumlah Pax" value={quotation.pax > 0 ? String(quotation.pax) : ""} />
            <DetailField label="Harga Paket" value={quotation.basePrice > 0 ? formatCurrency(quotation.basePrice) : ""} required />
          </CardContent>
        </Card>

        {/* --- Komposisi paket (B2/B3) --- */}
        <Card>
          <CardHeader
            title="Isi Paket"
            subtitle="Rincian yang tercetak di PO."
            action={
              editable && canManage ? (
                <Button
                  size="sm"
                  icon={<Plus className="h-3.5 w-3.5" />}
                  onClick={() => setBlockDraft({ index: null, value: { category: "", body: "", qtyText: "", bonusNote: "" } })}
                >
                  Tambah rincian
                </Button>
              ) : undefined
            }
          />
          <CardContent>
            {quotation.blocks.length === 0 ? (
              <p className="py-6 text-center text-[13px] text-text-secondary">Belum ada rincian paket.</p>
            ) : (
              <ul className="flex flex-col gap-2">
                {quotation.blocks.map((block, index) => (
                  <li key={block.id} className="rounded-lg border border-border bg-surface p-3">
                    <div className="flex items-start justify-between gap-3">
                      <div className="min-w-0 flex-1">
                        <p className="text-[12px] font-semibold uppercase tracking-wide text-text-secondary">
                          {block.category || "Tanpa kategori"}
                        </p>
                        <pre className="mt-1 whitespace-pre-wrap break-words font-sans text-[13px] text-text-primary">
                          {block.body || "—"}
                        </pre>
                        {(block.qtyText || block.bonusNote) && (
                          <div className="mt-2 grid gap-2 sm:grid-cols-2">
                            {block.qtyText && (
                              <p className="text-[12px] text-text-secondary">
                                <span className="font-medium">QTY:</span> {block.qtyText.replace(/\n/g, " · ")}
                              </p>
                            )}
                            {block.bonusNote && (
                              <p className="text-[12px] text-text-secondary">
                                <span className="font-medium">Bonus:</span> {block.bonusNote.replace(/\n/g, " · ")}
                              </p>
                            )}
                          </div>
                        )}
                      </div>
                      {editable && canManage && (
                        <div className="flex shrink-0 items-center gap-1">
                          <IconActionButton
                            icon={Pencil}
                            label="Ubah rincian"
                            tone="neutral"
                            onClick={() =>
                              setBlockDraft({
                                index,
                                value: {
                                  category: block.category,
                                  body: block.body,
                                  qtyText: block.qtyText,
                                  bonusNote: block.bonusNote,
                                },
                              })
                            }
                          />
                          <IconActionButton
                            icon={Trash2}
                            label="Hapus rincian"
                            tone="danger"
                            onClick={() =>
                              void run(() =>
                                saveBlocks(quotationId, toBlockInputs(quotation.blocks.filter((_, i) => i !== index))),
                              )
                            }
                          />
                        </div>
                      )}
                    </div>
                  </li>
                ))}
              </ul>
            )}
          </CardContent>
        </Card>

        {/* --- Additional & Takeout (B4) --- */}
        <Card>
          <CardHeader
            title="Additional & Takeout"
            subtitle="Tambahan atau pengurangan harga di luar paket."
            action={
              editable && canManage ? (
                <Button
                  size="sm"
                  icon={<Plus className="h-3.5 w-3.5" />}
                  onClick={() => setAdjustDraft({ index: null, description: "", amount: 0, isTakeout: false })}
                >
                  Tambah baris
                </Button>
              ) : undefined
            }
          />
          <CardContent>
            {quotation.adjustments.length === 0 ? (
              <p className="py-6 text-center text-[13px] text-text-secondary">Belum ada penyesuaian.</p>
            ) : (
              <ul className="flex flex-col divide-y divide-border">
                {quotation.adjustments.map((adj, index) => (
                  <li key={adj.id} className="flex items-center gap-3 py-2">
                    <span className="min-w-0 flex-1 truncate text-[13px] text-text-primary">{adj.description}</span>
                    <span
                      className={`shrink-0 text-[13px] font-semibold tabular-nums ${adj.amount < 0 ? "text-danger" : "text-text-primary"}`}
                    >
                      {adj.amount < 0 ? "−" : "+"}
                      {formatCurrency(Math.abs(adj.amount))}
                    </span>
                    {editable && canManage && (
                      <span className="flex shrink-0 items-center gap-1">
                        <IconActionButton
                          icon={Pencil}
                          label="Ubah penyesuaian"
                          tone="neutral"
                          onClick={() =>
                            setAdjustDraft({
                              index,
                              description: adj.description,
                              amount: Math.abs(adj.amount),
                              isTakeout: adj.amount < 0,
                            })
                          }
                        />
                        <IconActionButton
                          icon={Trash2}
                          label="Hapus penyesuaian"
                          tone="danger"
                          onClick={() =>
                            void run(() =>
                              saveAdjustments(
                                quotationId,
                                quotation.adjustments
                                  .filter((_, i) => i !== index)
                                  .map(({ description, amount }) => ({ description, amount })),
                              ),
                            )
                          }
                        />
                      </span>
                    )}
                  </li>
                ))}
              </ul>
            )}
          </CardContent>
        </Card>

        {/* --- Syarat & ketentuan + bonus (B5) --- */}
        <Card>
          <CardHeader
            title="Syarat & Ketentuan"
            subtitle="Tercetak di bagian akhir PO."
            action={
              editable && canManage ? (
                <Button
                  size="sm"
                  variant="secondary"
                  icon={<Pencil className="h-3.5 w-3.5" />}
                  onClick={openHeaderEditor}
                >
                  Ubah
                </Button>
              ) : undefined
            }
          />
          <CardContent className="flex flex-col gap-4">
            <pre className="whitespace-pre-wrap break-words font-sans text-[13px] text-text-primary">
              {quotation.termsText || "Belum diisi."}
            </pre>
            <div>
              <p className="text-[12px] font-semibold uppercase tracking-wide text-text-secondary">Bonus Tambahan</p>
              <pre className="mt-1 whitespace-pre-wrap break-words font-sans text-[13px] text-text-primary">
                {quotation.bonusNote || "Belum diisi."}
              </pre>
            </div>
          </CardContent>
        </Card>
      </div>

      {/* --- Ringkasan --- */}
      <div className="flex flex-col gap-4">
        <Card>
          <CardHeader
            title="Ringkasan"
            action={
              <span className="flex flex-wrap items-center gap-1.5">
                <Badge tone={statusTone}>{statusLabel}</Badge>
                {revisionUnsigned && <Badge tone="warning">Revisi belum ditandatangani</Badge>}
              </span>
            }
          />
          <CardContent className="flex flex-col gap-2 text-[13px]">
            {/* Client/Tanggal Acara/Venue pindah ke kartu Detail Kontrak —
                di sanalah mereka bisa diubah. Ringkasan tinggal mengurus uang
                dan kedudukan dokumennya. */}
            <SummaryRow label="Harga Paket Awal" value={formatCurrency(quotation.basePrice)} />
            <SummaryRow
              label="Additional & Takeout"
              value={`${quotation.totalAdjustments < 0 ? "−" : "+"}${formatCurrency(Math.abs(quotation.totalAdjustments))}`}
              tone={quotation.totalAdjustments < 0 ? "danger" : undefined}
            />
            <div className="mt-1 flex items-center justify-between border-t border-border pt-2">
              <span className="font-semibold text-text-primary">Total Pembayaran</span>
              <span className="text-[15px] font-semibold tabular-nums text-text-primary">{formatCurrency(quotation.total)}</span>
            </div>
            {quotation.poNumber && <SummaryRow label="Nomor PO" value={quotation.poNumber} />}
            {quotation.issuedAt && <SummaryRow label="Dikirim" value={formatDate(quotation.issuedAt)} />}
            {quotation.acceptedAt && <SummaryRow label="Diterima" value={formatDate(quotation.acceptedAt)} />}
            {quotation.signature && (
              <SummaryRow
                label="Ditandatangani"
                value={`${quotation.signature.signerName} (${quotation.signature.signerRole}) · ${formatDate(quotation.signature.signedAt)}`}
              />
            )}
            {/* D13a: penanda revisi yang belum ditandatangani — tanpa ini
                pengelola tidak tahu PO revisinya tercetak dengan kotak TTD
                kosong, dan tidak punya jalan memperbaikinya. */}
            {revisionUnsigned && (
              <p className="rounded-md border border-warning/30 bg-warning-soft px-3 py-2 text-[12.5px] leading-relaxed text-warning-strong">
                Revisi ini belum ditandatangani. Kirim link tanda tangan ke klien, atau bubuhkan TTD-nya di sini.
              </p>
            )}
            {quotation.projectId ? (
              <button
                type="button"
                onClick={() => navigate(ROUTE_PATHS.projectDetail(quotation.projectId))}
                className="mt-1 text-left text-[13px] font-semibold text-navy-700 underline-offset-2 hover:underline"
              >
                Buka project yang lahir dari penawaran ini →
              </button>
            ) : (
              <p className="mt-1 text-[12px] text-text-secondary">
                Project lahir otomatis begitu penawaran ini diterima — tidak perlu input ulang.
              </p>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardContent className="flex flex-col gap-2">
            <Button variant="secondary" icon={<FileText className="h-3.5 w-3.5" />} onClick={() => void handleDownloadPDF()}>
              {editable ? "Pratinjau PDF" : "Unduh PDF"}
            </Button>
            {canManage && editable && (
              <>
                <Button
                  icon={<Send className="h-3.5 w-3.5" />}
                  disabled={busy || missingForIssue.length > 0}
                  onClick={() => setConfirmIssue(true)}
                >
                  Terbitkan PO
                </Button>
                {/* Sebabnya disebutkan di sebelah tombolnya yang mati, bukan
                    disimpan sampai tombolnya ditekan. */}
                {missingForIssue.length > 0 && (
                  <p className="rounded-md border border-danger/30 bg-danger-soft px-3 py-2 text-[12.5px] leading-relaxed text-danger">
                    Belum bisa dikirim: {missingForIssue.join(", ")} masih kosong. Lengkapi di kartu{" "}
                    <button type="button" className="font-semibold underline underline-offset-2" onClick={openHeaderEditor}>
                      Detail Kontrak
                    </button>
                    .
                  </p>
                )}
              </>
            )}
            {canManage && quotation.status === "Ditawarkan" && (
              <>
                <Button
                  icon={<CheckCircle2 className="h-3.5 w-3.5" />}
                  disabled={busy}
                  onClick={() => setAcceptOpen(true)}
                >
                  Terima Penawaran
                </Button>
                <Button
                  variant="secondary"
                  icon={<Link2 className="h-3.5 w-3.5" />}
                  disabled={busy}
                  onClick={() => {
                    setLinkFeedback(null);
                    setConfirmLinkOpen(true);
                  }}
                >
                  Copy Link Signature
                </Button>
                {linkFeedback && (
                  <p className="rounded-md border border-success/30 bg-success-soft px-3 py-2 text-[12.5px] text-success">
                    {linkFeedback}
                  </p>
                )}
                <Button variant="secondary" icon={<Undo2 className="h-3.5 w-3.5" />} disabled={busy} onClick={() => void run(() => withdraw(quotationId))}>
                  Tarik ke Draft
                </Button>
              </>
            )}
            {/* T7: Diterima tanpa project tidak punya jalan keluar di
                halamannya sendiri — tombol Buat Project membuka dialog Terima
                yang sama (blok TTD-nya yang menentukan). */}
            {canManage && needsProject && (
              <>
                <Button
                  icon={<CheckCircle2 className="h-3.5 w-3.5" />}
                  disabled={busy}
                  onClick={() => setAcceptOpen(true)}
                >
                  Buat Project
                </Button>
                <Button variant="secondary" icon={<RotateCcw className="h-3.5 w-3.5" />} disabled={busy} onClick={() => void run(() => revise(quotationId))}>
                  Buat revisi
                </Button>
              </>
            )}
            {canManage && quotation.status === "Diterima" && !!quotation.projectId && (
              <>
                {/* D13a: revisi pasca-project yang belum berTTD — kirim link
                    atau bubuhkan langsung; tanpa ini tidak ada jalan
                    memperbaikinya. */}
                {revisionUnsigned && (
                  <>
                    <Button
                      variant="secondary"
                      icon={<Link2 className="h-3.5 w-3.5" />}
                      disabled={busy}
                      onClick={() => {
                        setLinkFeedback(null);
                        setConfirmLinkOpen(true);
                      }}
                    >
                      Copy Link Signature
                    </Button>
                    {linkFeedback && (
                      <p className="rounded-md border border-success/30 bg-success-soft px-3 py-2 text-[12.5px] text-success">
                        {linkFeedback}
                      </p>
                    )}
                    <Button
                      variant="secondary"
                      icon={<PenLine className="h-3.5 w-3.5" />}
                      disabled={busy}
                      onClick={() => setSignRevisionOpen(true)}
                    >
                      Unggah / Pakai TTD
                    </Button>
                  </>
                )}
                <Button variant="secondary" icon={<RotateCcw className="h-3.5 w-3.5" />} disabled={busy} onClick={() => void run(() => revise(quotationId))}>
                  Buat revisi
                </Button>
              </>
            )}
            {/* Satu tombol untuk tiga status akhir. Tolak/Kedaluwarsa/Batalkan
                dulu berdiri sendiri-sendiri, padahal di backend ketiganya
                operasi yang sama persis (Reject dan Expire sama-sama memanggil
                terminal(); Cancel identik, hanya juga menerima Draft) — yang
                berbeda cuma ALASAN penawaran itu mati. Alasan adalah isi
                dialog, bukan tiga tombol sejajar.
                Bukan lagi `danger`: ini perubahan status, dokumennya tetap ada
                dan tetap terbaca. Yang merah tinggal Hapus Penawaran, yang
                memang satu-satunya aksi permanen di sini. */}
            {canManage && closeReasons.length > 0 && (
              <Button
                variant="secondary"
                icon={<XCircle className="h-3.5 w-3.5" />}
                disabled={busy}
                onClick={() => {
                  // Satu-satunya pilihan (Draft) dipilihkan di muka supaya
                  // dialognya jadi konfirmasi biasa, bukan pertanyaan yang
                  // jawabannya cuma satu.
                  setCloseReason(closeReasons.length === 1 ? closeReasons[0].value : "");
                  setCloseOpen(true);
                }}
              >
                Tutup Penawaran…
              </Button>
            )}
            {canManage && (
              <Button variant="secondary" icon={<Copy className="h-3.5 w-3.5" />} disabled={busy} onClick={() => void run(async () => {
                const dup = await duplicate(quotationId);
                navigate(ROUTE_PATHS.quotationDetail(dup.id));
              })}>
                Duplikat Penawaran
              </Button>
            )}
            {canManage && (
              <Button variant="danger" icon={<Trash2 className="h-3.5 w-3.5" />} disabled={busy} onClick={() => void openDeleteConfirm()}>
                Hapus Penawaran
              </Button>
            )}
            {error && <p className="text-[13px] text-danger">{error}</p>}
          </CardContent>
        </Card>
      </div>

      {/* --- Modal: rincian paket --- */}
      <Modal
        open={blockDraft !== null}
        onClose={() => setBlockDraft(null)}
        title={blockDraft?.index === null ? "Tambah Rincian Paket" : "Ubah Rincian Paket"}
        size="lg"
        footer={
          <>
            <Button variant="ghost" onClick={() => setBlockDraft(null)}>
              Batal
            </Button>
            <Button
              disabled={busy || !blockDraft?.value.category.trim()}
              onClick={() => {
                if (!blockDraft) return;
                const next = toBlockInputs(quotation.blocks);
                if (blockDraft.index === null) next.push(blockDraft.value);
                else next[blockDraft.index] = blockDraft.value;
                void run(async () => {
                  await saveBlocks(quotationId, next);
                  setBlockDraft(null);
                });
              }}
            >
              Simpan rincian
            </Button>
          </>
        }
      >
        {blockDraft && (
          <div className="flex flex-col gap-3">
            <Field label="Kategori" htmlFor="po-block-category" required>
              <Input
                id="po-block-category"
                list="po-category-options"
                value={blockDraft.value.category}
                placeholder="CATERING"
                onChange={(e) => setBlockDraft({ ...blockDraft, value: { ...blockDraft.value, category: e.target.value } })}
              />
              <datalist id="po-category-options">
                {categories.map((c) => (
                  <option key={c} value={c} />
                ))}
              </datalist>
            </Field>
            <Field
              label="Isi paket"
              htmlFor="po-block-body"
              hint="Satu item per baris. Bisa ditempel langsung dari Excel. Baris yang diketik HURUF KAPITAL akan dicetak tebal."
            >
              <Textarea
                id="po-block-body"
                rows={10}
                value={blockDraft.value.body}
                placeholder={"BUFFET\nNasi Putih\nAneka Nasi Goreng"}
                onChange={(e) => setBlockDraft({ ...blockDraft, value: { ...blockDraft.value, body: e.target.value } })}
              />
            </Field>
            <div className="grid gap-3 sm:grid-cols-2">
              <Field label="QTY" htmlFor="po-block-qty" hint="Contoh: 700 PORSI. Boleh beberapa baris.">
                <Textarea
                  id="po-block-qty"
                  rows={4}
                  value={blockDraft.value.qtyText}
                  onChange={(e) => setBlockDraft({ ...blockDraft, value: { ...blockDraft.value, qtyText: e.target.value } })}
                />
              </Field>
              <Field label="Bonus" htmlFor="po-block-bonus">
                <Textarea
                  id="po-block-bonus"
                  rows={4}
                  value={blockDraft.value.bonusNote}
                  onChange={(e) => setBlockDraft({ ...blockDraft, value: { ...blockDraft.value, bonusNote: e.target.value } })}
                />
              </Field>
            </div>
          </div>
        )}
      </Modal>

      {/* --- Modal: penyesuaian --- */}
      <Modal
        open={adjustDraft !== null}
        onClose={() => setAdjustDraft(null)}
        title={adjustDraft?.index === null ? "Tambah Penyesuaian" : "Ubah Penyesuaian"}
        footer={
          <>
            <Button variant="ghost" onClick={() => setAdjustDraft(null)}>
              Batal
            </Button>
            <Button
              disabled={busy || !adjustDraft?.description.trim() || !adjustDraft?.amount}
              onClick={() => {
                if (!adjustDraft) return;
                // The sign lives in the amount (D2); the UI only offers a
                // clearer way to pick it than typing a minus.
                const signed = adjustDraft.isTakeout ? -Math.abs(adjustDraft.amount) : Math.abs(adjustDraft.amount);
                const next: QuotationAdjustmentInput[] = quotation.adjustments.map(({ description, amount }) => ({ description, amount }));
                const row = { description: adjustDraft.description, amount: signed };
                if (adjustDraft.index === null) next.push(row);
                else next[adjustDraft.index] = row;
                void run(async () => {
                  await saveAdjustments(quotationId, next);
                  setAdjustDraft(null);
                });
              }}
            >
              Simpan
            </Button>
          </>
        }
      >
        {adjustDraft && (
          <div className="flex flex-col gap-3">
            <Field label="Keterangan" htmlFor="po-adj-desc" required>
              <Input
                id="po-adj-desc"
                value={adjustDraft.description}
                placeholder="Add 100 buffet x 99k"
                onChange={(e) => setAdjustDraft({ ...adjustDraft, description: e.target.value })}
              />
            </Field>
            <Field label="Jenis" htmlFor="po-adj-kind">
              <Select
                value={adjustDraft.isTakeout ? "takeout" : "tambahan"}
                onChange={(e) => setAdjustDraft({ ...adjustDraft, isTakeout: e.target.value === "takeout" })}
              >
                <option value="tambahan">Tambahan (menambah total)</option>
                <option value="takeout">Takeout / cashback (mengurangi total)</option>
              </Select>
            </Field>
            <Field label="Nominal" htmlFor="po-adj-amount" required>
              <CurrencyInput
                id="po-adj-amount"
                value={adjustDraft.amount}
                onChange={(v) => setAdjustDraft({ ...adjustDraft, amount: v })}
              />
            </Field>
          </div>
        )}
      </Modal>

      {/* --- Modal: harga & ketentuan + identitas (D19: Select) --- */}
      <Modal
        open={headerDraft !== null}
        onClose={() => setHeaderDraft(null)}
        title="Ubah Detail Kontrak"
        size="lg"
        footer={
          <>
            <Button variant="ghost" onClick={() => setHeaderDraft(null)}>
              Batal
            </Button>
            <Button
              disabled={busy || !headerDraft?.clientId}
              onClick={() => {
                if (!headerDraft) return;
                void run(async () => {
                  await saveHeader(quotationId, {
                    basePrice: headerDraft.basePrice,
                    packageName: headerDraft.packageName,
                    termsText: headerDraft.termsText,
                    bonusNote: headerDraft.bonusNote,
                    eventDate: headerDraft.eventDate || null,
                    pax: headerDraft.pax,
                    venueId: headerDraft.venueId || null,
                    clientId: headerDraft.clientId,
                  });
                  setHeaderDraft(null);
                });
              }}
            >
              Simpan
            </Button>
          </>
        }
      >
        {headerDraft && (
          <div className="flex flex-col gap-3">
            <div className="grid gap-3 sm:grid-cols-2">
              <Field label="Client" htmlFor="q-client" required>
                <Select
                  value={headerDraft.clientId}
                  onChange={(e) => setHeaderDraft({ ...headerDraft, clientId: e.target.value })}
                >
                  <option value="">Pilih client</option>
                  {clients.map((c) => (
                    <option key={c.id} value={c.id}>
                      {c.displayName}
                    </option>
                  ))}
                </Select>
              </Field>
              <Field label="Venue" htmlFor="q-venue">
                <Select
                  value={headerDraft.venueId}
                  onChange={(e) => setHeaderDraft({ ...headerDraft, venueId: e.target.value })}
                >
                  <option value="">Tanpa venue</option>
                  {venues.map((v) => (
                    <option key={v.id} value={v.id}>
                      {v.name}
                    </option>
                  ))}
                </Select>
              </Field>
              <Field label="Tanggal Acara" htmlFor="q-event-date">
                <Input
                  id="q-event-date"
                  type="date"
                  value={headerDraft.eventDate}
                  onChange={(e) => setHeaderDraft({ ...headerDraft, eventDate: e.target.value })}
                />
              </Field>
              <Field label="Jumlah Pax" htmlFor="q-pax" hint="0 = belum ditentukan">
                <Input
                  id="q-pax"
                  type="number"
                  min={0}
                  value={headerDraft.pax}
                  onChange={(e) => setHeaderDraft({ ...headerDraft, pax: Number(e.target.value) || 0 })}
                />
              </Field>
            </div>
            <Field
              label="Nama Paket"
              htmlFor="po-package-name"
              hint="Tercetak di PO dan Invoice, dan menjadi Paket / Layanan pada project. Wajib diisi sebelum penawaran dikirim."
            >
              <Input
                id="po-package-name"
                value={headerDraft.packageName}
                placeholder="Silver"
                onChange={(e) => setHeaderDraft({ ...headerDraft, packageName: e.target.value })}
              />
            </Field>
            <Field label="Harga Paket Awal" htmlFor="po-base-price" hint="Total dihitung dari nilai ini ditambah penyesuaian.">
              <CurrencyInput
                id="po-base-price"
                value={headerDraft.basePrice}
                onChange={(v) => setHeaderDraft({ ...headerDraft, basePrice: v })}
              />
            </Field>
            <Field label="Syarat & Ketentuan" htmlFor="po-terms">
              <Textarea
                id="po-terms"
                rows={12}
                value={headerDraft.termsText}
                onChange={(e) => setHeaderDraft({ ...headerDraft, termsText: e.target.value })}
              />
            </Field>
            <Field label="Bonus Tambahan" htmlFor="po-bonus">
              <Textarea
                id="po-bonus"
                rows={6}
                value={headerDraft.bonusNote}
                onChange={(e) => setHeaderDraft({ ...headerDraft, bonusNote: e.target.value })}
              />
            </Field>
          </div>
        )}
      </Modal>

      {/* --- Konfirmasi terbit: nomor PO permanen (D7) --- */}
      <Modal
        open={confirmIssue}
        onClose={() => setConfirmIssue(false)}
        title="Terbitkan PO?"
        footer={
          <>
            <Button variant="ghost" onClick={() => setConfirmIssue(false)}>
              Batal
            </Button>
            <Button
              disabled={busy}
              onClick={() =>
                void run(async () => {
                  await issue(quotationId);
                  setConfirmIssue(false);
                })
              }
            >
              Terbitkan
            </Button>
          </>
        }
      >
        <div className="flex flex-col gap-2 text-[13px] text-text-primary">
          <p>
            Dokumen berjudul PURCHASE ORDER ini akan <strong>bernomor resmi</strong> dan komposisinya dikunci pada
            nilai saat ini. Untuk mengubahnya setelah diterbitkan, tarik kembali ke Draft.
          </p>
          <p className="text-text-secondary">Total pembayaran: {formatCurrency(quotation.total)}</p>
        </div>
      </Modal>

      {/* --- Konfirmasi link signature (D8): tidak ada layar status link
          (§4.2), jadi kalimat inilah satu-satunya yang mencegah pengelola
          diam-diam mematikan link yang sudah dikirim ke klien. --- */}
      <Modal
        open={confirmLinkOpen}
        onClose={() => setConfirmLinkOpen(false)}
        title="Buat Link Tanda Tangan?"
        footer={
          <>
            <Button variant="ghost" onClick={() => setConfirmLinkOpen(false)}>
              Batal
            </Button>
            <Button
              disabled={busy}
              onClick={() =>
                void run(async () => {
                  const { token } = await createSignatureLink(quotationId);
                  const url = `${window.location.origin}${ROUTE_PATHS.publicSignature(token)}`;
                  try {
                    await navigator.clipboard.writeText(url);
                    setLinkFeedback("Link tersalin, berlaku 24 jam.");
                  } catch {
                    setLinkFeedback(`Salin manual tautan ini (berlaku 24 jam): ${url}`);
                  }
                  setConfirmLinkOpen(false);
                })
              }
            >
              Buat & Salin Link
            </Button>
          </>
        }
      >
        <div className="flex flex-col gap-2 text-[13px] text-text-primary">
          <p>
            Link ini <strong>berlaku 24 jam dan hanya bisa dipakai sekali</strong>. Klien membukanya tanpa login
            untuk menerima atau menolak penawaran ini.
          </p>
          <p className="rounded-md border border-warning/30 bg-warning-soft px-3 py-2 text-[12.5px] text-warning-strong">
            Bila penawaran ini sudah pernah punya link, <strong>link yang dikirim sebelumnya akan berhenti
            berlaku</strong> begitu link baru dibuat.
          </p>
        </div>
      </Modal>

      {/* --- Tutup penawaran: satu dialog, tiga status akhir --- */}
      <Modal
        open={closeOpen}
        onClose={() => setCloseOpen(false)}
        title="Tutup Penawaran"
        description={`Penawaran ${quotation.poNumber || "Draft"} akan ditutup dan tidak dapat diubah lagi.`}
        footer={
          <>
            <Button variant="ghost" onClick={() => setCloseOpen(false)}>
              Batal
            </Button>
            <Button
              variant="danger"
              disabled={busy || closeReason === ""}
              onClick={() => {
                if (closeReason === "") return;
                const close =
                  closeReason === "Ditolak" ? reject : closeReason === "Kedaluwarsa" ? expire : cancelQuotation;
                void run(async () => {
                  await close(quotationId);
                  setCloseOpen(false);
                  setCloseReason("");
                });
              }}
            >
              Tutup Penawaran
            </Button>
          </>
        }
      >
        <div className="flex flex-col gap-3">
          <p className="text-[13px] text-text-primary">
            Pilih alasan penutupan. Alasan ini menjadi status akhir penawaran.
          </p>
          <div className="flex flex-col gap-2">
            {closeReasons.map((r) => (
              <label
                key={r.value}
                className={`flex cursor-pointer items-start gap-3 rounded-lg border px-3.5 py-3 transition-colors ${
                  closeReason === r.value ? "border-navy-900 bg-navy-50" : "border-border hover:border-navy-300"
                }`}
              >
                <input
                  type="radio"
                  name="close-reason"
                  className="mt-0.5 h-4 w-4 shrink-0 accent-navy-900"
                  checked={closeReason === r.value}
                  onChange={() => setCloseReason(r.value)}
                />
                {/* Judulnya adalah nama statusnya sendiri, jadi tidak perlu
                    lagi baris "Status menjadi X" — yang dipilih pengguna dan
                    yang tercatat adalah kata yang sama. */}
                <span className="min-w-0">
                  <span className="block text-[13.5px] font-semibold text-text-primary">{r.label}</span>
                  <span className="block text-[12.5px] text-text-secondary">{r.hint}</span>
                </span>
              </label>
            ))}
          </div>
          {/* Tidak ada jalan kembali dari status akhir: withdraw hanya
              menerima Ditawarkan, revise hanya menerima Diterima. Lebih baik
              dikatakan di sini daripada ditemukan sesudahnya. */}
          <p className="rounded-md border border-warning/30 bg-warning-soft px-3 py-2 text-[12.5px] text-warning-strong">
            Penawaran yang telah ditutup tidak dapat dibuka kembali. Dokumen tetap tersimpan dan dapat dibaca. Untuk
            menawarkan ulang, gunakan <strong>Duplikat Penawaran</strong>.
          </p>
        </div>
      </Modal>

      {/* --- Terima: project lahir otomatis (D11, T3.7) --- */}
      {acceptOpen && (
        <AcceptProjectDialog
          quotationId={quotationId}
          onClose={() => setAcceptOpen(false)}
          onAccepted={(newProjectId) => {
            setAcceptOpen(false);
            navigate(ROUTE_PATHS.projectDetail(newProjectId));
          }}
        />
      )}

      {/* --- Tanda tangani revisi pasca-project (D13a): status, AcceptedAt,
          dan project tidak disentuh — hanya TTD. --- */}
      {signRevisionOpen && (
        <AcceptProjectDialog
          quotationId={quotationId}
          signOnly
          onClose={() => setSignRevisionOpen(false)}
          onAccepted={() => {
            setSignRevisionOpen(false);
            void fetchQuotation(quotationId);
          }}
        />
      )}

      {/* --- Hapus: diblokir bila sudah menjadi project (hapus project-nya dulu) ---
          Dua keadaan, dua komponen — sengaja. "Tidak dapat dihapus" adalah
          PEMBERITAHUAN (satu tombol Tutup), bukan pertanyaan; menempelkannya ke
          dalam dialog konfirmasi membuat dialog itu kadang bertanya dan kadang
          tidak. */}
      {confirmDelete && deleteImpact?.projectId ? (
        <Modal
          open
          onClose={closeDeleteConfirm}
          title="Penawaran Tidak Dapat Dihapus"
          size="sm"
          footer={
            <Button variant="secondary" onClick={closeDeleteConfirm}>
              Tutup
            </Button>
          }
        >
          <div className="flex flex-col gap-2 text-[13.5px] leading-relaxed text-text-primary">
            <p>
              Penawaran <strong>{deleteImpact.quotation.poNumber || "ini"}</strong> sudah menjadi project{" "}
              <strong>{deleteImpact.projectName}</strong> — penawaran yang sudah memiliki project tidak dapat dihapus dari sini.
            </p>
            <p className="text-text-secondary">
              Hapus project-nya terlebih dahulu bila penawaran ini memang harus hilang. Penghapusan project akan ikut menghapus
              penawaran ini beserta seluruh isi project (timeline, vendor, tagihan, pembayaran, lampiran). Tagihan terbayar saat ini:{" "}
              <strong>{deleteImpact.paidInvoiceCount}</strong> ({formatCurrency(deleteImpact.paidInvoiceTotal)}).
            </p>
          </div>
        </Modal>
      ) : (
        <ConfirmDialog
          open={confirmDelete}
          onClose={closeDeleteConfirm}
          onConfirm={() =>
            void run(async () => {
              await deleteQuotation(quotationId);
              navigate(ROUTE_PATHS.quotations);
            })
          }
          title="Hapus Penawaran Permanen"
          message={
            <>
              Yakin ingin menghapus Penawaran <strong>{deleteImpact?.quotation.poNumber || "ini"}</strong> secara permanen?
            </>
          }
          details="Tindakan ini permanen dan tidak dapat dipulihkan."
          confirmLabel="Ya, Hapus Permanen"
          busyLabel="Menghapus..."
          busy={busy}
        />
      )}

      <IncompleteProfileDialog
        open={profileGateOpen}
        onClose={() => setProfileGateOpen(false)}
        missingFields={missingProfileFields}
        docLabel="Penawaran"
      />
    </div>
  );
}

// DetailField membedakan kosong-yang-menghalangi dari kosong-yang-wajar.
// Tanggal Acara yang belum diisi menahan pengiriman; Venue yang belum diisi
// tidak. Menampilkan keduanya dengan warna yang sama membuat orang harus
// menghafal syaratnya alih-alih membacanya.
function DetailField({ label, value, required }: { label: string; value: string; required?: boolean }) {
  const empty = value.trim() === "";
  return (
    <div className="min-w-0">
      <p className="text-[11.5px] font-medium uppercase tracking-wide text-text-secondary">{label}</p>
      <p
        className={
          empty
            ? `mt-0.5 text-[13.5px] font-medium ${required ? "text-danger" : "text-text-tertiary"}`
            : "mt-0.5 text-[13.5px] font-medium text-text-primary"
        }
      >
        {empty ? (required ? "Belum diisi — wajib" : "Belum diisi") : value}
      </p>
    </div>
  );
}

function SummaryRow({ label, value, tone }: { label: string; value: string; tone?: "danger" }) {
  return (
    <div className="flex items-center justify-between gap-3">
      <span className="text-text-secondary">{label}</span>
      <span className={`tabular-nums ${tone === "danger" ? "text-danger" : "text-text-primary"}`}>{value}</span>
    </div>
  );
}
