export type GradingMode =
  | "auto"
  | "ai_assisted_human_final"
  | "single_mark_with_sampling"
  | "double_mark_arbitration"
  | "blind_mark_arbitration";

export type WorkflowStatus =
  | "CREATED"
  | "UPLOADED"
  | "PREPROCESSED"
  | "OCR_DONE"
  | "SEGMENTED"
  | "RUBRIC_READY"
  | "AI_GRADED"
  | "VERIFIED"
  | "HUMAN_REVIEWING"
  | "FINALIZED"
  | "PUBLISHED"
  | "APPEALING"
  | "ARCHIVED";

export interface AiGradeEvidence {
  rubricPointId: string;
  score: number;
  evidence: string;
  bbox?: [number, number, number, number];
}

export interface AiGradeResult {
  answerId: string;
  questionId: string;
  suggestedScore: number;
  maxScore: number;
  confidence: number;
  modelVersion: string;
  rubricVersion: string;
  matchedPoints: AiGradeEvidence[];
  missingPoints: Array<{
    rubricPointId: string;
    lostScore: number;
    reason: string;
  }>;
  riskFlags: string[];
  needsHumanReview: boolean;
}
