import { describe, expect, it } from "vitest";
import { replaceTrailingTemplateShortcut, trailingTemplateShortcut } from "./templateShortcuts";

describe("review annotation template shortcuts", () => {
  it("detects only a trailing slash command", () => {
    expect(trailingTemplateShortcut("评分依据 /step-ok")).toBe("step-ok");
    expect(trailingTemplateShortcut("/TRAIT.2")).toBe("trait.2");
    expect(trailingTemplateShortcut("保留 /step-ok 后续文字")).toBeUndefined();
  });

  it("replaces the command while preserving editable surrounding text", () => {
    expect(replaceTrailingTemplateShortcut("依据充分 /step-ok", "关键步骤完整")).toBe("依据充分 关键步骤完整");
    expect(replaceTrailingTemplateShortcut("/step-ok", "关键步骤完整")).toBe("关键步骤完整");
  });
});
