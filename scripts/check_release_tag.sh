#!/usr/bin/env bash
#
# check_release_tag.sh <tag>
# Refuse to publish a tag that does not equal "v" + the Version constant in
# pkg/version/version.go, so a shipped binary can never report a version other
# than the one it was tagged with. Used by the release and docker-publish
# workflows before `make release-check`.
set -euo pipefail

if [[ $# -ne 1 ]]; then
    echo "usage: $0 <vX.Y.Z tag>" >&2
    exit 2
fi
tag="$1"

cd "$(git rev-parse --show-toplevel)"

if ! printf '%s' "${tag}" | grep -qE '^v[0-9]+\.[0-9]+\.[0-9]+$'; then
    echo "check-release-tag: invalid release tag: ${tag}" >&2
    exit 1
fi
source_version="$(sed -n 's/^[[:space:]]*Version = "\([0-9][0-9.]*\)"/\1/p' pkg/version/version.go)"
if [[ "${tag}" != "v${source_version}" ]]; then
    echo "check-release-tag: tag ${tag} does not match source version v${source_version}" >&2
    exit 1
fi
echo "check-release-tag: ${tag} matches pkg/version/version.go"
