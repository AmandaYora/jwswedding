import type { MilestoneStatus } from "@/modules/projects/types";

// One Timeline Project item (project_milestones), flattened with its
// project's identity — backs the standalone Monitoring Timeline page
// (PLAN.md mom-25082026-item-belum item 17). Scoped server-side: a Wedding
// Planner only ever receives timelines from projects they're PIC of;
// Owner/Admin receive every project's.
export interface ClientTimeline {
  id: string;
  order: number;
  name: string;
  status: MilestoneStatus;
  targetDate: string;
  completedDate: string | null;
  projectId: string;
  projectName: string;
  brideName: string;
  groomName: string;
  eventDate: string;
  // The project's PIC Wedding Planner (Blok D) — "0" = "Belum ditugaskan".
  // Name resolved client-side via useStaffStore.
  picStaffId: string;
}
