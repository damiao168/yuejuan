param(
    [switch]$Build
)

$ErrorActionPreference = "Stop"
$repositoryRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
$composeDirectory = Join-Path $repositoryRoot "infra\docker-compose"
$composeFile = Join-Path $composeDirectory "docker-compose.yml"
$envFile = Join-Path $composeDirectory ".env"
$ocrModelCache = Join-Path $repositoryRoot ".cache\ocr-models"

if (-not (Test-Path -LiteralPath $envFile -PathType Leaf)) {
    throw "Missing $envFile. Copy infra/docker-compose/.env.example to .env and configure it first."
}

# Bind mounts do not reliably create a writable leaf directory on every Docker
# Desktop version, so create the project-local model cache before Compose runs.
New-Item -ItemType Directory -Force -Path $ocrModelCache | Out-Null

$composeArguments = @(
    "compose",
    "--env-file", $envFile,
    "-f", $composeFile,
    "--profile", "*",
    "up", "-d"
)
if ($Build) {
    $composeArguments += "--build"
}

Write-Host "OCR model cache: $ocrModelCache"
if ($Build) {
    Write-Host "Starting services and rebuilding changed images..."
} else {
    Write-Host "Starting services with existing images (use -Build after dependency or source changes)..."
}

& docker @composeArguments
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
