import type { CSSProperties } from "react";
import { ArrowDown, ArrowUp, Plus, Trash2 } from "lucide-react";
import { Button } from "@/shared/components/ui/Button";
import { Input, Textarea, Select } from "@/shared/components/ui/Input";
import { IconActionButton } from "@/shared/components/ui/IconActionButton";
import { EmptyState } from "@/shared/components/feedback/EmptyState";
import { cn } from "@/shared/lib/cn";
import type { SectionErrors } from "@/modules/rundowns/lib/section-errors";

export interface ColumnDef<T> {
  key: Extract<keyof T, string>;
  label: string;
  /**
   * "text" satu baris, "multiline" kotak teks, "select" pilihan tetap,
   * "checkbox" saklar dua nilai (lihat `checkbox`), "display" hanya tampilan
   * (lihat `display`) — tidak bisa diketik.
   */
  kind?: "text" | "multiline" | "select" | "checkbox" | "display";
  options?: { value: string; label: string }[];
  /** Nilai sel saat tercentang / tidak — untuk kind "checkbox". */
  checkbox?: { on: string; off: string };
  /** Isi sel untuk kind "display". */
  display?: (row: T, index: number) => string;
  width?: string;
  placeholder?: string;
}

interface RowEditorProps<T> {
  columns: ColumnDef<T>[];
  rows: T[];
  onChange: (rows: T[]) => void;
  blank: () => T;
  addLabel?: string;
  emptyTitle?: string;
  emptyDescription?: string;
  /** Galat per sel, dikunci `${errorPrefix}.${index}.${key}`. */
  errors?: SectionErrors;
  errorPrefix?: string;
}

// Lebar kolom aksi: tiga tombol ikon 32 px beserta jaraknya.
const ACTIONS_WIDTH = "6.75rem";

/**
 * Editor baris yang dipakai ulang oleh hampir semua seksi buku acara —
 * semuanya berbentuk sama: daftar baris berurutan yang bisa ditambah, dihapus,
 * dan digeser.
 *
 * Urutan baris itu penting, bukan kosmetik: server menyimpan `sort_order`
 * mengikuti urutan array ini, dan urutan itulah yang tercetak di dokumen.
 *
 * Satu markup untuk semua ukuran layar: di layar lebar tiap baris adalah grid
 * dengan lebar kolom tetap (tampil seperti tabel); di HP grid-nya satu kolom
 * dan setiap baris menjadi kartu berlabel. Input tidak dirender dua kali.
 */
// T sengaja tidak dibatasi Record<string, string>: tipe seksi yang nyata
// punya field ber-union (mis. style: "Heading" | "Numbered"), dan batasan itu
// akan menolak semuanya. Nilai sel dibaca/ditulis sebagai string di sini,
// lalu dikembalikan ke bentuk T -- pilihan sadar, karena seluruh kolom yang
// diedit komponen ini memang bertipe string di baliknya.
export function RowEditor<T>({
  columns,
  rows,
  onChange,
  blank,
  addLabel = "Tambah baris",
  emptyTitle = "Belum ada baris",
  emptyDescription = "Seksi yang dibiarkan kosong tetap tercetak sebagai kerangka di dokumen.",
  errors,
  errorPrefix,
}: RowEditorProps<T>) {
  const cell = (row: T, key: Extract<keyof T, string>) => String(row[key] ?? "");

  const setCell = (index: number, key: Extract<keyof T, string>, value: string) => {
    onChange(rows.map((row, i) => (i === index ? ({ ...row, [key]: value } as T) : row)));
  };

  const move = (index: number, delta: number) => {
    const target = index + delta;
    if (target < 0 || target >= rows.length) return;
    const next = [...rows];
    [next[index], next[target]] = [next[target], next[index]];
    onChange(next);
  };

  const errorFor = (index: number, key: string) =>
    errorPrefix ? errors?.[`${errorPrefix}.${index}.${key}`] : undefined;

  const gridStyle = {
    "--row-cols": [...columns.map((c) => c.width ?? "minmax(0, 1fr)"), ACTIONS_WIDTH].join(" "),
  } as CSSProperties;
  const gridClass = "grid grid-cols-1 gap-2 md:gap-3 md:[grid-template-columns:var(--row-cols)]";

  return (
    <div className="flex flex-col gap-3">
      {rows.length === 0 ? (
        <EmptyState title={emptyTitle} description={emptyDescription} />
      ) : (
        <div className="flex flex-col gap-3 md:gap-2">
          <div className={cn(gridClass, "hidden md:grid")} style={gridStyle} aria-hidden="true">
            {columns.map((c, ci) => (
              // Indeks ikut dalam key: dua kolom boleh menyunting field yang
              // sama (mis. nomor tampilan + saklar "Tanpa nomor" di noLabel).
              <span key={`${c.key}:${ci}`} className="px-0.5 text-[12px] font-medium text-text-secondary">
                {c.label}
              </span>
            ))}
            <span />
          </div>

          {rows.map((row, i) => (
            <div
              key={i}
              className={cn(
                gridClass,
                "items-start rounded-md border border-border p-3",
                "md:rounded-none md:border-0 md:p-0"
              )}
              style={gridStyle}
            >
              {columns.map((c, ci) => {
                const error = c.kind === "display" ? undefined : errorFor(i, c.key);
                const a11yLabel = `${c.label} baris ${i + 1}`;
                const invalid = error ? "border-danger focus:border-danger focus:ring-danger/20" : undefined;
                return (
                  <div key={`${c.key}:${ci}`} className="flex min-w-0 flex-col gap-1">
                    {c.kind !== "checkbox" && (
                      <span className="text-[12px] font-medium text-text-secondary md:hidden">{c.label}</span>
                    )}
                    {c.kind === "multiline" ? (
                      <Textarea
                        rows={3}
                        value={cell(row, c.key)}
                        placeholder={c.placeholder}
                        aria-label={a11yLabel}
                        aria-invalid={error ? true : undefined}
                        className={invalid}
                        onChange={(e) => setCell(i, c.key, e.target.value)}
                      />
                    ) : c.kind === "select" ? (
                      <Select value={cell(row, c.key)} onChange={(e) => setCell(i, c.key, e.target.value)}>
                        {(c.options ?? []).map((o) => (
                          <option key={o.value} value={o.value}>
                            {o.label}
                          </option>
                        ))}
                      </Select>
                    ) : c.kind === "checkbox" && c.checkbox ? (
                      <label className="flex h-9 cursor-pointer items-center gap-2 text-sm text-text-primary">
                        <input
                          type="checkbox"
                          className="h-4 w-4 rounded border-border accent-navy-900"
                          checked={cell(row, c.key) === c.checkbox.on}
                          aria-label={a11yLabel}
                          onChange={(e) =>
                            setCell(i, c.key, e.target.checked ? c.checkbox!.on : c.checkbox!.off)
                          }
                        />
                        <span className="md:sr-only">{c.label}</span>
                      </label>
                    ) : c.kind === "display" ? (
                      <div
                        className="flex h-9 items-center px-0.5 text-sm font-medium tabular-nums text-text-secondary"
                        aria-label={a11yLabel}
                      >
                        {c.display?.(row, i) || "—"}
                      </div>
                    ) : (
                      <Input
                        value={cell(row, c.key)}
                        placeholder={c.placeholder}
                        aria-label={a11yLabel}
                        aria-invalid={error ? true : undefined}
                        className={invalid}
                        onChange={(e) => setCell(i, c.key, e.target.value)}
                      />
                    )}
                    {error && <span className="text-xs text-danger">{error}</span>}
                  </div>
                );
              })}
              <div className="flex items-center justify-between gap-1 md:justify-start md:pt-0.5">
                <span className="text-[12px] text-text-secondary md:hidden">Baris {i + 1}</span>
                <div className="flex items-center gap-1">
                  <IconActionButton
                    icon={ArrowUp}
                    label="Naikkan"
                    onClick={() => move(i, -1)}
                    disabled={i === 0}
                  />
                  <IconActionButton
                    icon={ArrowDown}
                    label="Turunkan"
                    onClick={() => move(i, 1)}
                    disabled={i === rows.length - 1}
                  />
                  <IconActionButton
                    icon={Trash2}
                    label="Hapus baris"
                    tone="danger"
                    onClick={() => onChange(rows.filter((_, x) => x !== i))}
                  />
                </div>
              </div>
            </div>
          ))}
        </div>
      )}

      <div>
        <Button
          variant="secondary"
          icon={<Plus className="h-4 w-4" />}
          onClick={() => onChange([...rows, blank()])}
        >
          {addLabel}
        </Button>
      </div>
    </div>
  );
}
