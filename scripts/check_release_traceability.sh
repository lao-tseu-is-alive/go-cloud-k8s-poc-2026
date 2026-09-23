#!/usr/bin/env bash
#
# check_release_traceability.sh
# Bidirectional roadmap <-> changelog traceability (docs/DOCUMENTATION.md,
# "Roadmap, changelog and version traceability"):
#   - every task marked done in docs/ROADMAP.md is named in a dated changelog section;
#   - every task named in a dated changelog section is marked done in the roadmap.
# The Unreleased section is ignored: it describes ongoing work, not delivery.
set -euo pipefail

roadmap="${1:-docs/ROADMAP.md}"
changelog="${2:-CHANGELOG.md}"

for file in "${roadmap}" "${changelog}"; do
    if [[ ! -f "${file}" ]]; then
        echo "release-traceability-check: file not found: ${file}" >&2
        exit 1
    fi
done

released_changelog="$(mktemp)"
trap 'rm -f -- "${released_changelog}"' EXIT

# Keep everything from the first dated semantic-version heading onwards.
awk '
    /^## \[[0-9]+\.[0-9]+\.[0-9]+\] - [0-9]{4}-[0-9]{2}-[0-9]{2}$/ { released=1 }
    released { print }
' "${changelog}" >"${released_changelog}"

failed=0
while IFS= read -r task_id; do
    [[ -n "${task_id}" ]] || continue
    if ! grep -Fq -- "${task_id}" "${released_changelog}"; then
        echo "release-traceability-check: completed task ${task_id} is missing from a dated changelog section" >&2
        failed=1
    fi
done < <(sed -n 's/^- \[x\] \*\*\(GLD-[0-9][0-9][0-9]\).*/\1/p' "${roadmap}")

while IFS= read -r task_id; do
    [[ -n "${task_id}" ]] || continue
    if ! grep -Fq -- "- [x] **${task_id} " "${roadmap}"; then
        echo "release-traceability-check: released task ${task_id} is not marked complete in the roadmap" >&2
        failed=1
    fi
done < <(grep -oE 'GLD-[0-9]{3}' "${released_changelog}" | sort -u || true)

if [[ "${failed}" -ne 0 ]]; then
    exit 1
fi
echo "release-traceability-check: OK"
