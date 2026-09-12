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
  type TermDraft,
  type PackageTemplateSubmitValues,
} from "@/modules/package-templates/stores/usePackageTemplateStore";
import type { PackageTemplate, PackageTermType } from "@/modules/package-templates/types";
import { getApiErrorMessage } from "@/shared/lib/api-error";
import { formatCurrency } from "@/shared/lib/formatters";

const EMPTY_TEMPLATE: PackageTemplateSubmitValues = {
  name: "",
  basePrice: 0,
  defaultTerms: "",
  defaultBonusNote: "",
  isActive: true,
};

const TERM_TYPES: PackageTermType[] = ["DP", "Termin", "Pelunasan"];

/**
 * Pengaturan → Template Paket.
 *
 * A template is what makes the PO worth having: its blocks and payment
 * schedule are typed once here and copied into every deal, so the composition
 * editor below accepts a whole category as pasted text rather than asking for
 * sixty separate rows.
 */
export default function PackageTemplateListPage() {
  const templates = usePackageTemplateStore((s) => s.templates);
  const loading = usePackageTemplateStore((s) => s.loading);
  const fetchTemplates = usePackageTemplateStore((s) => s.fetchTemplates);
  const createTemplate = usePackageTemplateStore((s) => s.createTemplate);
  const updateTemplate = usePackageTemplateStore((s) => s.updateTemplate);
  const deleteTemplate = usePackageTemplateStore((s) => s.deleteTemplate);
  const saveBlocks = usePackageTemplateStore((s) => s.saveBlocks);
  const saveTerms = usePackageTemplateStore((s) => s.saveTerms);

  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [formDraft, setFormDraft] = useState<{ id: string | null; values: PackageTemplateSubmitValues } | null>(null);
  const [blocksFor, setBlocksFor] = useState<PackageTemplate | null>(null);
  const [blockRows, setBlockRows] = useState<BlockDraft[]>([]);
  const [termsFor, setTermsFor] = useState<PackageTemplate | null>(null);
  const [termRows, setTermRows] = useState<TermDraft[]>([]);
  const [confirmDelete, setConfirmDelete] = useState<PackageTemplate | null>(null);

  useEffect(() => {
    void fetchTemplates();
  }, [fetchTemplates]);

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
          {error && <p className="mb-3 text-[13px] text-danger">{error}</p>}
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
                      {formatCurrency(t.basePrice)} · {t.blocks.length} rincian · {t.terms.length} termin
                    </p>
                  </div>
                  <div className="flex items-center gap-2">
                    <Button
                      size="sm"
                      variant="secondary"
                      onClick={() => {
                        setBlocksFor(t);
                        setBlockRows(t.blocks.map(({ category, body, qtyText, bonusNote }) => ({ category, body, qtyText, bonusNote })));
                      }}
                    >
                      Isi Paket
                    </Button>
                    <Button
                      size="sm"
                      variant="secondary"
                      onClick={() => {
                        setTermsFor(t);
                        setTermRows(
                          t.terms.map(({ label, type, percent, fixedAmount, daysBeforeEvent }) => ({
                            label,
                            type,
                            percent,
                            fixedAmount,
                            daysBeforeEvent,
                          })),
                        );
                      }}
                    >
                      Termin
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
        onClose={() => setFormDraft(null)}
        title={formDraft?.id === null ? "Tambah Paket" : "Ubah Paket"}
        size="lg"
        footer={
          <>
            <Button variant="ghost" onClick={() => setFormDraft(null)}>
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
            <Field label="Nama paket" htmlFor="pt-name" required>
              <Input
                id="pt-name"
                value={formDraft.values.name}
                placeholder="Paket Platinum 800 Pax"
                onChange={(e) => setFormDraft({ ...formDraft, values: { ...formDraft.values, name: e.target.value } })}
              />
            </Field>
            <Field
              label="Harga paket"
              htmlFor="pt-price"
              hint="Harga standar paket ini. Masih bisa disesuaikan di setiap project."
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
        onClose={() => setBlocksFor(null)}
        title={`Isi Paket — ${blocksFor?.name ?? ""}`}
        size="lg"
        footer={
          <>
            <Button variant="ghost" onClick={() => setBlocksFor(null)}>
              Batal
            </Button>
            <Button
              disabled={busy}
              onClick={() => {
                if (!blocksFor) return;
                void run(async () => {
                  await saveBlocks(blocksFor.id, blockRows);
                  setBlocksFor(null);
                });
              }}
            >
              Simpan isi paket
            </Button>
          </>
        }
      >
        <div className="flex flex-col gap-3">
          <p className="text-[13px] text-text-secondary">
            Isi paket yang akan tercetak di PO. Tempel daftar item langsung dari Excel; baris yang diketik HURUF
            KAPITAL akan dicetak tebal.
          </p>
          {blockRows.map((row, i) => (
            <div key={i} className="rounded-lg border border-border p-3">
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
            onClick={() => setBlockRows([...blockRows, { category: "", body: "", qtyText: "", bonusNote: "" }])}
          >
            Tambah rincian
          </Button>
        </div>
      </Modal>

      {/* --- Modal: preset termin --- */}
      <Modal
        open={termsFor !== null}
        onClose={() => setTermsFor(null)}
        title={`Termin — ${termsFor?.name ?? ""}`}
        size="lg"
        footer={
          <>
            <Button variant="ghost" onClick={() => setTermsFor(null)}>
              Batal
            </Button>
            <Button
              disabled={busy}
              onClick={() => {
                if (!termsFor) return;
                void run(async () => {
                  await saveTerms(termsFor.id, termRows);
                  setTermsFor(null);
                });
              }}
            >
              Simpan termin
            </Button>
          </>
        }
      >
        <div className="flex flex-col gap-3">
          <p className="text-[13px] text-text-secondary">
            Nominal setiap tahap dihitung otomatis dari total tiap project.
          </p>
          {termRows.map((row, i) => (
            <div key={i} className="grid items-end gap-2 rounded-lg border border-border p-3 sm:grid-cols-[1fr_auto_auto_auto_auto]">
              <Field label="Label" htmlFor={`term-label-${i}`}>
                <Input
                  id={`term-label-${i}`}
                  value={row.label}
                  placeholder="Pembayaran 1"
                  onChange={(e) => setTermRows(termRows.map((r, j) => (j === i ? { ...r, label: e.target.value } : r)))}
                />
              </Field>
              <Field label="Jenis" htmlFor={`term-type-${i}`}>
                <Select
                  value={row.type}
                  onChange={(e) => setTermRows(termRows.map((r, j) => (j === i ? { ...r, type: e.target.value as PackageTermType } : r)))}
                >
                  {TERM_TYPES.map((t) => (
                    <option key={t} value={t}>
                      {t}
                    </option>
                  ))}
                </Select>
              </Field>
              <Field label="Persen" htmlFor={`term-pct-${i}`}>
                <Input
                  id={`term-pct-${i}`}
                  type="number"
                  value={row.percent ?? ""}
                  placeholder="30"
                  onChange={(e) =>
                    setTermRows(
                      termRows.map((r, j) =>
                        j === i ? { ...r, percent: e.target.value === "" ? null : Number(e.target.value), fixedAmount: null } : r,
                      ),
                    )
                  }
                />
              </Field>
              <Field label="Nominal tetap" htmlFor={`term-fixed-${i}`}>
                <Input
                  id={`term-fixed-${i}`}
                  type="number"
                  value={row.fixedAmount ?? ""}
                  placeholder="10000000"
                  onChange={(e) =>
                    setTermRows(
                      termRows.map((r, j) =>
                        j === i ? { ...r, fixedAmount: e.target.value === "" ? null : Number(e.target.value), percent: null } : r,
                      ),
                    )
                  }
                />
              </Field>
              <div className="flex items-end gap-2">
                <Field label="Jatuh tempo" htmlFor={`term-days-${i}`}>
                  <Input
                    id={`term-days-${i}`}
                    type="number"
                    value={row.daysBeforeEvent}
                    onChange={(e) =>
                      setTermRows(termRows.map((r, j) => (j === i ? { ...r, daysBeforeEvent: Number(e.target.value) } : r)))
                    }
                  />
                </Field>
                <IconActionButton
                  icon={Trash2}
                  label="Hapus termin"
                  tone="danger"
                  onClick={() => setTermRows(termRows.filter((_, j) => j !== i))}
                />
              </div>
            </div>
          ))}
          <Button
            variant="secondary"
            icon={<Plus className="h-3.5 w-3.5" />}
            onClick={() =>
              setTermRows([...termRows, { label: "", type: "Termin", percent: null, fixedAmount: null, daysBeforeEvent: 30 }])
            }
          >
            Tambah termin
          </Button>
        </div>
      </Modal>

      <Modal
        open={confirmDelete !== null}
        onClose={() => setConfirmDelete(null)}
        title="Hapus template paket?"
        footer={
          <>
            <Button variant="ghost" onClick={() => setConfirmDelete(null)}>
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
        <p className="text-[13px] text-text-primary">
          <strong>{confirmDelete?.name}</strong> akan dihapus beserta isi dan terminnya. PO project yang sudah
          memakainya tidak terpengaruh.
        </p>
      </Modal>
    </div>
  );
}
