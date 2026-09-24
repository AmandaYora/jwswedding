import { useCallback, useEffect, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { ArrowLeft, BookmarkPlus, FileText, FileType } from "lucide-react";
import { Button } from "@/shared/components/ui/Button";
import { ConfirmDialog } from "@/shared/components/ui/ConfirmDialog";
import { useRundownStore, type SectionPayload } from "@/modules/rundowns/stores/useRundownStore";
import { RundownEditor, type EditorApi } from "@/modules/rundowns/components/RundownEditor";
import { ProjectResync } from "@/modules/rundowns/components/ProjectResyncDialog";
import { RUNDOWN_TABS } from "@/modules/rundowns/schemas/rundown.schema";
import { generateMessage, type GenerateFormat } from "@/modules/rundowns/lib/generate-message";
import { ROUTE_PATHS, type RundownTab } from "@/app/routes/route-paths";
import { getApiErrorMessage, getApiErrorMessageFromBlob } from "@/shared/lib/api-error";
import { blobToBase64 } from "@/shared/lib/image-compression";

/** Sama dengan maxLayoutImageSize di rundown_service.go. */
const MAX_LAYOUT_BYTES = 5 * 1024 * 1024;
const LAYOUT_TYPES = ["image/png", "image/jpeg"];

type Format = GenerateFormat;

interface Banner {
  tone: "success" | "danger";
  text: string;
  /** Tautan ke tab Dokumen project, untuk hasil generate yang tersimpan. */
  documentsLink?: boolean;
}

export default function RundownDetailPage() {
  const { id = "", tab = "cover" } = useParams<{ id: string; tab: RundownTab }>();
  const navigate = useNavigate();
  const detail = useRundownStore((s) => s.detail);
  const fetchDetail = useRundownStore((s) => s.fetchDetail);
  const saveSection = useRundownStore((s) => s.saveSection);
  const uploadLayout = useRundownStore((s) => s.uploadLayout);
  const generate = useRundownStore((s) => s.generate);
  const saveAsTemplate = useRundownStore((s) => s.saveAsTemplate);

  const [loadError, setLoadError] = useState("");
  const [generating, setGenerating] = useState<Format | null>(null);
  const [uploading, setUploading] = useState(false);
  const [banner, setBanner] = useState<Banner | null>(null);
  const [templateConfirm, setTemplateConfirm] = useState(false);
  const [templateBusy, setTemplateBusy] = useState(false);
  const [templateError, setTemplateError] = useState<string | null>(null);
  // API editor pada saat "Jadikan Template" ditekan — dipakai untuk menyimpan
  // seksi yang sedang terbuka sebelum isinya disalin ke template.
  const [pendingTemplateApi, setPendingTemplateApi] = useState<EditorApi | null>(null);

  useEffect(() => {
    if (!id) return;
    setLoadError("");
    fetchDetail(id).catch((e) => setLoadError(getApiErrorMessage(e, "Rundown gagal dimuat")));
  }, [id, fetchDetail]);

  // Pesan hasil generate milik seksi tempat tombolnya ditekan.
  useEffect(() => setBanner(null), [tab]);

  const onSaveSection = useCallback(
    (section: RundownTab, payload: SectionPayload) => saveSection(id, section, payload),
    [id, saveSection]
  );

  const onGenerate = async (api: EditorApi, format: Format) => {
    setBanner(null);
    // Generate membaca data dari server: simpan dulu apa yang terlihat di
    // layar, supaya yang tercetak sama dengan yang terlihat.
    if (!(await api.saveIfDirty())) return;
    setGenerating(format);
    try {
      const { archive } = await generate(id, format);
      setBanner(generateMessage(format, archive));
    } catch (e) {
      setBanner({ tone: "danger", text: await getApiErrorMessageFromBlob(e, "Gagal membuat berkas rundown") });
    } finally {
      setGenerating(null);
    }
  };

  const onPickLayout = async (file: File) => {
    // Diperiksa di sini juga, bukan hanya di server: `accept` cuma menyaring
    // dialog pemilih berkas, dan tanpa ini berkas 5 MB tetap di-encode lalu
    // dikirim utuh sebelum ditolak. Batasnya sama persis dengan server.
    if (!LAYOUT_TYPES.includes(file.type)) {
      setBanner({ tone: "danger", text: "Denah akad harus berupa berkas PNG atau JPG." });
      return;
    }
    if (file.size > MAX_LAYOUT_BYTES) {
      setBanner({ tone: "danger", text: "Ukuran denah maksimal 5 MB." });
      return;
    }
    setUploading(true);
    setBanner(null);
    try {
      // blobToBase64 (FileReader), bukan loop String.fromCharCode per byte:
      // pada berkas 5 MB loop itu membekukan UI beberapa detik. Sengaja TIDAK
      // lewat compressFileForUpload — server yang mengubah JPG ke PNG dan
      // mengecilkannya; PNG ditukar apa adanya ke dalam .docx.
      await uploadLayout(id, await blobToBase64(file));
      setBanner({ tone: "success", text: "Denah akad tersimpan." });
    } catch (e) {
      setBanner({ tone: "danger", text: getApiErrorMessage(e, "Gagal mengunggah denah") });
    } finally {
      setUploading(false);
    }
  };

  const onSaveAsTemplate = async () => {
    if (!pendingTemplateApi) return;
    setTemplateBusy(true);
    setTemplateError(null);
    try {
      if (!(await pendingTemplateApi.saveIfDirty())) {
        setTemplateConfirm(false);
        return;
      }
      await saveAsTemplate(id);
      setTemplateConfirm(false);
      setBanner({
        tone: "success",
        text: "Template Rundown diperbarui dari rundown ini. Rundown baru berikutnya bisa langsung memakainya.",
      });
    } catch (e) {
      setTemplateError(getApiErrorMessage(e, "Gagal memperbarui Template Rundown"));
    } finally {
      setTemplateBusy(false);
    }
  };

  if (loadError) {
    return (
      <div className="flex flex-col items-start gap-3">
        <p className="text-[13px] text-danger">{loadError}</p>
        <Button variant="secondary" onClick={() => navigate(ROUTE_PATHS.rundowns)}>
          Kembali ke daftar rundown
        </Button>
      </div>
    );
  }

  // `detail` bisa milik rundown lain yang terakhir dibuka — tunggu yang benar.
  if (!detail || detail.id !== id) {
    return <p className="text-[13px] text-text-secondary">Memuat rundown...</p>;
  }

  const busy = generating !== null;

  return (
    <>
      <RundownEditor
        value={detail}
        tabs={RUNDOWN_TABS}
        tab={tab}
        pathFor={(t) => ROUTE_PATHS.rundownDetail(id, t)}
        onSaveSection={onSaveSection}
        onPickLayout={onPickLayout}
        layoutBusy={uploading}
        banner={
          banner && (
            <div
              role="status"
              className={
                banner.tone === "success"
                  ? "rounded-md border border-success/30 bg-success/5 px-3 py-2 text-[13px] text-success"
                  : "rounded-md border border-danger/30 bg-danger/5 px-3 py-2 text-[13px] text-danger"
              }
            >
              {banner.text}{" "}
              {banner.documentsLink && (
                <Link
                  to={ROUTE_PATHS.projectDetail(detail.projectId, "dokumen")}
                  className="font-medium underline underline-offset-2"
                >
                  Lihat di Dokumen project
                </Link>
              )}
            </div>
          )
        }
        toolbarFor={(t, api) =>
          t === "cover" || t === "vendors" ? (
            <ProjectResync
              kind={t}
              rundownId={id}
              projectId={detail.projectId}
              draft={api.draft}
              patch={api.patch}
            />
          ) : null
        }
        header={(api) => (
          <div className="flex flex-wrap items-start justify-between gap-3">
            <div className="flex items-start gap-3">
              <Button variant="ghost" onClick={() => navigate(ROUTE_PATHS.rundowns)} aria-label="Kembali">
                <ArrowLeft className="h-4 w-4" />
              </Button>
              <div>
                <h1 className="text-xl font-semibold text-text-primary">{detail.projectName}</h1>
                <p className="text-[13px] text-text-secondary">
                  {api.filled.size} dari {RUNDOWN_TABS.length} seksi terisi. Seksi yang kosong tetap tercetak sebagai
                  kerangka.
                </p>
              </div>
            </div>
            <div className="flex flex-wrap gap-2">
              <Button
                variant="ghost"
                icon={<BookmarkPlus className="h-4 w-4" />}
                disabled={busy || api.saving}
                onClick={() => {
                  setTemplateError(null);
                  setPendingTemplateApi(api);
                  setTemplateConfirm(true);
                }}
              >
                Jadikan Template
              </Button>
              <Button
                variant="secondary"
                icon={<FileType className="h-4 w-4" />}
                disabled={busy || api.saving}
                onClick={() => void onGenerate(api, "docx")}
              >
                {generating === "docx" ? "Menyiapkan DOCX..." : "Unduh DOCX"}
              </Button>
              <Button
                icon={<FileText className="h-4 w-4" />}
                disabled={busy || api.saving}
                onClick={() => void onGenerate(api, "pdf")}
              >
                {generating === "pdf" ? "Menyiapkan PDF..." : "Unduh PDF"}
              </Button>
            </div>
          </div>
        )}
      />

      <ConfirmDialog
        open={templateConfirm}
        onClose={() => setTemplateConfirm(false)}
        onConfirm={() => void onSaveAsTemplate()}
        title="Jadikan Template"
        message="Ganti Template Rundown dengan isi rundown ini?"
        details="Yang disalin: List Nama, Panitia Keluarga, Ruangan Makeup, Susunan Acara Akad dan Resepsi, serta catatan Layout. Nama orang di List Nama dan Panitia dikosongkan. Rundown lain yang sudah ada tidak berubah."
        confirmLabel="Ganti template"
        busyLabel="Menyimpan..."
        busy={templateBusy}
        error={templateError}
        tone="default"
      />
    </>
  );
}
