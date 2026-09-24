import { useCallback, useEffect, useMemo, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { ArrowLeft } from "lucide-react";
import { Button } from "@/shared/components/ui/Button";
import { useRundownStore, type SectionPayload } from "@/modules/rundowns/stores/useRundownStore";
import { RundownEditor } from "@/modules/rundowns/components/RundownEditor";
import { TEMPLATE_TABS } from "@/modules/rundowns/schemas/rundown.schema";
import type { RundownDetail, RundownTemplate } from "@/modules/rundowns/types";
import { ROUTE_PATHS, type RundownTab } from "@/app/routes/route-paths";
import { getApiErrorMessage } from "@/shared/lib/api-error";

/**
 * Template dipetakan ke bentuk RundownDetail supaya editor seksi yang sama
 * dipakai apa adanya; field di luar enam seksi template dibiarkan kosong dan
 * tidak pernah dikirim (payloadFor hanya membaca seksi yang dibuka).
 */
export function templateToDetail(t: RundownTemplate): RundownDetail {
  return {
    id: "template",
    projectId: "",
    projectName: "Template Rundown",
    cover: {
      groomName: "", groomBirthOrder: "", groomParents: "", brideName: "", brideBirthOrder: "",
      brideParents: "", eventDateLabel: "", venueLabel: "", eventTimeLabel: "", coupleTitle: "",
      woPicName: "", woPicPhone: "",
    },
    dataLainnya: { siblingsBride: "", siblingsGroom: "", souvenirNote: "", tableClothNote: "" },
    vendors: [],
    roles: t.roles,
    committees: t.committees,
    menuItems: [],
    makeupRooms: t.makeupRooms,
    itemsAkad: t.itemsAkad,
    itemsResepsi: t.itemsResepsi,
    layoutNotes: t.layoutNotes,
    photoGroups: [],
    vipGuests: [],
    playlist: [],
    playlistNotes: "",
    hasLayoutImage: false,
    updatedAt: t.updatedAt ?? "",
  };
}

export default function RundownTemplatePage() {
  const { tab = TEMPLATE_TABS[0].key } = useParams<{ tab: RundownTab }>();
  const navigate = useNavigate();
  const template = useRundownStore((s) => s.template);
  const fetchTemplate = useRundownStore((s) => s.fetchTemplate);
  const saveTemplateSection = useRundownStore((s) => s.saveTemplateSection);
  const [loadError, setLoadError] = useState("");

  useEffect(() => {
    fetchTemplate().catch((e) => setLoadError(getApiErrorMessage(e, "Template Rundown gagal dimuat")));
  }, [fetchTemplate]);

  // Memo pada objek template: editor menyegarkan draftnya setiap kali
  // `value` berganti identitas, jadi pemetaan tidak boleh dibuat ulang tiap
  // render.
  const value = useMemo(() => (template ? templateToDetail(template) : null), [template]);

  const onSaveSection = useCallback(
    (section: RundownTab, payload: SectionPayload) => saveTemplateSection(section, payload),
    [saveTemplateSection]
  );

  if (loadError) return <p className="text-[13px] text-danger">{loadError}</p>;
  if (!value) return <p className="text-[13px] text-text-secondary">Memuat template...</p>;

  return (
    <RundownEditor
      value={value}
      tabs={TEMPLATE_TABS}
      tab={tab}
      pathFor={(t) => ROUTE_PATHS.rundownTemplate(t)}
      onSaveSection={onSaveSection}
      mode="template"
      header={() => (
        <div className="flex items-start gap-3">
          <Button variant="ghost" onClick={() => navigate(ROUTE_PATHS.rundowns)} aria-label="Kembali">
            <ArrowLeft className="h-4 w-4" />
          </Button>
          <div>
            <h1 className="text-xl font-semibold text-text-primary">Template Rundown</h1>
            <p className="max-w-2xl text-[13px] text-text-secondary">
              Isi standar yang disalin ke setiap rundown baru. Tulis perannya saja; nama orang diisi di rundown
              masing-masing acara. Mengubah template tidak mengubah rundown yang sudah ada.
            </p>
          </div>
        </div>
      )}
    />
  );
}
