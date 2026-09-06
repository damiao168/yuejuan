import fs from "node:fs";
import path from "node:path";

const root = process.cwd();
const gatewayRoot = path.join(root, "services", "api-gateway", "internal");
const openapiPath = path.join(root, "services", "api-gateway", "openapi", "edugrade-api.openapi.json");
const outputPath = path.join(root, "services", "api-gateway", "openapi", "route-coverage.json");
const routePattern = /mux\.Handle\("(GET|POST|PUT|PATCH|DELETE|HEAD|OPTIONS) ([^" ]+)"/g;

function walk(directory) {
  return fs.readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const absolute = path.join(directory, entry.name);
    if (entry.isDirectory()) return walk(absolute);
    return entry.isFile() && entry.name.endsWith(".go") && !entry.name.endsWith("_test.go") ? [absolute] : [];
  });
}

const openapi = JSON.parse(fs.readFileSync(openapiPath, "utf8"));
const routes = [];
for (const file of walk(gatewayRoot)) {
  const source = fs.readFileSync(file, "utf8");
  for (const match of source.matchAll(routePattern)) {
    const method = match[1].toLowerCase();
    const routePath = match[2];
    const relativeSource = path.relative(root, file).replaceAll("\\", "/");
    const operation = openapi.paths?.[routePath]?.[method];
    routes.push({
      method: match[1],
      path: routePath,
      source: relativeSource,
      coverage: operation ? "openapi" : "registered-gap",
      ...(operation?.operationId ? { operation_id: operation.operationId } : {}),
      ...(!operation ? { reason: "not yet part of the public generated SDK contract" } : {}),
    });
  }
}
routes.sort((left, right) => left.path.localeCompare(right.path) || left.method.localeCompare(right.method));
const duplicates = routes.filter((route, index) => index > 0 && route.method === routes[index - 1].method && route.path === routes[index - 1].path);
if (duplicates.length) throw new Error(`duplicate registered routes: ${duplicates.map((route) => `${route.method} ${route.path}`).join(", ")}`);

const document = {
  schema_version: 1,
  generated_from: "services/api-gateway/internal/**/*.go",
  policy: "Every registered API route is either covered by OpenAPI or explicitly listed as a registered gap.",
  counts: {
    registered: routes.length,
    openapi: routes.filter((route) => route.coverage === "openapi").length,
    registered_gaps: routes.filter((route) => route.coverage === "registered-gap").length,
  },
  routes,
};
fs.writeFileSync(outputPath, `${JSON.stringify(document, null, 2)}\n`);
console.log(`Registered ${document.counts.registered} routes: ${document.counts.openapi} OpenAPI, ${document.counts.registered_gaps} explicit gaps.`);
