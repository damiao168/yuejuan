# EduGrade TypeScript SDK

`src/generated/` is generated from the API gateway OpenAPI document. Do not edit generated files by hand.

```powershell
npm run generate:sdk
npm run check:openapi-breaking
```

The generated `EduGradeApi` only contains operations present in the OpenAPI contract. Domain helpers stay thin and accept an `ApiTransport`, so Web and Desktop clients retain their existing authentication, CSRF and idempotency behavior.

When a reviewed breaking API change is intentional, update the compatibility baseline in the same change:

```powershell
npm run openapi:baseline
```

CI regenerates the SDK and rejects a diff. It also rejects removed operations or schemas, newly required parameters/properties, changed types/references, and removed enum values relative to `contracts/openapi/edugrade-api.breaking-baseline.json`.
