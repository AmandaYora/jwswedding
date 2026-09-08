export function formatCurrency(amount: number): string {
  const sign = amount < 0 ? "-" : "";
  const abs = Math.abs(amount);
  return `${sign}Rp ${abs.toLocaleString("id-ID")}`;
}

export function formatCurrencyCompact(amount: number): string {
  const abs = Math.abs(amount);
  const sign = amount < 0 ? "-" : "";
  if (abs >= 1_000_000_000) return `${sign}Rp ${(abs / 1_000_000_000).toFixed(1).replace(/\.0$/, "")} M`;
  if (abs >= 1_000_000) return `${sign}Rp ${(abs / 1_000_000).toFixed(1).replace(/\.0$/, "")} Jt`;
  return formatCurrency(amount);
}

export const MONTHS_ID = [
  "Jan", "Feb", "Mar", "Apr", "Mei", "Jun", "Jul", "Agu", "Sep", "Okt", "Nov", "Des",
];

export function formatDate(dateStr: string | null): string {
  if (!dateStr) return "-";
  const [y, m, d] = dateStr.split("T")[0].split("-").map(Number);
  return `${d} ${MONTHS_ID[m - 1]} ${y}`;
}

// formatMonth turns a "YYYY-MM" string into an Indonesian long month + year
// label, e.g. "2026-09" -> "September 2026". Empty input returns "" (used by
// MonthSelect for the "Semua Bulan" option). Blok C, revisi-putri-mom-25082026.
export function formatMonth(ym: string): string {
  if (!ym) return "";
  const [y, m] = ym.split("-").map(Number);
  if (!y || !m) return "";
  return new Intl.DateTimeFormat("id-ID", { month: "long", year: "numeric" }).format(new Date(y, m - 1, 1));
}

export function formatDateTime(dateStr: string | null): string {
  if (!dateStr) return "-";
  const [datePart, timePart] = dateStr.split("T");
  const dateLabel = formatDate(datePart);
  if (!timePart) return dateLabel;
  return `${dateLabel}, ${timePart.slice(0, 5)}`;
}

export function addMonths(dateISO: string, months: number): string {
  const [y, m, d] = dateISO.split("-").map(Number);
  const next = new Date(y, m - 1 + months, d);
  return `${next.getFullYear()}-${String(next.getMonth() + 1).padStart(2, "0")}-${String(next.getDate()).padStart(2, "0")}`;
}

export function toDateOnly(dateStr: string): Date {
  const [y, m, d] = dateStr.split("T")[0].split("-").map(Number);
  return new Date(y, m - 1, d);
}

// Real "today" (ISO date, local time) — the single source of truth for
// "today" across the frontend, replacing the pre-integration mock's fixed
// TODAY constant now that all consoles talk to a real, live backend.
export function todayISO(): string {
  const now = new Date();
  return `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, "0")}-${String(now.getDate()).padStart(2, "0")}`;
}

export function daysBetween(fromISO: string, toISO: string): number {
  const from = toDateOnly(fromISO);
  const to = toDateOnly(toISO);
  const msPerDay = 1000 * 60 * 60 * 24;
  return Math.round((to.getTime() - from.getTime()) / msPerDay);
}

export function initialsFromName(name: string): string {
  const parts = name.trim().split(/\s+/);
  if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase();
  return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase();
}
