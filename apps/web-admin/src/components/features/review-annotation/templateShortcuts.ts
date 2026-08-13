const trailingShortcutPattern = /(?:^|\s)\/([a-z0-9][a-z0-9._-]*)$/i;

export function trailingTemplateShortcut(content: string): string | undefined {
  return content.match(trailingShortcutPattern)?.[1]?.toLowerCase();
}

// Keep the replacement narrowly scoped to the trailing command.  It lets a
// reviewer type a template command into a focused annotation editor, insert
// it with Enter, then immediately refine the inserted wording for this paper.
export function replaceTrailingTemplateShortcut(content: string, replacement: string): string {
  return content.replace(trailingShortcutPattern, (command) => {
    const leadingSpace = command.startsWith(" ") ? " " : "";
    return `${leadingSpace}${replacement}`;
  });
}
