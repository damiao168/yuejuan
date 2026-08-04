export function hashQueryParam(name: string): string {
  const query = window.location.hash.split("?", 2)[1] ?? "";
  return new URLSearchParams(query).get(name)?.trim() ?? "";
}
