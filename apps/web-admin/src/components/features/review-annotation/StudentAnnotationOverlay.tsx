import type { StudentReviewAnnotation } from "../../../api/reviewAnnotations";
import { geometryStyle } from "./geometry";
import "./reviewAnnotations.css";

export interface StudentAnnotationOverlayProps {
  annotations: readonly StudentReviewAnnotation[];
}

// This component intentionally accepts only the student-safe SDK DTO. Teacher
// annotations cannot be passed without an explicit, reviewed projection.
export function StudentAnnotationOverlay({ annotations }: StudentAnnotationOverlayProps) {
  return (
    <div className="student-annotation-overlay" aria-label="教师公开批注">
      {annotations.map((annotation) => {
        const point = annotation.geometry.width === 0 && annotation.geometry.height === 0;
        return (
          <span
            key={annotation.id}
            className={point ? "review-annotation-mark review-annotation-mark--point" : "review-annotation-mark"}
            style={geometryStyle(annotation.geometry)}
            title={annotation.content}
          />
        );
      })}
    </div>
  );
}
