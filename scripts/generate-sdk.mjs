import path from "node:path";
import { fileURLToPath } from "node:url";
import { generateClient, generateTypes, readOpenApi, writeIfChanged } from "./lib/openapi-sdk.mjs";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const spec = readOpenApi(path.join(root, "services/api-gateway/openapi/edugrade-api.openapi.json"));
const generated = path.join(root, "packages/sdk/src/generated");

const changed = [
  writeIfChanged(path.join(generated, "types.ts"), generateTypes(spec)),
  writeIfChanged(path.join(generated, "client.ts"), generateClient(spec))
].some(Boolean);

console.log(changed ? "Generated SDK updated." : "Generated SDK is current.");
