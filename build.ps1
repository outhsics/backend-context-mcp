#!/usr/bin/env pwsh
# Windows build script for backend-context-mcp

$ErrorActionPreference = "Stop"

$APP_NAME = "backend-context-mcp"
$PLATFORMS = @(
    @{ GoOS = "darwin"; GoArch = "amd64" },
    @{ GoOS = "darwin"; GoArch = "arm64" },
    @{ GoOS = "linux"; GoArch = "amd64" },
    @{ GoOS = "linux"; GoArch = "arm64" },
    @{ GoOS = "windows"; GoArch = "amd64" }
)

New-Item -ItemType Directory -Force -Path dist | Out-Null

Write-Host "Building $APP_NAME..." -ForegroundColor Green

foreach ($platform in $PLATFORMS) {
    $goos = $platform.GoOS
    $goarch = $platform.GoArch
    $output = "dist/${APP_NAME}-${goos}-${goarch}"
    if ($goos -eq "windows") {
        $output += ".exe"
    }

    Write-Host "  ${goos}/${goarch} -> ${output}" -ForegroundColor Cyan

    $env:GOOS = $goos
    $env:GOARCH = $goarch
    $env:CGO_ENABLED = "0"

    go build -ldflags="-s -w" -o $output .

    if ($LASTEXITCODE -ne 0) {
        Write-Host "Build failed for ${goos}/${goarch}" -ForegroundColor Red
        exit 1
    }
}

Write-Host "`nDone. Release assets are in .\dist" -ForegroundColor Green
