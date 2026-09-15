param(
  [Parameter(Mandatory=$true)][string]$PostgresDump,
  [Parameter(Mandatory=$true)][string]$TargetDatabase,
  [string]$ComposeFile = "..\docker-compose.yml",
  [string]$EnvFile = "..\.env",
  [string]$MinioBackupDirectory,
  [string]$TargetBucket,
  [switch]$CreateTargetDatabase,
  [switch]$AllowPrimaryDatabase,
  [switch]$AllowPrimaryBucket,
  [switch]$AllowProductionRestore,
  [switch]$ConfirmRestore
)

$ErrorActionPreference = "Stop"
$scriptRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
$composeCandidate = if ([IO.Path]::IsPathRooted($ComposeFile)) { $ComposeFile } else { Join-Path $scriptRoot $ComposeFile }
$envCandidate = if ([IO.Path]::IsPathRooted($EnvFile)) { $EnvFile } else { Join-Path $scriptRoot $EnvFile }
$composePath = (Resolve-Path -LiteralPath $composeCandidate).Path
$envPath = (Resolve-Path -LiteralPath $envCandidate).Path
$dumpPath = (Resolve-Path -LiteralPath $PostgresDump).Path
if (-not (Test-Path -LiteralPath $dumpPath -PathType Leaf)) { throw "PostgresDump must reference a dump file." }
$composeDir = Split-Path -Parent $composePath

$backupDirectory = Split-Path -Parent $dumpPath
& (Join-Path $scriptRoot "verify-backup.ps1") -BackupDirectory $backupDirectory | Out-Null
if (-not $?) { throw "Backup manifest verification failed." }

if (-not $ConfirmRestore) { throw "Restore requires -ConfirmRestore." }
$envValues = @{}
Get-Content -LiteralPath $envPath -Encoding utf8 | ForEach-Object {
  $line = $_.Trim()
  if ($line -and -not $line.StartsWith("#") -and $line.Contains("=")) {
    $parts = $line.Split("=", 2)
    $envValues[$parts[0].Trim()] = $parts[1].Trim().Trim('"').Trim("'")
  }
}
$primaryDatabase = $envValues["EDUGRADE_POSTGRES_DB"]
$postgresUser = $envValues["EDUGRADE_POSTGRES_USER"]
$targetEnvironment = $envValues["EDUGRADE_ENV"]
if ($targetEnvironment -in @("production", "prod") -and -not $AllowProductionRestore) {
  throw "Refusing to restore into a production environment without -AllowProductionRestore."
}
if ($TargetDatabase -eq $primaryDatabase -and -not $AllowPrimaryDatabase) {
  throw "Refusing to overwrite the primary database without -AllowPrimaryDatabase. Prefer an isolated verification database."
}
if ($TargetDatabase -notmatch '^[A-Za-z0-9_]+$') { throw "TargetDatabase contains unsupported characters." }
$primaryBucket = $envValues["EDUGRADE_FILE_BUCKET"]
if (-not $TargetBucket) { $TargetBucket = $primaryBucket }
if ($TargetBucket -notmatch '^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$') { throw "TargetBucket is not a valid S3 bucket name." }
if ($MinioBackupDirectory -and $TargetBucket -eq $primaryBucket -and -not $AllowPrimaryBucket) {
  throw "Refusing to overwrite the primary MinIO bucket without -AllowPrimaryBucket. Prefer an isolated verification bucket."
}

Push-Location $composeDir
try {
  $postgresContainerIds = @(
    @(& docker compose --env-file $envPath -f $composePath ps -q postgres) |
      Where-Object { -not [string]::IsNullOrWhiteSpace($_) }
  )
  if ($LASTEXITCODE -ne 0 -or $postgresContainerIds.Count -ne 1) {
    throw "Expected exactly one running PostgreSQL container for this Compose deployment."
  }
  $postgresContainerId = $postgresContainerIds[0].Trim()
  $remoteDump = "/tmp/restore-$([guid]::NewGuid().ToString('N')).dump"
  $remoteDumpCopied = $false
  try {
    docker cp $dumpPath "${postgresContainerId}:$remoteDump"
    $remoteDumpCopied = $true
    if ($LASTEXITCODE -ne 0) { throw "Copying restore dump failed." }
    docker compose --env-file $envPath -f $composePath exec -T postgres pg_restore --list $remoteDump | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "PostgreSQL dump validation failed." }
    if ($CreateTargetDatabase) {
      docker compose --env-file $envPath -f $composePath exec -T postgres sh -ec "dropdb -U `"`$POSTGRES_USER`" --if-exists '$TargetDatabase'; createdb -U `"`$POSTGRES_USER`" '$TargetDatabase'"
      if ($LASTEXITCODE -ne 0) { throw "Creating target database failed." }
    }
    docker compose --env-file $envPath -f $composePath exec -T postgres sh -ec "pg_restore -U `"`$POSTGRES_USER`" -d '$TargetDatabase' --clean --if-exists '$remoteDump'"
    if ($LASTEXITCODE -ne 0) { throw "PostgreSQL restore failed." }
  } finally {
    if ($remoteDumpCopied) {
      docker compose --env-file $envPath -f $composePath exec -T postgres rm -f $remoteDump
      if ($LASTEXITCODE -ne 0) { Write-Warning "Unable to remove temporary PostgreSQL dump $remoteDump from the container." }
    }
  }

  if ($MinioBackupDirectory) {
    $minioPath = (Resolve-Path -LiteralPath $MinioBackupDirectory).Path
    $backupMount = Split-Path -Parent $minioPath
    $backupName = Split-Path -Leaf $minioPath
    $minioScript = 'scheme=http; if [ "$EDUGRADE_MINIO_USE_SSL" = "true" ]; then scheme=https; fi; mc alias set edugrade "$scheme://minio:9000" "$EDUGRADE_MINIO_ROOT_USER" "$EDUGRADE_MINIO_ROOT_PASSWORD"; mc mb --ignore-existing "edugrade/{1}"; mc mirror --overwrite "/backup/{0}" "edugrade/{1}"' -f $backupName, $TargetBucket
    & docker compose --env-file $envPath -f $composePath --profile tools run --rm -v "${backupMount}:/backup:ro" --entrypoint /bin/sh minio-init -ec $minioScript
    if ($LASTEXITCODE -ne 0) { throw "MinIO restore failed." }
  }
  $schemaCount = docker compose --env-file $envPath -f $composePath exec -T postgres psql -U $postgresUser -d $TargetDatabase -Atc "SELECT count(*) FROM schema_migration"
  if ($LASTEXITCODE -ne 0 -or [int]$schemaCount -le 0) { throw "Restored database schema history validation failed." }
  Write-Host "Restore completed into database '$TargetDatabase'."
} finally {
  Pop-Location
}
