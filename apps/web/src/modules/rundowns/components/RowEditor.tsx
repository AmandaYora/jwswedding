import { ArrowDown, ArrowUp, Plus, Trash2 } from "lucide-react";
import { Button } from "@/shared/components/ui/Button";
import { Input, Textarea, Select } from "@/shared/components/ui/Input";
import { IconActionButton } from "@/shared/components/ui/IconActionButton";
import { EmptyState } from "@/shared/components/feedback/EmptyState";

export interface ColumnDef<T> {
  key: Extract<keyof T, string>;
  label: string;
  /** "text" satu baris, "multiline" kotak teks, "select" pilihan tetap. */
  kind?: "text" | "multiline" | "select";
  options?: { value: string; label: string }[];
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
}

/**
 * Editor tabel baris yang bisa dipakai ulang untuk sembilan dari dua belas
 * seksi buku acara — semuanya berbentuk sama: daftar baris berurutan yang bisa
 * ditambah, dihapus, dan digeser.
 *
 * Urutan baris itu penting, bukan kosmetik: server menyimpan `sort_order`
 * mengikuti urutan array ini, dan urutan itulah yang tercetak di dokumen.
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

  return (
    <div className="flex flex-col gap-3">
      {rows.length === 0 ? (
        <EmptyState title={emptyTitle} description={emptyDescription} />
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full border-separate border-spacing-y-2">
            <thead>
              <tr>
                {columns.map((c) => (
                  <th
                    key={c.key}
                    className="px-2 pb-1 text-left text-[12px] font-medium text-text-secondary"
                    style={c.width ? { width: c.width } : undefined}
                  >
                    {c.label}
                  </th>
                ))}
                <th className="w-24 px-2 pb-1" />
              </tr>
            </thead>
            <tbody>
              {rows.map((row, i) => (
                <tr key={i} className="align-top">
                  {columns.map((c) => (
                    <td key={c.key} className="px-2">
                      {c.kind === "multiline" ? (
                        <Textarea
                          rows={3}
                          value={cell(row, c.key)}
                          placeholder={c.placeholder}
                          onChange={(e) => setCell(i, c.key, e.target.value)}
                        />
                      ) : c.kind === "select" ? (
                        <Select
                          value={cell(row, c.key)}
                          onChange={(e) => setCell(i, c.key, e.target.value)}
                        >
                          {(c.options ?? []).map((o) => (
                            <option key={o.value} value={o.value}>
                              {o.label}
                            </option>
                          ))}
                        </Select>
                      ) : (
                        <Input
                          value={cell(row, c.key)}
                          placeholder={c.placeholder}
                          onChange={(e) => setCell(i, c.key, e.target.value)}
                        />
                      )}
                    </td>
                  ))}
                  <td className="px-2 pt-1.5">
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
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
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
