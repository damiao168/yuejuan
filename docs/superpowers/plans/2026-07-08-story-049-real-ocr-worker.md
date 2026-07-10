# STORY-049 Real OCR Worker Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the first real OCR worker closure without taking over the future generic worker runtime.

**Architecture:** API Gateway remains the source of truth for auth, tenant isolation, task state, audit, and persistence. A new Python `services/ocr-worker` process polls queued OCR tasks, downloads submission page files through API Gateway, runs a pluggable OCR engine with PaddleOCR support, and writes results back through the existing OCR task APIs.

**Tech Stack:** Go API Gateway, PostgreSQL migrations, Python worker, pytest, optional PaddleOCR adapter, Docker Compose.

---

### Task 1: API Gateway OCR Worker Endpoints

**Files:**
- Modify: `services/api-gateway/internal/ocr/types.go`
- Modify: `services/api-gateway/internal/ocr/handlers.go`
- Modify: `services/api-gateway/internal/ocr/store_memory.go`
- Modify: `services/api-gateway/internal/ocr/store_postgres.go`
- Modify: `services/api-gateway/internal/server/server.go`
- Modify: `services/api-gateway/internal/handlers/handlers.go`
- Create: `services/api-gateway/migrations/000021_story049_ocr_worker_metadata.sql`
- Test: `services/api-gateway/internal/ocr/handlers_test.go`

- [ ] Write failing Go tests for pending task listing, input metadata, bbox validation, and OCR metadata persistence.
- [ ] Run `go test ./internal/ocr` and verify expected failures.
- [ ] Implement store interfaces, handlers, routes, and migration.
- [ ] Run `go test ./internal/ocr`.

### Task 2: Python OCR Worker

**Files:**
- Create: `services/ocr-worker/pyproject.toml`
- Create: `services/ocr-worker/ocr_worker/config.py`
- Create: `services/ocr-worker/ocr_worker/api.py`
- Create: `services/ocr-worker/ocr_worker/engine.py`
- Create: `services/ocr-worker/ocr_worker/runner.py`
- Create: `services/ocr-worker/ocr_worker/__main__.py`
- Test: `services/ocr-worker/tests/test_runner.py`
- Test: `services/ocr-worker/tests/test_api.py`

- [ ] Write failing pytest tests for low confidence, empty OCR, download failure, and auth failure.
- [ ] Run `python -m pytest`.
- [ ] Implement minimal worker modules and optional PaddleOCR adapter.
- [ ] Run `python -m pytest`.

### Task 3: Production Guardrails

**Files:**
- Create: `scripts/check-story049-ocr-worker.mjs`
- Modify: `package.json`
- Modify: `docs/api/ocr.md`
- Create: `docs/evaluation/ocr-sample-set.md`
- Create: `docs/evaluation/ocr-metrics.md`

- [ ] Write static checks proving no hard-coded OCR text, worker exists, docs exist, and system/info no longer lists `ocr_engine_inference` as not implemented.
- [ ] Run the static check and verify it fails before final implementation.
- [ ] Implement missing docs and system capability updates.
- [ ] Run the static check.

### Task 4: Compose And Story Closure

**Files:**
- Modify: `infra/docker-compose/docker-compose.yml`
- Modify: `infra/docker-compose/.env.example`
- Modify: `docs/stories/STORY-049-real-ocr-worker.md`
- Create: `docs/stories/STORY-049-approval.md`
- Modify: `docs/stories/README.md`
- Modify: `docs/deployment/production-readiness-roadmap.md`

- [ ] Add optional `ocr-worker` compose profile.
- [ ] Run compose config verification.
- [ ] Update story implementation, review, fixes, verification, and approval sections.
- [ ] Run final Go, Python, static, and compose checks.
