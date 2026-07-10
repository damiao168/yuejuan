param(
  [string]$ComposeFile = "..\docker-compose.yml",
  [string]$EnvFile = "..\.env",
  [string]$OutputDir = "..\backups"
)

$ErrorActionPreference = "Stop"
$scriptRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
$composePath = (Resolve-Path -LiteralPath (Join-Path $scriptRoot $ComposeFile)).Path
$envPath = (Resolve-Path -LiteralPath (Join-Path $scriptRoot $EnvFile)).Path
$stamp = Get-Date -Format "yyyyMMdd-HHmmss"
$backupRoot = Join-Path $scriptRoot $OutputDir
$backupDir = Join-Path $backupRoot "edugrade-$stamp"
$backupDir = (New-Item -ItemType Directory -Force -Path $backupDir).FullName
$composeDir = Split-Path -Parent $composePath

Push-Location $composeDir
try {
  $postgresFile = "postgres-$stamp.dump"
  $postgresPath = Join-Path $backupDir $postgresFile
  $remoteDump = "/tmp/$postgresFile"
  docker compose --env-file $envPath -f $composePath exec -T postgres sh -ec "pg_dump -U `"`$POSTGRES_USER`" -d `"`$POSTGRES_DB`" --format=custom --file=$remoteDump"
  if ($LASTEXITCODE -ne 0) { throw "PostgreSQL backup failed." }
  docker compose --env-file $envPath -f $composePath exec -T postgres pg_restore --list $remoteDump | Out-Null
  if ($LASTEXITCODE -ne 0) { throw "PostgreSQL backup verification failed." }
  docker cp "edugrade-postgres:$remoteDump" $postgresPath
  if ($LASTEXITCODE -ne 0) { throw "Copying PostgreSQL backup failed." }
  docker compose --env-file $envPath -f $composePath exec -T postgres rm -f $remoteDump
  if (-not (Test-Path $postgresPath) -or (Get-Item $postgresPath).Length -eq 0) { throw "PostgreSQL backup is empty." }

  New-Item -ItemType Directory -Force -Path (Join-Path $backupDir "minio-$stamp") | Out-Null
  $backupMount = (Resolve-Path $backupDir).Path
  $minioScript = 'mc alias set edugrade http://minio:9000 "$EDUGRADE_MINIO_ACCESS_KEY" "$EDUGRADE_MINIO_SECRET_KEY"; mc mirror --overwrite "edugrade/$EDUGRADE_FILE_BUCKET" "/backup/minio-{0}"' -f $stamp
  & docker compose --env-file $envPath -f $composePath --profile tools run --rm -v "${backupMount}:/backup" --entrypoint /bin/sh minio-init -ec $minioScript
  if ($LASTEXITCODE -ne 0) { throw "MinIO backup failed." }

  $files = Get-ChildItem -Path $backupDir -File -Recurse | Where-Object { $_.Name -ne "manifest-$stamp.json" } | ForEach-Object {
    [ordered]@{
      path = $_.FullName.Substring($backupDir.Length).TrimStart('\', '/')
      bytes = $_.Length
      sha256 = (Get-FileHash -Algorithm SHA256 -LiteralPath $_.FullName).Hash.ToLowerInvariant()
    }
  }
  $images = @()
  $containerIds = @(docker compose --env-file $envPath -f $composePath ps -q)
  foreach ($containerId in $containerIds) {
    $imageEvidence = docker inspect --format '{{.Name}}|{{.Config.Image}}|{{.Image}}' $containerId
    if ($LASTEXITCODE -ne 0) { throw "Reading image evidence failed." }
    $parts = $imageEvidence.Trim().Split('|', 3)
    $images += [ordered]@{ container = $parts[0].TrimStart('/'); image = $parts[1]; id = $parts[2] }
  }
  $postgresUser = (docker exec edugrade-postgres printenv POSTGRES_USER).Trim()
  $postgresDatabase = (docker exec edugrade-postgres printenv POSTGRES_DB).Trim()
  $migrationCount = docker exec edugrade-postgres psql -U $postgresUser -d $postgresDatabase -Atc "SELECT count(*) FROM schema_migration"
  if ($LASTEXITCODE -ne 0) { throw "Reading migration evidence failed." }
  $manifest = [ordered]@{
    created_at = (Get-Date).ToUniversalTime().ToString("o")
    compose_project = "edugrade-enterprise"
    applied_migration_count = [int]$migrationCount
    postgres_file = $postgresFile
    minio_directory = "minio-$stamp"
    images = $images
    files = @($files)
  }
  $manifest | ConvertTo-Json -Depth 6 | Set-Content -Encoding UTF8 -LiteralPath (Join-Path $backupDir "manifest-$stamp.json")
  Write-Host "Backup written to $backupDir."
} finally {
  Pop-Location
}
