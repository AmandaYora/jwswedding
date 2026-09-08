// Real backend types for the `projects` module's Timeline Default Template
// sub-resource (PLAN.md) -- a tenant's own configurable checklist seeded
// into every new project's Timeline tab.

export interface MilestoneTemplate {
  id: string;
  name: string;
  // Display-grouping category seeded into each project milestone (Blok B).
  // "" = "Tanpa Kategori".
  category: string;
  daysBeforeEvent: number;
  sortOrder: number;
}
