@echo off
REM Windows build script wrapper for backend-context-mcp
REM This script requires PowerShell to be installed

powershell -ExecutionPolicy Bypass -File "%~dp0build.ps1"
pause
