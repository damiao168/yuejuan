// Formula recognition and its evidence UI are intentionally opt-in. Do not
// widen this list without subject-specific benchmark evidence and a rollout
// decision: language and humanities answers must stay on the general OCR path.
const subjectAliases: Record<string, "mathematics" | "physics" | "chemistry"> = {
  mathematics: "mathematics",
  math: "mathematics",
  "数学": "mathematics",
  physics: "physics",
  "物理": "physics",
  chemistry: "chemistry",
  "化学": "chemistry"
};

export function isFormulaEvidenceSubject(subject: string | undefined): boolean {
  return Boolean(subjectAliases[String(subject ?? "").trim().toLowerCase()]);
}

