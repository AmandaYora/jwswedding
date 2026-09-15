import { useEffect, useState } from "react";
import { Plus, Pencil, Trash2, Package } from "lucide-react";
import { Card, CardHeader, CardContent } from "@/shared/components/ui/Card";
import { Badge } from "@/shared/components/ui/Badge";
import { Button } from "@/shared/components/ui/Button";
import { Modal } from "@/shared/components/ui/Modal";
import { Input, Textarea, Select, Field } from "@/shared/components/ui/Input";
import { CurrencyInput } from "@/shared/components/ui/CurrencyInput";
import { IconActionButton } from "@/shared/components/ui/IconActionButton";
import {
  usePackageTemplateStore,
  type BlockDraft,
  type PackageTemplateSubmitValues,
} from "@/modules/package-templates/stores/usePackageTemplateStore";
import type { PackageTemplate, PackageTemplateSummary } from "@/modules/package-templates/types";
import { getApiErrorMessage } from "@/shared/lib/api-error";
import { formatCurrency } from "@/shared/lib/formatters";

const EMPTY_TEMPLATE: PackageTemplateSubmitValues = {
  name: "",
  basePrice: 0,
  defaultTerms: "",
  defaultBonusNote: "",
  isActive: true,
};

// Draft rows carry a local key rather than being addressed by array index, so
// deleting row 2 does not hand row 3's text to row 2's textarea mid-edit.
let rowSeq = 0;
const nextRowKey = () => `row-${(rowSeq += 1)}`;

type BlockRow = BlockDraft & { key: string };

/**
 * A failed save leaves its modal open, so the message has to render INSIDE
 * the modal — the card behind it is covered by the overlay, and an error shown
 * only there reads as "the button did nothing".
 */
function ErrorNote({ message }: { message: string }) {
  return (
    <p
      role="alert"
      className="rounded-md border border-danger/30 bg-danger-soft px-3.5 py-2.5 text-[13px] font-medium text-danger"
    >
      {message}
    </p>
  );
}

/**
 * Pengaturan → Template Paket.
 *
 * A template is what makes the PO worth having: its blocks are typed once here
 * and copied into every deal, so the composition editor below accepts a whole
 * category as pasted text rather than asking for sixty separate rows.
 *
 * List rows are summaries — a count, no composition. The editor loads the
 * template by id when it opens, so what it shows, and therefore what its
 * whole-list PUT writes back, is always what is actually stored.
 */
export default function PackageTemplateListPage() {
  const templates = usePackageTemplateStore((s) => s.templates);
  const loading = usePackageTemplateStore((s) => s.loading);
  const fetchTemplates = usePackageTemplateStore((s) => s.fetchTemplates);
  const getTemplate = usePackageTemplateStore((s) => s.getTemplate);
  const createTemplate = usePackageTemplateStore((s) => s.createTemplate);
  const updateTemplate = usePackageTemplateStore((s) => s.updateTemplate);
  const deleteTemplate = usePackageTemplateStore((s) => s.deleteTemplate);
  const saveBlocks = usePackageTemplateStore((s) => s.saveBlocks);

  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  /** Which row's editor is being fetched, e.g. "blocks:7" — drives its label. */
  const [opening, setOpening] = useState<string | null>(null);
  const [formDraft, setFormDraft] = useState<{ id: string | null; values: PackageTemplateSubmitValues } | null>(null);
  const [blocksFor, setBlocksFor] = useState<PackageTemplate | null>(null);
  const [blockRows, setBlockRows] = useState<BlockRow[]>([]);
  const [confirmDelete, setConfirmDelete] = useState<PackageTemplateSummary | null>(null);

  useEffect(() => {
    void fetchTemplates();
  }, [fetchTemplates]);

  // Closing a modal drops its error with it — left behind, the message would
  // resurface on the card detached from the action that caused it.
  function dismiss(close: () => void) {
    return () => {
      close();
      setError(null);
    };
  }

  async function run(action: () => Promise<void>) {
    setBusy(true);
    setError(null);
    try {
      await action();
    } catch (err) {
      setError(getApiErrorMessage(err, "Gagal menyimpan template"));
    } finally {
      setBusy(false);
    }
  }

  // The editor reads the template fresh rather than from the list row: the
  // list carries a count only, and the PUT below replaces the entire block
  // list, so editing anything but the stored rows would overwrite them.
  async function openEditor(summary: PackageTemplateSummary) {
    setError(null);
    setOpening(`blocks:${summary.id}`);
    try {
      const full = await getTemplate(summary.id);
      setBlocksFor(full);
      setBlockRows(
        full.blocks.map(({ category, body, qtyText, bonusNote }) => ({
          key: nextRowKey(),
          category,
          body,
          qtyText,
          bonusNote,
        })),
      );
    } catch (err) {
      setError(getApiErrorMessage(err, "Gagal memuat template"));
    } finally {
      setOpening(null);
    }
  }

  // Saving an empty editor is a legitimate way to clear a template, but it
  // must never happen unnoticed — the warning and the button say so outright.
  const clearingBlocks = blocksFor !== null && blockRows.length === 0 && blocksFor.blocks.length > 0;

  function submitBlocks() {
    if (!blocksFor) return;
    const blank = blockRows.findIndex((r) => !r.category.trim());
    if (blank !== -1) {
      setError(`Rincian ke-${blank + 1} belum diberi kategori.`);
      return;
    }
    void run(async () => {
      await saveBlocks(
        blocksFor.id,
        blockRows.map(({ category, body, qtyText, bonusNote }) => ({ category, body, qtyText, bonusNote })),
      );
      setBlocksFor(null);
    });
  }

  const modalOpen = formDraft !== null || blocksFor !== null || confirmDelete !== null;

  return (
    <div className="flex flex-col gap-4">
      <Card>
        <CardHeader
          title="Template Paket"
          subtitle="Daftar paket yang Anda jual. Dipilih saat membuat project baru."
          action={
            <Button size="sm" icon={<Plus className="h-3.5 w-3.5" />} onClick={() => setFormDraft({ id: null, values: EMPTY_TEMPLATE })}>
              Tambah paket
            </Button>
          }
        />
        <CardContent>
          {error && !modalOpen && (
            <div className="mb-3">
              <ErrorNote message={error} />
            </div>
          )}
          {loading && templates.length === 0 ? (
            <p className="py-10 text-center text-[13px] text-text-secondary">Memuat template...</p>
          ) : templates.length === 0 ? (
            <div className="flex flex-col items-center gap-3 py-12 text-center">
              <Package className="h-10 w-10 text-text-secondary" aria-hidden />
              <div>
                <p className="text-[15px] font-semibold text-text-primary">Belum ada template paket</p>
                <p className="mt-1 text-[13px] text-text-secondary">
                  Buat satu paket untuk setiap pilihan yang Anda tawarkan, lalu isi rinciannya sekali saja.
                </p>
              </div>
              <Button icon={<Plus className="h-3.5 w-3.5" />} onClick={() => setFormDraft({ id: null, values: EMPTY_TEMPLATE })}>
                Tambah paket
              </Button>
            </div>
          ) : (
            <ul className="flex flex-col gap-2">
              {templates.map((t) => (
                <li key={t.id} className="flex flex-wrap items-center gap-3 rounded-lg border border-border bg-surface p-3">
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2">
                      <p className="truncate text-[14px] font-semibold text-text-primary">{t.name}</p>
                      {!t.isActive && <Badge tone="neutral">Nonaktif</Badge>}
                    </div>
                    <p className="mt-0.5 text-[13px] text-text-secondary">
                      {formatCurrency(t.basePrice)} · {t.blockCount} rincian
                    </p>
                  </div>
                  <div className="flex items-center gap-2">
                    <Button size="sm" variant="secondary" disabled={opening !== null} onClick={() => void openEditor(t)}>
                      {opening === `blocks:${t.id}` ? "Memuat..." : "Isi Paket"}
                    </Button>
                    <IconActionButton
                      icon={Pencil}
                      label="Ubah paket"
                      tone="neutral"
                      onClick={() =>
                        setFormDraft({
                          id: t.id,
                          values: {
                            name: t.name,
                            basePrice: t.basePrice,
                            defaultTerms: t.defaultTerms,
                            defaultBonusNote: t.defaultBonusNote,
                            isActive: t.isActive,
                          },
                        })
                      }
                    />
                    <IconActionButton icon={Trash2} label="Hapus paket" tone="danger" onClick={() => setConfirmDelete(t)} />
                  </div>
                </li>
              ))}
            </ul>
          )}
        </CardContent>
      </Card>

      {/* --- Modal: identitas paket --- */}
      <Modal
        open={formDraft !== null}
        onClose={dismiss(() => setFormDraft(null))}
        title={formDraft?.id === null ? "Tambah Paket" : "Ubah Paket"}
        size="lg"
        footer={
          <>
            <Button variant="ghost" onClick={dismiss(() => setFormDraft(null))}>
              Batal
            </Button>
            <Button
              disabled={busy || !formDraft?.values.name.trim()}
              onClick={() => {
                if (!formDraft) return;
                void run(async () => {
                  if (formDraft.id === null) await createTemplate(formDraft.values);
                  else await updateTemplate(formDraft.id, formDraft.values);
                  setFormDraft(null);
                });
              }}
            >
              Simpan paket
            </Button>
          </>
        }
      >
        {formDraft && (
          <div className="flex flex-col gap-3">
            {error && <ErrorNote message={error} />}
            <Field label="Nama paket" htmlFor="pt-name" required>
              <Input
                id="pt-name"
                value={formDraft.values.name}
                placeholder="Paket Platinum 800 Pax"
                onChange={(e) => setFormDraft({ ...formDraft, values: { ...formDraft.values, name: e.target.value } })}
              />
            </Field>
            <Field
              label="Harga Standar"
              htmlFor="pt-price"
              hint="Nilai awal saat paket dipilih di penawaran — Sales/Admin/Owner boleh mengubahnya di tiap penawaran."
            >
              <CurrencyInput
                id="pt-price"
                value={formDraft.values.basePrice}
                onChange={(v) => setFormDraft({ ...formDraft, values: { ...formDraft.values, basePrice: v } })}
              />
            </Field>
            <Field label="Syarat & ketentuan bawaan" htmlFor="pt-terms">
              <Textarea
                id="pt-terms"
                rows={10}
                value={formDraft.values.defaultTerms}
                onChange={(e) => setFormDraft({ ...formDraft, values: { ...formDraft.values, defaultTerms: e.target.value } })}
              />
            </Field>
            <Field label="Bonus tambahan bawaan" htmlFor="pt-bonus">
              <Textarea
                id="pt-bonus"
                rows={5}
                value={formDraft.values.defaultBonusNote}
                onChange={(e) => setFormDraft({ ...formDraft, values: { ...formDraft.values, defaultBonusNote: e.target.value } })}
              />
            </Field>
            <Field label="Status" htmlFor="pt-active" hint="Paket nonaktif tidak ditawarkan saat membuat project baru.">
              <Select
                value={formDraft.values.isActive ? "aktif" : "nonaktif"}
                onChange={(e) =>
                  setFormDraft({ ...formDraft, values: { ...formDraft.values, isActive: e.target.value === "aktif" } })
                }
              >
                <option value="aktif">Aktif</option>
                <option value="nonaktif">Nonaktif</option>
              </Select>
            </Field>
          </div>
        )}
      </Modal>

      {/* --- Modal: isi paket --- */}
      <Modal
        open={blocksFor !== null}
        onClose={dismiss(() => setBlocksFor(null))}
        title={`Isi Paket — ${blocksFor?.name ?? ""}`}
        size="lg"
        footer={
          <>
            <Button variant="ghost" onClick={dismiss(() => setBlocksFor(null))}>
              Batal
            </Button>
            <Button variant={clearingBlocks ? "danger" : "primary"} disabled={busy} onClick={submitBlocks}>
              {clearingBlocks ? "Hapus semua rincian" : "Simpan isi paket"}
            </Button>
          </>
        }
      >
        <div className="flex flex-col gap-3">
          {error && <ErrorNote message={error} />}
          {clearingBlocks && (
            <ErrorNote
              message={`Daftar ini kosong. Menyimpan sekarang akan menghapus ${blocksFor?.blocks.length} rincian yang tersimpan.`}
            />
          )}
          <p className="text-[13px] text-text-secondary">
            Isi paket yang akan tercetak di PO. Tempel daftar item langsung dari Excel; baris yang diketik HURUF
            KAPITAL akan dicetak tebal.
          </p>
          {blockRows.map((row, i) => (
            <div key={row.key} className="rounded-lg border border-border p-3">
              <div className="mb-2 flex items-center justify-between gap-2">
                <Input
                  value={row.category}
                  placeholder="Kategori, misalnya CATERING"
                  onChange={(e) => setBlockRows(blockRows.map((r, j) => (j === i ? { ...r, category: e.target.value } : r)))}
                />
                <IconActionButton
                  icon={Trash2}
                  label="Hapus rincian"
                  tone="danger"
                  onClick={() => setBlockRows(blockRows.filter((_, j) => j !== i))}
                />
              </div>
              <Textarea
                rows={6}
                value={row.body}
                placeholder={"BUFFET\nNasi Putih\nAneka Nasi Goreng"}
                onChange={(e) => setBlockRows(blockRows.map((r, j) => (j === i ? { ...r, body: e.target.value } : r)))}
              />
              <div className="mt-2 grid gap-2 sm:grid-cols-2">
                <Textarea
                  rows={3}
                  value={row.qtyText}
                  placeholder="Jumlah, misalnya 700 PORSI"
                  onChange={(e) => setBlockRows(blockRows.map((r, j) => (j === i ? { ...r, qtyText: e.target.value } : r)))}
                />
                <Textarea
                  rows={3}
                  value={row.bonusNote}
                  placeholder="Bonus"
                  onChange={(e) => setBlockRows(blockRows.map((r, j) => (j === i ? { ...r, bonusNote: e.target.value } : r)))}
                />
              </div>
            </div>
          ))}
          <Button
            variant="secondary"
            icon={<Plus className="h-3.5 w-3.5" />}
            onClick={() => setBlockRows([...blockRows, { key: nextRowKey(), category: "", body: "", qtyText: "", bonusNote: "" }])}
          >
            Tambah rincian
          </Button>
        </div>
      </Modal>

      <Modal
        open={confirmDelete !== null}
        onClose={dismiss(() => setConfirmDelete(null))}
        title="Hapus template paket?"
        footer={
          <>
            <Button variant="ghost" onClick={dismiss(() => setConfirmDelete(null))}>
              Batal
            </Button>
            <Button
              variant="danger"
              disabled={busy}
              onClick={() => {
                if (!confirmDelete) return;
                void run(async () => {
                  await deleteTemplate(confirmDelete.id);
                  setConfirmDelete(null);
                });
              }}
            >
              Hapus
            </Button>
          </>
        }
      >
        <div className="flex flex-col gap-3">
          {error && <ErrorNote message={error} />}
          <p className="text-[13px] text-text-primary">
            <strong>{confirmDelete?.name}</strong> akan dihapus beserta isi dan terminnya. PO project yang sudah
            memakainya tidak terpengaruh.
          </p>
        </div>
      </Modal>
    </div>
  );
}
