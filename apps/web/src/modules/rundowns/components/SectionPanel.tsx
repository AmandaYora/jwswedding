import type { ReactNode } from "react";
import { Upload } from "lucide-react";
import { Button } from "@/shared/components/ui/Button";
import { Field, Input, Textarea } from "@/shared/components/ui/Input";
import { RowEditor, type ColumnDef } from "@/modules/rundowns/components/RowEditor";
import { previewNumbers } from "@/modules/rundowns/lib/numbering";
import type { SectionErrors } from "@/modules/rundowns/lib/section-errors";
import type { RundownTab } from "@/app/routes/route-paths";
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

export type EditorMode = "rundown" | "template";

interface SectionPanelProps {
  tab: RundownTab;
  draft: RundownDetail;
  patch: (fn: (d: RundownDetail) => RundownDetail) => void;
  /** "template": editor Template Rundown — tanpa unggah denah. */
  mode?: EditorMode;
  errors?: SectionErrors;
  /** Aksi tambahan di atas isi seksi, mis. "Tarik ulang dari project". */
  toolbar?: ReactNode;
  onPickLayout?: (file: File) => Promise<void>;
  busy?: boolean;
}

/** Format denah yang diterima server (SaveLayoutImage). */
export const LAYOUT_ACCEPT = "image/png,image/jpeg";

/**
 * Isi satu seksi buku acara. Dipakai editor rundown dan editor Template
 * Rundown — seksi yang sama, aturan yang sama.
 */
export function SectionPanel({
  tab,
  draft,
  patch,
  mode = "rundown",
  errors,
  toolbar,
  onPickLayout,
  busy = false,
}: SectionPanelProps) {
  return (
    <div className="flex flex-col gap-4">
      {toolbar}
      <SectionBody
        tab={tab}
        draft={draft}
        patch={patch}
        mode={mode}
        errors={errors}
        onPickLayout={onPickLayout}
        busy={busy}
      />
    </div>
  );
}

// Label "Bentuk baris" untuk dua seksi berbasis paragraf. Contoh di tiap
// label menunjukkan hasil cetaknya, bukan nama gaya di Word.
const MENU_STYLE_OPTIONS = [
  { value: "Heading", label: "Judul" },
  { value: "Numbered", label: "Bernomor (1, 2, 3)" },
  { value: "Bullet", label: "Berpoin (•)" },
  { value: "Plain", label: "Teks biasa" },
];

const MAKEUP_STYLE_OPTIONS = [
  { value: "Heading", label: "Judul" },
  { value: "Numbered", label: "Bernomor (1, 2, 3)" },
  { value: "Dash", label: "Tanda hubung (–)" },
  { value: "Note", label: 'Catatan ("Note:")' },
];

function acaraColumns(rows: RundownAcaraItem[]): ColumnDef<RundownAcaraItem>[] {
  const numbers = previewNumbers(rows);
  return [
    { key: "noLabel", label: "No", kind: "display", width: "3rem", display: (_, i) => numbers[i] },
    {
      key: "noLabel",
      label: "Tanpa nomor",
      kind: "checkbox",
      width: "3.5rem",
      checkbox: { on: NO_NUMBER_MARKER, off: "" },
    },
    { key: "timeLabel", label: "Waktu", width: "9rem", placeholder: "15.00 - 15.20" },
    { key: "item", label: "Acara", kind: "multiline" },
    { key: "pic", label: "PIC", kind: "multiline", width: "11rem" },
    { key: "note", label: "Keterangan", kind: "multiline", width: "11rem" },
  ];
}

function SectionBody({
  tab,
  draft,
  patch,
  mode,
  errors,
  onPickLayout,
  busy,
}: Omit<SectionPanelProps, "toolbar"> & { mode: EditorMode }) {
  const coverError = (key: string) => errors?.[`cover.${key}`];
  const dataLainnyaError = (key: string) => errors?.[`dataLainnya.${key}`];

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
            <Field key={key} label={label} hint={coverError(key)}>
              <Input
                value={draft.cover[key]}
                aria-invalid={coverError(key) ? true : undefined}
                className={coverError(key) ? "border-danger" : undefined}
                onChange={(e) => patch((d) => ({ ...d, cover: { ...d.cover, [key]: e.target.value } }))}
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
          errors={errors}
          errorPrefix="vendors"
        />
      );

    case "roles":
      return (
        <RowEditor<RundownRole>
          columns={[
            { key: "roleLabel", label: "Peran", placeholder: "Wali Nikah CPW" },
            { key: "personName", label: "Nama", placeholder: mode === "template" ? "Diisi per acara" : undefined },
            { key: "note", label: "Keterangan" },
          ]}
          rows={draft.roles}
          onChange={(rows) => patch((d) => ({ ...d, roles: rows }))}
          blank={() => ({ roleLabel: "", personName: "", note: "" })}
          addLabel="Tambah peran"
          errors={errors}
          errorPrefix="roles"
        />
      );

    case "committees":
      return (
        <RowEditor<RundownCommittee>
          columns={[
            { key: "roleLabel", label: "Peran", width: "12rem" },
            { key: "personText", label: "Nama / Kontak", kind: "multiline" },
            { key: "jobDesc", label: "Tugas", kind: "multiline" },
          ]}
          rows={draft.committees}
          onChange={(rows) => patch((d) => ({ ...d, committees: rows }))}
          blank={() => ({ roleLabel: "", personText: "", jobDesc: "" })}
          addLabel="Tambah panitia"
          emptyDescription="Satu nama per baris pada kolom Nama / Kontak akan tercetak sebagai baris terpisah."
          errors={errors}
          errorPrefix="committees"
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
                  patch((d) => ({ ...d, dataLainnya: { ...d.dataLainnya, siblingsBride: e.target.value } }))
                }
              />
            </Field>
            <Field label="Adik/Kakak Pengantin Pria" hint="Satu nama per baris.">
              <Textarea
                rows={3}
                value={draft.dataLainnya.siblingsGroom}
                onChange={(e) =>
                  patch((d) => ({ ...d, dataLainnya: { ...d.dataLainnya, siblingsGroom: e.target.value } }))
                }
              />
            </Field>
            <Field label="Souvenir" hint={dataLainnyaError("souvenirNote")}>
              <Input
                value={draft.dataLainnya.souvenirNote}
                aria-invalid={dataLainnyaError("souvenirNote") ? true : undefined}
                className={dataLainnyaError("souvenirNote") ? "border-danger" : undefined}
                onChange={(e) =>
                  patch((d) => ({ ...d, dataLainnya: { ...d.dataLainnya, souvenirNote: e.target.value } }))
                }
              />
            </Field>
            {/* Textarea, bukan Input: di berkas asli catatan Table Cloth
                memang beberapa baris ("meja VIP 4", "VIP = 100 porsi",
                "Reguler = 500 porsi"), dan renderer mencetak tiap baris
                sebagai <w:br/> di dalam selnya. */}
            <Field label="Table cloth" hint={dataLainnyaError("tableClothNote") ?? "Boleh beberapa baris."}>
              <Textarea
                rows={3}
                value={draft.dataLainnya.tableClothNote}
                aria-invalid={dataLainnyaError("tableClothNote") ? true : undefined}
                className={dataLainnyaError("tableClothNote") ? "border-danger" : undefined}
                onChange={(e) =>
                  patch((d) => ({ ...d, dataLainnya: { ...d.dataLainnya, tableClothNote: e.target.value } }))
                }
              />
            </Field>
          </div>
          <RowEditor<RundownMenuItem>
            columns={[
              {
                key: "groupKey",
                label: "Kolom menu",
                kind: "select",
                width: "11rem",
                options: [
                  { value: "Stall", label: "Stall" },
                  { value: "Buffet", label: "Catering buffet" },
                  { value: "AfterAkad", label: "Makanan after akad" },
                ],
              },
              { key: "style", label: "Bentuk baris", kind: "select", width: "11rem", options: MENU_STYLE_OPTIONS },
              { key: "content", label: "Isi" },
            ]}
            rows={draft.menuItems}
            onChange={(rows) => patch((d) => ({ ...d, menuItems: rows }))}
            blank={() => ({ groupKey: "Stall", style: "Numbered", content: "" })}
            addLabel="Tambah baris menu"
            errors={errors}
            errorPrefix="menuItems"
          />
        </div>
      );

    case "makeup":
      return <MakeupPanel draft={draft} patch={patch} errors={errors} />;

    case "acara-akad":
    case "acara-resepsi": {
      const key = tab === "acara-akad" ? "itemsAkad" : "itemsResepsi";
      return (
        <RowEditor<RundownAcaraItem>
          columns={acaraColumns(draft[key])}
          rows={draft[key]}
          onChange={(rows) => patch((d) => ({ ...d, [key]: rows }))}
          blank={() => ({ noLabel: "", timeLabel: "", item: "", pic: "", note: "" })}
          addLabel="Tambah acara"
          emptyDescription='Nomor terisi otomatis sesuai urutan. Centang "Tanpa nomor" untuk baris pembuka seperti persiapan dekor.'
          errors={errors}
          errorPrefix="items"
        />
      );
    }

    case "layout":
      return (
        <div className="flex flex-col gap-5">
          {mode === "rundown" && onPickLayout && (
            <div className="flex flex-wrap items-center gap-3 rounded-md border border-border bg-surface-muted px-3 py-3">
              <div className="flex-1 text-[13px]">
                <p className="font-medium text-text-primary">Denah akad</p>
                <p className="text-text-secondary">
                  {draft.hasLayoutImage
                    ? "Denah sudah diunggah dan akan tercetak di dokumen."
                    : "Belum ada denah. Dokumen akan memakai gambar placeholder."}{" "}
                  Format PNG atau JPG, maksimal 5 MB.
                </p>
              </div>
              <label className="cursor-pointer rounded-md focus-within:ring-2 focus-within:ring-navy-900/20">
                <input
                  type="file"
                  accept={LAYOUT_ACCEPT}
                  className="sr-only"
                  disabled={busy}
                  onChange={(e) => {
                    const f = e.target.files?.[0];
                    if (f) void onPickLayout(f);
                    e.target.value = "";
                  }}
                />
                <span className="inline-flex items-center gap-2 rounded-md border border-border bg-white px-3 py-2 text-[13px] font-medium hover:bg-surface-muted">
                  <Upload className="h-4 w-4" />
                  {busy ? "Mengunggah..." : draft.hasLayoutImage ? "Ganti denah" : "Unggah denah"}
                </span>
              </label>
            </div>
          )}
          <RowEditor<RundownLayoutNote>
            columns={[
              {
                key: "kind",
                label: "Jenis",
                kind: "select",
                width: "12rem",
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
            emptyDescription="Nomor aturan tamu dan keterangan denah terisi otomatis, masing-masing mulai dari 1."
            errors={errors}
            errorPrefix="layoutNotes"
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
          errors={errors}
          errorPrefix="photoGroups"
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
          errors={errors}
          errorPrefix="vipGuests"
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
            errors={errors}
            errorPrefix="playlist"
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
  errors,
}: {
  draft: RundownDetail;
  patch: (fn: (d: RundownDetail) => RundownDetail) => void;
  errors?: SectionErrors;
}) {
  const setRooms = (rooms: RundownMakeupRoom[]) => patch((d) => ({ ...d, makeupRooms: rooms }));

  return (
    <div className="flex flex-col gap-4">
      {draft.makeupRooms.length === 0 && (
        <p className="text-[13px] text-text-secondary">
          Belum ada ruangan. Tambahkan satu ruangan untuk setiap tempat makeup, misalnya ruang pengantin wanita dan
          ruang keluarga.
        </p>
      )}
      {draft.makeupRooms.map((room, ri) => {
        const roomError = errors?.[`makeupRooms.${ri}.roomLabel`];
        return (
          <div key={ri} className="rounded-md border border-border p-3">
            <div className="mb-3 flex items-end gap-2">
              <div className="flex-1">
                <Field label={`Ruangan ${ri + 1}`} hint={roomError}>
                  <Input
                    value={room.roomLabel}
                    aria-invalid={roomError ? true : undefined}
                    className={roomError ? "border-danger" : undefined}
                    onChange={(e) =>
                      setRooms(draft.makeupRooms.map((r, i) => (i === ri ? { ...r, roomLabel: e.target.value } : r)))
                    }
                  />
                </Field>
              </div>
              <Button variant="ghost" onClick={() => setRooms(draft.makeupRooms.filter((_, i) => i !== ri))}>
                Hapus ruangan
              </Button>
            </div>
            <RowEditor
              columns={[
                { key: "style", label: "Bentuk baris", kind: "select", width: "12rem", options: MAKEUP_STYLE_OPTIONS },
                { key: "content", label: "Isi" },
              ]}
              rows={room.lines}
              onChange={(lines) => setRooms(draft.makeupRooms.map((r, i) => (i === ri ? { ...r, lines } : r)))}
              blank={() => ({ style: "Numbered" as const, content: "" })}
              addLabel="Tambah baris"
              emptyDescription="Penomoran tiap ruangan dimulai ulang dari 1."
              errors={errors}
              errorPrefix={`makeupRooms.${ri}.lines`}
            />
          </div>
        );
      })}
      <div>
        <Button variant="secondary" onClick={() => setRooms([...draft.makeupRooms, { roomLabel: "", lines: [] }])}>
          Tambah ruangan
        </Button>
      </div>
    </div>
  );
}
