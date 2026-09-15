import { Badge } from "@/shared/components/ui/Badge";
import {
  ENGAGEMENT_STATUS_TONE,
  ISSUE_IMPACT_TONE,
  ISSUE_STATUS_TONE,
  MILESTONE_STATUS_TONE,
  PROJECT_STATUS_TONE,
} from "@/modules/projects/lib/status";
import {
  CLIENT_ENGAGEMENT_STATUS_LABEL,
  CLIENT_ISSUE_IMPACT_LABEL,
  CLIENT_ISSUE_STATUS_LABEL,
  CLIENT_MILESTONE_STATUS_LABEL,
  CLIENT_PROJECT_STATUS_LABEL,
} from "@/modules/client-portal/lib/labels";
import type {
  EngagementStatus,
  IssueImpact,
  IssueStatus,
  MilestoneStatus,
  ProjectStatus,
} from "@/modules/projects/types";

// Client-facing counterparts of `projects/components/StatusBadges.tsx` —
// same tones, Indonesian words (see lib/labels.ts). The WO Console keeps
// using StatusBadges; only the portal swaps in these.

export function ClientProjectStatusBadge({ status }: { status: ProjectStatus }) {
  return <Badge tone={PROJECT_STATUS_TONE[status]}>{CLIENT_PROJECT_STATUS_LABEL[status]}</Badge>;
}

export function ClientMilestoneStatusBadge({ status }: { status: MilestoneStatus }) {
  return <Badge tone={MILESTONE_STATUS_TONE[status]}>{CLIENT_MILESTONE_STATUS_LABEL[status]}</Badge>;
}

export function ClientEngagementStatusBadge({ status }: { status: EngagementStatus }) {
  return <Badge tone={ENGAGEMENT_STATUS_TONE[status]}>{CLIENT_ENGAGEMENT_STATUS_LABEL[status]}</Badge>;
}

export function ClientIssueStatusBadge({ status }: { status: IssueStatus }) {
  return <Badge tone={ISSUE_STATUS_TONE[status]}>{CLIENT_ISSUE_STATUS_LABEL[status]}</Badge>;
}

export function ClientIssueImpactBadge({ impact }: { impact: IssueImpact }) {
  return <Badge tone={ISSUE_IMPACT_TONE[impact]}>{CLIENT_ISSUE_IMPACT_LABEL[impact]}</Badge>;
}
