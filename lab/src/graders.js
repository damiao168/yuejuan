import { createAdapter } from "./adapters/index.js";
import { applyEvidenceVerification } from "./evidenceVerifier.js";
import { validateGradingInput, validateGradingOutput } from "./schemas/gradingSchema.js";

export class BaseQuestionGrader {
  constructor(adapter = createAdapter("mock")) {
    this.adapter = adapter;
  }

  grade(input) {
    const inputValidation = validateGradingInput(input);
    if (!inputValidation.valid) {
      throw new Error(`Invalid grading input: ${inputValidation.errors.join("; ")}`);
    }
    const output = applyEvidenceVerification(input, this.adapter.grade(input));
    const outputValidation = validateGradingOutput(output, input);
    if (!outputValidation.valid) {
      throw new Error(`Invalid grading output: ${outputValidation.errors.join("; ")}`);
    }
    return output;
  }
}

export class FillBlankGrader extends BaseQuestionGrader {}
export class NumericGrader extends BaseQuestionGrader {}
export class ShortAnswerGrader extends BaseQuestionGrader {}
export class CalculationGrader extends BaseQuestionGrader {}
export class EssayGrader extends BaseQuestionGrader {}
export class DiscussionGrader extends BaseQuestionGrader {}

export function createQuestionGrader(questionType, adapter = createAdapter("mock")) {
  const map = {
    fill_blank: FillBlankGrader,
    numeric: NumericGrader,
    short_answer: ShortAnswerGrader,
    calculation: CalculationGrader,
    essay: EssayGrader,
    discussion: DiscussionGrader
  };
  const Grader = map[questionType];
  if (!Grader) throw new Error(`Unsupported question type: ${questionType}`);
  return new Grader(adapter);
}
