param(
  [string]$ComposeFile = "..\docker-compose.yml",
  [string]$EnvFile = "..\.env",
  [switch]$SkipBuild,
  [switch]$SkipSmoke,
  [switch]$BootstrapAdmin,
  [switch]$BaselineExistingMigrations,
  [switch]$EnableOcr,
  [switch]$EnableQuality
)

$ErrorActionPreference = "Stop"
$scriptRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
$composeCandidate = if ([IO.Path]::IsPathRooted($ComposeFile)) { $ComposeFile } else { Join-Path $scriptRoot $ComposeFile }
$envCandidate = if ([IO.Path]::IsPathRooted($EnvFile)) { $EnvFile } else { Join-Path $scriptRoot $EnvFile }
$composePath = (Resolve-Path -LiteralPath $composeCandidate).Path
$envPath = (Resolve-Path -LiteralPath $envCandidate).Path
$composeDir = Split-Path -Parent $composePath

function Invoke-Compose {
  param([string[]]$Arguments)
  & docker compose --env-file $envPath -f $composePath @Arguments
  if ($LASTEXITCODE -ne 0) { throw "docker compose command failed: $($Arguments -join ' ')" }
}

function Wait-ComposeService([string]$Service, [int]$TimeoutSeconds = 180) {
  $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
  do {
    $containerId = (& docker compose --env-file $envPath -f $composePath ps -q $Service).Trim()
    if ($containerId) {
      $state = docker inspect --format '{{.State.Status}}|{{if .State.Health}}{{.State.Health.Status}}{{end}}' $containerId
      if ($LASTEXITCODE -eq 0) {
        $parts = $state.Trim().Split('|', 2)
        if ($parts[0] -eq 'running' -and ($parts.Count -eq 1 -or $parts[1] -eq '' -or $parts[1] -eq 'healthy')) {
          Write-Host "$Service ready."
          return
        }
        if ($parts[0] -eq 'exited' -or $parts[1] -eq 'unhealthy') { throw "$Service failed readiness with state '$state'." }
      }
    }
    Start-Sleep -Seconds 3
  } while ((Get-Date) -lt $deadline)
  throw "Timed out waiting for $Service."
}

Push-Location $composeDir
try {
  & (Join-Path $scriptRoot "preflight.ps1") -ComposeFile $composePath -EnvFile $envPath

  Invoke-Compose -Arguments @("up", "-d", "postgres", "redis", "minio", "qdrant")
  foreach ($service in @("postgres", "redis", "minio", "qdrant")) { Wait-ComposeService $service }

  $migrationArgs = @("--profile", "tools", "run", "--rm")
  if ($BaselineExistingMigrations) { $migrationArgs += @("-e", "EDUGRADE_MIGRATION_BASELINE_EXISTING=true") }
  $migrationArgs += "db-migrate"
  Invoke-Compose -Arguments $migrationArgs
  Invoke-Compose -Arguments @("--profile", "tools", "run", "--rm", "minio-init")

  $appArgs = @("up", "-d")
  if (-not $SkipBuild) { $appArgs += "--build" }
  $appArgs += @("grading-agent", "api-gateway", "web-admin", "nginx")
  Invoke-Compose -Arguments $appArgs
  foreach ($service in @("grading-agent", "api-gateway", "web-admin", "nginx")) { Wait-ComposeService $service 300 }

  if ($BootstrapAdmin) {
    if ([string]::IsNullOrWhiteSpace($env:EDUGRADE_BOOTSTRAP_PASSWORD)) {
      throw "Set EDUGRADE_BOOTSTRAP_PASSWORD in the current shell before using -BootstrapAdmin."
    }
    Invoke-Compose -Arguments @("run", "--rm", "-e", "EDUGRADE_BOOTSTRAP_PASSWORD", "api-gateway", "bootstrap-admin")
  }

  if ($EnableOcr) { Invoke-Compose -Arguments @("--profile", "ocr", "up", "-d", "--build", "ocr-worker") }
  if ($EnableQuality) { Invoke-Compose -Arguments @("--profile", "quality", "up", "-d", "--build", "image-quality-worker") }

  if (-not $SkipSmoke) { & (Join-Path $scriptRoot "smoke-test.ps1") }
  Invoke-Compose -Arguments @("ps")
  Write-Host "EduGrade private deployment initialized."
} finally {
  Pop-Location
}
