$ErrorActionPreference = "Stop"
$composeFile = Join-Path $PSScriptRoot "..\infra\docker-compose\docker-compose.yml"
# Keep the control plane up, but run one OCR worker and omit the heavy math worker.
docker compose -f $composeFile --profile ocr up -d postgres redis minio api-gateway ocr-worker
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
