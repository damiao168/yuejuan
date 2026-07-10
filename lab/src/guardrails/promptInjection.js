const DEFAULT_RULES = [
  { id: "ignore-rubric-cn", pattern: /忽略(之前|以上|评分|规则|标准)/i },
  { id: "full-score-cn", pattern: /给我满分|直接满分|应该给\s*\d+\s*分/i },
  { id: "role-override-cn", pattern: /你现在是|系统管理员|开发者模式|老师说/i },
  { id: "schema-bypass-cn", pattern: /输出.*schema.*之外|不要告诉老师/i },
  { id: "ignore-rubric-en", pattern: /ignore (all|previous|the) (rules|instructions|rubric)/i },
  { id: "full-score-en", pattern: /give me full marks|give me full score/i },
  { id: "role-override-en", pattern: /you are now (the )?(teacher|system|admin)/i }
];

export class PromptInjectionDetector {
  constructor(rules = DEFAULT_RULES) {
    this.rules = rules;
  }

  detect(text) {
    const source = String(text ?? "");
    const matches = this.rules.filter((rule) => rule.pattern.test(source)).map((rule) => rule.id);
    return {
      detected: matches.length > 0,
      matches,
      risk_flags: matches.length > 0 ? ["PROMPT_INJECTION_SUSPECTED", "HUMAN_REVIEW_REQUIRED"] : []
    };
  }
}

export function detectPromptInjection(text) {
  return new PromptInjectionDetector().detect(text);
}
