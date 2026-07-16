$ErrorActionPreference = "Stop"
$LabRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$PidFile = Join-Path $LabRoot ".runtime/llama-server.pid"
$CandidateFile = Join-Path $LabRoot ".runtime/llama-server.candidate"
if (-not (Test-Path $PidFile)) {
    Write-Output "llama-server is not recorded as running"
    exit 0
}
$ServerPid = [int](Get-Content -Raw $PidFile)
$Process = Get-Process -Id $ServerPid -ErrorAction SilentlyContinue
if ($Process) {
    $ExpectedRoot = (Resolve-Path (Join-Path $LabRoot ".runtime")).Path
    if (-not $Process.Path.StartsWith($ExpectedRoot, [System.StringComparison]::OrdinalIgnoreCase)) {
        throw "refusing to stop PID $ServerPid because it is outside the lab runtime"
    }
    Stop-Process -Id $ServerPid -Force
}
Remove-Item -LiteralPath $PidFile -Force
if (Test-Path $CandidateFile) { Remove-Item -LiteralPath $CandidateFile -Force }
Write-Output "llama-server stopped"
