[CmdletBinding()]
param(
  [string]$ProjectName
)

$ErrorActionPreference = "Stop"

# This runner is intentionally self-contained. It never references the normal
# deployment Compose file, its fixed container names, or its volumes. It
# exercises the OMR page-processing worker only; its disposable fixture
# deliberately fabricates the already-completed capture/registration boundary.
$scriptRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
$composePath = Join-Path (Split-Path -Parent $scriptRoot) "docker-compose.story056-e2e.yml"
if (-not (Test-Path -LiteralPath $composePath)) {
  throw "STORY-056 E2E Compose file was not found: $composePath"
}

if ([string]::IsNullOrWhiteSpace($ProjectName)) {
  $ProjectName = "edugrade-story056-e2e-$PID-$([Guid]::NewGuid().ToString('N').Substring(0, 8))"
}
if ($ProjectName -notmatch '^edugrade-story056-e2e-[a-z0-9-]+$') {
  throw "ProjectName must match ^edugrade-story056-e2e-[a-z0-9-]+$ to protect unrelated Docker resources."
}

function Get-DockerProjectResources {
  param([Parameter(Mandatory = $true)][string]$Name)

  $label = "label=com.docker.compose.project=$Name"
  $commands = @(
    @{ Kind = "container"; Args = @("ps", "-a", "--filter", $label, "--format", "{{.ID}}") },
    @{ Kind = "volume"; Args = @("volume", "ls", "--filter", $label, "--format", "{{.Name}}") },
    @{ Kind = "network"; Args = @("network", "ls", "--filter", $label, "--format", "{{.ID}}") }
  )
  $resources = @()
  foreach ($command in $commands) {
    $items = @(& docker @($command.Args))
    $exitCode = $LASTEXITCODE
    if ($exitCode -ne 0) {
      throw "Unable to inspect existing Docker $($command.Kind) resources for project '$Name' (exit $exitCode)."
    }
    foreach ($item in $items) {
      $item = [string]$item
      if (-not [string]::IsNullOrWhiteSpace($item)) {
        $resources += "$($command.Kind):$($item.Trim())"
      }
    }
  }
  return @($resources)
}

# A supplied project name is safe only when it owns no Docker resources yet.
# This prevents a reuse of a matching prefix from turning `down --volumes` into
# a deletion of an earlier run's containers, networks, or data volumes.
$existingResources = @(Get-DockerProjectResources -Name $ProjectName)
if ($existingResources.Count -gt 0) {
  throw "Refusing to reuse Docker Compose project '$ProjectName'; existing resources were found: $($existingResources -join ', ')"
}
Write-Host "Verified Docker Compose project $ProjectName has no pre-existing resources."

$createdAdminPassword = [string]::IsNullOrWhiteSpace($env:EDUGRADE_E2E_ADMIN_PASSWORD)
$createdWorkerPassword = [string]::IsNullOrWhiteSpace($env:EDUGRADE_E2E_WORKER_PASSWORD)
$createdUnprivilegedPassword = [string]::IsNullOrWhiteSpace($env:EDUGRADE_E2E_UNPRIVILEGED_PASSWORD)
if ($createdAdminPassword) {
  $env:EDUGRADE_E2E_ADMIN_PASSWORD = "S056e2e!A9$([Guid]::NewGuid().ToString('N'))"
}
if ($createdWorkerPassword) {
  $env:EDUGRADE_E2E_WORKER_PASSWORD = "S056e2e!W8$([Guid]::NewGuid().ToString('N'))"
}
if ($createdUnprivilegedPassword) {
  $env:EDUGRADE_E2E_UNPRIVILEGED_PASSWORD = "S056e2e!U7$([Guid]::NewGuid().ToString('N'))"
}

function Invoke-E2ECompose {
  param([Parameter(Mandatory = $true)][string[]]$ComposeArgs)
  & docker compose --project-name $ProjectName --profile e2e -f $composePath @ComposeArgs
  if ($LASTEXITCODE -ne 0) {
    throw "STORY-056 real-worker E2E compose command failed: $($ComposeArgs -join ' ')"
  }
}

function Wait-E2EService {
  param(
    [Parameter(Mandatory = $true)][string]$Service,
    [int]$TimeoutSeconds = 180
  )

  $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
  do {
    $containerId = (& docker compose --project-name $ProjectName --profile e2e -f $composePath ps -q $Service).Trim()
    if ($LASTEXITCODE -eq 0 -and $containerId) {
      $state = (& docker inspect --format '{{.State.Status}}|{{if .State.Health}}{{.State.Health.Status}}{{end}}' $containerId).Trim()
      if ($LASTEXITCODE -eq 0) {
        $parts = $state.Split('|', 2)
        $health = if ($parts.Count -gt 1) { $parts[1] } else { "" }
        if ($parts[0] -eq 'running' -and ($health -eq '' -or $health -eq 'healthy')) {
          Write-Host "$Service ready in isolated project $ProjectName."
          return
        }
        if ($parts[0] -eq 'exited' -or $health -eq 'unhealthy') {
          throw "$Service failed readiness in isolated project $ProjectName with state '$state'."
        }
      }
    }
    Start-Sleep -Seconds 2
  } while ((Get-Date) -lt $deadline)
  throw "Timed out waiting for $Service in isolated project $ProjectName."
}

$projectInvocationStarted = $false
$bodyFailure = $null
$cleanupFailure = $null
try {
  # Once this is true, the preflight above establishes that only this
  # invocation can have created resources under the project label.
  $projectInvocationStarted = $true
  # Keep Compose flags in an array. Passing -d directly to a PowerShell
  # function lets PowerShell consume it as the function's -Debug switch.
  Invoke-E2ECompose -ComposeArgs @("up", "-d", "postgres", "redis", "minio")
  foreach ($service in @('postgres', 'redis', 'minio')) { Wait-E2EService $service }

  # api-gateway owns these one-shot dependencies through depends_on. Running
  # them separately would execute bootstrap-admin twice, and the second pass
  # correctly rejects the already-created administrator.
  Invoke-E2ECompose -ComposeArgs @("up", "-d", "--build", "api-gateway")
  Wait-E2EService api-gateway 300

  Invoke-E2ECompose -ComposeArgs @("run", "--rm", "--no-deps", "--build", "postgres-e2e-tests")
  Invoke-E2ECompose -ComposeArgs @("run", "--rm", "--no-deps", "--build", "e2e-runner", "--phase", "seed")
  Invoke-E2ECompose -ComposeArgs @("run", "--rm", "--no-deps", "--build", "page-processing-once")
  Invoke-E2ECompose -ComposeArgs @("run", "--rm", "--no-deps", "e2e-runner", "--phase", "verify")
  Write-Host "STORY-056 PostgreSQL and OMR page-processing Worker E2E passed in isolated project $ProjectName (capture/registration are deliberate direct-SQL fixture data)."
}
catch {
  $bodyFailure = $_
}
finally {
  # The project name is validated and confirmed empty before the first Compose
  # command. Thus this can remove only resources created by this invocation.
  if ($projectInvocationStarted) {
    try {
      & docker compose --project-name $ProjectName --profile e2e -f $composePath down --volumes --remove-orphans
      $cleanupExitCode = $LASTEXITCODE
      if ($cleanupExitCode -ne 0) {
        $cleanupFailure = "STORY-056 E2E cleanup failed for verified project '$ProjectName' (exit $cleanupExitCode)."
      }
    }
    catch {
      $cleanupFailure = "STORY-056 E2E cleanup failed for verified project '$ProjectName': $($_.Exception.Message)"
    }
    if ($cleanupFailure) {
      [Console]::Error.WriteLine($cleanupFailure)
    }
  }
  if ($createdAdminPassword) { Remove-Item Env:EDUGRADE_E2E_ADMIN_PASSWORD -ErrorAction SilentlyContinue }
  if ($createdWorkerPassword) { Remove-Item Env:EDUGRADE_E2E_WORKER_PASSWORD -ErrorAction SilentlyContinue }
  if ($createdUnprivilegedPassword) { Remove-Item Env:EDUGRADE_E2E_UNPRIVILEGED_PASSWORD -ErrorAction SilentlyContinue }
}

if ($bodyFailure -and $cleanupFailure) {
  throw "STORY-056 real-worker E2E failed: $($bodyFailure.Exception.Message) Cleanup also failed: $cleanupFailure"
}
if ($bodyFailure) {
  throw $bodyFailure
}
if ($cleanupFailure) {
  throw $cleanupFailure
}
