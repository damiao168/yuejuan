param(
  [switch]$Repair,
  [ValidateRange(1, 1000)][int]$BatchSize = 200,
  [ValidateRange(1, 10000)][int]$ObjectLimit = 1000,
  [string]$StaleAfter = "1h"
)

$ErrorActionPreference = "Stop"
$composeRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
$arguments = @(
  "compose", "--env-file", (Join-Path $composeRoot ".env"),
  "-f", (Join-Path $composeRoot "docker-compose.yml"),
  "run", "--rm", "--no-deps", "--entrypoint", "file-reconcile", "api-gateway",
  "-batch-size", $BatchSize, "-object-limit", $ObjectLimit, "-stale-after", $StaleAfter
)
if ($Repair) {
  $arguments += "-repair"
  Write-Warning "Explicit repair mode enabled. Orphan objects are still report-only and are never deleted."
}
& docker @arguments
if ($LASTEXITCODE -ne 0) {
  throw "File reconciliation failed with exit code $LASTEXITCODE"
}
