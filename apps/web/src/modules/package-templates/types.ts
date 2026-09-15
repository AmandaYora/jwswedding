// Template Paket (Harga Standar) — master harga awal penawaran.
// PO/penawaran sendiri tinggal di modul `quotations` (types-nya sendiri).
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

/**
 * One row of the Template Paket list, and what the project pickers read.
 *
 * It carries a COUNT, never the composition: the list endpoint does not send
 * blocks at all. An editor that needs them loads the template by id
 * (`getTemplate`) — mirroring `domain.PackageTemplateSummary` on the backend,
 * for the same reason. Seeding an editor from a list row used to show an empty
 * composition for a template that had one, and saving from there wiped it.
 */
export interface PackageTemplateSummary {
  id: string;
  name: string;
  basePrice: number;
  defaultTerms: string;
  defaultBonusNote: string;
  isActive: boolean;
  sortOrder: number;
  blockCount: number;
}

/** A single template WITH its composition — what `GET /{id}` returns. */
export interface PackageTemplate extends Omit<PackageTemplateSummary, "blockCount"> {
  blocks: PackageBlock[];
}
