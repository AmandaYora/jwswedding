import type {
  EngagementStatus,
  EvidenceType,
  IssueImpact,
  IssueStatus,
  MilestoneStatus,
  ProjectStatus,
} from "@/modules/projects/types";

// Client-facing Indonesian labels for the backend's English enum values.
//
// The portal's own copy is written in Indonesian throughout ("Sedang
// dikerjakan oleh tim", "Berjalan sesuai rencana"), but every Badge rendered
// straight off a status field printed the raw enum next to it — "IN
// PROGRESS", "NOT STARTED", "DP PAID", "HIGH", "TRANSFER PROOF". The WO
// Console deliberately keeps the English values (staff work against the same
// vocabulary as the API and the database), so these maps live in the
// client-portal module instead of `projects/lib/status.ts`, alongside
// CLIENT_CONDITION_COPY, which already establishes exactly this pattern for
// ProjectCondition.
//
// Tone/colour still comes from `projects/lib/status.ts` — only the words are
// overridden here.

export const CLIENT_PROJECT_STATUS_LABEL: Record<ProjectStatus, string> = {
  Draft: "Draf",
  Preparation: "Persiapan",
  Ready: "Siap",
  Completed: "Selesai",
  Cancelled: "Dibatalkan",
};

export const CLIENT_MILESTONE_STATUS_LABEL: Record<MilestoneStatus, string> = {
  "Not Started": "Belum Dimulai",
  "In Progress": "Sedang Dikerjakan",
  Completed: "Selesai",
  Blocked: "Terhambat",
  Cancelled: "Dibatalkan",
};

export const CLIENT_ENGAGEMENT_STATUS_LABEL: Record<EngagementStatus, string> = {
  Planned: "Direncanakan",
  Negotiation: "Negosiasi",
  Booked: "Sudah Dipesan",
  "DP Paid": "DP Dibayar",
  "In Progress": "Sedang Berjalan",
  "Fully Paid": "Lunas",
  Ready: "Siap",
  Completed: "Selesai",
  Cancelled: "Dibatalkan",
};

export const CLIENT_ISSUE_STATUS_LABEL: Record<IssueStatus, string> = {
  Open: "Baru Ditemukan",
  "In Review": "Sedang Ditinjau",
  "In Resolution": "Sedang Ditangani",
  Resolved: "Sudah Ditangani",
  Closed: "Ditutup",
};

export const CLIENT_ISSUE_IMPACT_LABEL: Record<IssueImpact, string> = {
  Low: "Dampak Ringan",
  Medium: "Dampak Sedang",
  High: "Dampak Besar",
  Critical: "Dampak Kritis",
};

export const CLIENT_EVIDENCE_TYPE_LABEL: Record<EvidenceType, string> = {
  Quotation: "Penawaran",
  Invoice: "Tagihan",
  Contract: "Kontrak",
  "Transfer Proof": "Bukti Transfer",
  Receipt: "Kwitansi",
  "Purchase Order": "Purchase Order",
  Photo: "Foto",
  Document: "Dokumen",
  Screenshot: "Tangkapan Layar",
  "Minutes of Meeting": "Notulen Rapat",
  Other: "Lainnya",
  "Booking Proof": "Bukti Booking",
};

export function clientEvidenceTypeLabel(type: EvidenceType): string {
  return CLIENT_EVIDENCE_TYPE_LABEL[type] ?? type;
}
