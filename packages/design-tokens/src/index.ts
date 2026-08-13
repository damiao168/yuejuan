export { breakpointTokens } from "./breakpoints";
export { colorTokens } from "./color";
export { gradingColors } from "./grading";
export { motionTokens } from "./motion";
export { radiusTokens } from "./radius";
export { shadowTokens } from "./shadow";
export { spacingTokens } from "./spacing";
export { typographyTokens } from "./typography";
export { zIndexTokens } from "./zIndex";

export { colorTokens as semanticColors } from "./color";

export const workspaceSpacing = {
  compact: "var(--eg-space-2)",
  normal: "var(--eg-space-4)",
  section: "var(--eg-space-6)"
} as const;
