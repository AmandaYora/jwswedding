import { lazy } from "react";
import { Navigate, type RouteObject } from "react-router-dom";
import { AppLayout } from "@/shared/layouts/AppLayout";
import { RequireAuth } from "@/shared/components/RequireAuth";
import { RequireRole } from "@/shared/components/RequireRole";
import { ROUTE_PATHS } from "@/app/routes/route-paths";

const DashboardPage = lazy(() => import("@/modules/dashboard/pages/DashboardPage"));
const ProjectListPage = lazy(() => import("@/modules/projects/pages/ProjectListPage"));
const ProjectDetailLayout = lazy(() => import("@/modules/projects/pages/ProjectDetailLayout"));
const ProjectVendorTabPage = lazy(() => import("@/modules/projects/pages/tabs/ProjectVendorTabPage"));
const ProjectMilestoneTabPage = lazy(() => import("@/modules/projects/pages/tabs/ProjectMilestoneTabPage"));
const ProjectClientTabPage = lazy(() => import("@/modules/projects/pages/tabs/ProjectClientTabPage"));
const ProjectPaymentsTabPage = lazy(() => import("@/modules/projects/pages/tabs/ProjectPaymentsTabPage"));
const PembayaranClientTabPage = lazy(() => import("@/modules/projects/pages/tabs/PembayaranClientTabPage"));
const PembayaranVendorTabPage = lazy(() => import("@/modules/projects/pages/tabs/PembayaranVendorTabPage"));
const PembayaranVenueTabPage = lazy(() => import("@/modules/projects/pages/tabs/PembayaranVenueTabPage"));
const ProjectEvidenceTabPage = lazy(() => import("@/modules/projects/pages/tabs/ProjectEvidenceTabPage"));
const ProjectActivityTabPage = lazy(() => import("@/modules/projects/pages/tabs/ProjectActivityTabPage"));
const ProjectVenueTabPage = lazy(() => import("@/modules/projects/pages/tabs/ProjectVenueTabPage"));
const ClientListPage = lazy(() => import("@/modules/clients/pages/ClientListPage"));
const ClientDetailPage = lazy(() => import("@/modules/clients/pages/ClientDetailPage"));
const QuotationListPage = lazy(() => import("@/modules/quotations/pages/QuotationListPage"));
const QuotationDetailPage = lazy(() => import("@/modules/quotations/pages/QuotationDetailPage"));
const RundownListPage = lazy(() => import("@/modules/rundowns/pages/RundownListPage"));
const RundownDetailPage = lazy(() => import("@/modules/rundowns/pages/RundownDetailPage"));
const RundownTemplatePage = lazy(() => import("@/modules/rundowns/pages/RundownTemplatePage"));
const VendorCategoryListPage = lazy(() => import("@/modules/vendor-categories/pages/VendorCategoryListPage"));
const MilestoneTemplateListPage = lazy(() => import("@/modules/milestone-templates/pages/MilestoneTemplateListPage"));
const PackageTemplateListPage = lazy(() => import("@/modules/package-templates/pages/PackageTemplateListPage"));
const VendorListPage = lazy(() => import("@/modules/vendors/pages/VendorListPage"));
const VenueListPage = lazy(() => import("@/modules/venues/pages/VenueListPage"));
const UsersPage = lazy(() => import("@/modules/users/pages/UsersPage"));
const SubscriptionPage = lazy(() => import("@/modules/subscription/pages/SubscriptionPage"));
const CompanyProfilePage = lazy(() => import("@/modules/company-profile/pages/CompanyProfilePage"));
const TimelineMonitorPage = lazy(() => import("@/modules/timeline-monitor/pages/TimelineMonitorPage"));

export const protectedRoutes: RouteObject = {
  element: <RequireAuth allow={["staff"]} />,
  children: [
    {
      element: <AppLayout />,
      children: [
        // Project stays open to every staff role, including Wedding Planner
        // (scoped server-side to their own PIC'd projects — see PLAN.md's
        // RBAC section) — no RequireRole wrapper needed here.
        { path: ROUTE_PATHS.projects, element: <ProjectListPage /> },
        {
          path: "/projects/:projectId",
          element: <ProjectDetailLayout />,
          children: [
            { index: true, element: <Navigate to="vendor" replace /> },
            { path: "vendor", element: <ProjectVendorTabPage /> },
            { path: "milestone", element: <ProjectMilestoneTabPage /> },
            { path: "client", element: <ProjectClientTabPage /> },
            {
              path: "pembayaran",
              element: <ProjectPaymentsTabPage />,
              children: [
                { index: true, element: <Navigate to="client" replace /> },
                { path: "client", element: <PembayaranClientTabPage /> },
                { path: "vendor", element: <PembayaranVendorTabPage /> },
                { path: "venue", element: <PembayaranVenueTabPage /> },
              ],
            },
            { path: "dokumen", element: <ProjectEvidenceTabPage /> },
            { path: "aktivitas", element: <ProjectActivityTabPage /> },
            { path: "venue", element: <ProjectVenueTabPage /> },
            // Tab "Paket & PO" dihapus (lihat ProjectDetailLayout) — rute
            // lamanya dipertahankan sebagai pengalihan supaya tautan/bookmark
            // lama mendarat di sesuatu yang nyata, bukan 404. Preseden sama
            // dengan "ringkasan"/"venue" di client-portal.routes.tsx.
            { path: "paket", element: <Navigate to="../vendor" replace /> },
          ],
        },
        // Tanpa RequireRole: setiap staff membuka menu Pengguna. UsersPage
        // sendiri yang mencabang — Owner mendapat manajemen pengguna penuh,
        // role lain hanya TTD miliknya sendiri (jalur `me`, staffID dari klaim
        // JWT). Manajemen pengguna tetap Owner-only di backend.
        { path: ROUTE_PATHS.users, element: <UsersPage /> },
        {
          element: <RequireRole allow={["Owner", "Admin"]} />,
          children: [
            { path: ROUTE_PATHS.dashboard, element: <DashboardPage /> },
            { path: ROUTE_PATHS.vendors, element: <VendorListPage /> },
          ],
        },
        {
          // Venue terbuka untuk Sales juga (PLAN revisi-vendor-venue-portal
          // §1.1 poin 7 / §4.6/I6) — baca saja: backend sudah membedakan baca
          // (requireStaffTenant) dan tulis (requireManagerRole), dan
          // halamannya menyembunyikan tombol tulis untuk Sales. Dashboard dan
          // Vendor tetap Owner/Admin.
          element: <RequireRole allow={["Owner", "Admin", "Sales"]} />,
          children: [{ path: ROUTE_PATHS.venues, element: <VenueListPage /> }],
        },
        {
          // Client + Penawaran terbuka untuk Sales (menyusun penawaran butuh
          // memilih/menambah pasangan); Wedding Planner membuka keduanya
          // tidak.
          element: <RequireRole allow={["Owner", "Admin", "Sales"]} />,
          children: [
            { path: ROUTE_PATHS.clients, element: <ClientListPage /> },
            { path: "/clients/:clientId", element: <ClientDetailPage /> },
            { path: ROUTE_PATHS.quotations, element: <QuotationListPage /> },
            { path: "/quotations/:quotationId", element: <QuotationDetailPage /> },
          ],
        },
        {
          // Wider than the Owner/Admin-only block above: a Wedding Planner
          // has no Dashboard access at all (PLAN.md mom-25082026-item-belum
          // item 17, T-1), so Monitoring Timeline is its own standalone
          // route/menu entry reachable by Staff too — scoped server-side to
          // their own PIC'd projects.
          element: <RequireRole allow={["Owner", "Admin", "Staff"]} />,
          children: [
            { path: ROUTE_PATHS.clientTimelines, element: <TimelineMonitorPage /> },
            // Rundown ikut blok yang sama: buku acara justru paling sering
            // dibuka Wedding Planner di hari-H. Cakupannya tetap dibatasi
            // server-side ke project yang dia pegang.
            { path: ROUTE_PATHS.rundowns, element: <RundownListPage /> },
            // Template dibuka role yang sama (PLAN rundown-ux-ideal D4).
            { path: "/rundowns/template", element: <RundownTemplatePage /> },
            { path: "/rundowns/template/:tab", element: <RundownTemplatePage /> },
            { path: "/rundowns/:id", element: <RundownDetailPage /> },
            { path: "/rundowns/:id/:tab", element: <RundownDetailPage /> },
          ],
        },
        {
          element: <RequireRole allow={["Owner"]} />,
          children: [
            { path: ROUTE_PATHS.vendorCategories, element: <VendorCategoryListPage /> },
            { path: ROUTE_PATHS.milestoneTemplates, element: <MilestoneTemplateListPage /> },
            { path: ROUTE_PATHS.packageTemplates, element: <PackageTemplateListPage /> },
            { path: ROUTE_PATHS.companyProfile, element: <CompanyProfilePage /> },
            { path: ROUTE_PATHS.subscription, element: <SubscriptionPage /> },
          ],
        },
      ],
    },
  ],
};
