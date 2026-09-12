param(
  [string]$ComposeFile = "..\docker-compose.yml",
  [string]$EnvFile = "..\.env",
  [switch]$RequireApplicationBaseImages
)

$ErrorActionPreference = "Stop"
$scriptRoot = Split-Path -Parent $MyInvocation.MyCommand.Path

function Resolve-DeploymentPath([string]$Path) {
  $candidate = if ([IO.Path]::IsPathRooted($Path)) { $Path } else { Join-Path $scriptRoot $Path }
  return (Resolve-Path -LiteralPath $candidate).Path
}

function Read-EnvFile([string]$Path) {
  $values = @{}
  foreach ($line in Get-Content -LiteralPath $Path -Encoding utf8) {
    $trimmed = $line.Trim()
    if ($trimmed -eq "" -or $trimmed.StartsWith("#")) { continue }
    $parts = $trimmed.Split("=", 2)
    if ($parts.Count -ne 2) { throw "Invalid env line for key parsing." }
    $values[$parts[0].Trim()] = $parts[1].Trim().Trim('"').Trim("'")
  }
  return $values
}

$composePath = Resolve-DeploymentPath $ComposeFile
$envPath = Resolve-DeploymentPath $EnvFile
$composeDir = Split-Path -Parent $composePath
$envValues = Read-EnvFile $envPath

$requiredKeys = @(
  "EDUGRADE_ENV",
  "EDUGRADE_SESSION_COOKIE_SECURE",
  "EDUGRADE_CORS_ALLOWED_ORIGINS",
  "EDUGRADE_POSTGRES_DB",
  "EDUGRADE_POSTGRES_USER",
  "EDUGRADE_POSTGRES_PASSWORD",
  "EDUGRADE_POSTGRES_APP_USER",
  "EDUGRADE_POSTGRES_APP_PASSWORD",
  "EDUGRADE_POSTGRES_ADMIN_DSN",
  "EDUGRADE_POSTGRES_DSN",
  "EDUGRADE_REDIS_PASSWORD",
  "EDUGRADE_MINIO_ACCESS_KEY",
  "EDUGRADE_MINIO_SECRET_KEY",
  "EDUGRADE_FILE_BUCKET",
  "EDUGRADE_BARCODE_HMAC_KEYS",
  "EDUGRADE_QDRANT_API_KEY",
  "EDUGRADE_AI_SERVICE_URL",
  "EDUGRADE_AI_SERVICE_TOKEN",
  "EDUGRADE_AI_MODEL_VERSION",
  "EDUGRADE_AI_PROMPT_VERSION",
  "EDUGRADE_GRADING_MODEL_BASE_URL",
  "EDUGRADE_GRADING_MODEL_API_KEY"
)
foreach ($key in $requiredKeys) {
  if (-not $envValues.ContainsKey($key) -or [string]::IsNullOrWhiteSpace($envValues[$key])) {
    throw "Required deployment setting is missing: $key"
  }
}
if ($envValues["EDUGRADE_AI_SERVICE_TOKEN"].Length -lt 32) {
  throw "EDUGRADE_AI_SERVICE_TOKEN must contain at least 32 characters."
}
if ($envValues["EDUGRADE_QDRANT_API_KEY"].Length -lt 32) {
  throw "EDUGRADE_QDRANT_API_KEY must contain at least 32 characters."
}

$localEnvironments = @("local", "development", "dev", "test")
$environment = $envValues["EDUGRADE_ENV"].ToLowerInvariant()
$productionLike = $localEnvironments -notcontains $environment
if ($productionLike) {
  $problems = @()
  if ($envValues["EDUGRADE_SESSION_COOKIE_SECURE"] -ne "true") { $problems += "secure session cookie is required" }
  if ($envValues["EDUGRADE_POSTGRES_PASSWORD"] -match "change_me|edugrade_dev") { $problems += "PostgreSQL example password must be replaced" }
  if ($envValues["EDUGRADE_POSTGRES_APP_PASSWORD"] -match "change_me|edugrade_dev" -or $envValues["EDUGRADE_POSTGRES_APP_PASSWORD"].Length -lt 32) { $problems += "PostgreSQL application password must be replaced and contain at least 32 characters" }
  if ($envValues["EDUGRADE_POSTGRES_APP_USER"] -eq $envValues["EDUGRADE_POSTGRES_USER"]) { $problems += "PostgreSQL migration and application users must be different" }
  if ($envValues["EDUGRADE_POSTGRES_ADMIN_DSN"] -eq $envValues["EDUGRADE_POSTGRES_DSN"]) { $problems += "PostgreSQL admin and application DSNs must be different" }
  if ($envValues["EDUGRADE_POSTGRES_ADMIN_DSN"] -match "sslmode=disable|change_me|edugrade_dev") { $problems += "PostgreSQL admin DSN is not production-safe" }
  if ($envValues["EDUGRADE_POSTGRES_DSN"] -match "sslmode=disable|change_me|edugrade_dev") { $problems += "PostgreSQL DSN is not production-safe" }
  if ($envValues["EDUGRADE_REDIS_PASSWORD"] -match "change_me|edugrade_dev") { $problems += "Redis example password must be replaced" }
  if ($envValues["EDUGRADE_MINIO_ACCESS_KEY"] -eq "edugrade" -or $envValues["EDUGRADE_MINIO_SECRET_KEY"] -match "change_me|edugrade_dev") { $problems += "MinIO example credentials must be replaced" }
  if ($envValues["EDUGRADE_CORS_ALLOWED_ORIGINS"] -match "localhost|127\.0\.0\.1") { $problems += "local CORS origins are not allowed" }
  if ($envValues["EDUGRADE_QDRANT_API_KEY"] -match "change_me") { $problems += "Qdrant example API key must be replaced" }
  if ($envValues["EDUGRADE_AI_SERVICE_TOKEN"] -match "replace_with|change_me") { $problems += "AI service example token must be replaced" }
  if ($envValues["EDUGRADE_GRADING_MODEL_API_KEY"] -match "replace_with|change_me") { $problems += "grading model example API key must be replaced" }
  if ($envValues["EDUGRADE_BARCODE_HMAC_KEYS"] -match "replace_with|change_me") { $problems += "barcode example HMAC key must be replaced" }
  if ($envValues["EDUGRADE_GRAFANA_ADMIN_PASSWORD"] -match "change_me" -or $envValues["EDUGRADE_GRAFANA_ADMIN_PASSWORD"] -eq "admin") { $problems += "Grafana example password must be replaced" }
  if ($envValues.ContainsKey("EDUGRADE_INTERNAL_BIND_HOST") -and $envValues["EDUGRADE_INTERNAL_BIND_HOST"] -eq "0.0.0.0") { $problems += "internal service ports must not bind to 0.0.0.0 in production" }
  if (-not $envValues.ContainsKey("EDUGRADE_POSTGRES_TENANT_RLS") -or $envValues["EDUGRADE_POSTGRES_TENANT_RLS"] -ne "true") { $problems += "PostgreSQL tenant RLS must be enabled" }

  $immutableImageKeys = @(
    "EDUGRADE_POSTGRES_IMAGE",
    "EDUGRADE_REDIS_IMAGE",
    "EDUGRADE_MINIO_IMAGE",
    "EDUGRADE_MINIO_MC_IMAGE",
    "EDUGRADE_QDRANT_IMAGE",
    "EDUGRADE_API_GATEWAY_IMAGE",
    "EDUGRADE_WEB_ADMIN_IMAGE",
    "EDUGRADE_GRADING_AGENT_IMAGE",
    "EDUGRADE_OCR_WORKER_IMAGE",
    "EDUGRADE_IMAGE_QUALITY_WORKER_IMAGE",
    "EDUGRADE_SUBJECTIVE_GRADING_WORKER_IMAGE",
    "EDUGRADE_MATH_VERIFICATION_WORKER_IMAGE",
    "EDUGRADE_PAGE_PROCESSING_WORKER_IMAGE",
    "EDUGRADE_NGINX_IMAGE",
    "EDUGRADE_PROMETHEUS_IMAGE",
    "EDUGRADE_GRAFANA_IMAGE",
    "EDUGRADE_GO_BUILD_IMAGE",
    "EDUGRADE_API_RUNTIME_IMAGE",
    "EDUGRADE_NODE_BUILD_IMAGE",
    "EDUGRADE_WEB_RUNTIME_IMAGE",
    "EDUGRADE_AI_PYTHON_IMAGE",
    "EDUGRADE_OCR_PYTHON_IMAGE",
    "EDUGRADE_QUALITY_PYTHON_IMAGE",
    "EDUGRADE_SUBJECTIVE_PYTHON_IMAGE",
    "EDUGRADE_MATH_PYTHON_IMAGE",
    "EDUGRADE_PAGE_PROCESSING_PYTHON_IMAGE"
  )
  foreach ($key in $immutableImageKeys) {
    if (-not $envValues.ContainsKey($key) -or $envValues[$key] -notmatch "^[^\s@]+@sha256:[0-9a-fA-F]{64}$") {
      $problems += "$key must be pinned to an immutable image digest"
    }
  }
  if ($envValues.ContainsKey("EDUGRADE_PAPER_FORMULA_WORKER_IMAGE") -and
      -not [string]::IsNullOrWhiteSpace($envValues["EDUGRADE_PAPER_FORMULA_WORKER_IMAGE"]) -and
      $envValues["EDUGRADE_PAPER_FORMULA_WORKER_IMAGE"] -notmatch "^[^\s@]+@sha256:[0-9a-fA-F]{64}$") {
    $problems += "EDUGRADE_PAPER_FORMULA_WORKER_IMAGE must be empty or pinned to an immutable image digest"
  }
  if ($problems.Count -gt 0) { throw "Production preflight rejected unsafe configuration: $($problems -join '; ')" }
} elseif (
  $envValues["EDUGRADE_POSTGRES_PASSWORD"] -match "change_me" -or
  $envValues["EDUGRADE_POSTGRES_APP_PASSWORD"] -match "change_me" -or
  $envValues["EDUGRADE_REDIS_PASSWORD"] -match "change_me" -or
  $envValues["EDUGRADE_MINIO_SECRET_KEY"] -match "change_me"
) {
  Write-Warning "Local example credentials are still configured. Use only on an isolated development host."
}

docker version --format "Docker server {{.Server.Version}}" | Out-Host
if ($LASTEXITCODE -ne 0) { throw "Docker daemon is not available." }
docker compose version | Out-Host
if ($LASTEXITCODE -ne 0) { throw "Docker Compose is not available." }

Push-Location $composeDir
try {
  docker compose --env-file $envPath -f $composePath --profile tools --profile ocr --profile quality --profile processing --profile observability config --quiet
  if ($LASTEXITCODE -ne 0) { throw "Docker Compose configuration is invalid." }
} finally {
  Pop-Location
}

$baseImages = @(
  $envValues["EDUGRADE_GO_BUILD_IMAGE"],
  $envValues["EDUGRADE_API_RUNTIME_IMAGE"],
  $envValues["EDUGRADE_NODE_BUILD_IMAGE"],
  $envValues["EDUGRADE_WEB_RUNTIME_IMAGE"],
  $envValues["EDUGRADE_AI_PYTHON_IMAGE"],
  $envValues["EDUGRADE_OCR_PYTHON_IMAGE"],
  $envValues["EDUGRADE_QUALITY_PYTHON_IMAGE"],
  $envValues["EDUGRADE_SUBJECTIVE_PYTHON_IMAGE"],
  $envValues["EDUGRADE_MATH_PYTHON_IMAGE"],
  $envValues["EDUGRADE_PAGE_PROCESSING_PYTHON_IMAGE"]
) | Where-Object { -not [string]::IsNullOrWhiteSpace($_) } | Select-Object -Unique
$missingImages = @()
foreach ($image in $baseImages) {
  $previousErrorAction = $ErrorActionPreference
  $ErrorActionPreference = "SilentlyContinue"
  docker image inspect $image *> $null
  $inspectExitCode = $LASTEXITCODE
  $ErrorActionPreference = $previousErrorAction
  if ($inspectExitCode -ne 0) { $missingImages += $image }
}
if ($missingImages.Count -gt 0) {
  $message = "Application base images are not cached: $($missingImages -join ', '). Pull them, configure the EDUGRADE_*_IMAGE mirror variables, or import them with docker load."
  if ($RequireApplicationBaseImages) { throw $message }
  Write-Warning $message
}

Write-Host "EduGrade deployment preflight passed for environment '$environment'."
