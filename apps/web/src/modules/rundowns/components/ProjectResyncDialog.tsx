import { useEffect, useMemo, useState } from "react";
import { RefreshCw } from "lucide-react";
import { Button } from "@/shared/components/ui/Button";
import { Modal } from "@/shared/components/ui/Modal";
import { useRundownStore } from "@/modules/rundowns/stores/useRundownStore";
import { useProjectVendorPrefill } from "@/modules/rundowns/hooks/useProjectVendorPrefill";
import { getApiErrorMessage } from "@/shared/lib/api-error";
import type { RundownCoverPrefill, RundownDetail, RundownVendor } from "@/modules/rundowns/types";

export type ResyncKind = "cover" | "vendors";

interface ProjectResyncProps {
  kind: ResyncKind;
  rundownId: string;
  projectId: string;
  draft: RundownDetail;
  patch: (fn: (d: RundownDetail) => RundownDetail) => void;
}

/**
 * Tombol "Tarik ulang dari project" beserta dialognya (PLAN rundown-ux-ideal
 * §6.5). Tidak pernah menimpa diam-diam: WO melihat perbedaannya lebih dulu,
 * dan yang diterapkan hanya menambal draft — tersimpan lewat mekanisme
 * simpan-saat-transisi editor seperti suntingan biasa.
 */
export function ProjectResync(props: ProjectResyncProps) {
  const [open, setOpen] = useState(false);
  return (
    <>
      <div className="flex justify-end">
        <Button variant="ghost" size="sm" icon={<RefreshCw className="h-4 w-4" />} onClick={() => setOpen(true)}>
          Tarik ulang dari project
        </Button>
      </div>
      <Modal
        open={open}
        onClose={() => setOpen(false)}
        title={props.kind === "cover" ? "Tarik ulang sampul dari project" : "Tarik ulang vendor dari project"}
        description={
          props.kind === "cover"
            ? "Bandingkan isi sampul dengan data project terkini, lalu pilih yang ingin diperbarui."
            : "Bandingkan daftar vendor dengan vendor yang terpasang di project saat ini."
        }
        size="lg"
      >
        {open &&
          (props.kind === "cover" ? (
            <CoverResync {...props} onDone={() => setOpen(false)} />
          ) : (
            <VendorResync {...props} onDone={() => setOpen(false)} />
          ))}
      </Modal>
    </>
  );
}

const COVER_FIELDS: { key: keyof RundownCoverPrefill; label: string }[] = [
  { key: "groomName", label: "Nama Pengantin Pria" },
  { key: "brideName", label: "Nama Pengantin Wanita" },
  { key: "eventDateLabel", label: "Tanggal acara" },
  { key: "eventTimeLabel", label: "Jam acara" },
  { key: "venueLabel", label: "Venue" },
  { key: "coupleTitle", label: "Judul lampiran" },
];

function CoverResync({ rundownId, draft, patch, onDone }: ProjectResyncProps & { onDone: () => void }) {
  const fetchProjectPrefill = useRundownStore((s) => s.fetchProjectPrefill);
  const [prefill, setPrefill] = useState<RundownCoverPrefill | null>(null);
  const [error, setError] = useState("");
  const [chosen, setChosen] = useState<Set<string>>(new Set());

  useEffect(() => {
    let cancelled = false;
    fetchProjectPrefill(rundownId)
      .then((p) => {
        if (cancelled) return;
        setPrefill(p);
      })
      .catch((e) => !cancelled && setError(getApiErrorMessage(e, "Data project gagal dimuat")));
    return () => {
      cancelled = true;
    };
  }, [rundownId, fetchProjectPrefill]);

  // Hanya field yang di project TERISI dan berbeda: nilai kosong di project
  // tidak boleh diusulkan untuk menghapus isian WO.
  const diffs = useMemo(() => {
    if (!prefill) return [];
    return COVER_FIELDS.filter(
      (f) => prefill[f.key].trim() !== "" && prefill[f.key].trim() !== draft.cover[f.key].trim()
    );
  }, [prefill, draft.cover]);

  useEffect(() => setChosen(new Set(diffs.map((d) => d.key))), [diffs]);

  if (error) return <p className="text-[13px] text-danger">{error}</p>;
  if (!prefill) return <p className="text-[13px] text-text-secondary">Memuat data project...</p>;
  if (diffs.length === 0) {
    return (
      <div className="flex flex-col gap-4">
        <p className="text-[13px] text-text-secondary">Sampul sudah sama dengan data project.</p>
        <div className="flex justify-end">
          <Button variant="secondary" onClick={onDone}>
            Tutup
          </Button>
        </div>
      </div>
    );
  }

  const apply = () => {
    patch((d) => {
      const cover = { ...d.cover };
      for (const f of diffs) if (chosen.has(f.key)) cover[f.key] = prefill[f.key];
      return { ...d, cover };
    });
    onDone();
  };

  return (
    <div className="flex flex-col gap-4">
      <ul className="flex flex-col divide-y divide-border-light rounded-md border border-border">
        {diffs.map((f) => (
          <li key={f.key}>
            <label className="flex cursor-pointer items-start gap-3 px-3 py-2.5">
              <input
                type="checkbox"
                className="mt-0.5 h-4 w-4 rounded border-border accent-navy-900"
                checked={chosen.has(f.key)}
                onChange={(e) =>
                  setChosen((prev) => {
                    const next = new Set(prev);
                    if (e.target.checked) next.add(f.key);
                    else next.delete(f.key);
                    return next;
                  })
                }
              />
              <span className="flex min-w-0 flex-col gap-0.5 text-[13px]">
                <span className="font-medium text-text-primary">{f.label}</span>
                <span className="text-text-secondary">
                  <span className="line-through">{draft.cover[f.key] || "(kosong)"}</span>
                  {" → "}
                  <span className="text-text-primary">{prefill[f.key]}</span>
                </span>
              </span>
            </label>
          </li>
        ))}
      </ul>
      <div className="flex justify-end gap-2">
        <Button variant="secondary" onClick={onDone}>
          Batal
        </Button>
        <Button disabled={chosen.size === 0} onClick={apply}>
          Terapkan {chosen.size} perubahan
        </Button>
      </div>
    </div>
  );
}

function vendorKey(v: RundownVendor): string {
  return `${v.categoryLabel.trim().toUpperCase()}|${v.vendorName.trim().toUpperCase()}`;
}

/**
 * Menyisipkan vendor baru tepat setelah baris terakhir berkategori sama —
 * dokumen hanya mencetak judul kategori sekali untuk baris yang berurutan,
 * dan urutan buatan WO tetap terjaga. Kategori yang belum ada ditaruh di akhir.
 */
export function mergeVendors(current: RundownVendor[], additions: RundownVendor[]): RundownVendor[] {
  const out = [...current];
  for (const add of additions) {
    const category = add.categoryLabel.trim().toUpperCase();
    let at = -1;
    out.forEach((v, i) => {
      if (v.categoryLabel.trim().toUpperCase() === category) at = i;
    });
    if (at === -1) out.push(add);
    else out.splice(at + 1, 0, add);
  }
  return out;
}

function VendorResync({ projectId, draft, patch, onDone }: ProjectResyncProps & { onDone: () => void }) {
  const { rows, loading, error } = useProjectVendorPrefill(projectId);
  const missing = useMemo(() => {
    const have = new Set(draft.vendors.map(vendorKey));
    return rows.filter((r) => !have.has(vendorKey(r)));
  }, [rows, draft.vendors]);

  if (error) return <p className="text-[13px] text-danger">{error}</p>;
  if (loading) return <p className="text-[13px] text-text-secondary">Memuat vendor project...</p>;

  const replaceAll = () => {
    patch((d) => ({ ...d, vendors: rows }));
    onDone();
  };
  const addMissing = () => {
    patch((d) => ({ ...d, vendors: mergeVendors(d.vendors, missing) }));
    onDone();
  };

  return (
    <div className="flex flex-col gap-4">
      {rows.length === 0 ? (
        <p className="text-[13px] text-text-secondary">Belum ada vendor yang terpasang di project ini.</p>
      ) : missing.length === 0 ? (
        <p className="text-[13px] text-text-secondary">
          Semua {rows.length} vendor project sudah ada di daftar ini.
        </p>
      ) : (
        <div className="flex flex-col gap-2">
          <p className="text-[13px] text-text-primary">
            {missing.length} vendor di project belum ada di daftar ini:
          </p>
          <ul className="flex flex-col divide-y divide-border-light rounded-md border border-border text-[13px]">
            {missing.map((v) => (
              <li key={vendorKey(v)} className="flex justify-between gap-3 px-3 py-2">
                <span className="font-medium text-text-primary">{v.vendorName}</span>
                <span className="text-text-secondary">{v.categoryLabel || "Tanpa kategori"}</span>
              </li>
            ))}
          </ul>
        </div>
      )}
      <div className="flex flex-wrap justify-end gap-2">
        <Button variant="secondary" onClick={onDone}>
          {missing.length === 0 ? "Tutup" : "Batal"}
        </Button>
        {rows.length > 0 && (
          <Button variant="secondary" onClick={replaceAll}>
            Ganti seluruh daftar ({rows.length})
          </Button>
        )}
        {missing.length > 0 && <Button onClick={addMissing}>Tambahkan {missing.length} vendor</Button>}
      </div>
    </div>
  );
}
