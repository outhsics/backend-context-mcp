#!/usr/bin/env bash
# Test build script to verify all platforms can be cross-compiled

set -euo pipefail

APP_NAME="backend-context-mcp"
PLATFORMS=(
  "darwin arm64"
  "darwin amd64"
  "linux amd64"
  "linux arm64"
  "windows amd64"
)

echo "Testing cross-platform builds..."
echo ""

mkdir -p dist/test

for platform in "${PLATFORMS[@]}"; do
  read -r goos goarch <<< "${platform}"
  output="dist/test/${APP_NAME}-${goos}-${goarch}"
  if [[ "${goos}" == "windows" ]]; then
    output="${output}.exe"
  fi

  echo -n "Building ${goos}/${goarch}... "

  GOOS="${goos}" GOARCH="${goarch}" CGO_ENABLED=0 go build \
    -ldflags="-s -w" \
    -o "${output}" \
    . > /dev/null 2>&1

  if [[ $? -eq 0 ]]; then
    # Verify file exists and is not empty
    if [[ -s "${output}" ]]; then
      echo "✓ OK"
    else
      echo "✗ FAILED (empty file)"
      exit 1
    fi
  else
    echo "✗ FAILED"
    exit 1
  fi
done

echo ""
echo "All platforms built successfully!"
echo ""
echo "Test files in dist/test/:"
ls -lh dist/test/

# Cleanup
rm -rf dist/test
echo ""
echo "Cleanup complete."
