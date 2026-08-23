import { useOutletContext } from "react-router-dom";
import { ClientInvoicesSection } from "@/modules/projects/components/detail/ClientInvoicesSection";
import { ClientPaymentsSection } from "@/modules/projects/components/detail/ClientPaymentsSection";
import type { ProjectDetailContext } from "@/modules/projects/pages/ProjectDetailLayout";

export default function PembayaranClientTabPage() {
  const { projectId } = useOutletContext<ProjectDetailContext>();
  return (
    <div className="flex flex-col gap-6">
      <ClientInvoicesSection projectId={projectId} />
      <ClientPaymentsSection projectId={projectId} />
    </div>
  );
}
