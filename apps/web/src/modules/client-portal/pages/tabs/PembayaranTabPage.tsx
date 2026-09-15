import { useState } from "react";
import { useOutletContext } from "react-router-dom";
import { Wallet, FileText, Receipt, Loader2 } from "lucide-react";
import { ClientEvidenceViewerModal } from "@/modules/client-portal/components/ClientEvidenceViewerModal";
import { PortalEmpty, PortalError, PortalLoading } from "@/modules/client-portal/components/PortalState";
import { usePortalSections } from "@/modules/client-portal/hooks/usePortalSections";
import { useProjectStore } from "@/modules/projects/stores/useProjectStore";
import { useTenantBrandingStore } from "@/shared/stores/useTenantBrandingStore";
import type { ClientPayment, Evidence } from "@/modules/projects/types";
import { formatCurrency, formatDate } from "@/shared/lib/formatters";
import { getApiErrorMessageFromBlob } from "@/shared/lib/api-error";
import { openPdfInNewTab } from "@/shared/lib/open-pdf";
import type { ClientPortalContext } from "@/modules/client-portal/layouts/ClientPortalLayout";

// Repurposed (PLAN.md "Uang Masuk dari Client", §3.8) — shows the client's
// OWN payment history against the project's contractValue, not the WO's
// spend on vendors (not relevant to a client, and a cost-management detail
// they were never meant to see). Same three-card visual layout as before,
// new source numbers; each card now shows a single "Bukti Transfer" button
// (a client payment has only one evidence slot) instead of an Invoice/Bukti
// Transfer pair, and drops the vendor-name line entirely — there is no
// vendor on a client payment.
export default function PembayaranTabPage() {
  const { projectId } = useOutletContext<ClientPortalContext>();
  const project = useProjectStore((s) => s.currentProject);
  const clientPayments = useProjectStore((s) => s.clientPayments);
  const evidence = useProjectStore((s) => s.evidence);
  const downloadClientPaymentReceipt = useProjectStore((s) => s.downloadClientPaymentReceipt);
  // Kwitansi print is gated on the WO tenant's own business-profile
  // completeness (PLAN.md redesain-pdf-invoice-kwitansi-v2 §6.3.4/K6) — a
  // client has no way to fix that, and no business seeing an internal
  // dialog about the WO's own profile, so the button is hidden entirely
  // rather than shown and then failing.
  const profileComplete = useTenantBrandingStore((s) => s.profileComplete);
  const [viewingEvidence, setViewingEvidence] = useState<Evidence | null>(null);
  const [receiptError, setReceiptError] = useState<string | null>(null);
  const [receiptBusyId, setReceiptBusyId] = useState<string | null>(null);

  const { loading, error, reload } = usePortalSections(projectId, ["clientPayments", "evidence"]);

  async function handleDownloadReceipt(payment: ClientPayment) {
    setReceiptError(null);
    setReceiptBusyId(payment.id);
    try {
      // Must stay synchronous up to openPdfInNewTab's first line — it claims
      // the tab while the click's user activation is still alive. The old
      // `window.open(URL.createObjectURL(await ...))` spelling here returned
      // null once the request outlived that window, so the button did
      // nothing at all and reported no error either. See open-pdf.ts.
      await openPdfInNewTab(
        () => downloadClientPaymentReceipt(projectId, payment.id),
        // receiptNumber is "" until the first print, and contains slashes
        // when it isn't — neither is usable as a filename as-is.
        payment.receiptNumber
          ? `Kwitansi-${payment.receiptNumber.replace(/\//g, "-")}`
          : `Kwitansi-${payment.paymentDate}`
      );
    } catch (err) {
      setReceiptError(await getApiErrorMessageFromBlob(err, "Gagal membuka Kwitansi"));
    } finally {
      setReceiptBusyId(null);
    }
  }

  const sortedPayments = [...clientPayments].sort((a, b) => (a.paymentDate < b.paymentDate ? 1 : -1));
  const contractValue = project?.contractValue ?? 0;
  const totalReceived = clientPayments.reduce((sum, p) => (p.type === "Refund" ? sum - p.amount : sum + p.amount), 0);
  const totalRemaining = contractValue - totalReceived;
  // An overpaid project produced "Sisa Tagihan -Rp 2.000.000", which reads as
  // a bill, not a credit.
  const isOverpaid = totalRemaining < 0;

  return (
    <div className="flex flex-col gap-6 sm:gap-8">
      <section>
        <div className="mb-4 flex items-center gap-2 sm:mb-5">
          <Wallet className="h-5 w-5 shrink-0 text-navy-900" />
          <h2 className="text-base font-bold text-text-primary sm:text-lg">Transparansi Pembayaran</h2>
        </div>
        <p className="mb-5 max-w-2xl text-[13px] leading-relaxed text-text-secondary sm:mb-6 sm:text-[13.5px]">
          Halaman ini menampilkan riwayat pembayaran yang telah Anda lakukan untuk pernikahan Anda, lengkap
          dengan bukti transfernya — tidak ada yang kami sembunyikan.
        </p>

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-3 sm:gap-6">
          <div className="rounded-2xl border border-border bg-white p-5 shadow-sm transition-shadow hover:shadow-md sm:p-6">
            <p className="text-[12.5px] font-bold uppercase tracking-wider text-text-secondary">Nilai Kontrak</p>
            <p className="mt-3 text-2xl font-bold tabular-nums text-navy-950">{formatCurrency(contractValue)}</p>
          </div>
          <div className="rounded-2xl border border-emerald-100 bg-emerald-50/30 p-5 shadow-sm transition-shadow hover:shadow-md sm:p-6">
            <p className="text-[12.5px] font-bold uppercase tracking-wider text-emerald-700">Total Diterima</p>
            {/* Held back while loading: computed from an empty array these
                read "Rp 0" and "the whole contract is still outstanding",
                then correct themselves a beat later. */}
            <p className="mt-3 text-2xl font-bold tabular-nums text-emerald-600">
              {loading ? "—" : formatCurrency(totalReceived)}
            </p>
          </div>
          <div
            className={
              isOverpaid && !loading
                ? "rounded-2xl border border-blue-100 bg-blue-50/30 p-5 shadow-sm transition-shadow hover:shadow-md sm:p-6"
                : "rounded-2xl border border-amber-100 bg-amber-50/30 p-5 shadow-sm transition-shadow hover:shadow-md sm:p-6"
            }
          >
            <p
              className={
                isOverpaid && !loading
                  ? "text-[12.5px] font-bold uppercase tracking-wider text-blue-700"
                  : "text-[12.5px] font-bold uppercase tracking-wider text-amber-700"
              }
            >
              {isOverpaid && !loading ? "Kelebihan Bayar" : "Sisa Tagihan"}
            </p>
            <p
              className={
                isOverpaid && !loading
                  ? "mt-3 text-2xl font-bold tabular-nums text-blue-600"
                  : "mt-3 text-2xl font-bold tabular-nums text-amber-600"
              }
            >
              {loading ? "—" : formatCurrency(Math.abs(totalRemaining))}
            </p>
          </div>
        </div>
      </section>

      <section>
        <h3 className="mb-3 text-[14px] font-bold text-text-primary sm:mb-4 sm:text-[15px]">Riwayat Pembayaran</h3>
        {receiptError && (
          <p className="mb-4 rounded-md border border-danger/30 bg-danger-soft px-3.5 py-2.5 text-[13px] font-medium text-danger">
            {receiptError}
          </p>
        )}
        {loading ? (
          <PortalLoading label="Memuat riwayat pembayaran..." />
        ) : error ? (
          <PortalError message={error} onRetry={reload} />
        ) : sortedPayments.length === 0 ? (
          <PortalEmpty
            icon={Wallet}
            title="Belum ada pembayaran tercatat"
            description="Pembayaran akan muncul di sini seiring proses persiapan berjalan."
          />
        ) : (
          <div className="flex flex-col gap-4">
            {sortedPayments.map((payment) => {
              const proofEvidence = evidence.find((e) => e.relatedKind === "clientPayment" && e.relatedId === payment.id);
              const isRefund = payment.type === "Refund";
              const isBusy = receiptBusyId === payment.id;

              return (
                <div
                  key={payment.id}
                  className="group relative flex flex-col gap-4 overflow-hidden rounded-2xl border border-border bg-white p-5 shadow-sm transition-shadow hover:shadow-md sm:p-6"
                >
                  <div className="absolute bottom-0 left-0 top-0 w-1.5 bg-navy-900 opacity-0 transition-opacity group-hover:opacity-100"></div>

                  <div className="flex flex-col items-start gap-3 sm:flex-row sm:items-center sm:justify-between sm:gap-4">
                    <div>
                      <p className="text-[15px] font-bold text-navy-950 sm:text-[16px]">
                        {isRefund ? "Pengembalian Dana" : payment.type}
                      </p>
                      <p className="mt-1 inline-block rounded-md bg-surface-muted px-2.5 py-1 text-[13px] font-medium text-text-secondary">
                        {formatDate(payment.paymentDate)} · {payment.method || "Metode tidak dicatat"}
                      </p>
                    </div>
                    <div className="flex flex-col items-start gap-2 sm:items-end">
                      {/* A Refund is SUBTRACTED from "Total Diterima" above,
                          but used to be printed here as a plain positive
                          amount identical to a payment in. */}
                      <span
                        className={
                          isRefund
                            ? "text-[18px] font-bold tabular-nums text-danger sm:text-[20px]"
                            : "text-[18px] font-bold tabular-nums text-navy-950 sm:text-[20px]"
                        }
                      >
                        {isRefund ? `- ${formatCurrency(payment.amount)}` : formatCurrency(payment.amount)}
                      </span>
                      {payment.evidenceComplete ? (
                        <span className="inline-flex items-center gap-1.5 rounded-full bg-emerald-50 px-2.5 py-1 text-[11px] font-bold text-emerald-700 ring-1 ring-emerald-500/20">
                          BUKTI TERSEDIA
                        </span>
                      ) : (
                        <span className="inline-flex items-center gap-1.5 rounded-full bg-warning-soft px-2.5 py-1 text-[11px] font-bold text-warning-strong ring-1 ring-warning/30">
                          BUKTI BELUM ADA
                        </span>
                      )}
                      {!isRefund &&
                        (profileComplete ? (
                          <button
                            type="button"
                            disabled={isBusy}
                            onClick={() => void handleDownloadReceipt(payment)}
                            className="flex items-center gap-1.5 rounded-lg border border-border bg-surface px-3 py-1.5 text-[12px] font-semibold text-navy-900 shadow-sm transition-colors hover:border-navy-200 hover:bg-navy-50 disabled:cursor-not-allowed disabled:opacity-60"
                          >
                            {isBusy ? (
                              <Loader2 className="h-3.5 w-3.5 animate-spin" />
                            ) : (
                              <Receipt className="h-3.5 w-3.5" />
                            )}
                            {isBusy ? "Menyiapkan..." : "Unduh Kwitansi"}
                          </button>
                        ) : (
                          <span className="text-[12px] font-medium text-text-secondary">Kwitansi belum tersedia</span>
                        ))}
                    </div>
                  </div>

                  {proofEvidence && (
                    <div className="mt-2 flex flex-wrap items-center gap-2 border-t border-border/50 pt-4">
                      <FileText className="h-4 w-4 shrink-0 text-text-tertiary" />
                      <span className="mr-2 text-[13px] font-medium text-text-secondary">Dokumen Pendukung:</span>
                      <button
                        type="button"
                        onClick={() => setViewingEvidence(proofEvidence)}
                        className="flex items-center gap-1.5 rounded-lg border border-border bg-surface px-3 py-1.5 text-[12px] font-semibold text-navy-900 shadow-sm transition-colors hover:border-navy-200 hover:bg-navy-50"
                      >
                        Bukti Transfer
                      </button>
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        )}
      </section>

      {viewingEvidence && (
        <ClientEvidenceViewerModal
          evidence={viewingEvidence}
          projectId={projectId}
          onClose={() => setViewingEvidence(null)}
        />
      )}
    </div>
  );
}
