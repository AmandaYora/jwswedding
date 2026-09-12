import { useEffect, useMemo, useState } from "react";
import { FileText, Pencil, Plus, Trash2, Send, RotateCcw, Ban, PackageOpen } from "lucide-react";
import { Card, CardHeader, CardContent } from "@/shared/components/ui/Card";
import { Badge, type BadgeTone } from "@/shared/components/ui/Badge";
import { Button } from "@/shared/components/ui/Button";
import { Modal } from "@/shared/components/ui/Modal";
import { Input, Textarea, Select, Field } from "@/shared/components/ui/Input";
import { CurrencyInput } from "@/shared/components/ui/CurrencyInput";
import { IconActionButton } from "@/shared/components/ui/IconActionButton";
import { useAuthStore } from "@/shared/stores/useAuthStore";
import { useTenantBrandingStore } from "@/shared/stores/useTenantBrandingStore";
import { IncompleteProfileDialog } from "@/shared/components/IncompleteProfileDialog";
import { usePackageOrderStore, type AdjustmentInput, type BlockInput } from "@/modules/projects/stores/usePackageOrderStore";
import { usePackageTemplateStore } from "@/modules/package-templates/stores/usePackageTemplateStore";
import type { PackageBlock, PackageOrderStatus } from "@/modules/package-templates/types";
import { getApiErrorMessage, getApiErrorMessageFromBlob } from "@/shared/lib/api-error";
import { openPdfInNewTab } from "@/shared/lib/open-pdf";
import { formatCurrency, formatDate } from "@/shared/lib/formatters";
import { httpClient } from "@/shared/services/http-client";
import { API } from "@/shared/services/api-endpoints";

const STATUS_TONE: Record<PackageOrderStatus, BadgeTone> = {
  Draft: "neutral",
  Terbit: "success",
  Dibatalkan: "danger",
};

/**
 * The "Paket & PO" tab (PLAN.md po-paket-client §5.6).
 *
 * Laid out to mirror the printed document top to bottom — composition, then
 * adjustments, then terms — with a live summary alongside. The consultant is
 * usually editing this with a client in front of them, so there is no
 * tab-within-tab: everything a negotiation touches is on one scroll.
 */
export function PackageOrderSection({ projectId }: { projectId: string }) {
  const order = usePackageOrderStore((s) => s.order);
  const loading = usePackageOrderStore((s) => s.loading);
  const fetchOrder = usePackageOrderStore((s) => s.fetchOrder);
  const applyTemplate = usePackageOrderStore((s) => s.applyTemplate);
  const startBlank = usePackageOrderStore((s) => s.startBlank);
  const saveHeader = usePackageOrderStore((s) => s.saveHeader);
  const saveBlocks = usePackageOrderStore((s) => s.saveBlocks);
  const saveAdjustments = usePackageOrderStore((s) => s.saveAdjustments);
  const issue = usePackageOrderStore((s) => s.issue);
  const revise = usePackageOrderStore((s) => s.revise);
  const cancelOrder = usePackageOrderStore((s) => s.cancel);
  const resetOrder = usePackageOrderStore((s) => s.reset);

  const templates = usePackageTemplateStore((s) => s.templates);
  const fetchTemplates = usePackageTemplateStore((s) => s.fetchTemplates);

  const role = useAuthStore((s) => s.session?.role);
  const canManage = role === "Owner" || role === "Admin";
  const profileComplete = useTenantBrandingStore((s) => s.profileComplete);
  const missingProfileFields = useTenantBrandingStore((s) => s.missingProfileFields);

  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [profileGateOpen, setProfileGateOpen] = useState(false);
  const [blockDraft, setBlockDraft] = useState<{ index: number | null; value: BlockInput } | null>(null);
  const [adjustDraft, setAdjustDraft] = useState<{ index: number | null; description: string; amount: number; isTakeout: boolean } | null>(null);
  const [headerDraft, setHeaderDraft] = useState<{ basePrice: number; termsText: string; bonusNote: string } | null>(null);
  const [confirmIssue, setConfirmIssue] = useState(false);
  const [templateChoice, setTemplateChoice] = useState("");

  useEffect(() => {
    // Clear before fetching: the store is module-level, so navigating from a
    // project that has a PO to one that does not would otherwise show the
    // previous project's order for the duration of the request — and
    // ProjectHeaderCard reads the same flag to lock its Nilai Kontrak field.
    resetOrder();
    void fetchOrder(projectId);
  }, [projectId, fetchOrder, resetOrder]);

  useEffect(() => {
    if (!order && canManage) void fetchTemplates(true);
  }, [order, canManage, fetchTemplates]);

  const editable = order?.status === "Draft";
  const categories = useMemo(
    () => Array.from(new Set((order?.blocks ?? []).map((b) => b.category).filter(Boolean))),
    [order],
  );

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

  function toBlockInputs(blocks: PackageBlock[]): BlockInput[] {
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
        const res = await httpClient.get(API.projects.packageOrderPdf(projectId), { responseType: "blob" });
        return res.data as Blob;
      }, order?.poNumber ? order.poNumber.replace(/\//g, "-") : "PO-Paket");
    } catch (err) {
      setError(await getApiErrorMessageFromBlob(err, "Gagal membuat PDF PO"));
    }
  }

  if (loading && !order) {
    return <p className="py-10 text-center text-[13px] text-text-secondary">Memuat PO Paket...</p>;
  }

  // --- Empty state (D23) -------------------------------------------------
  // Every project that predates this feature lands here, so the copy has to
  // answer the question that actually stops someone pressing the button:
  // what happens to the contract value already on this project.
  if (!order) {
    return (
      <Card>
        <CardContent className="flex flex-col items-center gap-4 py-12 text-center">
          <PackageOpen className="h-10 w-10 text-text-secondary" aria-hidden />
          <div>
            <p className="text-[15px] font-semibold text-text-primary">Project ini belum punya PO Paket</p>
            <p className="mt-1 text-[13px] text-text-secondary">
              Pilih template paket untuk mengisi isi paket, syarat &amp; ketentuan, dan jadwal pembayaran sekaligus.
            </p>
          </div>
          {canManage ? (
            <>
              <div className="flex w-full max-w-sm flex-col gap-2 sm:flex-row">
                <Select value={templateChoice} onChange={(e) => setTemplateChoice(e.target.value)} placeholder="Pilih template paket">
                  {templates.map((t) => (
                    <option key={t.id} value={t.id}>
                      {t.name} — {formatCurrency(t.basePrice)}
                    </option>
                  ))}
                </Select>
                <Button
                  disabled={!templateChoice || busy}
                  onClick={() => void run(() => applyTemplate(projectId, templateChoice))}
                >
                  Terapkan
                </Button>
              </div>
              <Button variant="ghost" disabled={busy} onClick={() => void run(() => startBlank(projectId))}>
                Isi sendiri
              </Button>
              <p className="max-w-sm text-[12px] text-text-secondary">
                Nilai Kontrak yang sudah tercatat tetap dipakai sebagai Harga Paket Awal.
              </p>
            </>
          ) : (
            <p className="text-[13px] text-text-secondary">Hubungi Owner atau Admin untuk membuatnya.</p>
          )}
          {error && <p className="text-[13px] text-danger">{error}</p>}
        </CardContent>
      </Card>
    );
  }

  const statusLabel = order.status === "Terbit" && order.revision > 0 ? `Terbit · Revisi ${order.revision}` : order.status;

  return (
    <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_320px]">
      <div className="flex flex-col gap-4">
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
            {order.blocks.length === 0 ? (
              <p className="py-6 text-center text-[13px] text-text-secondary">Belum ada rincian paket.</p>
            ) : (
              <ul className="flex flex-col gap-2">
                {order.blocks.map((block, index) => (
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
                                saveBlocks(projectId, toBlockInputs(order.blocks.filter((_, i) => i !== index))),
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
            {order.adjustments.length === 0 ? (
              <p className="py-6 text-center text-[13px] text-text-secondary">Belum ada penyesuaian.</p>
            ) : (
              <ul className="flex flex-col divide-y divide-border">
                {order.adjustments.map((adj, index) => (
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
                                projectId,
                                order.adjustments
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
                  onClick={() =>
                    setHeaderDraft({ basePrice: order.basePrice, termsText: order.termsText, bonusNote: order.bonusNote })
                  }
                >
                  Ubah
                </Button>
              ) : undefined
            }
          />
          <CardContent className="flex flex-col gap-4">
            <pre className="whitespace-pre-wrap break-words font-sans text-[13px] text-text-primary">
              {order.termsText || "Belum diisi."}
            </pre>
            <div>
              <p className="text-[12px] font-semibold uppercase tracking-wide text-text-secondary">Bonus Tambahan</p>
              <pre className="mt-1 whitespace-pre-wrap break-words font-sans text-[13px] text-text-primary">
                {order.bonusNote || "Belum diisi."}
              </pre>
            </div>
          </CardContent>
        </Card>
      </div>

      {/* --- Ringkasan (D25: cermin read-only, bukan ledger) --- */}
      <div className="flex flex-col gap-4">
        <Card>
          <CardHeader
            title="Ringkasan"
            action={<Badge tone={STATUS_TONE[order.status]}>{statusLabel}</Badge>}
          />
          <CardContent className="flex flex-col gap-2 text-[13px]">
            <SummaryRow label="Harga Paket Awal" value={formatCurrency(order.basePrice)} />
            <SummaryRow
              label="Additional & Takeout"
              value={`${order.totalAdjustments < 0 ? "−" : "+"}${formatCurrency(Math.abs(order.totalAdjustments))}`}
              tone={order.totalAdjustments < 0 ? "danger" : undefined}
            />
            <div className="mt-1 flex items-center justify-between border-t border-border pt-2">
              <span className="font-semibold text-text-primary">Total Pembayaran</span>
              <span className="text-[15px] font-semibold tabular-nums text-text-primary">{formatCurrency(order.total)}</span>
            </div>
            {order.poNumber && <SummaryRow label="Nomor PO" value={order.poNumber} />}
            {order.issuedAt && <SummaryRow label="Diterbitkan" value={formatDate(order.issuedAt)} />}
            <p className="mt-1 text-[12px] text-text-secondary">
              Pembayaran yang masuk dicatat di tab Pembayaran.
            </p>
          </CardContent>
        </Card>

        {order.termsPlan.length > 0 && (
          <Card>
            <CardHeader title="Tahap Pembayaran" subtitle="Jadwal pembayaran yang tercetak di PO." />
            <CardContent>
              <ul className="flex flex-col divide-y divide-border">
                {order.termsPlan.map((term) => (
                  <li key={term.id} className="flex items-center justify-between gap-3 py-2 text-[13px]">
                    <span className="min-w-0 truncate text-text-primary">{term.label || term.type}</span>
                    <span className="shrink-0 font-semibold tabular-nums text-text-primary">
                      {formatCurrency(term.amount)}
                    </span>
                  </li>
                ))}
              </ul>
            </CardContent>
          </Card>
        )}

        <Card>
          <CardContent className="flex flex-col gap-2">
            <Button variant="secondary" icon={<FileText className="h-3.5 w-3.5" />} onClick={() => void handleDownloadPDF()}>
              {editable ? "Pratinjau PDF" : "Unduh PDF"}
            </Button>
            {canManage && editable && (
              <Button icon={<Send className="h-3.5 w-3.5" />} disabled={busy} onClick={() => setConfirmIssue(true)}>
                Terbitkan PO
              </Button>
            )}
            {canManage && order.status === "Terbit" && (
              <>
                <Button variant="secondary" icon={<RotateCcw className="h-3.5 w-3.5" />} disabled={busy} onClick={() => void run(() => revise(projectId))}>
                  Buat revisi
                </Button>
                <Button variant="danger" icon={<Ban className="h-3.5 w-3.5" />} disabled={busy} onClick={() => void run(() => cancelOrder(projectId))}>
                  Batalkan PO
                </Button>
              </>
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
                const next = toBlockInputs(order.blocks);
                if (blockDraft.index === null) next.push(blockDraft.value);
                else next[blockDraft.index] = blockDraft.value;
                void run(async () => {
                  await saveBlocks(projectId, next);
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
                const next: AdjustmentInput[] = order.adjustments.map(({ description, amount }) => ({ description, amount }));
                const row = { description: adjustDraft.description, amount: signed };
                if (adjustDraft.index === null) next.push(row);
                else next[adjustDraft.index] = row;
                void run(async () => {
                  await saveAdjustments(projectId, next);
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

      {/* --- Modal: harga & ketentuan --- */}
      <Modal
        open={headerDraft !== null}
        onClose={() => setHeaderDraft(null)}
        title="Ubah Harga & Ketentuan"
        size="lg"
        footer={
          <>
            <Button variant="ghost" onClick={() => setHeaderDraft(null)}>
              Batal
            </Button>
            <Button
              disabled={busy}
              onClick={() => {
                if (!headerDraft) return;
                void run(async () => {
                  await saveHeader(projectId, headerDraft);
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

      {/* --- Konfirmasi terbit: sebut jumlah tagihan agar tidak mengejutkan (D8) --- */}
      <Modal
        open={confirmIssue}
        onClose={() => setConfirmIssue(false)}
        title="Terbitkan PO Paket?"
        footer={
          <>
            <Button variant="ghost" onClick={() => setConfirmIssue(false)}>
              Batal
            </Button>
            <Button
              disabled={busy}
              onClick={() =>
                void run(async () => {
                  await issue(projectId);
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
            Komposisi dan ketentuan akan dikunci pada nilai saat ini. Untuk mengubahnya setelah terbit, buat revisi.
          </p>
          {order.termsPlan.length > 0 && (
            <p>
              <strong>{order.termsPlan.length} tagihan terjadwal</strong> akan dibuat di tab Pembayaran, berstatus Draft
              sampai Anda mengirimkannya.
            </p>
          )}
          <p className="text-text-secondary">Total pembayaran: {formatCurrency(order.total)}</p>
        </div>
      </Modal>

      <IncompleteProfileDialog
        open={profileGateOpen}
        onClose={() => setProfileGateOpen(false)}
        missingFields={missingProfileFields}
        docLabel="PO Paket"
      />
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
