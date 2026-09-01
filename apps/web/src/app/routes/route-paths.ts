export type ProjectDetailTab =
  | "vendor"
  | "milestone"
  | "client"
  | "pembayaran/client"
  | "pembayaran/vendor"
  | "pembayaran/venue"
  | "dokumen"
  | "aktivitas"
  | "venue";
// "ringkasan" removed (PLAN.md's Client Portal restructure) — that content
// is now the persistent header, not a tab; "timeline" added, completing
// parity with WO Console's own tab set. "venue" removed separately (Vendor/
// Venue merge, client-facing only) — Venue is now a pinned card inside the
// "vendor" tab itself, not its own route; ProjectDetailTab above is WO
// Console's own (unaffected) tab set and keeps its separate "venue" entry.
export type ClientPortalTab = "vendor" | "pembayaran" | "kendala" | "timeline" | "dokumen";

export const ROUTE_PATHS = {
  home: "/",
  login: "/login",
  dashboard: "/dashboard",
  projects: "/projects",
  projectDetail: (id: string, tab: ProjectDetailTab = "vendor") => `/projects/${id}/${tab}`,
  clients: "/clients",
  vendorCategories: "/vendor-categories",
  vendors: "/vendors",
  venues: "/venues",
  users: "/pengguna",
  subscription: "/langganan",
  milestoneTemplates: "/timeline-default",
  clientTimelines: "/monitoring-timeline",
  companyProfile: "/profil-usaha",
  portal: (tab: ClientPortalTab = "vendor") => `/portal/${tab}`,
} as const;
