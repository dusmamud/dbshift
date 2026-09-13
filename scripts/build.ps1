# Build script for dbshift on Windows
Param(
    [string]$Version = "v1.0.0",
    [switch]$RunTests = $true
)

$ErrorActionPreference = "Stop"

Write-Host "=========================================" -ForegroundColor Cyan
Write-Host " Building dbshift ($Version) " -ForegroundColor Yellow
Write-Host "=========================================" -ForegroundColor Cyan

if ($RunTests) {
    Write-Host "--> Running tests..." -ForegroundColor Gray
    go test -v ./...
    if ($LASTEXITCODE -ne 0) {
        Write-Error "Tests failed! Aborting build."
        exit 1
    }
}

$OutputDir = "bin"
if (!(Test-Path $OutputDir)) {
    New-Item -ItemType Directory -Path $OutputDir | Out-Null
}

$BinaryPath = "$OutputDir/dbshift.exe"
Write-Host "--> Compiling binary to $BinaryPath..." -ForegroundColor Gray

$ldflags = "-s -w -X main.Version=$Version"
go build -ldflags $ldflags -o $BinaryPath ./cmd/dbshift

if ($LASTEXITCODE -eq 0) {
    Write-Host "--> Build SUCCESSFUL!" -ForegroundColor Green
    $fileInfo = Get-Item $BinaryPath
    $sizeMB = [math]::Round($fileInfo.Length / 1MB, 2)
    Write-Host "--> Binary location: $BinaryPath ($sizeMB MB)" -ForegroundColor Cyan
    Write-Host "--> Run it: .\$BinaryPath --help" -ForegroundColor Yellow
} else {
    Write-Error "Compilation failed."
    exit 1
}
