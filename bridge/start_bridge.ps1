# Tinker Bridge Server Startup Script (uv)

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location "$scriptDir\.."

# Check if uv is available
try {
    $uvVersion = uv --version 2>&1
    Write-Host "✓ Found $uvVersion" -ForegroundColor Green
} catch {
    Write-Host "✗ uv not found. Please install uv." -ForegroundColor Red
    exit 1
}

Write-Host "Starting Tinker Bridge Server via uv..." -ForegroundColor Cyan
uv run tinker-bridge
