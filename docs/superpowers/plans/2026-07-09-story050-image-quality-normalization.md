# STORY-050 Image Quality Normalization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the first production-grade answer-sheet image quality gate: immutable quality runs, claim/lease APIs, normalized RGB PNG asset references, and a real Python OpenCV-based quality worker.

**Architecture:** Keep the Go API Gateway as the control plane for tenant auth, quality run state, idempotency, and file metadata. Add a focused Python `services/image-quality-worker` process for image/PDF decode, OpenCV metrics, conservative normalization, and structured result submission. Keep `lab/` disconnected from production.

**Tech Stack:** Go `net/http`, PostgreSQL migrations, in-memory stores for unit tests, Python 3.11, Pillow, OpenCV/NumPy optional runtime imports, pypdfium2 optional PDF support.

---

### Task 1: Backend Quality Run Model and Memory Store

**Files:**
- Create: `services/api-gateway/internal/imagequality/types.go`
- Create: `services/api-gateway/internal/imagequality/store_memory.go`
- Create: `services/api-gateway/internal/imagequality/validation.go`
- Test: `services/api-gateway/internal/imagequality/store_memory_test.go`

- [ ] **Step 1: Write failing store tests**

Create tests that prove:

```go
func TestCreateRunsForCurrentSubmissionPages(t *testing.T) {
  store := imagequality.NewMemoryStore()
  runs, err := store.CreateRuns(ctx, tenantID, submission, pages, imagequality.DefaultProfile(), "actor-1")
  if err != nil { t.Fatal(err) }
  if len(runs) != 1 { t.Fatalf("expected one run") }
  if runs[0].ProcessingStatus != "pending" { t.Fatalf("expected pending") }
  if runs[0].SourceFileAssetID != pages[0].FileAssetID { t.Fatalf("wrong source") }
}

func TestClaimUsesLeaseAndSkipsAlreadyClaimedRuns(t *testing.T) {
  first, _ := store.Claim(ctx, tenantID, imagequality.ClaimInput{WorkerInstanceID: "a", Limit: 1, LeaseSeconds: 300})
  second, _ := store.Claim(ctx, tenantID, imagequality.ClaimInput{WorkerInstanceID: "b", Limit: 1, LeaseSeconds: 300})
  if len(first) != 1 || len(second) != 0 { t.Fatalf("claim lease failed") }
}

func TestCompleteResultIsIdempotentAndRejectsChangedPayload(t *testing.T) {
  completed, err := store.CompleteRun(ctx, tenantID, runID, imagequality.ResultInput{LeaseToken: token, AttemptNo: 1, ResultVersion: "v1", QualityStatus: "passed", NormalizedFileAssetID: "file-normalized"})
  if err != nil { t.Fatal(err) }
  duplicate, err := store.CompleteRun(ctx, tenantID, runID, sameInput)
  if err != nil || duplicate.ID != completed.ID { t.Fatalf("duplicate should be idempotent") }
  _, err = store.CompleteRun(ctx, tenantID, runID, changedInput)
  if !errors.Is(err, imagequality.ErrConflict) { t.Fatalf("expected conflict") }
}
```

- [ ] **Step 2: Run tests and verify RED**

Run:

```powershell
cd services/api-gateway
go test ./internal/imagequality -run Test -count=1
```

Expected: package or symbols missing.

- [ ] **Step 3: Implement minimal memory store**

Implement `Run`, `Profile`, `ClaimInput`, `ClaimedJob`, `ResultInput`, `Store`, errors, allowed statuses, lease token generation, source hash tracking, and idempotent result payload hashing.

- [ ] **Step 4: Run tests and verify GREEN**

Run:

```powershell
cd services/api-gateway
go test ./internal/imagequality -count=1
```

Expected: PASS.

### Task 2: Submission Integration

**Files:**
- Modify: `services/api-gateway/internal/submission/types.go`
- Modify: `services/api-gateway/internal/submission/store_memory.go`
- Modify: `services/api-gateway/internal/submission/store_postgres.go`
- Test: `services/api-gateway/internal/submission/handlers_test.go`

- [ ] **Step 1: Write failing submission tests**

Add tests for:

```go
func TestReplacePageInvalidatesImageQualityPointers(t *testing.T) {
  page := addPage(...)
  markPageQualityPassed(...)
  replaced := replacePage(...)
  if replaced.QualityStatus != "unchecked" { t.Fatalf("quality should reset") }
  if replaced.LatestQualityRunID != "" || replaced.NormalizedFileAssetID != "" { t.Fatalf("quality pointers should clear") }
}
```

- [ ] **Step 2: Run tests and verify RED**

Run:

```powershell
cd services/api-gateway
go test ./internal/submission -run TestReplacePageInvalidatesImageQualityPointers -count=1
```

Expected: fields or behavior missing.

- [ ] **Step 3: Implement page quality fields**

Add `LatestQualityRunID`, `NormalizedFileAssetID`, `QualityStatus`, `QualityOverride` to `SubmissionPage`. Reset these fields on `AddPage` and `ReplacePage`.

- [ ] **Step 4: Run tests and verify GREEN**

Run:

```powershell
cd services/api-gateway
go test ./internal/submission -count=1
```

Expected: PASS.

### Task 3: Quality HTTP APIs

**Files:**
- Create: `services/api-gateway/internal/imagequality/handlers.go`
- Modify: `services/api-gateway/internal/server/server.go`
- Test: `services/api-gateway/internal/imagequality/handlers_test.go`

- [ ] **Step 1: Write failing handler tests**

Add route tests for:

```go
func TestRunQualityCheckCreatesRunsAndClaimOmitsStudentIdentity(t *testing.T) {
  POST /api/v1/submissions/{id}/run-quality-check -> 202 with runs
  POST /api/v1/internal/image-quality/jobs/claim -> 200 with run_id, lease_token, source_file_asset_id
  response body must not contain student_id or candidate_no
}

func TestQualityResultRequiresLeaseAndActivatesPage(t *testing.T) {
  claim run
  POST /api/v1/internal/image-quality/runs/{run_id}/result with lease
  expect page normalized_file_asset_id and quality_status updated
}
```

- [ ] **Step 2: Run tests and verify RED**

Run:

```powershell
cd services/api-gateway
go test ./internal/imagequality -run Test.*Handler -count=1
```

Expected: routes missing.

- [ ] **Step 3: Implement handlers and route wiring**

Add:

```text
POST /api/v1/submissions/{id}/run-quality-check
POST /api/v1/internal/image-quality/jobs/claim
POST /api/v1/internal/image-quality/runs/{run_id}/normalized-assets
POST /api/v1/internal/image-quality/runs/{run_id}/result
```

Use `submission:manage` for user-triggered run creation and `ocr:manage` for first internal worker auth until STORY-051 introduces dedicated service permissions.

- [ ] **Step 4: Run tests and verify GREEN**

Run:

```powershell
cd services/api-gateway
go test ./internal/imagequality ./internal/server -count=1
```

Expected: PASS.

### Task 4: PostgreSQL Migration and Store

**Files:**
- Create: `services/api-gateway/migrations/000022_story050_image_quality_run.sql`
- Create: `services/api-gateway/internal/imagequality/store_postgres.go`
- Test: `services/api-gateway/internal/server/migration_split_test.go`

- [ ] **Step 1: Write failing migration test**

Extend migration split/static tests to require:

```text
submission_page.latest_quality_run_id
submission_page.normalized_file_asset_id
submission_page.quality_status includes review
submission_page_quality_run table
source_to_normalized_matrix stored in normalization_transform JSONB
```

- [ ] **Step 2: Run tests and verify RED**

Run:

```powershell
cd services/api-gateway
go test ./internal/server -run TestMigrations -count=1
```

Expected: migration requirement missing.

- [ ] **Step 3: Implement migration and Postgres store**

Add columns with `IF NOT EXISTS`, create run table, indexes for claim, checks for statuses, and Postgres methods matching memory store behavior.

- [ ] **Step 4: Run tests and verify GREEN**

Run:

```powershell
cd services/api-gateway
go test ./internal/server ./internal/imagequality -count=1
```

Expected: PASS.

### Task 5: Python Image Quality Worker Engine

**Files:**
- Create: `services/image-quality-worker/pyproject.toml`
- Create: `services/image-quality-worker/image_quality/engine.py`
- Create: `services/image-quality-worker/image_quality/schemas.py`
- Test: `services/image-quality-worker/tests/test_engine.py`

- [ ] **Step 1: Write failing engine tests**

Tests:

```python
def test_clear_image_passes_and_outputs_rgb_png(tmp_path):
    image = make_clear_answer_like_image(tmp_path)
    result = analyze_and_normalize(image.read_bytes())
    assert result.quality_status == "passed"
    assert result.quality_report["normalized_color_mode"] == "RGB"
    assert result.normalized_png.startswith(b"\x89PNG")

def test_blurry_image_triggers_low_sharpness_review(tmp_path):
    image = make_blurry_image(tmp_path)
    result = analyze_and_normalize(image.read_bytes())
    assert result.quality_status == "review"
    assert any(issue["code"] == "low_sharpness" for issue in result.quality_issues)

def test_exif_rotation_records_transform(tmp_path):
    image = make_rotated_exif_image(tmp_path)
    result = analyze_and_normalize(image.read_bytes())
    assert result.normalization_transform["exif_rotation_degrees"] in (90, 180, 270)
```

- [ ] **Step 2: Run tests and verify RED**

Run:

```powershell
cd services/image-quality-worker
python -m pytest -q
```

Expected: package missing.

- [ ] **Step 3: Implement minimal engine**

Use Pillow for decode/EXIF/RGB PNG output. Use OpenCV/NumPy when available for Laplacian sharpness and simple brightness/contrast; fall back to Pillow statistics if OpenCV is unavailable.

- [ ] **Step 4: Run tests and verify GREEN**

Run:

```powershell
cd services/image-quality-worker
python -m pytest -q
```

Expected: PASS.

### Task 6: Python Worker API Client and Runner

**Files:**
- Create: `services/image-quality-worker/image_quality/api.py`
- Create: `services/image-quality-worker/image_quality/config.py`
- Create: `services/image-quality-worker/image_quality/runner.py`
- Create: `services/image-quality-worker/image_quality/__main__.py`
- Test: `services/image-quality-worker/tests/test_runner.py`

- [ ] **Step 1: Write failing runner tests**

Use a fake client:

```python
def test_runner_claims_downloads_processes_uploads_and_submits():
    client = FakeClient(job=job, image_bytes=clear_png)
    runner = ImageQualityRunner(client, EngineConfig())
    runner.run_once()
    assert client.upload_requested
    assert client.result_submitted["quality_status"] == "passed"
    assert "student_id" not in json.dumps(client.result_submitted)
```

- [ ] **Step 2: Run tests and verify RED**

Run:

```powershell
cd services/image-quality-worker
python -m pytest -q tests/test_runner.py
```

Expected: runner missing.

- [ ] **Step 3: Implement runner and API client**

Implement login, claim, download, request normalized asset slot, upload, submit result, and retryable failure handling.

- [ ] **Step 4: Run tests and verify GREEN**

Run:

```powershell
cd services/image-quality-worker
python -m pytest -q
```

Expected: PASS.

### Task 7: Deployment and Static Check

**Files:**
- Modify: `infra/docker-compose/docker-compose.yml`
- Create: `services/image-quality-worker/Dockerfile`
- Create: `scripts/check-story050-image-quality.mjs`
- Modify: `package.json`
- Modify: `.env.example`
- Test: `scripts/check-story050-image-quality.mjs`

- [ ] **Step 1: Write failing static check**

Check script must require:

```text
services/image-quality-worker
000022_story050_image_quality_run.sql
POST /api/v1/internal/image-quality/jobs/claim route
image-quality-worker compose service with quality profile
```

- [ ] **Step 2: Run check and verify RED**

Run:

```powershell
npm run check:story050
```

Expected: fails before compose/script wiring exists.

- [ ] **Step 3: Add Dockerfile, compose profile, env vars, npm script**

Add `image-quality-worker` service with profile `quality`, depending on `api-gateway`, and environment values for API login, batch size, lease seconds, profile, and thresholds.

- [ ] **Step 4: Run full verification**

Run:

```powershell
npm run check:story050
cd services/api-gateway; go test ./...
cd ../image-quality-worker; python -m pytest -q
docker compose -f infra/docker-compose/docker-compose.yml --profile quality config
```

Expected: PASS or report exact missing local dependency.

