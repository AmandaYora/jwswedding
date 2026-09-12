import { useOutletContext } from "react-router-dom";
import { PackageOrderSection } from "@/modules/projects/components/detail/PackageOrderSection";
import type { ProjectDetailContext } from "@/modules/projects/pages/ProjectDetailLayout";

export default function ProjectPackageTabPage() {
  const { projectId } = useOutletContext<ProjectDetailContext>();
  return <PackageOrderSection projectId={projectId} />;
}
