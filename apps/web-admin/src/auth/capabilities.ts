import type { ProductExperience } from "../router/experience";
import type { SessionUser } from "./session";

export function canWorkTeacherAppeals(
  user: Pick<SessionUser, "permissions"> | null,
  experience: ProductExperience,
): boolean {
  return experience === "teacher" && Boolean(user?.permissions.includes("appeal:work"));
}

export interface TeacherAppealRecommendationContext {
  experience: ProductExperience;
  canWork: boolean;
  terminal: boolean;
  assignedTo?: string | null;
  currentUserId: string;
}

export function canSubmitTeacherAppealRecommendation({
  experience,
  canWork,
  terminal,
  assignedTo,
  currentUserId,
}: TeacherAppealRecommendationContext): boolean {
  return experience === "teacher"
    && canWork
    && !terminal
    && Boolean(currentUserId)
    && assignedTo === currentUserId;
}
