import type { ComponentType, ReactNode } from "react";
import { AlertTriangle, Loader2, RefreshCw } from "lucide-react";
import { cn } from "@/shared/lib/cn";

/**
 * The three non-content states every Client Portal tab needs, in one shape.
 *
 * Before this, each tab rendered its own ad-hoc version of at most one of
 * them, and none had an error state at all: a failed request and a genuinely
 * empty project were shown to the client identically ("Belum ada dokumen"),
 * with no way to retry. These live in the client-portal module rather than
 * `shared/components/ui` on purpose — the copy is client-facing and in the
 * portal's own reassuring register, which is not domain-agnostic.
 */

export function PortalLoading({
  label = "Memuat...",
  bare = false,
  className,
}: {
  label?: string;
  /** Drops the card chrome, for a caller that already renders one around it. */
  bare?: boolean;
  className?: string;
}) {
  return (
    <div
      role="status"
      aria-live="polite"
      className={cn(
        "flex flex-col items-center justify-center gap-3 px-6 py-16 text-center",
        !bare && "rounded-3xl border border-border bg-white shadow-sm",
        className
      )}
    >
      <Loader2 className="h-6 w-6 animate-spin text-navy-600" />
      <p className="text-[13.5px] text-text-secondary">{label}</p>
    </div>
  );
}

export function PortalError({
  message,
  onRetry,
  className,
}: {
  message: string;
  onRetry?: () => void;
  className?: string;
}) {
  return (
    <div
      role="alert"
      className={cn(
        "flex flex-col items-center gap-3 rounded-3xl border border-danger/25 bg-danger-soft/40 px-6 py-12 text-center",
        className
      )}
    >
      <div className="flex h-12 w-12 items-center justify-center rounded-full bg-danger-soft text-danger">
        <AlertTriangle className="h-6 w-6" />
      </div>
      <p className="max-w-md text-[14px] font-semibold text-text-primary">{message}</p>
      {onRetry && (
        <button
          type="button"
          onClick={onRetry}
          className="mt-1 inline-flex items-center gap-1.5 rounded-lg border border-border bg-white px-3.5 py-2 text-[13px] font-semibold text-navy-900 shadow-sm transition-colors hover:bg-navy-50"
        >
          <RefreshCw className="h-3.5 w-3.5" /> Coba lagi
        </button>
      )}
    </div>
  );
}

export function PortalEmpty({
  icon: Icon,
  title,
  description,
  tone = "neutral",
  className,
}: {
  icon: ComponentType<{ className?: string }>;
  title: ReactNode;
  description?: ReactNode;
  /** "positive" is for an empty state that is genuinely good news (no kendala). */
  tone?: "neutral" | "positive" | "info";
  className?: string;
}) {
  const iconTone =
    tone === "positive"
      ? "border border-emerald-100 bg-emerald-50 text-emerald-500"
      : tone === "info"
      ? "bg-info-soft text-info"
      : "bg-surface-muted text-text-secondary";

  return (
    <div
      className={cn(
        "flex flex-col items-center gap-3 rounded-3xl border border-dashed border-border bg-white px-6 py-12 text-center shadow-sm sm:py-16",
        className
      )}
    >
      <div className={cn("mb-1 flex h-16 w-16 items-center justify-center rounded-full", iconTone)}>
        <Icon className="h-8 w-8" />
      </div>
      <p className="max-w-md text-[15px] font-bold text-navy-950 sm:text-[16px]">{title}</p>
      {description && <p className="max-w-sm text-[13.5px] leading-relaxed text-text-secondary sm:text-[14px]">{description}</p>}
    </div>
  );
}
