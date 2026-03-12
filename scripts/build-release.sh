#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 1 || $# -gt 2 ]]; then
  echo "usage: $0 <version> [output-dir]" >&2
  exit 2
fi

version="$1"
output_dir="${2:-dist}"
commit="${COMMIT:-unknown}"
build_date="${BUILD_DATE:-$(date -u +"%Y-%m-%dT%H:%M:%SZ")}"
platform="$(go env GOOS)-$(go env GOARCH)"
binary_name="mcfg"
binary_path="${output_dir}/${binary_name}-${platform}"
checksum_path="${binary_path}.sha256"

mkdir -p "${output_dir}"

env GOCACHE="${GOCACHE:-/tmp/go-build}" GOMODCACHE="${GOMODCACHE:-/tmp/gomodcache}" \
  go build \
  -trimpath \
  -ldflags "-s -w -X mcfg/internal/buildinfo.Version=${version} -X mcfg/internal/buildinfo.Commit=${commit} -X mcfg/internal/buildinfo.BuildDate=${build_date}" \
  -o "${binary_path}" \
  .

if command -v sha256sum >/dev/null 2>&1; then
  (cd "${output_dir}" && sha256sum "$(basename "${binary_path}")" > "$(basename "${checksum_path}")")
elif command -v shasum >/dev/null 2>&1; then
  (cd "${output_dir}" && shasum -a 256 "$(basename "${binary_path}")" > "$(basename "${checksum_path}")")
else
  echo "sha256sum or shasum is required to generate checksum files" >&2
  exit 3
fi

echo "built ${binary_path}"
echo "checksum ${checksum_path}"
