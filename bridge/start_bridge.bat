@echo off
REM Tinker Bridge Server Launcher (uv)

cd /d "%~dp0"
cd ..

REM Check if uv is available
uv --version >nul 2>&1
if errorlevel 1 (
    echo [ERROR] uv not found. Please install uv.
    exit /b 1
)

echo Starting Tinker Bridge Server via uv...
uv run tinker-bridge
