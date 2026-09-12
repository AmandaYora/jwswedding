// Template Paket + PO Paket (PLAN.md po-paket-client).
//
// A block is ONE ROW of the printed PO table, not one item (D20): `body` holds
// the item list as free text, one per line. That is why the editor is a
// textarea rather than a row-per-item grid — a whole category pastes in from
// the spreadsheet the WO already keeps, in one go.

export interface PackageBlock {
  id: string;
  /** Merged first column of the printed table, e.g. "CATERING". */
  category: string;
  /** Item list, one per line. An ALL-CAPS line prints bold as a sub-heading (D21). */
  body: string;
  /** Free text, may span several lines ("150 PORSI" x4) or hold a range ("10-12 METER"). */
  qtyText: string;
  bonusNote: string;
  sortOrder: number;
}

export type PackageTermType = "DP" | "Termin" | "Pelunasan";

export interface PackageTemplateTerm {
  id: string;
  sequence: number;
  label: string;
  type: PackageTermType;
  /** Exactly one of percent/fixedAmount is set. */
  percent: number | null;
  fixedAmount: number | null;
  /** Offset in days before the event date; the due date is derived from it. */
  daysBeforeEvent: number;
}

export interface PackageTemplate {
  id: string;
  name: string;
  basePrice: number;
  defaultTerms: string;
  defaultBonusNote: string;
  isActive: boolean;
  sortOrder: number;
  blocks: PackageBlock[];
  terms: PackageTemplateTerm[];
}

export type PackageOrderStatus = "Draft" | "Terbit" | "Dibatalkan";

export interface PackageAdjustment {
  id: string;
  description: string;
  /** Signed: negative is a takeout or cashback (D2). */
  amount: number;
  sortOrder: number;
}

export interface PackageOrderTerm extends PackageTemplateTerm {
  /** This term's share of the current total, computed backend-side. */
  amount: number;
}

export interface PackageOrder {
  /** Empty until the PO is first issued (D26) — never a placeholder. */
  poNumber: string;
  revision: number;
  status: PackageOrderStatus;
  basePrice: number;
  termsText: string;
  bonusNote: string;
  issuedAt: string | null;
  blocks: PackageBlock[];
  adjustments: PackageAdjustment[];
  termsPlan: PackageOrderTerm[];
  totalAdjustments: number;
  total: number;
}
