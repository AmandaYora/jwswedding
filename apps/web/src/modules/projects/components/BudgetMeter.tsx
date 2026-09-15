import { AlertTriangle, ArrowUpRight } from "lucide-react";
import { Link } from "react-router-dom";
import { formatCurrency } from "@/shared/lib/formatters";
import type { BudgetState, BudgetTone } from "@/modules/projects/lib/budget";

// BudgetMeter menampilkan Nilai Kontrak, biaya yang sudah dikomitmenkan, dan
// sisanya sebagai satu bacaan — di header project maupun di dalam form vendor,
// dengan komponen yang sama supaya angka di kedua tempat tidak pernah tampak
// berbeda.
//
// Nadanya berubah SEBELUM batas terlampaui (lihat BUDGET_TIGHT_RATIO): sisa
// yang menipis tampil kuning, sisa negatif tampil merah. Peringatan yang baru
// berbunyi setelah minus datang sesudah harga vendor disepakati — sudah
// terlambat untuk mengubah apa pun.

const TONE_STYLES: Record<BudgetTone, { wrap: string; bar: string; value: string; label: string }> = {
  healthy: {
    wrap: "border-border bg-surface-muted/60",
    bar: "bg-success",
    value: "text-text-primary",
    label: "Sisa Anggaran / Margin",
  },
  tight: {
    wrap: "border-warning/40 bg-warning-soft",
    bar: "bg-warning",
    value: "text-warning-strong",
    label: "Sisa Anggaran menipis",
  },
  over: {
    wrap: "border-danger/40 bg-danger-soft",
    bar: "bg-danger",
    value: "text-danger",
    label: "Biaya melampaui Nilai Kontrak",
  },
};

export function BudgetMeter({
  budget,
  quotationHref,
  compact,
}: {
  budget: BudgetState;
  /**
   * Tautan ke penawaran project ini. Jalan keluar yang benar saat biaya
   * melampaui: yang direvisi adalah kesepakatannya, bukan Nilai Kontrak yang
   * diketik ulang diam-diam. Dihilangkan untuk peran yang tidak boleh membuka
   * penawaran.
   */
  quotationHref?: string;
  /** Varian rapat untuk di dalam modal. */
  compact?: boolean;
}) {
  const tone = TONE_STYLES[budget.tone];
  return (
    <div className={`rounded-md border px-3.5 ${compact ? "py-2.5" : "py-3"} ${tone.wrap}`}>
      <div className="flex flex-wrap items-baseline justify-between gap-x-3 gap-y-1">
        <span className="flex items-center gap-1.5 text-[11.5px] font-semibold uppercase tracking-wide text-text-secondary">
          {budget.tone !== "healthy" && <AlertTriangle className={`h-3.5 w-3.5 ${tone.value}`} />}
          {tone.label}
        </span>
        <span className={`text-[15px] font-bold tabular-nums ${tone.value}`}>
          {budget.tone === "over" ? `-${formatCurrency(budget.overage)}` : formatCurrency(budget.remaining)}
        </span>
      </div>

      <div className="mt-2 h-1.5 w-full overflow-hidden rounded-full bg-border-light">
        <div className={`h-full rounded-full ${tone.bar}`} style={{ width: `${budget.usedRatio * 100}%` }} />
      </div>

      <p className="mt-2 text-[12px] leading-relaxed text-text-secondary">
        Terkomitmen {formatCurrency(budget.committedCost)} dari Nilai Kontrak {formatCurrency(budget.contractValue)}
        <span className="text-text-tertiary">
          {" "}
          · vendor {formatCurrency(budget.vendorCost)} · venue {formatCurrency(budget.venueCost)}
        </span>
      </p>

      {budget.tone === "over" && quotationHref && (
        <Link
          to={quotationHref}
          className="mt-2 inline-flex items-center gap-1 text-[12.5px] font-semibold text-danger underline-offset-2 hover:underline"
        >
          Revisi penawaran untuk menyesuaikan Nilai Kontrak
          <ArrowUpRight className="h-3.5 w-3.5 shrink-0" />
        </Link>
      )}
    </div>
  );
}
