#!/usr/bin/env bash
#
# check_generated.sh
# Generated-code reproducibility gate (GLD-008): regenerate gen/, api/openapi/
# and the SPA contract types (GLD-072) from proto/ into a temporary directory
# with the pinned generators (buf.gen.yaml: protoc-gen-go and
# protoc-gen-connect-go as go.mod tools, the remote OpenAPI and protobuf-es
# plugins by version) and fail on any difference
# with the committed output, missing or extra files included. Needs network
# access for the remote plugins.
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

out="$(mktemp -d)"
trap 'rm -rf "${out}"' EXIT

buf generate --output "${out}"

status=0
for dir in gen api/openapi cmd/goeland-server/goeland-front/src/api/gen; do
    if ! diff -r "${dir}" "${out}/${dir}" >&2; then
        echo "generated-check: ${dir} differs from a fresh buf generate; run make generate and commit the result" >&2
        status=1
    fi
done
if [[ "${status}" -eq 0 ]]; then
    echo "generated-check: gen/, api/openapi/ and the SPA contract types are reproducible"
fi
exit "${status}"
