import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { Plus, MoreHorizontal, FileText, FileType, Trash2, LayoutTemplate } from "lucide-react";
import { Button } from "@/shared/components/ui/Button";
import { SearchInput } from "@/shared/components/ui/SearchInput";
import { Card, CardContent } from "@/shared/components/ui/Card";
import { Table, THead, TBody, TR, TH, TD } from "@/shared/components/ui/Table";
import { Pagination } from "@/shared/components/ui/Pagination";
import { DropdownMenu } from "@/shared/components/ui/DropdownMenu";
import { ConfirmDialog } from "@/shared/components/ui/ConfirmDialog";
import { EmptyState } from "@/shared/components/feedback/EmptyState";
import { useRundownStore } from "@/modules/rundowns/stores/useRundownStore";
import { useAuthStore } from "@/shared/stores/useAuthStore";
import { CreateRundownDialog } from "@/modules/rundowns/components/CreateRundownDialog";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { useDebouncedValue } from "@/shared/hooks/useDebouncedValue";
import { getApiErrorMessageFromBlob } from "@/shared/lib/api-error";
import { generateMessage } from "@/modules/rundowns/lib/generate-message";

interface Notice {
  tone: "success" | "danger";
  text: string;
  /** Project yang berkasnya baru tersimpan di tab Dokumen. */
  projectId?: string;
}

/** Waktu lokal perubahan terakhir, mis. "24 Sep 2026, 10.42". */
function formatUpdatedAt(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "-";
  return d.toLocaleString("id-ID", { day: "numeric", month: "short", year: "numeric", hour: "2-digit", minute: "2-digit" });
}

export default function RundownListPage() {
  const navigate = useNavigate();
  const list = useRundownStore((s) => s.list);
  const meta = useRundownStore((s) => s.listMeta);
  const fetchList = useRundownStore((s) => s.fetchList);
  const generate = useRundownStore((s) => s.generate);
  const remove = useRundownStore((s) => s.remove);
  const role = useAuthStore((s) => s.session?.role);
  const canDelete = role === "Owner" || role === "Admin";

  const [query, setQuery] = useState("");
  const debounced = useDebouncedValue(query);
  const [page, setPage] = useState(1);
  const [createOpen, setCreateOpen] = useState(false);
  const [deleting, setDeleting] = useState<{ id: string; name: string } | null>(null);
  const [busyId, setBusyId] = useState("");
  const [notice, setNotice] = useState<Notice | null>(null);

  useEffect(() => {
    void fetchList(page, debounced);
  }, [fetchList, page, debounced]);

  useEffect(() => {
    setPage(1);
  }, [debounced]);

  const onGenerate = async (id: string, projectId: string, format: "docx" | "pdf") => {
    setBusyId(id);
    setNotice(null);
    try {
      const { archive } = await generate(id, format);
      const msg = generateMessage(format, archive);
      setNotice({ tone: msg.tone, text: msg.text, projectId: msg.documentsLink ? projectId : undefined });
    } catch (e) {
      // Respons galat datang sebagai Blob karena permintaannya
      // responseType: "blob" -- dibaca dengan helper khusus itu, kalau tidak
      // pesannya keluar sebagai "[object Blob]".
      setNotice({ tone: "danger", text: await getApiErrorMessageFromBlob(e, "Gagal membuat berkas rundown") });
    } finally {
      setBusyId("");
    }
  };

  return (
    <div className="flex flex-col gap-5">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold text-text-primary">Rundown</h1>
          <p className="text-[13px] text-text-secondary">
            Buku acara hari-H. Satu project satu rundown.
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button
            variant="secondary"
            icon={<LayoutTemplate className="h-4 w-4" />}
            onClick={() => navigate(ROUTE_PATHS.rundownTemplate())}
          >
            Template Rundown
          </Button>
          <Button icon={<Plus className="h-4 w-4" />} onClick={() => setCreateOpen(true)}>
            Buat Rundown
          </Button>
        </div>
      </div>

      <SearchInput
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        placeholder="Cari project atau nama pengantin..."
      />

      {notice && (
        <div
          role="status"
          className={
            notice.tone === "success"
              ? "rounded-md border border-success/30 bg-success/5 px-3 py-2 text-[13px] text-success"
              : "rounded-md border border-danger/30 bg-danger/5 px-3 py-2 text-[13px] text-danger"
          }
        >
          {notice.text}{" "}
          {notice.projectId && (
            <Link
              to={ROUTE_PATHS.projectDetail(notice.projectId, "dokumen")}
              className="font-medium underline underline-offset-2"
            >
              Lihat di Dokumen project
            </Link>
          )}
        </div>
      )}

      <Card>
        <CardContent className="p-0">
          {list.length === 0 ? (
            <EmptyState
              title="Belum ada rundown"
              description="Buat rundown dari salah satu project untuk mulai menyusun buku acara."
            />
          ) : (
            <Table>
              <THead>
                <TR>
                  <TH>Project</TH>
                  <TH>Pengantin</TH>
                  <TH>Tanggal</TH>
                  <TH>Venue</TH>
                  <TH>Diubah</TH>
                  <TH className="w-16 text-right">Aksi</TH>
                </TR>
              </THead>
              <TBody>
                {list.map((r) => (
                  <TR key={r.id}>
                    <TD>
                      <button
                        type="button"
                        className="text-left font-medium text-text-primary hover:underline"
                        onClick={() => navigate(ROUTE_PATHS.rundownDetail(r.id))}
                      >
                        {r.projectName || "(tanpa nama)"}
                      </button>
                    </TD>
                    <TD>{[r.brideName, r.groomName].filter(Boolean).join(" & ") || "-"}</TD>
                    <TD>{r.eventDateLabel || "-"}</TD>
                    <TD>{r.venueLabel || "-"}</TD>
                    <TD className="whitespace-nowrap text-text-secondary">{formatUpdatedAt(r.updatedAt)}</TD>
                    <TD className="text-right">
                      <DropdownMenu
                        label={`Aksi rundown ${r.projectName}`}
                        trigger={
                          <span className="rounded-md p-1.5 text-text-secondary hover:bg-surface-muted">
                            <MoreHorizontal className="h-4 w-4" />
                          </span>
                        }
                        items={[
                          {
                            label: busyId === r.id ? "Menyiapkan..." : "Unduh PDF",
                            icon: <FileText className="h-4 w-4" />,
                            disabled: busyId === r.id,
                            onSelect: () => void onGenerate(r.id, r.projectId, "pdf"),
                          },
                          {
                            label: busyId === r.id ? "Menyiapkan..." : "Unduh DOCX",
                            icon: <FileType className="h-4 w-4" />,
                            disabled: busyId === r.id,
                            onSelect: () => void onGenerate(r.id, r.projectId, "docx"),
                          },
                          ...(canDelete
                            ? [
                                {
                                  label: "Hapus",
                                  icon: <Trash2 className="h-4 w-4" />,
                                  tone: "danger" as const,
                                  onSelect: () => setDeleting({ id: r.id, name: r.projectName }),
                                },
                              ]
                            : []),
                        ]}
                      />
                    </TD>
                  </TR>
                ))}
              </TBody>
            </Table>
          )}
        </CardContent>
      </Card>

      {meta && meta.total_pages > 1 && (
        <Pagination
          page={meta.page}
          totalPages={meta.total_pages}
          totalItems={meta.total}
          pageSize={meta.limit}
          onPageChange={setPage}
        />
      )}

      <CreateRundownDialog
        open={createOpen}
        onClose={() => setCreateOpen(false)}
        onCreated={(id) => {
          setCreateOpen(false);
          navigate(ROUTE_PATHS.rundownDetail(id));
        }}
      />

      <ConfirmDialog
        open={deleting !== null}
        onClose={() => setDeleting(null)}
        onConfirm={async () => {
          if (!deleting) return;
          await remove(deleting.id);
          setDeleting(null);
        }}
        title="Hapus rundown"
        message={`Buku acara "${deleting?.name ?? ""}" akan dihapus permanen beserta denah yang diunggah. Project-nya sendiri tidak tersentuh.`}
        confirmLabel="Hapus"
        tone="danger"
      />
    </div>
  );
}
