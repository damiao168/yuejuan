# Shared UI showcases

## Exam Workspace

Run `npm run showcase:exam-workspace` to render the deterministic school-admin
Exam Workspace at 1366×768, 1440×900, and 1920×1080. It uses the existing API
mock fixture, attaches the rendered screens to the Playwright HTML report, and
compares them with the checked-in visual snapshots.

Run `npm run showcase:exam-workspace:ui` for the same scenario in Playwright UI
mode. This is the executable shared-UI showcase for the current repository;
Storybook is deliberately not introduced because the project has no existing
Storybook setup and adding its build/runtime dependencies would not improve the
production UI proof supplied by this browser scenario.

These mocked visual checks are a frontend-regression aid, not evidence that the
production API and worker path have been exercised.
