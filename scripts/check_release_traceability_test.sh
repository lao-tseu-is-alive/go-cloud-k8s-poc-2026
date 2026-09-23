#!/usr/bin/env bash
#
# check_release_traceability_test.sh
# Accepted and rejected cases for check_release_traceability.sh (run by
# `make scripts-check`), as docs/DOCUMENTATION.md requires for every checker.
set -euo pipefail

script="$(cd "$(dirname "$0")" && pwd)/check_release_traceability.sh"
work="$(mktemp -d)"
trap 'rm -rf -- "${work}"' EXIT

write_case() {
    printf '%s\n' "$1" >"${work}/ROADMAP.md"
    printf '%s\n' "$2" >"${work}/CHANGELOG.md"
}

expect() {
    local want="$1" name="$2"
    if bash "${script}" "${work}/ROADMAP.md" "${work}/CHANGELOG.md" >/dev/null 2>&1; then got=pass; else got=fail; fi
    if [[ "${got}" != "${want}" ]]; then
        echo "traceability-test: ${name}: got ${got}, want ${want}" >&2
        exit 1
    fi
}

write_case '- [x] **GLD-001 — Done**: shipped.
- [ ] **GLD-002 — Todo**: later.' '## [Unreleased]

- GLD-002 in progress.

## [0.1.0] - 2026-01-01

- **GLD-001** — shipped.'
expect pass "done task released, pending task only in Unreleased"

write_case '- [x] **GLD-001 — Done**: shipped.' '## [Unreleased]

- **GLD-001** — not yet released.'
expect fail "done task missing from a dated section"

write_case '- [~] **GLD-001 — Doing**: in progress.' '## [0.1.0] - 2026-01-01

- **GLD-001** — claimed as released.'
expect fail "released task not marked done"

echo "traceability-test: OK"
