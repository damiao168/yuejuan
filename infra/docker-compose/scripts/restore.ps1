param(
  [Parameter(Mandatory=$true)][string]$PostgresDump,
  [Parameter(Mandatory=$true)][string]$TargetDatabase,
  [string]$ComposeFile = "..\docker-compose.yml",
  [string]$EnvFile = "..\.env",
  [string]$MinioBackupDirectory,
  [switch]$CreateTargetDatabase,
  [switch]$AllowPrimaryDatabase,
  [switch]$ConfirmRestore
)

$ErrorActionPreference = "Stop"
$scriptRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
$composePath = Resolve-Path -Path (Join-Path $scriptRoot $ComposeFile)
$envPath = Resolve-Path -Path (Join-Path $scriptRoot $EnvFile)
$dumpPath = Resolve-Path -Path $PostgresDump
$composeDir = Split-Path -Parent $composePath

if (-not $ConfirmRestore) { throw "Restore requires -ConfirmRestore." }
$envValues = @{}
Get-Content -LiteralPath $envPath | ForEach-Object {
  $line = $_.Trim()
  if ($line -and -not $line.StartsWith("#") -and $line.Contains("=")) {
    $parts = $line.Split("=", 2)
    $envValues[$parts[0].Trim()] = $parts[1].Trim().Trim('"').Trim("'")
  }
}
$primaryDatabase = $envValues["EDUGRADE_POSTGRES_DB"]
if ($TargetDatabase -eq $primaryDatabase -and -not $AllowPrimaryDatabase) {
  throw "Refusing to overwrite the primary database without -AllowPrimaryDatabase. Prefer an isolated verification database."
}
if ($TargetDatabase -notmatch '^[A-Za-z0-9_]+$') { throw "TargetDatabase contains unsupported characters." }

Push-Location $composeDir
try {
  $remoteDump = "/tmp/restore-$([guid]::NewGuid().ToString('N')).dump"
  docker cp $dumpPath "edugrade-postgres:$remoteDump"
  if ($LASTEXITCODE -ne 0) { throw "Copying restore dump failed." }
  docker compose --env-file $envPath -f $composePath exec -T postgres pg_restore --list $remoteDump | Out-Null
  if ($LASTEXITCODE -ne 0) { throw "PostgreSQL dump validation failed." }
  if ($CreateTargetDatabase) {
    docker compose --env-file $envPath -f $composePath exec -T postgres sh -ec "dropdb -U `"`$POSTGRES_USER`" --if-exists '$TargetDatabase'; createdb -U `"`$POSTGRES_USER`" '$TargetDatabase'"
    if ($LASTEXITCODE -ne 0) { throw "Creating target database failed." }
  }
  docker compose --env-file $envPath -f $composePath exec -T postgres sh -ec "pg_restore -U `"`$POSTGRES_USER`" -d '$TargetDatabase' --clean --if-exists '$remoteDump'"
  if ($LASTEXITCODE -ne 0) { throw "PostgreSQL restore failed." }
  docker compose --env-file $envPath -f $composePath exec -T postgres rm -f $remoteDump

  if ($MinioBackupDirectory) {
    $minioPath = (Resolve-Path -LiteralPath $MinioBackupDirectory).Path
    $backupMount = Split-Path -Parent $minioPath
    $backupName = Split-Path -Leaf $minioPath
    $minioScript = 'mc alias set edugrade http://minio:9000 "$EDUGRADE_MINIO_ACCESS_KEY" "$EDUGRADE_MINIO_SECRET_KEY"; mc mirror --overwrite "/backup/{0}" "edugrade/$EDUGRADE_FILE_BUCKET"' -f $backupName
    & docker compose --env-file $envPath -f $composePath --profile tools run --rm -v "${backupMount}:/backup:ro" --entrypoint /bin/sh minio-init -ec $minioScript
    if ($LASTEXITCODE -ne 0) { throw "MinIO restore failed." }
  }
  Write-Host "Restore completed into database '$TargetDatabase'."
} finally {
  Pop-Location
}
