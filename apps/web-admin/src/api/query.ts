export type QueryValue = string | number | boolean | readonly string[] | null | undefined;

export function buildQueryString<T extends object>(values: T) {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(values) as Array<[string, QueryValue]>) {
    if (value === undefined || value === null || value === "" || value === false || value === 0) continue;
    if (Array.isArray(value)) {
      const items = value.map((item) => item.trim()).filter(Boolean);
      if (items.length > 0) params.set(key, items.join(","));
      continue;
    }
    params.set(key, String(value));
  }
  const query = params.toString();
  return query ? `?${query}` : "";
}
