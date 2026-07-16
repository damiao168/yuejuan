param(
    [Parameter(Mandatory = $true)][string]$Candidate
)
$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"
$LabRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$Registry = Get-Content -Raw (Join-Path $LabRoot "config/model-candidates.json") | ConvertFrom-Json
$Entry = $Registry.candidates | Where-Object { $_.candidate_id -eq $Candidate }
if (-not $Entry) { throw "unknown model candidate: $Candidate" }
$ModelPath = Join-Path $LabRoot $Entry.model_path
New-Item -ItemType Directory -Force (Split-Path $ModelPath) | Out-Null
if (-not (Test-Path $ModelPath) -or (Get-Item $ModelPath).Length -ne $Entry.expected_bytes) {
    & curl.exe -L --fail --retry 5 --retry-delay 2 -C - --output $ModelPath $Entry.url
    if ($LASTEXITCODE -ne 0) { throw "model candidate download failed: $Candidate" }
}
if ((Get-Item $ModelPath).Length -ne $Entry.expected_bytes) { throw "model candidate size mismatch: $Candidate" }
$Hash = (Get-FileHash -Algorithm SHA256 $ModelPath).Hash.ToLowerInvariant()
if ($Hash -ne $Entry.expected_sha256) { throw "model candidate SHA256 mismatch: $Candidate" }
Write-Output "Prepared $Candidate at $ModelPath"
