export const SUBJECT_STRATEGIES = Object.freeze({
  math: {
    default_review_policy: "review_when_steps_or_units_missing",
    required_risk_flags: [],
    scoring_constraints: ["do_not_award_for_final_answer_only_when_steps_required", "respect_numeric_tolerance"],
    prompt_template_id: "calculation"
  },
  physics: {
    default_review_policy: "review_when_key_concept_missing",
    required_risk_flags: [],
    scoring_constraints: ["require_concept_evidence", "respect_units"],
    prompt_template_id: "short_answer"
  },
  chemistry: {
    default_review_policy: "review_when_formula_or_unit_ambiguous",
    required_risk_flags: [],
    scoring_constraints: ["require_reaction_or_concept_evidence"],
    prompt_template_id: "short_answer"
  },
  biology: {
    default_review_policy: "review_when_process_or_term_missing",
    required_risk_flags: [],
    scoring_constraints: ["require_key_term_evidence"],
    prompt_template_id: "short_answer"
  },
  english: {
    default_review_policy: "essay_and_discussion_always_review",
    required_risk_flags: ["HUMAN_REVIEW_REQUIRED"],
    scoring_constraints: ["do_not_reward_fluency_without_rubric_evidence"],
    prompt_template_id: "essay"
  },
  chinese: {
    default_review_policy: "review_open_ended_reading_answers",
    required_risk_flags: [],
    scoring_constraints: ["do_not_reward_length_alone", "require_textual_evidence"],
    prompt_template_id: "short_answer"
  },
  history: {
    default_review_policy: "review_when_claim_lacks_evidence",
    required_risk_flags: [],
    scoring_constraints: ["require_event_or_causality_evidence"],
    prompt_template_id: "short_answer"
  },
  politics: {
    default_review_policy: "review_when_policy_term_missing",
    required_risk_flags: [],
    scoring_constraints: ["require_concept_and_argument_evidence"],
    prompt_template_id: "short_answer"
  },
  geography: {
    default_review_policy: "review_when_spatial_cause_missing",
    required_risk_flags: [],
    scoring_constraints: ["require_location_or_process_evidence"],
    prompt_template_id: "short_answer"
  },
  computer_science: {
    default_review_policy: "always_review_external_technical_answers",
    required_risk_flags: ["HUMAN_REVIEW_REQUIRED"],
    scoring_constraints: ["require_technical_claim_evidence", "do_not_infer_missing_code_or_configuration"],
    prompt_template_id: "short_answer"
  }
});

export function getSubjectStrategy(subject) {
  const strategy = SUBJECT_STRATEGIES[subject];
  if (!strategy) throw new Error(`Unsupported subject strategy: ${subject}`);
  return strategy;
}
