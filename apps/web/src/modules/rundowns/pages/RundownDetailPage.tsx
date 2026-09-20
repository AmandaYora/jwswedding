import { useCallback, useEffect, useMemo, useState } from "react";
import { Navigate, useNavigate, useParams } from "react-router-dom";
import { ArrowLeft, FileText, FileType, Upload } from "lucide-react";
import { Button } from "@/shared/components/ui/Button";
import { Card, CardContent } from "@/shared/components/ui/Card";
import { Field, Input, Textarea } from "@/shared/components/ui/Input";
import { cn } from "@/shared/lib/cn";
import { useRundownStore, type SectionPayload } from "@/modules/rundowns/stores/useRundownStore";
import { RowEditor, type ColumnDef } from "@/modules/rundowns/components/RowEditor";
import { RUNDOWN_TABS } from "@/modules/rundowns/schemas/rundown.schema";
import type {
  RundownAcaraItem,
  RundownCommittee,
  RundownDetail,
  RundownLayoutNote,
  RundownMakeupRoom,
  RundownMenuItem,
  RundownPhotoGroup,
  RundownPlaylistEntry,
  RundownRole,
  RundownVIPGuest,
  RundownVendor,
} from "@/modules/rundowns/types";
import { NO_NUMBER_MARKER } from "@/modules/rundowns/types";
import { ROUTE_PATHS, type RundownTab } from "@/app/routes/route-paths";
import { getApiErrorMessage, getApiErrorMessageFromBlob } from "@/shared/lib/api-error";
import { blobToBase64 } from "@/shared/lib/image-compression";
import { ConfirmDialog } from "@/shared/components/ui/ConfirmDialog";

/** Sama dengan maxLayoutImageSize di rundown_service.go. */
const MAX_LAYOUT_BYTES = 5 * 1024 * 1024;

/** Seksi mana yang dianggap "sudah diisi" — dipakai indikator kelengkapan. */
function filledSections(d: RundownDetail): Set<string> {
  const filled = new Set<string>();
  if (d.cover.brideName || d.cover.groomName) filled.add("cover");
  if (d.vendors.length) filled.add("vendors");
  if (d.roles.length) filled.add("roles");
  if (d.committees.length) filled.add("committees");
  if (d.menuItems.length || d.dataLainnya.siblingsBride || d.dataLainnya.siblingsGroom)
    filled.add("data-lainnya");
  if (d.makeupRooms.length) filled.add("makeup");
  if (d.itemsAkad.length) filled.add("acara-akad");
  if (d.itemsResepsi.length) filled.add("acara-resepsi");
  if (d.layoutNotes.length || d.hasLayoutImage) filled.add("layout");
  if (d.photoGroups.length) filled.add("foto-tamu");
  if (d.vipGuests.length) filled.add("tamu-vip");
  if (d.playlist.length || d.playlistNotes) filled.add("playlist");
  return filled;
}

export default function RundownDetailPage() {
  const { id = "", tab = "cover" } = useParams<{ id: string; tab: RundownTab }>();
  const navigate = useNavigate();
  const detail = useRundownStore((s) => s.detail);
  const fetchDetail = useRundownStore((s) => s.fetchDetail);
  const saveSection = useRundownStore((s) => s.saveSection);
  const uploadLayout = useRundownStore((s) => s.uploadLayout);
  const generate = useRundownStore((s) => s.generate);

  const [draft, setDraft] = useState<RundownDetail | null>(null);
  const [saving, setSaving] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [pendingTab, setPendingTab] = useState<RundownTab | null>(null);

  useEffect(() => {
    if (id) void fetchDetail(id);
  }, [id, fetchDetail]);

  // Draft lokal disegarkan tiap kali data server berubah, termasuk setelah
  // menyimpan — server menomori ulang SUSUNAN ACARA, jadi hasilnya harus
  // menang atas apa pun yang sedang dipegang form.
  useEffect(() => {
    setDraft(detail);
  }, [detail]);

  const filled = useMemo(() => (detail ? filledSections(detail) : new Set<string>()), [detail]);

  const patch = useCallback((fn: (d: RundownDetail) => RundownDetail) => {
    setDraft((prev) => (prev ? fn(prev) : prev));
  }, []);

  // Draft itu SATU untuk dua belas tab, dan `saveSection` mengganti `detail`
  // yang kemudian me-reset draft. Tanpa penjaga ini urutan "sunting tab A ->
  // pindah tab B -> simpan B" membuang suntingan tab A tanpa sepatah kata pun.
  const dirty = useMemo(
    () => draft !== null && detail !== null && JSON.stringify(draft) !== JSON.stringify(detail),
    [draft, detail]
  );

  const goToTab = useCallback(
    (next: RundownTab) => {
      if (next === tab) return;
      if (dirty) {
        setPendingTab(next);
        return;
      }
      navigate(ROUTE_PATHS.rundownDetail(id, next));
    },
    [dirty, id, navigate, tab]
  );

  const save = async (payload: SectionPayload) => {
    setSaving(true);
    setError("");
    setNotice("");
    try {
      await saveSection(id, tab, payload);
      setNotice("Tersimpan.");
    } catch (e) {
      setError(getApiErrorMessage(e, "Gagal menyimpan seksi"));
    } finally {
      setSaving(false);
    }
  };

  const onGenerate = async (format: "docx" | "pdf") => {
    setBusy(true);
    setError("");
    try {
      await generate(id, format);
    } catch (e) {
      setError(await getApiErrorMessageFromBlob(e, "Gagal membuat berkas rundown"));
    } finally {
      setBusy(false);
    }
  };

  const onPickLayout = async (file: File) => {
    // Diperiksa di sini juga, bukan hanya di server: `accept="image/png"` cuma
    // menyaring dialog pemilih berkas, dan tanpa ini berkas 5 MB tetap
    // di-encode lalu dikirim utuh sebelum ditolak. Batasnya sengaja sama
    // persis dengan milik server (maxLayoutImageSize).
    if (file.type !== "image/png") {
      setError("Denah akad harus berupa berkas PNG.");
      return;
    }
    if (file.size > MAX_LAYOUT_BYTES) {
      setError("Ukuran denah maksimal 5 MB.");
      return;
    }
    setBusy(true);
    setError("");
    setNotice("");
    try {
      // blobToBase64 (FileReader), bukan loop String.fromCharCode per byte:
      // pada PNG 5 MB loop itu berarti jutaan konkatenasi string dan UI yang
      // membeku beberapa detik. Sengaja TIDAK lewat compressFileForUpload —
      // byte denah ditukar apa adanya ke dalam .docx, jadi tidak boleh
      // di-re-encode canvas.
      await uploadLayout(id, await blobToBase64(file));
      setNotice("Denah akad tersimpan.");
    } catch (e) {
      setError(getApiErrorMessage(e, "Gagal mengunggah denah"));
    } finally {
      setBusy(false);
    }
  };

  // Segmen tab datang dari URL, jadi bisa berisi apa saja. Tanpa pembelokan
  // ini `SectionPanel` jatuh ke undefined: panel kosong dengan tombol Simpan
  // yang mengirim payload undefined dan dijawab 422 oleh server.
  if (!RUNDOWN_TABS.some((t) => t.key === tab)) {
    return <Navigate to={ROUTE_PATHS.rundownDetail(id)} replace />;
  }

  if (!draft) {
    return <p className="text-[13px] text-text-secondary">Memuat rundown...</p>;
  }

  return (
    <div className="flex flex-col gap-5">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="flex items-start gap-3">
          <Button variant="ghost" onClick={() => navigate(ROUTE_PATHS.rundowns)} aria-label="Kembali">
            <ArrowLeft className="h-4 w-4" />
          </Button>
          <div>
            <h1 className="text-xl font-semibold text-text-primary">{draft.projectName}</h1>
            <p className="text-[13px] text-text-secondary">
              {filled.size} dari {RUNDOWN_TABS.length} seksi terisi. Seksi yang belum diisi tetap
              tercetak sebagai kerangka kosong.
            </p>
          </div>
        </div>
        <div className="flex gap-2">
          <Button
            variant="secondary"
            icon={<FileType className="h-4 w-4" />}
            disabled={busy}
            onClick={() => void onGenerate("docx")}
          >
            DOCX
          </Button>
          <Button
            icon={<FileText className="h-4 w-4" />}
            disabled={busy}
            onClick={() => void onGenerate("pdf")}
          >
            PDF
          </Button>
        </div>
      </div>

      <div className="flex flex-wrap gap-1.5">
        {RUNDOWN_TABS.map((t) => (
          <button
            key={t.key}
            type="button"
            onClick={() => goToTab(t.key)}
            className={cn(
              "rounded-md px-3 py-1.5 text-[13px] font-medium transition-colors",
              t.key === tab
                ? "bg-navy-900 text-white"
                : "bg-surface-muted text-text-secondary hover:bg-border"
            )}
          >
            {t.label}
            {filled.has(t.key) && <span className="ml-1.5 text-success">•</span>}
          </button>
        ))}
      </div>

      {error && (
        <div className="rounded-md border border-danger/30 bg-danger/5 px-3 py-2 text-[13px] text-danger">
          {error}
        </div>
      )}
      {notice && (
        <div className="rounded-md border border-success/30 bg-success/5 px-3 py-2 text-[13px] text-success">
          {notice}
        </div>
      )}

      <Card>
        <CardContent className="flex flex-col gap-4">
          <SectionPanel
            tab={tab as RundownTab}
            draft={draft}
            patch={patch}
            onPickLayout={onPickLayout}
            busy={busy}
          />
          <div className="flex items-center justify-end gap-3 border-t border-border pt-4">
            {dirty && (
              <span className="text-[13px] text-text-secondary">Ada perubahan yang belum disimpan.</span>
            )}
            <Button disabled={saving} onClick={() => void save(payloadFor(tab as RundownTab, draft))}>
              {saving ? "Menyimpan..." : "Simpan seksi ini"}
            </Button>
          </div>
        </CardContent>
      </Card>

      <ConfirmDialog
        open={pendingTab !== null}
        onClose={() => setPendingTab(null)}
        onConfirm={async () => {
          const next = pendingTab;
          setPendingTab(null);
          setDraft(detail);
          if (next) navigate(ROUTE_PATHS.rundownDetail(id, next));
        }}
        title="Perubahan belum disimpan"
        message="Suntingan pada seksi ini belum disimpan dan akan hilang kalau Anda pindah tab sekarang."
        confirmLabel="Pindah tanpa menyimpan"
        tone="danger"
      />
    </div>
  );
}

/** Menyusun payload PUT untuk seksi yang sedang dibuka. */
function payloadFor(tab: RundownTab, d: RundownDetail): SectionPayload {
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

const ACARA_COLUMNS: ColumnDef<RundownAcaraItem>[] = [
  { key: "noLabel", label: "No", width: "70px", placeholder: "-" },
  { key: "timeLabel", label: "Waktu", width: "150px", placeholder: "15.00 - 15.20" },
  { key: "item", label: "Item", kind: "multiline" },
  { key: "pic", label: "PIC", kind: "multiline", width: "180px" },
  { key: "note", label: "Keterangan", kind: "multiline", width: "180px" },
];

function SectionPanel({
  tab,
  draft,
  patch,
  onPickLayout,
  busy,
}: {
  tab: RundownTab;
  draft: RundownDetail;
  patch: (fn: (d: RundownDetail) => RundownDetail) => void;
  onPickLayout: (file: File) => Promise<void>;
  busy: boolean;
}) {
  switch (tab) {
    case "cover":
      return (
        <div className="grid gap-4 md:grid-cols-2">
          {(
            [
              ["groomName", "Nama Pengantin Pria"],
              ["groomBirthOrder", "Urutan anak (Pria)"],
              ["groomParents", "Orang tua (Pria)"],
              ["brideName", "Nama Pengantin Wanita"],
              ["brideBirthOrder", "Urutan anak (Wanita)"],
              ["brideParents", "Orang tua (Wanita)"],
              ["eventDateLabel", "Tanggal acara"],
              ["venueLabel", "Venue"],
              ["eventTimeLabel", "Jam acara"],
              ["coupleTitle", "Judul lampiran"],
              ["woPicName", "Nama PIC WO"],
              ["woPicPhone", "Nomor PIC WO"],
            ] as const
          ).map(([key, label]) => (
            <Field key={key} label={label}>
              <Input
                value={draft.cover[key]}
                onChange={(e) =>
                  patch((d) => ({ ...d, cover: { ...d.cover, [key]: e.target.value } }))
                }
              />
            </Field>
          ))}
        </div>
      );

    case "vendors":
      return (
        <RowEditor<RundownVendor>
          columns={[
            { key: "categoryLabel", label: "Kategori", placeholder: "VENUE & CATERING" },
            { key: "vendorName", label: "Nama vendor" },
          ]}
          rows={draft.vendors}
          onChange={(rows) => patch((d) => ({ ...d, vendors: rows }))}
          blank={() => ({ categoryLabel: "", vendorName: "" })}
          addLabel="Tambah vendor"
          emptyDescription="Baris dengan kategori yang sama dan berurutan hanya mencetak satu judul kategori."
        />
      );

    case "roles":
      return (
        <RowEditor<RundownRole>
          columns={[
            { key: "roleLabel", label: "Keterangan", placeholder: "Wali Nikah CPW" },
            { key: "personName", label: "Nama PIC" },
            { key: "note", label: "Ket" },
          ]}
          rows={draft.roles}
          onChange={(rows) => patch((d) => ({ ...d, roles: rows }))}
          blank={() => ({ roleLabel: "", personName: "", note: "" })}
          addLabel="Tambah peran"
        />
      );

    case "committees":
      return (
        <RowEditor<RundownCommittee>
          columns={[
            { key: "roleLabel", label: "Peran", width: "200px" },
            { key: "personText", label: "Nama / Kontak", kind: "multiline" },
            { key: "jobDesc", label: "Keterangan", kind: "multiline" },
          ]}
          rows={draft.committees}
          onChange={(rows) => patch((d) => ({ ...d, committees: rows }))}
          blank={() => ({ roleLabel: "", personText: "", jobDesc: "" })}
          addLabel="Tambah panitia"
          emptyDescription="Satu nama per baris pada kolom Nama / Kontak akan tercetak sebagai baris terpisah."
        />
      );

    case "data-lainnya":
      return (
        <div className="flex flex-col gap-5">
          <div className="grid gap-4 md:grid-cols-2">
            <Field label="Adik/Kakak Pengantin Wanita" hint="Satu nama per baris.">
              <Textarea
                rows={3}
                value={draft.dataLainnya.siblingsBride}
                onChange={(e) =>
                  patch((d) => ({
                    ...d,
                    dataLainnya: { ...d.dataLainnya, siblingsBride: e.target.value },
                  }))
                }
              />
            </Field>
            <Field label="Adik/Kakak Pengantin Pria" hint="Satu nama per baris.">
              <Textarea
                rows={3}
                value={draft.dataLainnya.siblingsGroom}
                onChange={(e) =>
                  patch((d) => ({
                    ...d,
                    dataLainnya: { ...d.dataLainnya, siblingsGroom: e.target.value },
                  }))
                }
              />
            </Field>
            <Field label="Souvenir">
              <Input
                value={draft.dataLainnya.souvenirNote}
                onChange={(e) =>
                  patch((d) => ({
                    ...d,
                    dataLainnya: { ...d.dataLainnya, souvenirNote: e.target.value },
                  }))
                }
              />
            </Field>
            {/* Textarea, bukan Input: di berkas asli catatan Table Cloth
                memang beberapa baris ("meja VIP 4", "VIP = 100 porsi",
                "Reguler = 500 porsi"), dan renderer mencetak tiap baris
                sebagai <w:br/> di dalam selnya. */}
            <Field label="Table cloth" hint="Boleh beberapa baris.">
              <Textarea
                rows={3}
                value={draft.dataLainnya.tableClothNote}
                onChange={(e) =>
                  patch((d) => ({
                    ...d,
                    dataLainnya: { ...d.dataLainnya, tableClothNote: e.target.value },
                  }))
                }
              />
            </Field>
          </div>
          <RowEditor<RundownMenuItem>
            columns={[
              {
                key: "groupKey",
                label: "Kolom",
                kind: "select",
                width: "150px",
                options: [
                  { value: "Stall", label: "Stall" },
                  { value: "Buffet", label: "Catering buffet" },
                  { value: "AfterAkad", label: "Makanan after akad" },
                ],
              },
              {
                key: "style",
                label: "Gaya baris",
                kind: "select",
                width: "150px",
                options: [
                  { value: "Heading", label: "Judul" },
                  { value: "Numbered", label: "Bernomor" },
                  { value: "Bullet", label: "Berpoin" },
                  { value: "Plain", label: "Polos" },
                ],
              },
              { key: "content", label: "Isi" },
            ]}
            rows={draft.menuItems}
            onChange={(rows) => patch((d) => ({ ...d, menuItems: rows }))}
            blank={() => ({ groupKey: "Stall", style: "Numbered", content: "" })}
            addLabel="Tambah baris menu"
          />
        </div>
      );

    case "makeup":
      return <MakeupPanel draft={draft} patch={patch} />;

    case "acara-akad":
    case "acara-resepsi": {
      const key = tab === "acara-akad" ? "itemsAkad" : "itemsResepsi";
      return (
        <RowEditor<RundownAcaraItem>
          columns={ACARA_COLUMNS}
          rows={draft[key]}
          onChange={(rows) => patch((d) => ({ ...d, [key]: rows }))}
          blank={() => ({ noLabel: "", timeLabel: "", item: "", pic: "", note: "" })}
          addLabel="Tambah acara"
          emptyDescription={`Nomor diisi server saat menyimpan. Isi "${NO_NUMBER_MARKER}" pada kolom No untuk baris yang tampil tanpa nomor.`}
        />
      );
    }

    case "layout":
      return (
        <div className="flex flex-col gap-5">
          <div className="flex flex-wrap items-center gap-3 rounded-md border border-border bg-surface-muted px-3 py-3">
            <div className="flex-1 text-[13px]">
              <p className="font-medium text-text-primary">Denah akad</p>
              <p className="text-text-secondary">
                {draft.hasLayoutImage
                  ? "Denah sudah diunggah dan akan tercetak di dokumen."
                  : "Belum ada denah. Dokumen akan memakai gambar placeholder."}{" "}
                Format PNG, maksimal 5 MB.
              </p>
            </div>
            <label className="cursor-pointer">
              <input
                type="file"
                accept="image/png"
                className="hidden"
                disabled={busy}
                onChange={(e) => {
                  const f = e.target.files?.[0];
                  if (f) void onPickLayout(f);
                  e.target.value = "";
                }}
              />
              <span className="inline-flex items-center gap-2 rounded-md border border-border bg-white px-3 py-2 text-[13px] font-medium hover:bg-surface-muted">
                <Upload className="h-4 w-4" />
                {draft.hasLayoutImage ? "Ganti denah" : "Unggah denah"}
              </span>
            </label>
          </div>
          <RowEditor<RundownLayoutNote>
            columns={[
              {
                key: "kind",
                label: "Jenis",
                kind: "select",
                width: "170px",
                options: [
                  { value: "Rule", label: "Aturan tamu" },
                  { value: "Legend", label: "Keterangan denah" },
                ],
              },
              { key: "content", label: "Isi", kind: "multiline" },
            ]}
            rows={draft.layoutNotes}
            onChange={(rows) => patch((d) => ({ ...d, layoutNotes: rows }))}
            blank={() => ({ kind: "Legend", numberLabel: "", content: "" })}
            addLabel="Tambah keterangan"
            emptyDescription="Nomor untuk kedua jenis diisi server saat menyimpan."
          />
        </div>
      );

    case "foto-tamu":
      return (
        <RowEditor<RundownPhotoGroup>
          columns={[{ key: "groupName", label: "Nama keluarga / grup / instansi" }]}
          rows={draft.photoGroups}
          onChange={(rows) => patch((d) => ({ ...d, photoGroups: rows }))}
          blank={() => ({ groupName: "" })}
          addLabel="Tambah grup foto"
          emptyDescription="Tabel selalu dicetak 21 baris; sisanya kosong untuk diisi tangan di lapangan."
        />
      );

    case "tamu-vip":
      return (
        <RowEditor<RundownVIPGuest>
          columns={[
            { key: "fullName", label: "Nama lengkap & gelar" },
            { key: "position", label: "Jabatan" },
          ]}
          rows={draft.vipGuests}
          onChange={(rows) => patch((d) => ({ ...d, vipGuests: rows }))}
          blank={() => ({ fullName: "", position: "" })}
          addLabel="Tambah tamu VIP"
          emptyDescription="Tabel selalu dicetak 15 baris; sisanya kosong untuk diisi tangan di lapangan."
        />
      );

    case "playlist":
      return (
        <div className="flex flex-col gap-5">
          <RowEditor<RundownPlaylistEntry>
            columns={[
              { key: "title", label: "Judul lagu" },
              { key: "artist", label: "Penyanyi" },
            ]}
            rows={draft.playlist}
            onChange={(rows) => patch((d) => ({ ...d, playlist: rows }))}
            blank={() => ({ title: "", artist: "" })}
            addLabel="Tambah lagu"
            emptyDescription="Tabel selalu dicetak 10 baris; sisanya kosong."
          />
          <Field label="Catatan" hint="Satu catatan per baris.">
            <Textarea
              rows={3}
              value={draft.playlistNotes}
              onChange={(e) => patch((d) => ({ ...d, playlistNotes: e.target.value }))}
            />
          </Field>
        </div>
      );
  }
}

/**
 * Ruangan makeup bertingkat dua — ruangan, lalu baris isi di dalamnya —
 * sehingga tidak muat di RowEditor yang datar.
 */
function MakeupPanel({
  draft,
  patch,
}: {
  draft: RundownDetail;
  patch: (fn: (d: RundownDetail) => RundownDetail) => void;
}) {
  const setRooms = (rooms: RundownMakeupRoom[]) => patch((d) => ({ ...d, makeupRooms: rooms }));

  return (
    <div className="flex flex-col gap-4">
      {draft.makeupRooms.map((room, ri) => (
        <div key={ri} className="rounded-md border border-border p-3">
          <div className="mb-3 flex items-end gap-2">
            <div className="flex-1">
              <Field label={`Ruangan ${ri + 1}`}>
                <Input
                  value={room.roomLabel}
                  onChange={(e) =>
                    setRooms(
                      draft.makeupRooms.map((r, i) =>
                        i === ri ? { ...r, roomLabel: e.target.value } : r
                      )
                    )
                  }
                />
              </Field>
            </div>
            <Button
              variant="ghost"
              onClick={() => setRooms(draft.makeupRooms.filter((_, i) => i !== ri))}
            >
              Hapus ruangan
            </Button>
          </div>
          <RowEditor
            columns={[
              {
                key: "style",
                label: "Gaya baris",
                kind: "select",
                width: "170px",
                options: [
                  { value: "Heading", label: "Judul" },
                  { value: "Numbered", label: "Bernomor" },
                  { value: "Dash", label: "Tanda hubung" },
                  { value: "Note", label: 'Label "Note:"' },
                ],
              },
              { key: "content", label: "Isi" },
            ]}
            rows={room.lines}
            onChange={(lines) =>
              setRooms(draft.makeupRooms.map((r, i) => (i === ri ? { ...r, lines } : r)))
            }
            blank={() => ({ style: "Numbered" as const, content: "" })}
            addLabel="Tambah baris"
            emptyDescription="Penomoran tiap ruangan dimulai ulang dari 1."
          />
        </div>
      ))}
      <div>
        <Button
          variant="secondary"
          onClick={() => setRooms([...draft.makeupRooms, { roomLabel: "", lines: [] }])}
        >
          Tambah ruangan
        </Button>
      </div>
    </div>
  );
}
