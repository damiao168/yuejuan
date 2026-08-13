import path from "node:path";
import { fileURLToPath } from "node:url";
import { createBreakingBaseline, readOpenApi, stableJson, writeIfChanged } from "./lib/openapi-sdk.mjs";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const spec = readOpenApi(path.join(root, "services/api-gateway/openapi/edugrade-api.openapi.json"));
const baselineFile = path.join(root, "contracts/openapi/edugrade-api.breaking-baseline.json");
writeIfChanged(baselineFile, stableJson(createBreakingBaseline(spec)));
console.log(`Updated ${path.relative(root, baselineFile)}.`);
