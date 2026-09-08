import { Building2, Camera, FileText, Flower2, Gift, Mail, Mic, Shirt, UtensilsCrossed, type LucideIcon } from "lucide-react";

// Timeline categories (Blok B, revisi-putri-mom-25082026, item 14). This lives
// in the `projects` module — not shared/ — because a milestone category is a
// domain concept owned by project_milestones, and shared/ must stay
// domain-agnostic. Other modules (milestone-templates, client-portal) import it
// from here rather than duplicating the constant.

// DEFAULT_MILESTONE_CATEGORIES is the built-in list, in the display order shown
// in gambar 2. It is the seed for the category dropdown; tenants extend it by
// editing template rows (D1), which categoryOptions then unions in.
export const DEFAULT_MILESTONE_CATEGORIES = [
  "Venue",
  "Dekorasi",
  "Catering",
  "Make Up & Busana",
  "Dokumentasi",
  "Souvenir",
  "Undangan",
  "MC & Entertain",
  "KUA",
] as const;

// categoryOptions returns the dropdown options: the built-in categories, then
// any additional categories already used by this tenant's data (deduped),
// sorted built-ins-first (in their fixed order) then the rest alphabetically.
// "" (Tanpa Kategori) is never returned here — callers render it as the first
// option themselves.
export function categoryOptions(usedCategories: string[]): string[] {
  const builtins = DEFAULT_MILESTONE_CATEGORIES as readonly string[];
  const extras = [...new Set(usedCategories)]
    .filter((c) => c !== "" && !builtins.includes(c))
    .sort((a, b) => a.localeCompare(b, "id-ID"));
  return [...builtins, ...extras];
}

// CATEGORY_ICONS maps the 9 built-in categories (gambar 2, D1,
// docs/plan/revisi-putri-lanjutan/PLAN.md) to a lucide-react icon — all 9
// names verified present in the installed lucide-react version, not assumed.
// Kept alongside the category list itself rather than in the display
// component, since "which icon represents this category" is the same kind
// of category-metadata as the list/order above.
export const CATEGORY_ICONS: Record<string, LucideIcon> = {
  Venue: Building2,
  Dekorasi: Flower2,
  Catering: UtensilsCrossed,
  "Make Up & Busana": Shirt,
  Dokumentasi: Camera,
  Souvenir: Gift,
  Undangan: Mail,
  "MC & Entertain": Mic,
  KUA: FileText,
};

// iconForCategory falls back to FileText for a tenant-added category (one
// that edited the template beyond the 9 built-ins) or "" (Tanpa Kategori) --
// so this mapping can never be a source of a crash from an unrecognized name.
export function iconForCategory(category: string): LucideIcon {
  return CATEGORY_ICONS[category] ?? FileText;
}
