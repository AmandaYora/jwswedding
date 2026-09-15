import type { Project, ProjectVendor } from "@/modules/projects/types";

// Anggaran sebuah project adalah Nilai Kontraknya. Setiap komitmen ke vendor
// dan biaya venue memakan ruang itu; sisanya adalah Margin.
//
// SATU tempat rumus ini hidup di frontend. Dulu ia mengendap di dalam
// ProjectHeaderCard, sehingga form vendor tidak punya cara menunjukkan sisa
// anggaran tanpa menyalinnya. Kembarannya ada di backend
// (ProjectService.CostSummary + guardBudget) dan memang harus ada di sana:
// layar hanya memberi tahu, gerbanglah yang memutuskan. Yang TIDAK bisa
// dilakukan backend adalah memproyeksikan angka yang sedang diketik — itu
// alasan modul ini ada.

// BUDGET_TIGHT_RATIO menentukan kapan peringatan mulai menajam: saat sisa
// anggaran tinggal di bawah 10% Nilai Kontrak.
//
// Menajam SEBELUM batas, bukan tepat di batas, adalah keseluruhan gunanya.
// Peringatan yang baru muncul setelah angkanya minus datang sesudah harga
// vendor dinegosiasikan dan disepakati — terlambat untuk mengubah apa pun.
const BUDGET_TIGHT_RATIO = 0.1;

export type BudgetTone = "healthy" | "tight" | "over";

export interface BudgetState {
  /** Nilai Kontrak — plafon yang dibagi-bagi. */
  contractValue: number;
  vendorCost: number;
  venueCost: number;
  committedCost: number;
  /** Boleh negatif: justru keadaan itulah yang harus terlihat. */
  remaining: number;
  /** Porsi plafon yang terpakai, dipotong di 1 untuk bar progres. */
  usedRatio: number;
  tone: BudgetTone;
  /** Besar pelampauan sebagai bilangan positif; 0 selama belum melampaui. */
  overage: number;
}

// venueCostOf membaca snapshot biaya venue milik project — bukan harga master
// venue yang hidup (lihat komentar venueRentalPrice di types.ts).
export function venueCostOf(project: Project): number {
  return (project.venueRentalPrice ?? 0) + (project.venueCharge ?? 0);
}

// activeVendorCost menjumlahkan komitmen vendor yang masih berlaku. Engagement
// Cancelled bukan biaya — aturan yang sama persis dipakai guardBudget di
// backend, dan sama seperti statistik timeline yang juga mengabaikannya.
export function activeVendorCost(engagements: ProjectVendor[]): number {
  return engagements
    .filter((v) => v.engagementStatus !== "Cancelled")
    .reduce((sum, v) => sum + v.contractValue, 0);
}

// projectedVendorCost menjawab "berapa total biaya vendor SESUDAH tulisan ini
// jadi", untuk dipakai selagi orang masih mengetik.
//
// `excludeId` adalah engagement yang sedang DIGANTI; tanpanya, mengubah satu
// engagement akan menghitung nilai lama dan nilai barunya sekaligus.
export function projectedVendorCost(
  engagements: ProjectVendor[],
  excludeId: string | undefined,
  nextValue: number,
  nextStatus: ProjectVendor["engagementStatus"]
): number {
  const others = activeVendorCost(engagements.filter((v) => v.id !== excludeId));
  return nextStatus === "Cancelled" ? others : others + nextValue;
}

export function budgetStateOf(project: Project, vendorCost: number): BudgetState {
  const venueCost = venueCostOf(project);
  const committedCost = vendorCost + venueCost;
  const remaining = project.contractValue - committedCost;
  // Nilai Kontrak 0 (project yang belum berharga) tidak punya rasio yang
  // bermakna — komitmen apa pun di atasnya sudah memenuhi bar.
  const usedRatio =
    project.contractValue > 0 ? Math.min(committedCost / project.contractValue, 1) : committedCost > 0 ? 1 : 0;
  return {
    contractValue: project.contractValue,
    vendorCost,
    venueCost,
    committedCost,
    remaining,
    usedRatio,
    tone: toneFor(remaining, project.contractValue),
    overage: remaining < 0 ? -remaining : 0,
  };
}

// worsensOverBudget menjawab: apakah tulisan ini MEMBUAT atau MEMPERDALAM
// pelampauan? Kembaran syarat kedua di guardBudget.
//
// Sebuah project yang terlanjur minus tidak boleh meminta alasan pada setiap
// penyuntingan vendor sesudahnya — termasuk saat nilainya justru diturunkan.
// Yang perlu diakui adalah pelampauan yang dibuat atau diperdalam, bukan yang
// sedang diperbaiki.
export function worsensOverBudget(projected: BudgetState, current: BudgetState): boolean {
  return projected.tone === "over" && projected.committedCost > current.committedCost;
}

function toneFor(remaining: number, contractValue: number): BudgetTone {
  if (remaining < 0) return "over";
  if (contractValue > 0 && remaining < contractValue * BUDGET_TIGHT_RATIO) return "tight";
  return "healthy";
}
