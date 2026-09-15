import { useEffect, useState, type ReactNode } from "react";
import { Plus, Pencil, Trash2, Eye, Receipt } from "lucide-react";
import { Card, CardHeader, CardContent } from "@/shared/components/ui/Card";
import { Badge } from "@/shared/components/ui/Badge";
import { Button } from "@/shared/components/ui/Button";
import { ConfirmDialog } from "@/shared/components/ui/ConfirmDialog";
import { Modal } from "@/shared/components/ui/Modal";
import { Input, Textarea, Select, Field } from "@/shared/components/ui/Input";
import { CurrencyInput } from "@/shared/components/ui/CurrencyInput";
import { Table, THead, TBody, TR, TH, TD } from "@/shared/components/ui/Table";
import { CardList, CardListField } from "@/shared/components/ui/CardList";
import { IconActionButton } from "@/shared/components/ui/IconActionButton";
import { EvidenceListModal } from "@/shared/components/ui/EvidenceListModal";
import { Pagination } from "@/shared/components/ui/Pagination";
import { usePagination } from "@/shared/hooks/usePagination";
import { useProjectStore, ClientPaymentEvidenceError } from "@/modules/projects/stores/useProjectStore";
import { useAuthStore } from "@/shared/stores/useAuthStore";
import { useTenantBrandingStore } from "@/shared/stores/useTenantBrandingStore";
import { IncompleteProfileDialog } from "@/shared/components/IncompleteProfileDialog";
import { openPdfInNewTab } from "@/shared/lib/open-pdf";
import {
  clientPaymentSchema,
  clientPaymentUpdateSchema,
  CLIENT_PAYMENT_TYPE_OPTIONS,
  type ClientPaymentFormValues,
  type ClientPaymentUpdateFormValues,
} from "@/modules/projects/schemas/client-payment.schema";
import { PAYMENT_METHOD_OPTIONS } from "@/modules/projects/schemas/payment.schema";
import type { ClientInvoice, ClientPayment } from "@/modules/projects/types";
import { todayISO } from "@/modules/projects/lib/dates";
import { getApiErrorMessage, getApiErrorMessageFromBlob } from "@/shared/lib/api-error";
import { formatCurrency, formatDate } from "@/shared/lib/formatters";

// Simpler sibling of ProjectPaymentsSection (PLAN.md "Uang Masuk dari
// Client") — no vendor picker (nothing to pick), a single Bukti Ada/Belum
// Ada badge instead of the vendor version's two-part Lengkap/Belum Lengkap
// (there's only one evidence slot to begin with, nothing to be "partially"
// complete about), and the proof file is attached in the same submit that
// records the payment.
export function ClientPaymentsSection({ projectId }: { projectId: string }) {
  const project = useProjectStore((s) => s.currentProject);
  const clientPayments = useProjectStore((s) => s.clientPayments);
  const evidence = useProjectStore((s) => s.evidence);
  const fetchClientPayments = useProjectStore((s) => s.fetchClientPayments);
  const fetchEvidence = useProjectStore((s) => s.fetchEvidence);
  const createClientPayment = useProjectStore((s) => s.createClientPayment);
  // Tagihan dibaca di sini, bukan menumpang pada ClientInvoicesSection yang
  // kebetulan dirender di atas: seksi ini mengambil sendiri apa yang
  // dibutuhkannya, idiom yang sudah dipakai setiap seksi di modul ini.
  // markClientInvoicePaid memuat ulang keduanya setelah sukses, jadi daftar
  // pilihan di modal tidak pernah basi.
  const clientInvoices = useProjectStore((s) => s.clientInvoices);
  const fetchClientInvoices = useProjectStore((s) => s.fetchClientInvoices);
  const markClientInvoicePaid = useProjectStore((s) => s.markClientInvoicePaid);
  const updateClientPayment = useProjectStore((s) => s.updateClientPayment);
  const deleteClientPayment = useProjectStore((s) => s.deleteClientPayment);
  const downloadClientPaymentReceipt = useProjectStore((s) => s.downloadClientPaymentReceipt);
  // Owner-or-Admin only for every write action here (Tambah/Ubah/Hapus) —
  // confirmed role rule, PLAN.md mom-25082026-item-sebagian §3c. Was
  // previously `role !== "Staff"` (Delete only, Edit unrestricted), an idiom
  // that assumed Owner/Admin/Staff were the only 3 roles — that assumption
  // went stale the moment "Sales" was added, letting Sales see a Hapus
  // button and hit a 403. `canManage` now also gates Edit, matching the
  // backend's createClientPayment/updateClientPayment gates.
  const canManage = useAuthStore((s) => s.session?.role === "Owner" || s.session?.role === "Admin");
  const profileComplete = useTenantBrandingStore((s) => s.profileComplete);
  const missingProfileFields = useTenantBrandingStore((s) => s.missingProfileFields);

  const [modalOpen, setModalOpen] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);
  const [editingPayment, setEditingPayment] = useState<ClientPayment | null>(null);
  const [deletingPayment, setDeletingPayment] = useState<ClientPayment | null>(null);
  const [deleteError, setDeleteError] = useState<string | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [viewingEvidencePayment, setViewingEvidencePayment] = useState<ClientPayment | null>(null);
  const [profileGateOpen, setProfileGateOpen] = useState(false);

  useEffect(() => {
    void fetchClientPayments(projectId);
    void fetchEvidence(projectId);
    void fetchClientInvoices(projectId);
  }, [projectId, fetchClientPayments, fetchEvidence, fetchClientInvoices]);

  const contractValue = project?.contractValue ?? 0;
  const totalReceived = clientPayments.reduce(
    (sum, p) => (p.type === "Refund" ? sum - p.amount : sum + p.amount),
    0
  );
  const totalRemaining = contractValue - totalReceived;
  const { page, setPage, totalPages, totalItems, pageSize, pageItems } = usePagination(clientPayments);

  // Hanya Tagihan yang benar-benar bisa dilunasi. MarkPaid menolak yang
  // sudah Lunas maupun yang Dibatalkan, jadi menawarkannya di dropdown hanya
  // akan berujung 422 setelah formnya diisi.
  const payableInvoices = clientInvoices.filter((inv) => inv.status !== "Lunas" && inv.status !== "Dibatalkan");

  async function handleAddPayment(values: ClientPaymentFormValues) {
    setActionError(null);
    try {
      // Satu modal, dua endpoint. Memilih Tagihan mengalihkan submit ke
      // mark-paid — bukan menyalin angkanya lalu tetap membuat pembayaran
      // lepas, yang akan meninggalkan Tagihan itu berstatus "Belum Dibayar"
      // padahal uangnya sudah masuk.
      if (values.invoiceId) {
        await markClientInvoicePaid(projectId, values.invoiceId, values);
      } else {
        await createClientPayment(projectId, values);
      }
      setModalOpen(false);
    } catch (err) {
      if (err instanceof ClientPaymentEvidenceError) {
        // The payment itself was saved — close the modal so the user isn't
        // invited to resubmit (which would create a duplicate payment), just
        // surface that the proof didn't make it.
        setActionError(err.message);
        setModalOpen(false);
        return;
      }
      setActionError(getApiErrorMessage(err, "Gagal mencatat pembayaran client"));
      throw err;
    }
  }

  async function handleEditPayment(values: ClientPaymentUpdateFormValues) {
    if (!editingPayment) return;
    await updateClientPayment(projectId, editingPayment.id, values);
    setEditingPayment(null);
  }

  // Tombol "Cetak Kwitansi" disembunyikan untuk Refund (§1.11) -- arah
  // uangnya berlawanan, "Telah terima dari <client>" akan salah.
  async function handlePrintReceipt(payment: ClientPayment) {
    setActionError(null);
    if (!profileComplete) {
      setProfileGateOpen(true);
      return;
    }
    try {
      // Harus sinkron sampai baris pertama openPdfInNewTab: helper itu memesan
      // tab selagi klik masih berstatus user activation. Ejaan lama
      // `window.open(URL.createObjectURL(await ...))` mengembalikan null begitu
      // permintaan melewati jendela itu — tombolnya diam tanpa pesan apa pun.
      // Perbaikan yang sama sudah lebih dulu dilakukan di portal klien; dua
      // layar staff ini tertinggal. Lihat open-pdf.ts.
      await openPdfInNewTab(
        () => downloadClientPaymentReceipt(projectId, payment.id),
        payment.receiptNumber
          ? `Kwitansi-${payment.receiptNumber.replace(/\//g, "-")}`
          : `Kwitansi-${payment.paymentDate}`
      );
    } catch (err) {
      setActionError(await getApiErrorMessageFromBlob(err, "Gagal membuat PDF Kwitansi"));
    }
  }

  async function handleDeletePayment() {
    if (!deletingPayment) return;
    setDeleteError(null);
    setDeleting(true);
    try {
      await deleteClientPayment(projectId, deletingPayment.id);
      setDeletingPayment(null);
    } catch (err) {
      setDeleteError(getApiErrorMessage(err, "Gagal menghapus pembayaran client"));
    } finally {
      setDeleting(false);
    }
  }

  // Berkas evidence yang ikut terhapus bersama pembayaran ini — angka
  // yang harus dibaca SEBELUM memutuskan, jadi ia dihitung di sini dan
  // diserahkan ke dialog, bukan diselipkan sebagai IIFE di dalam JSX.
  const deleteEvidenceCount = deletingPayment
    ? evidence.filter((e) => e.relatedKind === "clientPayment" && e.relatedId === deletingPayment.id).length
    : 0;

  return (
    <div id="pembayaran-client">
      <Card>
        <CardHeader
          title="Uang Masuk dari Client"
          subtitle="Ringkasan nilai kontrak dan seluruh riwayat pembayaran dari client untuk project ini."
          action={
            canManage ? (
              <Button size="sm" icon={<Plus className="h-3.5 w-3.5" />} onClick={() => setModalOpen(true)}>
                Tambah Pembayaran
              </Button>
            ) : undefined
          }
        />
        <CardContent className="flex flex-col gap-5">
          {actionError && (
            <p className="rounded-md border border-danger/30 bg-danger-soft px-3.5 py-2.5 text-[13px] font-medium text-danger">{actionError}</p>
          )}
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
            <SummaryStat label="Nilai Kontrak" value={formatCurrency(contractValue)} />
            <SummaryStat label="Total Diterima" value={formatCurrency(totalReceived)} />
            <SummaryStat label="Sisa Tagihan" value={formatCurrency(totalRemaining)} />
          </div>

          {clientPayments.length === 0 ? (
            <p className="rounded-md border border-dashed border-border px-4 py-6 text-center text-[13px] text-text-secondary">
              Belum ada pembayaran client tercatat untuk project ini.
            </p>
          ) : (
            <>
            <CardList
              className="sm:hidden"
              items={pageItems}
              keyFor={(payment) => payment.id}
              renderItem={(payment) => (
                <>
                  <div className="flex items-start justify-between gap-3">
                    <span className="font-medium text-text-primary">{payment.type}</span>
                    {payment.evidenceComplete ? <Badge tone="success">Ada</Badge> : <Badge tone="warning">Belum Ada</Badge>}
                  </div>
                  <div className="flex flex-col gap-1.5">
                    <CardListField label="Nominal" value={formatCurrency(payment.amount)} />
                    <CardListField label="Tanggal" value={formatDate(payment.paymentDate)} />
                    <CardListField label="Metode" value={payment.method} />
                    <CardListField label="No. Referensi" value={payment.referenceNumber} />
                  </div>
                  <div className="flex items-center gap-2 pt-1">
                    <IconActionButton icon={Eye} label="Lihat Bukti" tone="info" onClick={() => setViewingEvidencePayment(payment)} />
                    {payment.type !== "Refund" && (
                      <IconActionButton icon={Receipt} label="Cetak Kwitansi" tone="navy" onClick={() => void handlePrintReceipt(payment)} />
                    )}
                    {canManage && (
                      <IconActionButton icon={Pencil} label="Ubah Pembayaran" tone="neutral" onClick={() => setEditingPayment(payment)} />
                    )}
                    {canManage && (
                      <IconActionButton icon={Trash2} label="Hapus Pembayaran" tone="danger" onClick={() => { setDeletingPayment(payment); setDeleteError(null); }} />
                    )}
                  </div>
                </>
              )}
            />
            <div className="hidden sm:block">
            <Table>
              <THead>
                <TR>
                  <TH>Jenis</TH>
                  <TH className="text-right">Nominal</TH>
                  <TH>Tanggal</TH>
                  <TH>Metode</TH>
                  <TH>No. Referensi</TH>
                  <TH>Bukti</TH>
                  <TH>Aksi</TH>
                </TR>
              </THead>
              <TBody>
                {pageItems.map((payment) => (
                  <TR key={payment.id}>
                    <TD className="font-medium">{payment.type}</TD>
                    <TD className="text-right tabular-nums">{formatCurrency(payment.amount)}</TD>
                    <TD>{formatDate(payment.paymentDate)}</TD>
                    <TD>{payment.method}</TD>
                    <TD>{payment.referenceNumber}</TD>
                    <TD>{payment.evidenceComplete ? <Badge tone="success">Ada</Badge> : <Badge tone="warning">Belum Ada</Badge>}</TD>
                    <TD>
                      <div className="flex items-center gap-2">
                        <IconActionButton icon={Eye} label="Lihat Bukti" tone="info" onClick={() => setViewingEvidencePayment(payment)} />
                        {payment.type !== "Refund" && (
                          <IconActionButton icon={Receipt} label="Cetak Kwitansi" tone="navy" onClick={() => void handlePrintReceipt(payment)} />
                        )}
                        {canManage && (
                          <IconActionButton icon={Pencil} label="Ubah Pembayaran" tone="neutral" onClick={() => setEditingPayment(payment)} />
                        )}
                        {canManage && (
                          <IconActionButton icon={Trash2} label="Hapus Pembayaran" tone="danger" onClick={() => { setDeletingPayment(payment); setDeleteError(null); }} />
                        )}
                      </div>
                    </TD>
                  </TR>
                ))}
              </TBody>
            </Table>
            </div>
            <Pagination
              page={page}
              totalPages={totalPages}
              totalItems={totalItems}
              pageSize={pageSize}
              onPageChange={setPage}
              className="-mx-5 -mb-4 mt-1"
            />
            </>
          )}
        </CardContent>
      </Card>

      {modalOpen && (
        <AddClientPaymentModal
          open={modalOpen}
          onClose={() => { setModalOpen(false); setActionError(null); }}
          onSubmit={handleAddPayment}
          error={actionError}
          payableInvoices={payableInvoices}
        />
      )}

      {editingPayment && (
        <EditClientPaymentModal
          open={Boolean(editingPayment)}
          payment={editingPayment}
          onClose={() => setEditingPayment(null)}
          onSubmit={handleEditPayment}
        />
      )}

      <ConfirmDialog
        open={deletingPayment !== null}
        onClose={() => setDeletingPayment(null)}
        onConfirm={() => void handleDeletePayment()}
        title="Hapus Pembayaran Client"
        message={
          <>
            Yakin ingin menghapus pembayaran client <strong>{formatCurrency(deletingPayment?.amount ?? 0)}</strong> (
            {deletingPayment?.type}) ini secara permanen?
          </>
        }
        details={deleteEvidenceCount > 0 ? `${deleteEvidenceCount} berkas evidence yang terlampir pada pembayaran ini ikut terhapus.` : undefined}
        confirmLabel="Ya, Hapus Permanen"
        busyLabel="Menghapus..."
        busy={deleting}
        error={deleteError}
      />

      {viewingEvidencePayment && (
        <EvidenceListModal
          open
          projectId={projectId}
          title={`Bukti — ${viewingEvidencePayment.type}`}
          items={evidence.filter((e) => e.relatedKind === "clientPayment" && e.relatedId === viewingEvidencePayment.id)}
          onClose={() => setViewingEvidencePayment(null)}
        />
      )}

      <IncompleteProfileDialog
        open={profileGateOpen}
        onClose={() => setProfileGateOpen(false)}
        missingFields={missingProfileFields}
        docLabel="Kwitansi"
      />
    </div>
  );
}

function SummaryStat({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-md border border-border bg-surface-muted/40 px-4 py-3">
      <p className="text-[11.5px] font-medium uppercase tracking-wide text-text-secondary">{label}</p>
      <p className="mt-1 text-[15px] font-bold tabular-nums text-navy-900">{value}</p>
    </div>
  );
}

// Menampilkan nilai yang tidak diketik pengguna dengan tinggi/bentuk yang
// sama seperti input di sebelahnya, supaya barisnya tidak melompat saat
// sebuah Tagihan dipilih.
function ReadOnlyValue({ children }: { children: ReactNode }) {
  return (
    <div className="flex h-9 items-center rounded-md border border-border bg-surface-muted px-3 text-sm font-medium text-text-primary">
      {children}
    </div>
  );
}

function emptyValues(): ClientPaymentFormValues {
  return {
    invoiceId: "",
    type: "DP",
    amount: 0,
    paymentDate: todayISO(),
    method: "Transfer Bank",
    referenceNumber: "",
    notes: "",
    proofFile: undefined,
  };
}

function AddClientPaymentModal({
  open,
  onClose,
  onSubmit,
  error,
  payableInvoices,
}: {
  open: boolean;
  onClose: () => void;
  onSubmit: (values: ClientPaymentFormValues) => Promise<void>;
  error: string | null;
  payableInvoices: ClientInvoice[];
}) {
  const [values, setValues] = useState<ClientPaymentFormValues>(emptyValues);
  const [errors, setErrors] = useState<Partial<Record<keyof ClientPaymentFormValues, string>>>({});
  const [submitting, setSubmitting] = useState(false);

  const selectedInvoice = payableInvoices.find((inv) => inv.id === values.invoiceId) ?? null;

  function set<K extends keyof ClientPaymentFormValues>(key: K, value: ClientPaymentFormValues[K]) {
    setValues((prev) => ({ ...prev, [key]: value }));
  }

  // Memilih Tagihan mengunci jenis + nominal ke nilai Tagihan itu. Keduanya
  // tetap ditulis ke state form (bukan sekadar ditampilkan) supaya validasi
  // yang sudah ada tetap berjalan apa adanya dan angka yang terlihat adalah
  // angka yang benar-benar akan tercatat. Backend memaksanya sekali lagi,
  // jadi ini kenyamanan, bukan otoritas.
  function selectInvoice(invoiceId: string) {
    const inv = payableInvoices.find((i) => i.id === invoiceId);
    setErrors({});
    if (!inv) {
      // Kembali ke "tidak terkait tagihan": jenis/nominal dikembalikan ke
      // nilai awal, bukan ditinggalkan berisi angka Tagihan yang barusan
      // dibatalkan — itu akan tercatat diam-diam sebagai pembayaran lepas
      // bernominal sama.
      const blank = emptyValues();
      setValues((prev) => ({ ...prev, invoiceId: "", type: blank.type, amount: blank.amount }));
      return;
    }
    setValues((prev) => ({ ...prev, invoiceId: inv.id, type: inv.type, amount: inv.amount }));
  }

  async function handleSubmit() {
    // Tagihan yang dipilih bisa lenyap dari daftar selagi modal terbuka
    // (dilunasi/dibatalkan dari tempat lain). Menyimpan diam-diam sebagai
    // pembayaran lepas adalah hasil terburuknya — uang tercatat, tautannya
    // hilang — jadi submitnya dihentikan dan alasannya disebutkan.
    if (values.invoiceId && !selectedInvoice) {
      setErrors({ invoiceId: "Tagihan itu sudah tidak bisa dilunasi lagi. Pilih ulang, atau lepaskan tautannya." });
      return;
    }
    const result = clientPaymentSchema.safeParse(values);
    if (!result.success) {
      const fieldErrors: Partial<Record<keyof ClientPaymentFormValues, string>> = {};
      for (const issue of result.error.issues) {
        fieldErrors[issue.path[0] as keyof ClientPaymentFormValues] = issue.message;
      }
      setErrors(fieldErrors);
      return;
    }
    setSubmitting(true);
    try {
      await onSubmit(result.data);
      // Only reset on a resolved submit — on failure the modal stays open
      // (parent doesn't close it), so the form must keep the user's input
      // rather than silently wiping it out from under them.
      setValues(emptyValues());
      setErrors({});
    } catch {
      // The `error` prop (parent's actionError, passed back down) surfaces
      // this inside the modal itself -- the modal stays open here, and its
      // own fixed-overlay backdrop would otherwise hide a banner rendered
      // only in the parent's now-obscured page content.
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="Tambah Pembayaran Client"
      description="Catat pembayaran baru dari client untuk project ini, beserta bukti transfernya jika ada."
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>Batal</Button>
          <Button onClick={() => void handleSubmit()} disabled={submitting}>
            {submitting ? "Menyimpan..." : values.invoiceId ? "Simpan & Tandai Lunas" : "Simpan Pembayaran"}
          </Button>
        </>
      }
    >
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        {error && (
          <p className="sm:col-span-2 rounded-md border border-danger/30 bg-danger-soft px-3.5 py-2.5 text-[13px] font-medium text-danger">{error}</p>
        )}

        {/* Ditawarkan hanya bila ada yang bisa dipilih — kontrol yang tidak
            pernah bisa berbuat apa-apa cuma jadi kebisingan pada project yang
            memang belum punya Tagihan. */}
        {payableInvoices.length > 0 && (
          <div className="sm:col-span-2">
            <Field
              label="Lunasi Tagihan"
              hint={
                values.invoiceId
                  ? "Jenis dan nominal mengikuti Tagihan ini, dan statusnya otomatis menjadi Lunas setelah disimpan."
                  : "Pilih bila pembayaran ini melunasi salah satu Tagihan — supaya status Tagihannya ikut berubah."
              }
            >
              <Select value={values.invoiceId} onChange={(e) => selectInvoice(e.target.value)}>
                <option value="">— Tidak terkait Tagihan —</option>
                {payableInvoices.map((inv) => (
                  <option key={inv.id} value={inv.id}>
                    {`${inv.invoiceNumber} · ${inv.type} · ${formatCurrency(inv.amount)} · jatuh tempo ${formatDate(inv.dueDate)}`}
                  </option>
                ))}
              </Select>
            </Field>
            {/* Field.hint dirender netral (abu-abu) — itu tepat untuk
                keterangan, tidak untuk kegagalan. Galat ditampilkan terpisah
                dengan gaya yang sama seperti banner galat modal ini. */}
            {errors.invoiceId && (
              <p className="mt-2 rounded-md border border-danger/30 bg-danger-soft px-3 py-2 text-[12.5px] font-medium text-danger">
                {errors.invoiceId}
              </p>
            )}
            {/* Pengaman dobel-catat: tidak ada apa pun di backend yang
                mencegah "bayar manual sekarang, Tandai Lunas nanti" untuk
                Tagihan yang sama — dan hasilnya uang tercatat dua kali. */}
            {!values.invoiceId && !errors.invoiceId && (
              <p className="mt-2 rounded-md border border-warning/30 bg-warning-soft px-3 py-2 text-[12.5px] text-warning-strong">
                Ada {payableInvoices.length} Tagihan yang belum dibayar. Kalau pembayaran ini melunasi salah satunya,
                pilih di atas — mencatatnya sebagai pembayaran lepas membuat Tagihan itu tetap berstatus belum dibayar,
                dan berisiko tercatat dua kali saat nanti ditandai Lunas.
              </p>
            )}
          </div>
        )}

        {selectedInvoice ? (
          <>
            {/* Baca-saja, bukan input yang di-disable: nilainya bukan "belum
                boleh diubah" melainkan "ditentukan Tagihan" — MarkPaid di
                backend menimpa apa pun yang dikirim UI. */}
            <Field label="Jenis Pembayaran">
              <ReadOnlyValue>{selectedInvoice.type}</ReadOnlyValue>
            </Field>
            <Field label="Nominal (Rp)">
              <ReadOnlyValue>{formatCurrency(selectedInvoice.amount)}</ReadOnlyValue>
            </Field>
          </>
        ) : (
          <>
            <Field label="Jenis Pembayaran" required>
              <Select value={values.type} onChange={(e) => set("type", e.target.value as ClientPaymentFormValues["type"])}>
                {CLIENT_PAYMENT_TYPE_OPTIONS.map((t) => (
                  <option key={t} value={t}>{t}</option>
                ))}
              </Select>
            </Field>
            <Field label="Nominal (Rp)" required hint={errors.amount}>
              <CurrencyInput value={values.amount} onChange={(n) => set("amount", n)} />
            </Field>
          </>
        )}
        <Field label="Tanggal" required hint={errors.paymentDate}>
          <Input type="date" value={values.paymentDate} onChange={(e) => set("paymentDate", e.target.value)} />
        </Field>
        <Field label="Metode" required hint={errors.method}>
          <Select value={values.method} onChange={(e) => set("method", e.target.value as ClientPaymentFormValues["method"])}>
            {PAYMENT_METHOD_OPTIONS.map((m) => (
              <option key={m} value={m}>{m}</option>
            ))}
          </Select>
        </Field>
        <Field label="No. Referensi" hint={errors.referenceNumber}>
          <Input value={values.referenceNumber} onChange={(e) => set("referenceNumber", e.target.value)} />
        </Field>
        <Field label="Bukti Transfer">
          <input
            type="file"
            accept="image/jpeg,image/png,application/pdf"
            onChange={(e) => set("proofFile", e.target.files?.[0] ?? undefined)}
            className="block w-full text-[13px] text-text-secondary file:mr-3 file:rounded-md file:border-0 file:bg-navy-900 file:px-3 file:py-1.5 file:text-[12.5px] file:font-semibold file:text-white"
          />
        </Field>
        <div className="sm:col-span-2">
          <Field label="Catatan">
            <Textarea rows={2} value={values.notes} onChange={(e) => set("notes", e.target.value)} />
          </Field>
        </div>
      </div>
    </Modal>
  );
}

// Edit scope is the ledger fields only -- no file input (already covered by
// the existing generic evidence tab), see PLAN.md §2.5.
function EditClientPaymentModal({
  open,
  payment,
  onClose,
  onSubmit,
}: {
  open: boolean;
  payment: ClientPayment;
  onClose: () => void;
  onSubmit: (values: ClientPaymentUpdateFormValues) => Promise<void>;
}) {
  const [values, setValues] = useState<ClientPaymentUpdateFormValues>({
    type: payment.type,
    amount: payment.amount,
    paymentDate: payment.paymentDate,
    method: payment.method as ClientPaymentUpdateFormValues["method"],
    referenceNumber: payment.referenceNumber,
    notes: payment.notes,
  });
  const [errors, setErrors] = useState<Partial<Record<keyof ClientPaymentUpdateFormValues, string>>>({});
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  function set<K extends keyof ClientPaymentUpdateFormValues>(key: K, value: ClientPaymentUpdateFormValues[K]) {
    setValues((prev) => ({ ...prev, [key]: value }));
  }

  async function handleSubmit() {
    const result = clientPaymentUpdateSchema.safeParse(values);
    if (!result.success) {
      const fieldErrors: Partial<Record<keyof ClientPaymentUpdateFormValues, string>> = {};
      for (const issue of result.error.issues) {
        fieldErrors[issue.path[0] as keyof ClientPaymentUpdateFormValues] = issue.message;
      }
      setErrors(fieldErrors);
      return;
    }
    setSubmitting(true);
    setError(null);
    try {
      await onSubmit(result.data);
    } catch (err) {
      setError(getApiErrorMessage(err, "Gagal memperbarui pembayaran client"));
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="Ubah Pembayaran Client"
      description="Perbarui detail pembayaran ini. Berkas evidence yang sudah terlampir tidak berubah."
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>Batal</Button>
          <Button onClick={() => void handleSubmit()} disabled={submitting}>
            {submitting ? "Menyimpan..." : "Simpan Perubahan"}
          </Button>
        </>
      }
    >
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        {error && (
          <p className="sm:col-span-2 rounded-md border border-danger/30 bg-danger-soft px-3.5 py-2.5 text-[13px] font-medium text-danger">{error}</p>
        )}
        <Field label="Jenis Pembayaran" required>
          <Select value={values.type} onChange={(e) => set("type", e.target.value as ClientPaymentUpdateFormValues["type"])}>
            {CLIENT_PAYMENT_TYPE_OPTIONS.map((t) => (
              <option key={t} value={t}>{t}</option>
            ))}
          </Select>
        </Field>
        <Field label="Nominal (Rp)" required hint={errors.amount}>
          <CurrencyInput value={values.amount} onChange={(n) => set("amount", n)} />
        </Field>
        <Field label="Tanggal" required hint={errors.paymentDate}>
          <Input type="date" value={values.paymentDate} onChange={(e) => set("paymentDate", e.target.value)} />
        </Field>
        <Field label="Metode" required hint={errors.method}>
          <Select value={values.method} onChange={(e) => set("method", e.target.value as ClientPaymentUpdateFormValues["method"])}>
            {PAYMENT_METHOD_OPTIONS.map((m) => (
              <option key={m} value={m}>{m}</option>
            ))}
          </Select>
        </Field>
        <Field label="No. Referensi" hint={errors.referenceNumber}>
          <Input value={values.referenceNumber} onChange={(e) => set("referenceNumber", e.target.value)} />
        </Field>
        <div className="sm:col-span-2">
          <Field label="Catatan">
            <Textarea rows={2} value={values.notes} onChange={(e) => set("notes", e.target.value)} />
          </Field>
        </div>
      </div>
    </Modal>
  );
}
