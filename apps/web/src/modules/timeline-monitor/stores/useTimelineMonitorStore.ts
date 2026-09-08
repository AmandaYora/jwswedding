import { create } from "zustand";
import { httpClient } from "@/shared/services/http-client";
import { API } from "@/shared/services/api-endpoints";
import type { ClientTimeline } from "@/modules/timeline-monitor/types";

interface RawClientTimeline {
  id: number;
  order: number;
  name: string;
  status: ClientTimeline["status"];
  targetDate: string;
  completedDate: string | null;
  projectId: number;
  projectName: string;
  brideName: string;
  groomName: string;
  eventDate: string;
  picStaffId: number;
}

function toClientTimeline(raw: RawClientTimeline): ClientTimeline {
  return {
    id: String(raw.id), order: raw.order, name: raw.name, status: raw.status,
    targetDate: raw.targetDate, completedDate: raw.completedDate,
    projectId: String(raw.projectId), projectName: raw.projectName,
    brideName: raw.brideName, groomName: raw.groomName, eventDate: raw.eventDate,
    picStaffId: String(raw.picStaffId),
  };
}

interface TimelineMonitorState {
  timelines: ClientTimeline[];
  fetchTimelines: () => Promise<void>;
}

// Backed by GET /api/v1/client-timelines (PLAN.md mom-25082026-item-belum
// item 17) — one fetch, already scoped server-side to the caller's own
// PIC'd projects for a Wedding Planner (Owner/Admin get every project's).
// Every filter on the page itself (search/bulan/status) works client-side
// over this one list, same pattern as ClientListPage's own allClients.
export const useTimelineMonitorStore = create<TimelineMonitorState>((set) => ({
  timelines: [],

  fetchTimelines: async () => {
    const res = await httpClient.get(API.clientTimelines);
    set({ timelines: (res.data.data as RawClientTimeline[]).map(toClientTimeline) });
  },
}));
