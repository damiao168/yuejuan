import type { ReactNode } from "react";

/** A rubric row with a stable score/required affordance for grading surfaces. */
export function RubricCriterion({
  label,
  score,
  required = false,
  leading,
  children
}: {
  label: ReactNode;
  score: number;
  required?: boolean;
  /** Optional control displayed before the criterion label, e.g. a checkbox. */
  leading?: ReactNode;
  /** Optional trailing control, e.g. an earned-score input. */
  children?: ReactNode;
}) {
  return (
    <div className="eg-rubric-criterion">
      <div className="eg-rubric-criterion-main">{leading}<span>{label}</span>{children}</div>
      <div className="eg-rubric-criterion-meta"><b>{score} 分</b>{required ? <em>必选</em> : null}</div>
    </div>
  );
}
