import { Select } from "@/shared/components/ui/Input";
import { formatMonth } from "@/shared/lib/formatters";

interface MonthSelectProps {
  value: string;
  onChange: (value: string) => void;
  options: string[];
  allLabel?: string;
  className?: string;
}

// MonthSelect is a dumb month dropdown (Blok C, revisi-putri-mom-25082026): it
// renders the "Semua" option plus whatever "YYYY-MM" options the caller passes,
// formatted as Indonesian month names. It is deliberately domain-agnostic —
// which months are offered (and in what order) is decided by the caller, since
// the rule differs per screen (§1.3 #5) — so it stays valid in shared/. When
// `options` is empty it renders just the "Semua" option; callers already guard
// the empty-data case one level up, so it never hides itself.
export function MonthSelect({ value, onChange, options, allLabel = "Semua Bulan", className }: MonthSelectProps) {
  return (
    <Select className={className} value={value} onChange={(e) => onChange(e.target.value)}>
      <option value="">{allLabel}</option>
      {options.map((ym) => (
        <option key={ym} value={ym}>
          {formatMonth(ym)}
        </option>
      ))}
    </Select>
  );
}
