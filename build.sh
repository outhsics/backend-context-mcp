#!/usr/bin/env bash
set -euo pipefail

APP_NAME="backend-context-mcp"
PLATFORMS=(
  "darwin arm64"
  "darwin amd64"
  "linux amd64"
  "linux arm64"
  "windows amd64"
)

mkdir -p dist

echo "Building ${APP_NAME}..."
for platform in "${PLATFORMS[@]}"; do
  read -r goos goarch <<< "${platform}"
  output="dist/${APP_NAME}-${goos}-${goarch}"
  if [[ "${goos}" == "windows" ]]; then
    output="${output}.exe"
  fi
  echo "  ${goos}/${goarch} -> ${output}"
  GOOS="${goos}" GOARCH="${goarch}" CGO_ENABLED=0 go build \
    -ldflags="-s -w" \
    -o "${output}" .
done

echo "Done. Release assets are in ./dist"
