# STORY-048 Approval

## Result

Approved.

## Scope Closed

- Production Web Admin navigation now hides mock routes by default.
- Direct hash access uses the same production visibility rules.
- Dashboard no longer uses static demo data and now aggregates real API responses.
- Demo/development mock routes remain available through explicit environment visibility and retain MockBadge labeling.
- `lab/` remains outside the production chain.

## Verification

```powershell
npm.cmd --workspace apps/web-admin run check:production-routes
```

Passed with `Production route checks passed.`

```powershell
npm.cmd --workspace apps/web-admin run typecheck
```

Passed with `tsc --noEmit`.

```powershell
npm.cmd run typecheck
```

Passed for all workspaces with typecheck scripts.

```powershell
npm.cmd --workspace apps/web-admin run build
```

Passed. Vite reported that the main JS chunk is larger than 500 kB after minification; this is a frontend performance follow-up and does not block STORY-048 approval.

## Residual Risk

- Dashboard currently aggregates several existing APIs in the browser. If production data volume grows, a backend dashboard aggregate endpoint may be needed.
- The Web Admin production bundle still needs future code splitting because Vite reports a chunk larger than 500 kB.
- Quality, Permissions, Settings, OCR worker, subjective AI adapter, and Agent worker runtime remain future stories.
