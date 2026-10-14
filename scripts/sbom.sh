#!/usr/bin/env bash
#
# sbom.sh [output-dir]
# Software bill of materials and license inventory of a release (GLD-059,
# v2 §43), made with Trivy from the repository: the Go modules of go.mod and the
# runtime packages of the embedded SPA (bun.lock, without dev dependencies).
#   <output-dir>/goeland-server-<version>.cdx.json       CycloneDX SBOM with licenses
#   <output-dir>/goeland-server-<version>-licenses.txt   license inventory by package
# Licenses are read from the Go module cache and the SPA's node_modules, so run
# it after `make release-check` (or `go mod download` and `bun install`). The
# image published by docker-publish carries its own SBOM attestation.
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

output_dir="${1:-bin/sbom}"

if ! command -v trivy >/dev/null 2>&1; then
    echo "sbom: trivy is required (https://trivy.dev)" >&2
    exit 1
fi
if [[ ! -d cmd/goeland-server/goeland-front/node_modules ]]; then
    echo "sbom: run 'bun install' in cmd/goeland-server/goeland-front first (licenses come from node_modules)" >&2
    exit 1
fi

version="$(sed -n 's/^[[:space:]]*Version = "\([0-9][0-9.]*\)"/\1/p' pkg/version/version.go)"
name="goeland-server-${version}"
mkdir -p "${output_dir}"

trivy fs --quiet --scanners license --format cyclonedx --output "${output_dir}/${name}.cdx.json" .
trivy fs --quiet --scanners license --format table --output "${output_dir}/${name}-licenses.txt" .

echo "sbom: ${output_dir}/${name}.cdx.json and ${output_dir}/${name}-licenses.txt"
