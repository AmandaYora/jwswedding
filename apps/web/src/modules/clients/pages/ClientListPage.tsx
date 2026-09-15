import { useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { Plus } from "lucide-react";
import { Avatar } from "@/shared/components/ui/Avatar";
import { SearchInput } from "@/shared/components/ui/SearchInput";
import { Input, Textarea, Field } from "@/shared/components/ui/Input";
import { Modal } from "@/shared/components/ui/Modal";
import { Button } from "@/shared/components/ui/Button";
import { Pagination } from "@/shared/components/ui/Pagination";
import { EmptyState } from "@/shared/components/feedback/EmptyState";
import { Card, CardContent } from "@/shared/components/ui/Card";
import { useClientStore } from "@/modules/clients/stores/useClientStore";
import type { ClientMasterFormValues } from "@/modules/clients/schemas/client.schema";
import { getApiErrorMessage } from "@/shared/lib/api-error";
import { useDebouncedValue } from "@/shared/hooks/useDebouncedValue";
import { ROUTE_PATHS } from "@/app/routes/route-paths";

export default function ClientListPage() {
  const navigate = useNavigate();
  const clients = useClientStore((s) => s.clients);
  const meta = useClientStore((s) => s.clientsMeta);
  const fetchClients = useClientStore((s) => s.fetchClients);
  const createClient = useClientStore((s) => s.createClient);

  const [query, setQuery] = useState("");
  const debouncedQuery = useDebouncedValue(query);
  const [page, setPage] = useState(1);
  const [createOpen, setCreateOpen] = useState(false);
  const [values, setValues] = useState<ClientMasterFormValues>({
    brideName: "",
    groomName: "",
    phone: "",
    email: "",
    notes: "",
  });
  const [busy, setBusy] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);

  const search = useMemo(() => debouncedQuery.trim(), [debouncedQuery]);

  useEffect(() => {
    setPage(1);
  }, [search]);

  useEffect(() => {
    void fetchClients(page, search);
  }, [fetchClients, page, search]);

  async function handleCreate() {
    if (!values.brideName.trim() || !values.groomName.trim()) {
      setActionError("Nama kedua mempelai wajib diisi");
      return;
    }
    setBusy(true);
    setActionError(null);
    try {
      const client = await createClient(values);
      setCreateOpen(false);
      setValues({ brideName: "", groomName: "", phone: "", email: "", notes: "" });
      navigate(ROUTE_PATHS.clientDetail(client.id));
    } catch (err) {
      setActionError(getApiErrorMessage(err, "Gagal menambah client"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="flex flex-col gap-5">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-xl font-bold text-text-primary">Client</h1>
          <p className="mt-1 text-[13px] text-text-secondary">
            Master pasangan — satu baris per pasangan, lepas dari project. Kontak dan akun portal dikelola di dalam tiap client.
          </p>
        </div>
        <Button icon={<Plus className="h-4 w-4" />} onClick={() => setCreateOpen(true)}>
          Tambah Client
        </Button>
      </div>

      <div className="flex flex-wrap gap-3">
        <SearchInput
          className="max-w-xs"
          placeholder="Cari nama pasangan atau telepon..."
          value={query}
          onChange={(e) => setQuery(e.target.value)}
        />
      </div>

      {clients.length === 0 ? (
        <div className="rounded-xl border border-border bg-surface">
          <EmptyState title="Tidak ada client ditemukan" description="Tambah pasangan pertama, atau ubah kata kunci pencarian." />
        </div>
      ) : (
        <>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3">
            {clients.map((c) => (
              <Card key={c.id}>
                <CardContent className="flex items-center gap-3 py-4">
                  <Avatar name={c.displayName} />
                  <div className="min-w-0 flex-1">
                    <button
                      type="button"
                      onClick={() => navigate(ROUTE_PATHS.clientDetail(c.id))}
                      className="truncate text-left text-[15px] font-semibold text-text-primary hover:text-navy-900 hover:underline"
                    >
                      {c.displayName}
                    </button>
                    <p className="truncate text-[12.5px] text-text-secondary">
                      {c.phone || "Tanpa telepon"} · {c.contactCount} kontak · {c.projectCount} project
                    </p>
                  </div>
                </CardContent>
              </Card>
            ))}
          </div>
          <div className="rounded-xl border border-border bg-surface">
            <Pagination page={meta.page} totalPages={meta.totalPages} totalItems={meta.total} pageSize={meta.limit} onPageChange={setPage} />
          </div>
        </>
      )}

      <Modal open={createOpen} onClose={() => setCreateOpen(false)} title="Tambah Client">
        <div className="flex flex-col gap-3">
          <div className="grid gap-3 sm:grid-cols-2">
            <Field label="Mempelai Wanita" htmlFor="client-bride" required>
              <Input id="client-bride" value={values.brideName} onChange={(e) => setValues({ ...values, brideName: e.target.value })} />
            </Field>
            <Field label="Mempelai Pria" htmlFor="client-groom" required>
              <Input id="client-groom" value={values.groomName} onChange={(e) => setValues({ ...values, groomName: e.target.value })} />
            </Field>
          </div>
          <div className="grid gap-3 sm:grid-cols-2">
            <Field label="Telepon" htmlFor="client-phone">
              <Input id="client-phone" value={values.phone} onChange={(e) => setValues({ ...values, phone: e.target.value })} />
            </Field>
            <Field label="Email" htmlFor="client-email">
              <Input id="client-email" value={values.email} onChange={(e) => setValues({ ...values, email: e.target.value })} />
            </Field>
          </div>
          <Field label="Catatan" htmlFor="client-notes">
            <Textarea id="client-notes" rows={3} value={values.notes} onChange={(e) => setValues({ ...values, notes: e.target.value })} />
          </Field>
          {actionError && <p className="text-[13px] text-danger">{actionError}</p>}
          <div className="flex justify-end gap-2">
            <Button variant="ghost" onClick={() => setCreateOpen(false)}>
              Batal
            </Button>
            <Button disabled={busy} onClick={() => void handleCreate()}>
              {busy ? "Menyimpan..." : "Simpan"}
            </Button>
          </div>
        </div>
      </Modal>
    </div>
  );
}
