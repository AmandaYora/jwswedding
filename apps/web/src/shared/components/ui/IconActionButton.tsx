import type { LucideIcon } from "lucide-react";
import { cn } from "@/shared/lib/cn";

export type IconActionTone = "neutral" | "info" | "success" | "danger" | "navy";

const TONE_CLASSES: Record<IconActionTone, string> = {
  neutral: "bg-neutral-soft text-text-secondary hover:bg-border",
  info: "bg-info-soft text-info hover:bg-info/20",
  success: "bg-success-soft text-success hover:bg-success/20",
  danger: "bg-danger-soft text-danger hover:bg-danger/20",
  navy: "bg-navy-900/10 text-navy-900 hover:bg-navy-900/20",
};

interface IconActionButtonProps {
  icon: LucideIcon;
  label: string;
  tone?: IconActionTone;
  onClick?: () => void;
  disabled?: boolean;
  // href turns the button into an external link (PLAN revisi-vendor-venue-
  // portal §1.1 poin 3 / §4.6/G3) — same look, rendered as <a> with a new tab.
  // The component stays domain-agnostic: it doesn't know about Instagram, only
  // about linking out.
  href?: string;
}

export function IconActionButton({ icon: Icon, label, tone = "neutral", onClick, disabled = false, href }: IconActionButtonProps) {
  const className = cn(
    "flex h-8 w-8 shrink-0 items-center justify-center rounded-md transition-colors disabled:pointer-events-none disabled:opacity-40",
    TONE_CLASSES[tone]
  );
  if (href) {
    return (
      <a
        href={href}
        target="_blank"
        rel="noopener noreferrer"
        title={label}
        aria-label={label}
        className={className}
      >
        <Icon className="h-4 w-4" />
      </a>
    );
  }
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      title={label}
      aria-label={label}
      className={className}
    >
      <Icon className="h-4 w-4" />
    </button>
  );
}
