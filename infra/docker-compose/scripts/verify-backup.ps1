param(
  [Parameter(Mandatory=$true)][string]$BackupDirectory,
  [string]$ExpectedEnvironment,
  [string]$ExpectedComposeProject
)

$ErrorActionPreference = "Stop"
$backupRoot = (Resolve-Path -LiteralPath $BackupDirectory).Path
$manifests = @(Get-ChildItem -LiteralPath $backupRoot -File -Filter "manifest-*.json")
if ($manifests.Count -ne 1) { throw "Backup must contain exactly one manifest-*.json file." }
$manifest = Get-Content -LiteralPath $manifests[0].FullName -Encoding utf8 -Raw | ConvertFrom-Json
if (-not $manifest.format_version -or [int]$manifest.format_version -lt 2) { throw "Backup manifest version is unsupported." }
if (-not $manifest.created_at -or -not $manifest.postgres_file -or -not $manifest.minio_directory) { throw "Backup manifest is incomplete." }
if ($ExpectedEnvironment -and $manifest.source_environment -ne $ExpectedEnvironment) { throw "Backup source environment does not match ExpectedEnvironment." }
if ($ExpectedComposeProject -and $manifest.compose_project -ne $ExpectedComposeProject) { throw "Backup Compose project does not match ExpectedComposeProject." }

$listed = @{}
foreach ($entry in @($manifest.files)) {
  $relative = [string]$entry.path
  if ([string]::IsNullOrWhiteSpace($relative) -or [IO.Path]::IsPathRooted($relative) -or $relative.Split([char[]]@('/', '\')) -contains '..') {
    throw "Backup manifest contains an unsafe path."
  }
  $candidate = [IO.Path]::GetFullPath((Join-Path $backupRoot $relative))
  if (-not $candidate.StartsWith($backupRoot + [IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase)) {
    throw "Backup manifest path escapes the backup directory."
  }
  if (-not (Test-Path -LiteralPath $candidate -PathType Leaf)) { throw "Backup file is missing: $relative" }
  $file = Get-Item -LiteralPath $candidate
  if ($file.Length -ne [long]$entry.bytes) { throw "Backup file size mismatch: $relative" }
  $actualHash = (Get-FileHash -LiteralPath $candidate -Algorithm SHA256).Hash.ToLowerInvariant()
  if ($actualHash -ne ([string]$entry.sha256).ToLowerInvariant()) { throw "Backup file hash mismatch: $relative" }
  $listed[$relative.Replace('\', '/')] = $true
}

$postgresRelative = ([string]$manifest.postgres_file).Replace('\', '/')
if (-not $listed.ContainsKey($postgresRelative)) { throw "PostgreSQL dump is not covered by the manifest." }
$minioPrefix = (([string]$manifest.minio_directory).TrimEnd('/', '\') + '/').Replace('\', '/')
$unlisted = @(Get-ChildItem -LiteralPath (Join-Path $backupRoot $manifest.minio_directory) -File -Recurse | Where-Object {
  $relative = $_.FullName.Substring($backupRoot.Length).TrimStart('\', '/').Replace('\', '/')
  -not $listed.ContainsKey($relative)
})
if ($unlisted.Count -gt 0) { throw "MinIO backup contains files not covered by the manifest." }

[ordered]@{
  valid = $true
  manifest = $manifests[0].FullName
  format_version = [int]$manifest.format_version
  source_environment = [string]$manifest.source_environment
  schema_version = [string]$manifest.schema_version
  file_count = $listed.Count
} | ConvertTo-Json -Depth 3
