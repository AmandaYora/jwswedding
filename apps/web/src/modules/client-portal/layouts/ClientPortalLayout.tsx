import { Suspense, useCallback, useEffect, useState } from "react";
import { Navigate, Outlet, useNavigate } from "react-router-dom";
import { AlertTriangle, FolderKanban, Heart, Loader2, LogOut, RefreshCw } from "lucide-react";
import { TabNav } from "@/shared/components/ui/TabNav";
import { useProjectStore } from "@/modules/projects/stores/useProjectStore";
import type { Project } from "@/modules/projects/types";
import { daysUntil } from "@/modules/projects/lib/dates";
import { APP_NAME } from "@/shared/constants/brand";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { logoutAndRedirect } from "@/shared/lib/auth-actions";
import { useTenantBrandingStore } from "@/shared/stores/useTenantBrandingStore";
import { ClientProjectHeaderCard } from "@/modules/client-portal/components/ClientProjectHeaderCard";
import { resetPortalSectionCache } from "@/modules/client-portal/hooks/usePortalSections";
import { getApiErrorMessage } from "@/shared/lib/api-error";
import { formatDate } from "@/shared/lib/formatters";

export interface ClientPortalContext {
  projectId: string;
}

// "Ringkasan" no longer exists as its own tab (PLAN.md's Client Portal
// restructure) — its compact summary now lives in the persistent
// ClientProjectHeaderCard below, always visible above these tabs; its
// detailed milestone stepper lives in the new "Timeline" tab. "Venue" no
// longer has its own tab either (client-facing-only Vendor/Venue merge) —
// Venue is now a pinned card inside "Vendor" itself (VendorTabPage), so this
// list no longer mirrors WO Console's own separate Vendor/Venue tab split.
// "Dokumen" is appended at the end deliberately, not interleaved with the
// existing tabs (PLAN.md) — it's a new, separate capability (general project
// documents the staff explicitly opted to share), not a replacement or
// reordering of anything already here.
const TABS = [
  { to: ROUTE_PATHS.portal("vendor"), label: "Vendor" },
  { to: ROUTE_PATHS.portal("timeline"), label: "Timeline" },
  { to: ROUTE_PATHS.portal("pembayaran"), label: "Pembayaran" },
  { to: ROUTE_PATHS.portal("kendala"), label: "Kendala" },
  { to: ROUTE_PATHS.portal("dokumen"), label: "Dokumen" },
];

type PortalStatus = "loading" | "ready" | "denied" | "choose" | "failed";

// countdownCopy — the header pill's wording. `d >= 0 ? d : 0` used to render
// "0 hari lagi" for BOTH the wedding day itself and any day after it, so a
// project left open past its event date told the client, every day, that the
// wedding was still zero days away.
function countdownCopy(days: number): { value: string; unit: string } {
  if (days > 0) return { value: String(days), unit: "hari lagi" };
  if (days === 0) return { value: "Hari ini", unit: "" };
  return { value: String(Math.abs(days)), unit: "hari berlalu" };
}

export default function ClientPortalLayout() {
  const navigate = useNavigate();
  const project = useProjectStore((s) => s.currentProject);
  const fetchMyProjects = useProjectStore((s) => s.fetchMyProjects);
  const fetchProjectDetail = useProjectStore((s) => s.fetchProjectDetail);
  const logoUrl = useTenantBrandingStore((s) => s.logoUrl);
  const brandName = useTenantBrandingStore((s) => s.businessName);
  const hydrateBranding = useTenantBrandingStore((s) => s.hydrate);
  const brandingHydrated = useTenantBrandingStore((s) => s.hydrated);
  const [status, setStatus] = useState<PortalStatus>("loading");
  const [myProjects, setMyProjects] = useState<Project[]>([]);
  const [loadError, setLoadError] = useState<string | null>(null);
  // Which project the picker is currently opening. Without it a slow pick
  // looked like a dead button, and a second click fired a second request.
  const [pickingId, setPickingId] = useState<string | null>(null);
  const [pickError, setPickError] = useState<string | null>(null);
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    let cancelled = false;
    setStatus("loading");
    fetchMyProjects()
      .then((list) => {
        if (cancelled) return;
        setMyProjects(list);
        if (list.length === 0) setStatus("denied");
        // D5: pemilih project HANYA bila >1 — satu project langsung masuk.
        else if (list.length === 1) setStatus("ready");
        else setStatus("choose");
      })
      .catch((err) => {
        if (cancelled) return;
        // A failed REQUEST is not the same as "this account owns no project".
        // Treating both as `denied` bounced the client out to /login on any
        // transient 5xx or dropped connection, with no explanation.
        setLoadError(getApiErrorMessage(err, "Gagal memuat data pernikahan Anda."));
        setStatus("failed");
      });
    return () => {
      cancelled = true;
    };
  }, [fetchMyProjects, attempt]);

  useEffect(() => {
    void hydrateBranding("Portal Klien");
  }, [hydrateBranding]);

  // Leaving the portal (logout, or navigating away) drops the "already
  // loaded" marks, so the next session shows its loading states again
  // instead of the previous one's data.
  useEffect(() => resetPortalSectionCache, []);

  const pickProject = useCallback(
    (id: string) => {
      setPickingId(id);
      setPickError(null);
      fetchProjectDetail(id)
        .then(() => setStatus("ready"))
        .catch((err) => setPickError(getApiErrorMessage(err, "Gagal membuka project ini.")))
        .finally(() => setPickingId(null));
    },
    [fetchProjectDetail]
  );

  if (status === "denied") {
    return <Navigate to={ROUTE_PATHS.login} replace />;
  }

  if (status === "failed") {
    return (
      <div className="flex min-h-screen items-center justify-center bg-background px-4">
        <div className="w-full max-w-sm rounded-2xl border border-border bg-white p-6 text-center shadow-sm">
          <div className="mx-auto mb-3 flex h-12 w-12 items-center justify-center rounded-full bg-danger-soft text-danger">
            <AlertTriangle className="h-6 w-6" />
          </div>
          <p className="text-[14.5px] font-semibold text-text-primary">{loadError}</p>
          <p className="mt-1 text-[13px] text-text-secondary">Koneksi mungkin sedang terganggu.</p>
          <button
            type="button"
            onClick={() => setAttempt((a) => a + 1)}
            className="mt-4 inline-flex items-center gap-1.5 rounded-lg border border-border bg-white px-3.5 py-2 text-[13px] font-semibold text-navy-900 shadow-sm transition-colors hover:bg-navy-50"
          >
            <RefreshCw className="h-3.5 w-3.5" /> Coba lagi
          </button>
          <button
            type="button"
            onClick={() => void logoutAndRedirect(navigate)}
            className="mt-4 block w-full text-[12.5px] font-medium text-text-secondary underline-offset-2 hover:text-text-primary hover:underline"
          >
            Keluar
          </button>
        </div>
      </div>
    );
  }

  if (status === "choose") {
    return (
      <div className="flex min-h-screen items-center justify-center bg-background px-4 py-10">
        <div className="w-full max-w-md rounded-xl border border-border bg-surface p-6">
          <h1 className="text-lg font-bold text-text-primary">Pilih Project</h1>
          <p className="mt-1 text-[13px] text-text-secondary">
            Akun ini terhubung ke {myProjects.length} project — pilih yang ingin dilihat.
          </p>
          {pickError && (
            <p className="mt-3 rounded-md border border-danger/30 bg-danger-soft px-3 py-2 text-[12.5px] font-medium text-danger">
              {pickError}
            </p>
          )}
          <div className="mt-4 flex flex-col gap-2">
            {myProjects.map((p) => {
              const isPicking = pickingId === p.id;
              return (
                <button
                  key={p.id}
                  type="button"
                  disabled={pickingId !== null}
                  onClick={() => pickProject(p.id)}
                  className="flex items-center gap-3 rounded-lg border border-border px-4 py-3 text-left transition-colors hover:border-navy-300 hover:bg-navy-50 disabled:cursor-not-allowed disabled:opacity-60"
                >
                  {isPicking ? (
                    <Loader2 className="h-5 w-5 shrink-0 animate-spin text-navy-700" />
                  ) : (
                    <FolderKanban className="h-5 w-5 shrink-0 text-navy-700" />
                  )}
                  <span className="min-w-0">
                    <span className="block truncate text-[14px] font-semibold text-text-primary">{p.name}</span>
                    <span className="block text-[12.5px] text-text-secondary">
                      {p.brideName} &amp; {p.groomName} · {formatDate(p.eventDate)}
                    </span>
                  </span>
                </button>
              );
            })}
          </div>
          <div className="mt-4 flex items-center gap-4">
            {/* Only reachable via the header's "Ganti project" — on first
                entry there is nothing to go back to yet. */}
            {project && (
              <button
                type="button"
                disabled={pickingId !== null}
                onClick={() => setStatus("ready")}
                className="text-[12.5px] font-medium text-navy-900 underline-offset-2 hover:underline disabled:opacity-60"
              >
                Kembali ke {project.name}
              </button>
            )}
            <button
              type="button"
              onClick={() => void logoutAndRedirect(navigate)}
              className="text-[12.5px] font-medium text-text-secondary underline-offset-2 hover:text-text-primary hover:underline"
            >
              Keluar
            </button>
          </div>
        </div>
      </div>
    );
  }

  if (status === "loading" || !project || !brandingHydrated) {
    // Also waits on brandingHydrated, not just fetchMyProjects -- the header
    // below reads logoUrl/brandName and `text-navy-950` (a live
    // --brand-navy-* CSS var) straight off useTenantBrandingStore; painting
    // it before hydrate() resolves would show the app's own generic
    // Heart-icon/navy default first, then swap to the tenant's real one.
    return (
      <div className="flex min-h-screen items-center justify-center bg-white">
        <Loader2 className="h-6 w-6 animate-spin text-text-secondary" />
      </div>
    );
  }

  const isOpenProject = project.status !== "Completed" && project.status !== "Cancelled";
  const countdown = countdownCopy(daysUntil(project.eventDate));

  return (
    // No `overflow-hidden` here. It used to be, to contain the decorative
    // grid below -- but that layer is `fixed`, so it never needed containing,
    // and an `overflow` ancestor makes itself the scrollport for every
    // `position: sticky` descendant. Since this box does not scroll (the page
    // does), the header below simply scrolled away instead of sticking.
    <div className="relative min-h-screen bg-background">
      {/* Decorative grid background for depth */}
      <div className="pointer-events-none fixed inset-0 z-0">
        <div className="absolute inset-0 bg-[linear-gradient(to_right,#f1f5f9_1px,transparent_1px),linear-gradient(to_bottom,#f1f5f9_1px,transparent_1px)] bg-[size:4rem_4rem] [mask-image:radial-gradient(ellipse_60%_50%_at_50%_0%,#000_70%,transparent_100%)] opacity-60"></div>
      </div>

      <div className="sticky top-0 z-30 border-b border-border bg-white/80 shadow-sm backdrop-blur-xl">
        <div className="border-b border-border/50 bg-white/50">
          <div className="mx-auto flex max-w-5xl items-center justify-between gap-3 px-4 py-3 sm:px-6">
            <span className="flex shrink-0 items-center gap-2 text-[14px] font-bold text-navy-950">
              {logoUrl ? (
                <img src={logoUrl} alt={brandName ?? APP_NAME} className="h-7 w-auto max-w-[140px] object-contain" />
              ) : (
                <>
                  <div className="flex h-7 w-7 items-center justify-center rounded-lg bg-gradient-to-tr from-navy-900 to-navy-800 text-white shadow-sm">
                    <Heart className="h-4 w-4" fill="currentColor" />
                  </div>
                  {brandName ?? APP_NAME}
                </>
              )}
            </span>

            <div className="flex min-w-0 flex-1 items-center justify-end gap-2.5 sm:justify-center sm:gap-4">
              <span className="hidden min-w-0 truncate text-[14px] font-semibold text-navy-950 sm:block">
                {project.brideName} &amp; {project.groomName}
              </span>
              {isOpenProject ? (
                <span className="flex shrink-0 items-center gap-1.5 whitespace-nowrap rounded-full border border-blue-100 bg-blue-50 px-3 py-1 shadow-sm">
                  <span className="text-[13px] font-bold tabular-nums text-blue-700">{countdown.value}</span>
                  {countdown.unit && <span className="text-[11px] font-medium text-blue-600/80">{countdown.unit}</span>}
                </span>
              ) : (
                <span className="shrink-0 whitespace-nowrap rounded-full border border-border bg-surface-muted px-3 py-1 text-[11.5px] font-semibold text-text-secondary">
                  {project.status === "Completed" ? "Acara selesai" : "Dibatalkan"}
                </span>
              )}
            </div>

            <div className="flex shrink-0 items-center gap-1">
              {/* A client with more than one project picked one on entry and
                  then had no way back to the picker short of logging out. */}
              {myProjects.length > 1 && (
                <button
                  type="button"
                  onClick={() => {
                    setPickError(null);
                    setStatus("choose");
                  }}
                  aria-label="Ganti project"
                  className="flex items-center gap-1.5 rounded-full px-3 py-1.5 text-[12.5px] font-medium text-text-secondary transition-colors hover:bg-navy-50 hover:text-navy-900"
                >
                  <FolderKanban className="h-3.5 w-3.5" /> <span className="hidden sm:inline">Ganti project</span>
                </button>
              )}
              <button
                onClick={() => void logoutAndRedirect(navigate)}
                aria-label="Keluar"
                className="flex items-center gap-1.5 rounded-full px-3 py-1.5 text-[12.5px] font-medium text-text-secondary transition-colors hover:bg-danger-soft hover:text-danger"
              >
                <LogOut className="h-3.5 w-3.5" /> <span className="hidden sm:inline">Keluar</span>
              </button>
            </div>
          </div>
          <div className="mx-auto max-w-5xl px-4 pb-3 sm:hidden">
            <span className="block truncate text-[13.5px] font-semibold text-navy-950">
              {project.brideName} &amp; {project.groomName}
            </span>
          </div>
        </div>

        {/* `bare`: this header already paints its own surface and bottom
            border. TabNav's defaults added a second, visibly doubled rule and
            a grey `bg-background` band across the otherwise white bar. */}
        <div className="mx-auto max-w-5xl px-4 sm:px-6">
          <TabNav items={TABS} sticky={false} bare />
        </div>
      </div>

      {/* `relative` without a z-index: enough to paint above the decorative
          z-0 layer, but NOT a stacking context, which would trap every
          overlay rendered inside beneath the z-30 header above. */}
      <main className="relative mx-auto flex max-w-5xl flex-col gap-6 px-4 py-8 sm:gap-8 sm:px-6 sm:py-10">
        <ClientProjectHeaderCard projectId={project.id} />
        <Suspense fallback={<div className="animate-pulse py-16 text-center text-sm text-text-secondary">Memuat...</div>}>
          <Outlet context={{ projectId: project.id } satisfies ClientPortalContext} />
        </Suspense>
      </main>
    </div>
  );
}
