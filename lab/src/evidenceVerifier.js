import { OCR_REVIEW_THRESHOLD } from "./schemas/gradingSchema.js";
import { detectPromptInjection } from "./guardrails/promptInjection.js";

const CHINESE_PUNCTUATION = /[，。！？；：“”‘’（）【】《》、]/g;

export function normalizeText(text) {
  return String(text ?? "")
    .normalize("NFKC")
    .toLowerCase()
    .replace(CHINESE_PUNCTUATION, "")
    .replace(/[.,!?;:'"()[\]{}<>/\\|-]/g, "")
    .replace(/\s+/g, "");
}

function pushUnique(list, value) {
  if (!list.includes(value)) list.push(value);
}

export function verifyEvidence(input, output) {
  const invalidPoints = [];
  const warnings = [];
  const forcedRiskFlags = [];
  let forcedNeedsHumanReview = false;
  const answerText = String(input.answer_text ?? "");
  const normalizedAnswer = normalizeText(answerText);
  const rubricPointIds = new Set(input.rubric.points.map((point) => point.id));
  const evidenceByRubricPoint = new Map();
  const evidenceById = new Map();
  let validMatchedPoints = 0;

  for (const evidence of output.evidence ?? []) {
    if (evidenceById.has(evidence.evidence_id)) {
      invalidPoints.push({ evidence_id: evidence.evidence_id, reason: "duplicate_evidence_id" });
    }
    evidenceById.set(evidence.evidence_id, evidence);
    if (!rubricPointIds.has(evidence.rubric_point_id)) {
      invalidPoints.push({ rubric_point_id: evidence.rubric_point_id, reason: "evidence_rubric_point_not_found" });
    }
    if (!evidenceByRubricPoint.has(evidence.rubric_point_id)) evidenceByRubricPoint.set(evidence.rubric_point_id, []);
    evidenceByRubricPoint.get(evidence.rubric_point_id).push(evidence);
  }

  if (input.ocr_confidence < OCR_REVIEW_THRESHOLD) {
    pushUnique(forcedRiskFlags, "OCR_LOW_CONFIDENCE");
    pushUnique(forcedRiskFlags, "HUMAN_REVIEW_REQUIRED");
    forcedNeedsHumanReview = true;
  }
  if (answerText.trim().length === 0) {
    pushUnique(forcedRiskFlags, "INSUFFICIENT_EVIDENCE");
    pushUnique(forcedRiskFlags, "HUMAN_REVIEW_REQUIRED");
    forcedNeedsHumanReview = true;
    if (input.answer_image_ref) pushUnique(forcedRiskFlags, "OCR_TEXT_EMPTY_REVIEW_REQUIRED");
  }

  const injection = detectPromptInjection(answerText);
  for (const flag of injection.risk_flags) pushUnique(forcedRiskFlags, flag);
  if (injection.detected) forcedNeedsHumanReview = true;

  let matchedScoreTotal = 0;
  for (const matchedPoint of output.matched_points ?? []) {
    const rubricPointId = matchedPoint.rubric_point_id;
    matchedScoreTotal += Number(matchedPoint.score ?? 0);
    if (!rubricPointIds.has(rubricPointId)) {
      invalidPoints.push({ rubric_point_id: rubricPointId, reason: "rubric_point_id_not_found" });
      continue;
    }
    const rubricPoint = input.rubric.points.find((point) => point.id === rubricPointId);
    const linkedEvidence = evidenceByRubricPoint.get(rubricPointId) ?? [];
    if (rubricPoint.evidence_required && linkedEvidence.length === 0) {
      invalidPoints.push({ rubric_point_id: rubricPointId, reason: "required_evidence_missing" });
      continue;
    }
    let idLinksValid = true;
    for (const evidenceId of matchedPoint.evidence_ids ?? []) {
      const linkedById = evidenceById.get(evidenceId);
      if (!linkedById || linkedById.rubric_point_id !== rubricPointId) {
        invalidPoints.push({ rubric_point_id: rubricPointId, evidence_id: evidenceId, reason: "evidence_id_link_invalid" });
        idLinksValid = false;
      }
    }
    const evidenceIsValid = linkedEvidence.every((evidence) => {
      const excerpt = normalizeText(evidence.text_excerpt);
      return excerpt.length > 0 && normalizedAnswer.includes(excerpt);
    });
    if (!evidenceIsValid) {
      invalidPoints.push({ rubric_point_id: rubricPointId, reason: "evidence_excerpt_not_in_answer" });
      continue;
    }
    if (idLinksValid) validMatchedPoints += 1;
  }

  for (const missing of output.missing_points ?? []) {
    if (!rubricPointIds.has(missing.rubric_point_id)) {
      invalidPoints.push({ rubric_point_id: missing.rubric_point_id, reason: "missing_point_not_in_rubric" });
    }
  }
  const deductionIds = new Set((input.rubric.deductions ?? []).map((deduction) => deduction.id));
  for (const deduction of output.deductions ?? []) {
    if (deduction.rubric_deduction_id && !deductionIds.has(deduction.rubric_deduction_id)) {
      invalidPoints.push({ rubric_deduction_id: deduction.rubric_deduction_id, reason: "deduction_not_in_rubric" });
    }
  }

  const allowance = Number(input.rubric.holistic_score_allowance ?? 0);
  if (output.suggested_score > matchedScoreTotal + allowance) {
    invalidPoints.push({ reason: "suggested_score_exceeds_matched_points", suggested_score: output.suggested_score });
  }
  if (invalidPoints.length > 0) {
    pushUnique(forcedRiskFlags, "INSUFFICIENT_EVIDENCE");
    pushUnique(forcedRiskFlags, "HUMAN_REVIEW_REQUIRED");
    forcedNeedsHumanReview = true;
  }

  const matchedCount = (output.matched_points ?? []).length;
  const evidenceValidityRate = matchedCount === 0 ? (answerText.trim() ? 1 : 0) : validMatchedPoints / matchedCount;
  return {
    verification_passed: invalidPoints.length === 0,
    evidence_validity_rate: evidenceValidityRate,
    invalid_points: invalidPoints,
    warnings,
    forced_risk_flags: forcedRiskFlags,
    forced_needs_human_review: forcedNeedsHumanReview
  };
}

export function applyEvidenceVerification(input, output) {
  const verification = verifyEvidence(input, output);
  return {
    ...output,
    risk_flags: [...new Set([...(output.risk_flags ?? []), ...verification.forced_risk_flags])],
    needs_human_review: output.needs_human_review || verification.forced_needs_human_review
  };
}
