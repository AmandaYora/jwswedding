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

// Dua belas tab editor rundown -- Cover berdiri sendiri dan SUSUNAN ACARA
// dipecah akad/resepsi, jadi jumlahnya 12 meski WO menghitung 10 halaman
// dokumen. Nilainya sama persis dengan kunci seksi di API.
export type RundownTab =
  | "cover"
  | "vendors"
  | "roles"
  | "committees"
  | "data-lainnya"
  | "makeup"
  | "acara-akad"
  | "acara-resepsi"
  | "layout"
  | "foto-tamu"
  | "tamu-vip"
  | "playlist";

export const ROUTE_PATHS = {
  home: "/",
  login: "/login",
  dashboard: "/dashboard",
  projects: "/projects",
  projectDetail: (id: string, tab: ProjectDetailTab = "vendor") => `/projects/${id}/${tab}`,
  clients: "/clients",
  clientDetail: (id: string) => `/clients/${id}`,
  quotations: "/quotations",
  quotationDetail: (id: string) => `/quotations/${id}`,
  rundowns: "/rundowns",
  rundownDetail: (id: string, tab: RundownTab = "cover") => `/rundowns/${id}/${tab}`,
  // Template Rundown: enam seksi yang sama dengan editor rundown. Segmen
  // statis "template" mengalahkan ":id" di pencocokan react-router.
  rundownTemplate: (tab: RundownTab = "roles") => `/rundowns/template/${tab}`,
  vendorCategories: "/vendor-categories",
  vendors: "/vendors",
  venues: "/venues",
  users: "/pengguna",
  subscription: "/langganan",
  milestoneTemplates: "/timeline-default",
  packageTemplates: "/template-paket",
  clientTimelines: "/monitoring-timeline",
  companyProfile: "/profil-usaha",
  portal: (tab: ClientPortalTab = "vendor") => `/portal/${tab}`,
  // Magic link tanda tangan (jalur C, tanpa login).
  publicSignature: (token: string) => `/tanda-tangan/${token}`,
} as const;
