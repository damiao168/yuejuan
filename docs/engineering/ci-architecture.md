# CI architecture

CI is organized as independent layers so one failure does not hide unrelated failures:

- Workflow syntax: the independent `workflow-lint.yml` runs actionlint whenever workflow files change.
- Static: `web-static`, `go-static`, `python-static`.
- Unit: `web-unit`, `go-unit`, `go-race`, and `python-tests` matrix entries.
- Contracts and acceptance: `story-gates` (fail-fast disabled), `lab`.
- Database/integration: `go-postgres`, `go-boundary`, `compose-config`.
- Security: `node-security`, `python-security`, `go-security`, and `supply-chain` (SBOM plus Trivy).
- Release supply chain: `release-images.yml` builds all runtime images from repository variables pinned by digest, runs image-level Trivy and CycloneDX generation before publication, then signs the published digest and attaches an SBOM attestation with Cosign.
- Infrastructure: `compose-config` validates all profiles and isolated stacks; `compose-image-smoke` builds and loads the subjective worker configuration.
- E2E: `web-e2e-mocked` tests browser flows with API mocks. `real-system-e2e` starts PostgreSQL, the API, workers and grading agent; checks persistence; runs real Playwright; preserves failure evidence; and always tears the stack down.
- `ci-gate` runs with `always()` and fails on failure, cancellation, or unexpected skip.

Jobs only use `needs` for the final aggregate; independent checks therefore run in parallel. CI refactoring must never reduce validation coverage. Local entry points are `npm run ci:workflow-lint`, `npm run ci:web:fast`, `npm run ci:contracts`, and `npm run ci:web:full`.

Initial bootstrap intentionally accepts only the `platform` tenant and `platform_admin` role. Ordinary users must be provisioned through normal user administration flows. For a new migration, create the file, run `npm run sync:schema-version`, then `npm run check:schema-version` and PostgreSQL tests.
