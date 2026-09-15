import { useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { Plus } from "lucide-react";
import { Button } from "@/shared/components/ui/Button";
import { SearchInput } from "@/shared/components/ui/SearchInput";
import { Select } from "@/shared/components/ui/Input";
import { Pagination } from "@/shared/components/ui/Pagination";
import { EmptyState } from "@/shared/components/feedback/EmptyState";
import { Card, CardContent } from "@/shared/components/ui/Card";
import { Badge, type BadgeTone } from "@/shared/components/ui/Badge";
import { Modal } from "@/shared/components/ui/Modal";
import { Field, Input } from "@/shared/components/ui/Input";
import { useQuotationStore } from "@/modules/quotations/stores/useQuotationStore";
import { useClientStore } from "@/modules/clients/stores/useClientStore";
import { usePackageTemplateStore } from "@/modules/package-templates/stores/usePackageTemplateStore";
import { QUOTATION_STATUS_OPTIONS } from "@/modules/quotations/schemas/quotation.schema";
import type { QuotationStatus } from "@/modules/quotations/types";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { getApiErrorMessage } from "@/shared/lib/api-error";
import { useDebouncedValue } from "@/shared/hooks/useDebouncedValue";
import { formatCurrency, formatDate } from "@/shared/lib/formatters";

const STATUS_TONE: Record<QuotationStatus, BadgeTone> = {
  Draft: "neutral",
  Ditawarkan: "info",
  Diterima: "success",
  Ditolak: "danger",
  Kedaluwarsa: "warning",
  Dibatalkan: "danger",
};

export default function QuotationListPage() {
  const navigate = useNavigate();
  const page = useQuotationStore((s) => s.quotationPage);
  const meta = useQuotationStore((s) => s.quotationPageMeta);
  const fetchQuotationPage = useQuotationStore((s) => s.fetchQuotationPage);
  const createQuotation = useQuotationStore((s) => s.createQuotation);
  const clients = useClientStore((s) => s.clients);
  const fetchClients = useClientStore((s) => s.fetchClients);
  const templates = usePackageTemplateStore((s) => s.templates);
  const fetchTemplates = usePackageTemplateStore((s) => s.fetchTemplates);

  const [query, setQuery] = useState("");
  const debouncedQuery = useDebouncedValue(query);
  const [statusFilter, setStatusFilter] = useState("Semua");
  const [pageNum, setPageNum] = useState(1);
  const [createOpen, setCreateOpen] = useState(false);
  const [clientChoice, setClientChoice] = useState("");
  const [templateChoice, setTemplateChoice] = useState("");
  const [packageName, setPackageName] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const filters = useMemo(
    () => ({ status: statusFilter === "Semua" ? "" : statusFilter, search: debouncedQuery }),
    [statusFilter, debouncedQuery]
  );

  useEffect(() => {
    setPageNum(1);
  }, [debouncedQuery, statusFilter]);

  useEffect(() => {
    void fetchQuotationPage(pageNum, filters);
  }, [fetchQuotationPage, pageNum, filters]);

  useEffect(() => {
    if (createOpen) {
      void fetchClients(1, "", 100);
      void fetchTemplates(true);
    }
  }, [createOpen, fetchClients, fetchTemplates]);

  // Mengganti template mengisi Nama Paket, TAPI hanya bila field-nya masih
  // persis sama dengan nama template sebelumnya — artinya pengguna belum
  // menyentuhnya. Sekali diketik manual, template tidak pernah menimpanya
  // lagi. Selalu-timpa akan membuang ketikan orang tanpa peringatan;
  // tidak-pernah-timpa membuat salah pilih template mustahil diperbaiki
  // tanpa mengosongkan field sendiri.
  function chooseTemplate(nextId: string) {
    const previousName = templates.find((t) => t.id === templateChoice)?.name ?? "";
    const untouched = packageName.trim() === "" || packageName === previousName;
    setTemplateChoice(nextId);
    if (untouched) {
      setPackageName(templates.find((t) => t.id === nextId)?.name ?? "");
    }
  }

  async function handleCreate() {
    if (!clientChoice) return;
    setBusy(true);
    setError(null);
    try {
      const quotation = await createQuotation({
        clientId: clientChoice,
        templateId: templateChoice,
        packageName,
        eventDate: "",
        pax: 0,
        venueId: null,
      });
      setCreateOpen(false);
      setClientChoice("");
      setTemplateChoice("");
      setPackageName("");
      navigate(ROUTE_PATHS.quotationDetail(quotation.id));
    } catch (err) {
      setError(getApiErrorMessage(err, "Gagal membuat penawaran"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="flex flex-col gap-5">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-xl font-bold text-text-primary">Penawaran</h1>
          <p className="mt-1 text-[13px] text-text-secondary">
            PO pra-deal: susun penawaran, kirim ke klien, dan biarkan project lahir otomatis begitu diterima.
          </p>
        </div>
        <Button icon={<Plus className="h-4 w-4" />} onClick={() => setCreateOpen(true)}>
          Tambah Penawaran
        </Button>
      </div>

      <div className="flex flex-wrap gap-3">
        <SearchInput
          className="max-w-xs"
          placeholder="Cari nomor PO..."
          value={query}
          onChange={(e) => setQuery(e.target.value)}
        />
        <Select className="w-48" value={statusFilter} onChange={(e) => setStatusFilter(e.target.value)}>
          <option value="Semua">Semua Status</option>
          {QUOTATION_STATUS_OPTIONS.map((s) => (
            <option key={s} value={s}>{s}</option>
          ))}
        </Select>
      </div>

      {page.length === 0 ? (
        <div className="rounded-lg border border-border bg-surface">
          <EmptyState
            title="Tidak ada penawaran ditemukan"
            description="Buat penawaran pertama dari client yang sudah ada."
          />
        </div>
      ) : (
        <>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3">
            {page.map((item) => (
              <Card key={item.id}>
                <CardContent className="flex flex-col gap-2 py-4">
                  <div className="flex items-start justify-between gap-2">
                    <div className="min-w-0">
                      <p className="truncate text-[15px] font-semibold text-text-primary">
                        {item.poNumber || "Draft tanpa nomor"}
                        {item.revision > 0 && <span className="text-text-secondary"> · Rev {item.revision}</span>}
                      </p>
                      <p className="truncate text-[13px] text-text-secondary">{item.clientName || "—"}</p>
                    </div>
                    {/* T7: Diterima tanpa project diturunkan di tempatnya —
                        bukan status baru. D13a: revisi berproject yang belum
                        berTTD ikut ditandai. */}
                    <span className="flex shrink-0 flex-wrap justify-end gap-1">
                      <Badge tone={item.status === "Diterima" && !item.projectId ? "warning" : STATUS_TONE[item.status]}>
                        {item.status === "Diterima" && !item.projectId ? "Diterima · Perlu dibuatkan project" : item.status}
                      </Badge>
                      {item.status === "Diterima" && !!item.projectId && !item.signed && (
                        <Badge tone="warning">Revisi belum ditandatangani</Badge>
                      )}
                    </span>
                  </div>
                  <div className="flex items-center justify-between text-[13px]">
                    <span className="text-text-secondary">
                      {item.eventDate ? formatDate(item.eventDate) : "Tanggal menyusul"}
                    </span>
                    <span className="font-semibold tabular-nums text-text-primary">{formatCurrency(item.basePrice)}</span>
                  </div>
                  <div className="flex gap-2 pt-1">
                    <Button size="sm" variant="secondary" onClick={() => navigate(ROUTE_PATHS.quotationDetail(item.id))} className="flex-1">
                      Buka
                    </Button>
                    {item.projectId && (
                      <Button size="sm" variant="ghost" onClick={() => navigate(ROUTE_PATHS.projectDetail(item.projectId))}>
                        Project →
                      </Button>
                    )}
                  </div>
                </CardContent>
              </Card>
            ))}
          </div>
          <div className="rounded-lg border border-border bg-surface">
            <Pagination page={meta.page} totalPages={meta.totalPages} totalItems={meta.total} pageSize={meta.limit} onPageChange={setPageNum} />
          </div>
        </>
      )}

      <Modal open={createOpen} onClose={() => setCreateOpen(false)} title="Tambah Penawaran">
        <div className="flex flex-col gap-3">
          <Field label="Client" htmlFor="q-new-client" required hint="Satu client boleh punya banyak penawaran.">
            <Select value={clientChoice} onChange={(e) => setClientChoice(e.target.value)}>
              <option value="">Pilih client</option>
              {clients.map((c) => (
                <option key={c.id} value={c.id}>
                  {c.displayName}
                </option>
              ))}
            </Select>
          </Field>
          <Field label="Template Paket" htmlFor="q-new-template" hint="Opsional — harganya hanya nilai awal (Harga Standar), bisa diubah di penawaran.">
            <Select value={templateChoice} onChange={(e) => chooseTemplate(e.target.value)}>
              <option value="">Mulai kosong</option>
              {templates.map((t) => (
                <option key={t.id} value={t.id}>
                  {t.name} — {formatCurrency(t.basePrice)}
                </option>
              ))}
            </Select>
          </Field>
          <Field
            label="Nama Paket"
            htmlFor="q-new-package-name"
            hint="Tercetak di PO dan Invoice, dan menjadi Paket / Layanan pada project. Wajib diisi sebelum penawaran dikirim."
          >
            <Input
              id="q-new-package-name"
              value={packageName}
              placeholder="Silver"
              onChange={(e) => setPackageName(e.target.value)}
            />
          </Field>
          {error && <p className="text-[13px] text-danger">{error}</p>}
          <div className="flex justify-end gap-2">
            <Button variant="ghost" onClick={() => setCreateOpen(false)}>
              Batal
            </Button>
            <Button disabled={!clientChoice || busy} onClick={() => void handleCreate()}>
              {busy ? "Membuat..." : "Buat Penawaran"}
            </Button>
          </div>
        </div>
      </Modal>
    </div>
  );
}
