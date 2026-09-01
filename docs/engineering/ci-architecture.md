# CI architecture

CI is organized as independent layers so one failure does not hide unrelated failures:

- Static: `web-static`, `go-static`, `python-static`.
- Unit: `web-unit`, `go-unit`, `go-race`, and `python-tests` matrix entries.
- Contracts and acceptance: `story-gates` (fail-fast disabled), `lab`.
- Database/integration: `go-postgres`, `go-boundary`, `compose-config`.
- E2E and security: `web-e2e-mocked`, `real-system-e2e`, `supply-chain`.
- `ci-gate` runs with `always()` and fails on failure, cancellation, or unexpected skip.

Jobs only use `needs` for the final aggregate; independent checks therefore run in parallel. Local entry points are `npm run ci:fast`, `npm run ci:contracts`, and `npm run ci:full`.

Initial bootstrap intentionally accepts only the `platform` tenant and `platform_admin` role. Ordinary users must be provisioned through normal user administration flows. For a new migration, create the file, run `npm run sync:schema-version`, then `npm run check:schema-version` and PostgreSQL tests.
