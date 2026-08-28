import { useEffect, useState } from "react";
import { Plus, Pencil, Trash2, CheckCircle2, Undo2, FileDown } from "lucide-react";
import { Card, CardHeader, CardContent } from "@/shared/components/ui/Card";
import { Badge, type BadgeTone } from "@/shared/components/ui/Badge";
import { Button } from "@/shared/components/ui/Button";
import { Modal } from "@/shared/components/ui/Modal";
import { Input, Textarea, Select, Field } from "@/shared/components/ui/Input";
import { CurrencyInput } from "@/shared/components/ui/CurrencyInput";
import { Table, THead, TBody, TR, TH, TD } from "@/shared/components/ui/Table";
import { CardList, CardListField } from "@/shared/components/ui/CardList";
import { IconActionButton } from "@/shared/components/ui/IconActionButton";
import { Pagination } from "@/shared/components/ui/Pagination";
import { usePagination } from "@/shared/hooks/usePagination";
import { useProjectStore, ClientPaymentEvidenceError } from "@/modules/projects/stores/useProjectStore";
import { useAuthStore } from "@/shared/stores/useAuthStore";
import { useTenantBrandingStore } from "@/shared/stores/useTenantBrandingStore";
import { IncompleteProfileDialog } from "@/shared/components/IncompleteProfileDialog";
import {
  clientInvoiceSchema,
  markInvoicePaidSchema,
  CLIENT_INVOICE_TYPE_OPTIONS,
  type ClientInvoiceFormValues,
  type MarkInvoicePaidFormValues,
} from "@/modules/projects/schemas/client-invoice.schema";
import { PAYMENT_METHOD_OPTIONS } from "@/modules/projects/schemas/payment.schema";
import type { ClientInvoice, InvoiceStatus } from "@/modules/projects/types";
import { todayISO } from "@/modules/projects/lib/dates";
import { getApiErrorMessage, getApiErrorMessageFromBlob } from "@/shared/lib/api-error";
import { formatCurrency, formatDate } from "@/shared/lib/formatters";

const STATUS_TONE: Record<InvoiceStatus, BadgeTone> = {
  Draft: "neutral",
  Terkirim: "info",
  Lunas: "success",
  Dibatalkan: "danger",
};

// Sits above ClientPaymentsSection on the "Pembayaran → Client" tab (PLAN.md
// invoice-kwitansi-client) — Tagihan (bills) the WO issues, distinct from
// the actual money-received ledger below it. Marking one "Lunas" creates a
// row there automatically.
export function ClientInvoicesSection({ projectId }: { projectId: string }) {
  const invoices = useProjectStore((s) => s.clientInvoices);
  const fetchClientInvoices = useProjectStore((s) => s.fetchClientInvoices);
  const createClientInvoice = useProjectStore((s) => s.createClientInvoice);
  const updateClientInvoice = useProjectStore((s) => s.updateClientInvoice);
  const deleteClientInvoice = useProjectStore((s) => s.deleteClientInvoice);
  const markClientInvoicePaid = useProjectStore((s) => s.markClientInvoicePaid);
  const unmarkClientInvoicePaid = useProjectStore((s) => s.unmarkClientInvoicePaid);
  const downloadClientInvoicePDF = useProjectStore((s) => s.downloadClientInvoicePDF);
  // Owner-or-Admin only for every write action here (Tambah/Ubah/Tandai
  // Lunas/Batalkan Pelunasan/Hapus) — confirmed role rule, PLAN.md
  // mom-25082026-item-sebagian §3c, matching the backend's
  // createClientInvoice/updateClientInvoice/markClientInvoicePaid/
  // unmarkClientInvoicePaid/deleteClientInvoice gates. Was previously
  // `canDelete`, scoped only to the Hapus button — the idiom itself
  // (role === "Owner" || "Admin", not the stale `!== "Staff"` pattern
  // ClientPaymentsSection used to carry) was already correct, so only the
  // name and its reach widen here.
  const role = useAuthStore((s) => s.session?.role);
  const canManage = role === "Owner" || role === "Admin";
  const profileComplete = useTenantBrandingStore((s) => s.profileComplete);
  const missingProfileFields = useTenantBrandingStore((s) => s.missingProfileFields);

  const [modalOpen, setModalOpen] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);
  const [editingInvoice, setEditingInvoice] = useState<ClientInvoice | null>(null);
  const [payingInvoice, setPayingInvoice] = useState<ClientInvoice | null>(null);
  const [unmarkingInvoice, setUnmarkingInvoice] = useState<ClientInvoice | null>(null);
  const [profileGateOpen, setProfileGateOpen] = useState(false);
  const [unmarking, setUnmarking] = useState(false);
  const [unmarkError, setUnmarkError] = useState<string | null>(null);
  const [deletingInvoice, setDeletingInvoice] = useState<ClientInvoice | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [deleteError, setDeleteError] = useState<string | null>(null);

  useEffect(() => {
    void fetchClientInvoices(projectId);
  }, [projectId, fetchClientInvoices]);

  const { page, setPage, totalPages, totalItems, pageSize, pageItems } = usePagination(invoices);

  async function handleAddInvoice(values: ClientInvoiceFormValues) {
    setActionError(null);
    try {
      await createClientInvoice(projectId, values);
      setModalOpen(false);
    } catch (err) {
      setActionError(getApiErrorMessage(err, "Gagal membuat Tagihan"));
      throw err;
    }
  }

  async function handleEditInvoice(values: ClientInvoiceFormValues) {
    if (!editingInvoice) return;
    await updateClientInvoice(projectId, editingInvoice.id, values);
    setEditingInvoice(null);
  }

  async function handleMarkPaid(values: MarkInvoicePaidFormValues) {
    if (!payingInvoice) return;
    try {
      await markClientInvoicePaid(projectId, payingInvoice.id, values);
      setPayingInvoice(null);
    } catch (err) {
      if (err instanceof ClientPaymentEvidenceError) {
        setActionError(err.message);
        setPayingInvoice(null);
        return;
      }
      throw err;
    }
  }

  async function handleUnmarkPaid() {
    if (!unmarkingInvoice) return;
    setUnmarkError(null);
    setUnmarking(true);
    try {
      await unmarkClientInvoicePaid(projectId, unmarkingInvoice.id);
      setUnmarkingInvoice(null);
    } catch (err) {
      setUnmarkError(getApiErrorMessage(err, "Gagal membatalkan pelunasan"));
    } finally {
      setUnmarking(false);
    }
  }

  async function handleDeleteInvoice() {
    if (!deletingInvoice) return;
    setDeleteError(null);
    setDeleting(true);
    try {
      await deleteClientInvoice(projectId, deletingInvoice.id);
      setDeletingInvoice(null);
    } catch (err) {
      setDeleteError(getApiErrorMessage(err, "Gagal menghapus Tagihan"));
    } finally {
      setDeleting(false);
    }
  }

  async function handlePrintPDF(invoice: ClientInvoice) {
    setActionError(null);
    if (!profileComplete) {
      setProfileGateOpen(true);
      return;
    }
    try {
      const blob = await downloadClientInvoicePDF(projectId, invoice.id);
      window.open(URL.createObjectURL(blob), "_blank");
    } catch (err) {
      setActionError(await getApiErrorMessageFromBlob(err, "Gagal membuat PDF Tagihan"));
    }
  }

  function rowActions(invoice: ClientInvoice) {
    const isPaid = invoice.status === "Lunas";
    return (
      <div className="flex items-center gap-1">
        {!isPaid && canManage && (
          <>
            <IconActionButton icon={Pencil} label="Ubah Tagihan" tone="neutral" onClick={() => setEditingInvoice(invoice)} />
            <IconActionButton icon={CheckCircle2} label="Tandai Lunas" tone="success" onClick={() => setPayingInvoice(invoice)} />
          </>
        )}
        {isPaid && canManage && (
          <IconActionButton icon={Undo2} label="Batalkan Pelunasan" tone="danger" onClick={() => setUnmarkingInvoice(invoice)} />
        )}
        <IconActionButton icon={FileDown} label="Cetak PDF" tone="info" onClick={() => void handlePrintPDF(invoice)} />
        {canManage && (
          <IconActionButton
            icon={Trash2}
            label="Hapus Tagihan"
            tone="danger"
            disabled={isPaid}
            onClick={() => { setDeletingInvoice(invoice); setDeleteError(null); }}
          />
        )}
      </div>
    );
  }

  return (
    <div id="tagihan-client">
      <Card>
        <CardHeader
          title="Tagihan (Invoice)"
          subtitle="Tagihan yang diterbitkan ke client. Menandai Lunas otomatis mencatatnya di riwayat pembayaran di bawah."
          action={
            canManage ? (
              <Button size="sm" icon={<Plus className="h-3.5 w-3.5" />} onClick={() => setModalOpen(true)}>
                Tambah Tagihan
              </Button>
            ) : undefined
          }
        />
        <CardContent className="flex flex-col gap-4">
          {actionError && (
            <p className="rounded-md border border-danger/30 bg-danger-soft px-3.5 py-2.5 text-[13px] font-medium text-danger">{actionError}</p>
          )}

          {invoices.length === 0 ? (
            <p className="rounded-md border border-dashed border-border px-4 py-6 text-center text-[13px] text-text-secondary">
              Belum ada Tagihan untuk project ini.
            </p>
          ) : (
            <>
            <CardList
              className="sm:hidden"
              items={pageItems}
              keyFor={(inv) => inv.id}
              renderItem={(inv) => (
                <>
                  <div className="flex items-start justify-between gap-3">
                    <span className="font-medium text-text-primary">{inv.invoiceNumber}</span>
                    <Badge tone={STATUS_TONE[inv.status]}>{inv.status}</Badge>
                  </div>
                  <div className="flex flex-col gap-1.5">
                    <CardListField label="Jenis" value={inv.type} />
                    <CardListField label="Jumlah" value={formatCurrency(inv.amount)} />
                    <CardListField label="Jatuh Tempo" value={formatDate(inv.dueDate)} />
                  </div>
                  <div className="pt-1">{rowActions(inv)}</div>
                </>
              )}
            />
            <div className="hidden sm:block">
            <Table>
              <THead>
                <TR>
                  <TH>No. Invoice</TH>
                  <TH>Jenis</TH>
                  <TH className="text-right">Jumlah</TH>
                  <TH>Jatuh Tempo</TH>
                  <TH>Status</TH>
                  <TH>Aksi</TH>
                </TR>
              </THead>
              <TBody>
                {pageItems.map((inv) => (
                  <TR key={inv.id}>
                    <TD className="font-medium">{inv.invoiceNumber}</TD>
                    <TD>{inv.type}</TD>
                    <TD className="text-right tabular-nums">{formatCurrency(inv.amount)}</TD>
                    <TD>{formatDate(inv.dueDate)}</TD>
                    <TD><Badge tone={STATUS_TONE[inv.status]}>{inv.status}</Badge></TD>
                    <TD>{rowActions(inv)}</TD>
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
        <InvoiceFormModal
          open={modalOpen}
          title="Tambah Tagihan"
          submitLabel="Simpan Tagihan"
          onClose={() => { setModalOpen(false); setActionError(null); }}
          onSubmit={handleAddInvoice}
          error={actionError}
        />
      )}

      {editingInvoice && (
        <InvoiceFormModal
          key={editingInvoice.id}
          open
          title="Ubah Tagihan"
          submitLabel="Simpan Perubahan"
          initial={editingInvoice}
          onClose={() => setEditingInvoice(null)}
          onSubmit={handleEditInvoice}
        />
      )}

      {payingInvoice && (
        <MarkPaidModal
          key={payingInvoice.id}
          open
          invoice={payingInvoice}
          onClose={() => setPayingInvoice(null)}
          onSubmit={handleMarkPaid}
        />
      )}

      {unmarkingInvoice && (
        <Modal
          open
          onClose={() => setUnmarkingInvoice(null)}
          title="Batalkan Pelunasan"
          description={`Tagihan ${unmarkingInvoice.invoiceNumber} akan dikembalikan ke status "Terkirim".`}
          footer={
            <>
              <Button variant="secondary" onClick={() => setUnmarkingInvoice(null)} disabled={unmarking}>Batal</Button>
              <Button variant="danger" onClick={() => void handleUnmarkPaid()} disabled={unmarking}>
                {unmarking ? "Memproses..." : "Ya, Batalkan Pelunasan"}
              </Button>
            </>
          }
        >
          <div className="flex flex-col gap-3">
            {unmarkError && (
              <p className="rounded-md border border-danger/30 bg-danger-soft px-3.5 py-2.5 text-[13px] font-medium text-danger">{unmarkError}</p>
            )}
            <p className="text-[13.5px] text-text-primary">
              Pembayaran yang tercatat dari pelunasan ini <strong>beserta bukti transfernya akan dihapus</strong>. Tindakan ini tidak
              dapat dibatalkan.
            </p>
          </div>
        </Modal>
      )}

      {deletingInvoice && (
        <Modal
          open
          onClose={() => setDeletingInvoice(null)}
          title="Hapus Tagihan"
          description="Tindakan ini permanen dan tidak dapat dibatalkan."
          footer={
            <>
              <Button variant="secondary" onClick={() => setDeletingInvoice(null)} disabled={deleting}>Batal</Button>
              <Button variant="danger" onClick={() => void handleDeleteInvoice()} disabled={deleting}>
                {deleting ? "Menghapus..." : "Ya, Hapus Permanen"}
              </Button>
            </>
          }
        >
          <div className="flex flex-col gap-3">
            {deleteError && (
              <p className="rounded-md border border-danger/30 bg-danger-soft px-3.5 py-2.5 text-[13px] font-medium text-danger">{deleteError}</p>
            )}
            <p className="text-[13.5px] text-text-primary">
              Yakin ingin menghapus Tagihan <strong>{deletingInvoice.invoiceNumber}</strong> secara permanen?
            </p>
          </div>
        </Modal>
      )}

      <IncompleteProfileDialog
        open={profileGateOpen}
        onClose={() => setProfileGateOpen(false)}
        missingFields={missingProfileFields}
        docLabel="Tagihan"
      />
    </div>
  );
}

function emptyValues(): ClientInvoiceFormValues {
  return { type: "DP", description: "", amount: 0, dueDate: todayISO() };
}

function InvoiceFormModal({
  open,
  title,
  submitLabel,
  initial,
  onClose,
  onSubmit,
  error,
}: {
  open: boolean;
  title: string;
  submitLabel: string;
  initial?: ClientInvoice;
  onClose: () => void;
  onSubmit: (values: ClientInvoiceFormValues) => Promise<void>;
  error?: string | null;
}) {
  const [values, setValues] = useState<ClientInvoiceFormValues>(() =>
    initial
      ? { type: initial.type as ClientInvoiceFormValues["type"], description: initial.description, amount: initial.amount, dueDate: initial.dueDate }
      : emptyValues()
  );
  const [errors, setErrors] = useState<Partial<Record<keyof ClientInvoiceFormValues, string>>>({});
  const [localError, setLocalError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  function set<K extends keyof ClientInvoiceFormValues>(key: K, value: ClientInvoiceFormValues[K]) {
    setValues((prev) => ({ ...prev, [key]: value }));
  }

  async function handleSubmit() {
    const result = clientInvoiceSchema.safeParse(values);
    if (!result.success) {
      const fieldErrors: Partial<Record<keyof ClientInvoiceFormValues, string>> = {};
      for (const issue of result.error.issues) {
        fieldErrors[issue.path[0] as keyof ClientInvoiceFormValues] = issue.message;
      }
      setErrors(fieldErrors);
      return;
    }
    setSubmitting(true);
    setLocalError(null);
    try {
      await onSubmit(result.data);
      if (!initial) setValues(emptyValues());
      setErrors({});
    } catch (err) {
      if (!initial) return; // parent's `error` prop already surfaces create failures
      setLocalError(getApiErrorMessage(err, "Gagal menyimpan Tagihan"));
    } finally {
      setSubmitting(false);
    }
  }

  const shownError = initial ? localError : error;

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={title}
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>Batal</Button>
          <Button onClick={() => void handleSubmit()} disabled={submitting}>
            {submitting ? "Menyimpan..." : submitLabel}
          </Button>
        </>
      }
    >
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        {shownError && (
          <p className="sm:col-span-2 rounded-md border border-danger/30 bg-danger-soft px-3.5 py-2.5 text-[13px] font-medium text-danger">{shownError}</p>
        )}
        <Field label="Jenis Tagihan" required>
          <Select value={values.type} onChange={(e) => set("type", e.target.value as ClientInvoiceFormValues["type"])}>
            {CLIENT_INVOICE_TYPE_OPTIONS.map((t) => (
              <option key={t} value={t}>{t}</option>
            ))}
          </Select>
        </Field>
        <Field label="Nominal (Rp)" required hint={errors.amount}>
          <CurrencyInput value={values.amount} onChange={(n) => set("amount", n)} />
        </Field>
        <Field label="Jatuh Tempo" required hint={errors.dueDate}>
          <Input type="date" value={values.dueDate} onChange={(e) => set("dueDate", e.target.value)} />
        </Field>
        <div className="sm:col-span-2">
          <Field label="Keterangan">
            <Textarea rows={2} value={values.description} onChange={(e) => set("description", e.target.value)} />
          </Field>
        </div>
      </div>
    </Modal>
  );
}

function MarkPaidModal({
  open,
  invoice,
  onClose,
  onSubmit,
}: {
  open: boolean;
  invoice: ClientInvoice;
  onClose: () => void;
  onSubmit: (values: MarkInvoicePaidFormValues) => Promise<void>;
}) {
  const [values, setValues] = useState<MarkInvoicePaidFormValues>({
    paymentDate: todayISO(), method: "Transfer Bank", referenceNumber: "", notes: "", proofFile: undefined,
  });
  const [errors, setErrors] = useState<Partial<Record<keyof MarkInvoicePaidFormValues, string>>>({});
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  function set<K extends keyof MarkInvoicePaidFormValues>(key: K, value: MarkInvoicePaidFormValues[K]) {
    setValues((prev) => ({ ...prev, [key]: value }));
  }

  async function handleSubmit() {
    const result = markInvoicePaidSchema.safeParse(values);
    if (!result.success) {
      const fieldErrors: Partial<Record<keyof MarkInvoicePaidFormValues, string>> = {};
      for (const issue of result.error.issues) {
        fieldErrors[issue.path[0] as keyof MarkInvoicePaidFormValues] = issue.message;
      }
      setErrors(fieldErrors);
      return;
    }
    setSubmitting(true);
    setError(null);
    try {
      await onSubmit(result.data);
    } catch (err) {
      setError(getApiErrorMessage(err, "Gagal menandai Tagihan Lunas"));
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="Tandai Lunas"
      description={`Tagihan ${invoice.invoiceNumber} (${formatCurrency(invoice.amount)}) akan tercatat sebagai pembayaran diterima.`}
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>Batal</Button>
          <Button onClick={() => void handleSubmit()} disabled={submitting}>
            {submitting ? "Menyimpan..." : "Tandai Lunas"}
          </Button>
        </>
      }
    >
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        {error && (
          <p className="sm:col-span-2 rounded-md border border-danger/30 bg-danger-soft px-3.5 py-2.5 text-[13px] font-medium text-danger">{error}</p>
        )}
        <Field label="Tanggal Bayar" required hint={errors.paymentDate}>
          <Input type="date" value={values.paymentDate} onChange={(e) => set("paymentDate", e.target.value)} />
        </Field>
        <Field label="Metode" required hint={errors.method}>
          <Select value={values.method} onChange={(e) => set("method", e.target.value as MarkInvoicePaidFormValues["method"])}>
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
