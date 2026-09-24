import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { Navigate, useBlocker, useNavigate } from "react-router-dom";
import axios from "axios";
import { Check } from "lucide-react";
import { Button } from "@/shared/components/ui/Button";
import { Card, CardContent } from "@/shared/components/ui/Card";
import { ConfirmDialog } from "@/shared/components/ui/ConfirmDialog";
import { cn } from "@/shared/lib/cn";
import { getApiErrorMessage } from "@/shared/lib/api-error";
import { SectionPanel, type EditorMode } from "@/modules/rundowns/components/SectionPanel";
import { RUNDOWN_TAB_HINTS } from "@/modules/rundowns/schemas/rundown.schema";
import {
  fieldErrorsFromServer,
  validateSectionPayload,
  type SectionErrors,
} from "@/modules/rundowns/lib/section-errors";
import type { SectionPayload } from "@/modules/rundowns/stores/useRundownStore";
import type { RundownDetail } from "@/modules/rundowns/types";
import type { RundownTab } from "@/app/routes/route-paths";

/** Menyusun payload PUT untuk satu seksi dari draft editor. */
export function payloadFor(tab: RundownTab, d: RundownDetail): SectionPayload {
  switch (tab) {
    case "cover":
      return { cover: d.cover };
    case "vendors":
      return { vendors: d.vendors };
    case "roles":
      return { roles: d.roles };
    case "committees":
      return { committees: d.committees };
    case "data-lainnya":
      return { dataLainnya: d.dataLainnya, menuItems: d.menuItems };
    case "makeup":
      return { makeupRooms: d.makeupRooms };
    case "acara-akad":
      return { items: d.itemsAkad };
    case "acara-resepsi":
      return { items: d.itemsResepsi };
    case "layout":
      return { layoutNotes: d.layoutNotes };
    case "foto-tamu":
      return { photoGroups: d.photoGroups };
    case "tamu-vip":
      return { vipGuests: d.vipGuests };
    case "playlist":
      return { playlist: d.playlist, playlistNotes: d.playlistNotes };
  }
}

/** Seksi mana yang dianggap "sudah diisi" — dipakai indikator kelengkapan. */
export function filledSections(d: RundownDetail): Set<RundownTab> {
  const filled = new Set<RundownTab>();
  if (d.cover.brideName || d.cover.groomName) filled.add("cover");
  if (d.vendors.length) filled.add("vendors");
  if (d.roles.length) filled.add("roles");
  if (d.committees.length) filled.add("committees");
  if (d.menuItems.length || d.dataLainnya.siblingsBride || d.dataLainnya.siblingsGroom) filled.add("data-lainnya");
  if (d.makeupRooms.length) filled.add("makeup");
  if (d.itemsAkad.length) filled.add("acara-akad");
  if (d.itemsResepsi.length) filled.add("acara-resepsi");
  if (d.layoutNotes.length || d.hasLayoutImage) filled.add("layout");
  if (d.photoGroups.length) filled.add("foto-tamu");
  if (d.vipGuests.length) filled.add("tamu-vip");
  if (d.playlist.length || d.playlistNotes) filled.add("playlist");
  return filled;
}

export interface EditorApi {
  /**
   * Menyimpan seksi yang sedang terbuka bila ada perubahan. `true` bila
   * tidak ada yang perlu disimpan atau penyimpanan berhasil; `false` bila
   * isian tidak valid atau server menolak — pesannya sudah tampil di editor.
   * Wajib dipanggil sebelum aksi apa pun yang membaca data dari server
   * (generate, jadikan template).
   */
  saveIfDirty: () => Promise<boolean>;
  saving: boolean;
  dirty: boolean;
  draft: RundownDetail;
  patch: (fn: (d: RundownDetail) => RundownDetail) => void;
  filled: Set<RundownTab>;
}

interface RundownEditorProps {
  /** Data dari server. Draft editor disegarkan setiap kali ini berganti. */
  value: RundownDetail;
  tabs: readonly { key: RundownTab; label: string }[];
  tab: RundownTab;
  pathFor: (tab: RundownTab) => string;
  onSaveSection: (tab: RundownTab, payload: SectionPayload) => Promise<void>;
  mode?: EditorMode;
  header: (api: EditorApi) => ReactNode;
  /** Pesan dari halaman (mis. hasil generate), tampil di atas isi seksi. */
  banner?: ReactNode;
  toolbarFor?: (tab: RundownTab, api: EditorApi) => ReactNode;
  onPickLayout?: (file: File) => Promise<void>;
  layoutBusy?: boolean;
}

function timeLabel(d: Date): string {
  return d.toLocaleTimeString("id-ID", { hour: "2-digit", minute: "2-digit" });
}

/**
 * Editor seksi-per-tab untuk buku acara dan Template Rundown.
 *
 * Penyimpanan terjadi otomatis saat TRANSISI (PLAN rundown-ux-ideal D1): pindah
 * tab, keluar halaman lewat apa pun (tombol Kembali, Sidebar, tautan, back
 * browser), generate, dan unggah denah. Satu mekanisme menangani semuanya —
 * `useBlocker` — sehingga tidak ada jalur navigasi yang lolos dari penjaga.
 * Tidak ada simpan-saat-mengetik, jadi ketikan tidak pernah tertimpa respons
 * server yang datang belakangan.
 */
export function RundownEditor({
  value,
  tabs,
  tab,
  pathFor,
  onSaveSection,
  mode = "rundown",
  header,
  banner,
  toolbarFor,
  onPickLayout,
  layoutBusy = false,
}: RundownEditorProps) {
  const navigate = useNavigate();

  // Draft disegarkan SELAMA render setiap kali data server berganti, bukan
  // lewat useEffect: dengan useEffect ada satu render di mana `value` sudah
  // baru tetapi draft masih lama, sehingga editor sesaat tampak "kotor" tepat
  // setelah tersimpan. Server menomori ulang SUSUNAN ACARA, jadi hasilnya
  // harus menang atas apa pun yang sedang dipegang form.
  const [draft, setDraft] = useState(value);
  const [base, setBase] = useState(value);
  if (base !== value) {
    setBase(value);
    setDraft(value);
  }

  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [errors, setErrors] = useState<SectionErrors>({});
  const [savedAt, setSavedAt] = useState<Date | null>(null);
  const [leaveOpen, setLeaveOpen] = useState(false);

  const dirty = useMemo(() => JSON.stringify(draft) !== JSON.stringify(value), [draft, value]);
  const filled = useMemo(() => filledSections(value), [value]);

  // Ref dibaca oleh penjaga navigasi dan saveIfDirty, yang bisa berjalan di
  // antara dua render.
  const dirtyRef = useRef(dirty);
  dirtyRef.current = dirty;
  const draftRef = useRef(draft);
  draftRef.current = draft;
  const savingRef = useRef(false);

  const patch = useCallback((fn: (d: RundownDetail) => RundownDetail) => {
    setDraft((prev) => fn(prev));
  }, []);

  const saveIfDirty = useCallback(async (): Promise<boolean> => {
    if (!dirtyRef.current) return true;
    if (savingRef.current) return false;

    const payload = payloadFor(tab, draftRef.current);
    const clientErrors = validateSectionPayload(tab, payload);
    if (Object.keys(clientErrors).length > 0) {
      setErrors(clientErrors);
      setError("Ada isian yang belum sesuai. Perbaiki yang ditandai merah, lalu simpan lagi.");
      return false;
    }

    savingRef.current = true;
    setSaving(true);
    setError("");
    try {
      await onSaveSection(tab, payload);
      dirtyRef.current = false;
      setErrors({});
      setSavedAt(new Date());
      return true;
    } catch (e) {
      if (axios.isAxiosError(e)) {
        setErrors(fieldErrorsFromServer((e.response?.data as { errors?: unknown } | undefined)?.errors));
      }
      setError(getApiErrorMessage(e, "Gagal menyimpan seksi"));
      return false;
    } finally {
      savingRef.current = false;
      setSaving(false);
    }
  }, [onSaveSection, tab]);

  // Satu penjaga untuk SEMUA navigasi keluar dari seksi ini, termasuk pindah
  // tab (tab adalah segmen URL). Diblok hanya bila ada perubahan.
  const blocker = useBlocker(
    ({ currentLocation, nextLocation }) =>
      dirtyRef.current && currentLocation.pathname !== nextLocation.pathname
  );
  const blockerRef = useRef(blocker);
  blockerRef.current = blocker;

  useEffect(() => {
    if (blocker.state !== "blocked") return;
    let active = true;
    void saveIfDirty().then((ok) => {
      if (!active) return;
      if (ok) blockerRef.current.proceed?.();
      else setLeaveOpen(true);
    });
    return () => {
      active = false;
    };
    // Hanya bereaksi pada perubahan status blok; saveIfDirty dibaca dari
    // render yang sama dengan navigasi yang diblok (tab asalnya).
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [blocker.state]);

  // Menutup/memuat ulang tab browser tidak bisa disimpan secara asinkron;
  // cukup minta konfirmasi bawaan browser.
  useEffect(() => {
    if (!dirty) return;
    const onBeforeUnload = (e: BeforeUnloadEvent) => {
      e.preventDefault();
      e.returnValue = "";
    };
    window.addEventListener("beforeunload", onBeforeUnload);
    return () => window.removeEventListener("beforeunload", onBeforeUnload);
  }, [dirty]);

  // Galat milik seksi sebelumnya tidak relevan di seksi berikutnya.
  useEffect(() => {
    setErrors({});
    setError("");
  }, [tab]);

  const pickLayout = useCallback(
    async (file: File) => {
      // Unggah denah menyegarkan data dari server; simpan catatan layout yang
      // sedang disunting lebih dulu supaya tidak terbuang.
      if (!onPickLayout || !(await saveIfDirty())) return;
      await onPickLayout(file);
    },
    [onPickLayout, saveIfDirty]
  );

  if (!tabs.some((t) => t.key === tab)) {
    return <Navigate to={pathFor(tabs[0].key)} replace />;
  }

  const api: EditorApi = { saveIfDirty, saving, dirty, draft, patch, filled };

  return (
    <div className="flex flex-col gap-5">
      {header(api)}

      <div className="flex flex-col gap-2">
        <nav aria-label="Seksi" className="-mx-1 flex gap-1.5 overflow-x-auto px-1 pb-1 md:flex-wrap md:overflow-visible">
          {tabs.map((t) => (
            <button
              key={t.key}
              type="button"
              disabled={saving}
              aria-current={t.key === tab ? "page" : undefined}
              onClick={() => t.key !== tab && navigate(pathFor(t.key))}
              className={cn(
                "shrink-0 rounded-md px-3 py-1.5 text-[13px] font-medium transition-colors",
                "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-navy-900/30",
                "disabled:cursor-wait",
                t.key === tab ? "bg-navy-900 text-white" : "bg-surface-muted text-text-secondary hover:bg-border"
              )}
            >
              {t.label}
              {filled.has(t.key) && (
                <span className={cn("ml-1.5", t.key === tab ? "text-white/80" : "text-success")} aria-label="terisi">
                  •
                </span>
              )}
            </button>
          ))}
        </nav>
        <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1">
          <p className="text-[13px] text-text-secondary">{RUNDOWN_TAB_HINTS[tab]}</p>
          <SaveStatus saving={saving} dirty={dirty} savedAt={savedAt} />
        </div>
      </div>

      {banner}
      {error && (
        <div role="alert" className="rounded-md border border-danger/30 bg-danger/5 px-3 py-2 text-[13px] text-danger">
          {error}
        </div>
      )}

      <Card>
        <CardContent className="flex flex-col gap-4">
          <SectionPanel
            tab={tab}
            draft={draft}
            patch={patch}
            mode={mode}
            errors={errors}
            toolbar={toolbarFor?.(tab, api)}
            onPickLayout={onPickLayout ? pickLayout : undefined}
            busy={layoutBusy || saving}
          />
          <div className="flex flex-wrap items-center justify-end gap-3 border-t border-border pt-4">
            {dirty && (
              <span className="text-[13px] text-text-secondary">
                Perubahan tersimpan otomatis saat Anda pindah seksi.
              </span>
            )}
            <Button disabled={!dirty || saving} onClick={() => void saveIfDirty()}>
              {saving ? "Menyimpan..." : dirty ? "Simpan" : "Tersimpan"}
            </Button>
          </div>
        </CardContent>
      </Card>

      <ConfirmDialog
        open={leaveOpen}
        onClose={() => {
          setLeaveOpen(false);
          blockerRef.current.reset?.();
        }}
        onConfirm={() => {
          setLeaveOpen(false);
          dirtyRef.current = false;
          setDraft(value);
          blockerRef.current.proceed?.();
        }}
        title="Perubahan belum tersimpan"
        message="Seksi ini gagal disimpan. Tinggalkan dan buang perubahannya?"
        details="Pilih Tetap di sini untuk memperbaiki isian lalu menyimpannya."
        confirmLabel="Buang perubahan"
        cancelLabel="Tetap di sini"
        tone="danger"
      />
    </div>
  );
}

function SaveStatus({ saving, dirty, savedAt }: { saving: boolean; dirty: boolean; savedAt: Date | null }) {
  if (saving) return <span className="text-[12px] text-text-secondary">Menyimpan...</span>;
  if (dirty) return <span className="text-[12px] text-text-secondary">Belum tersimpan</span>;
  if (!savedAt) return null;
  return (
    <span className="inline-flex items-center gap-1 text-[12px] text-success">
      <Check className="h-3.5 w-3.5" aria-hidden="true" />
      Tersimpan {timeLabel(savedAt)}
    </span>
  );
}
