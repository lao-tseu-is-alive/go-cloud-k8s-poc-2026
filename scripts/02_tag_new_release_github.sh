#!/bin/bash
#
# 02_tag_new_release_github.sh
# Guarded release (docs/DOCUMENTATION.md, "Control chain"): tag v<Version> from
# pkg/version/version.go and push main + the tag atomically, so the release and
# docker-publish workflows fire on exactly the audited commit.
#
# Usage (from the repository root, after the release commit):
#   CONFIRM_RELEASE=vX.Y.Z ./scripts/02_tag_new_release_github.sh   # or: make release
#
# Refuses unless: CONFIRM_RELEASE equals the tag, the branch is main, the tree
# is clean, the tag exists neither locally nor on origin, and
# `make release-check` passes. The tag is annotated.

set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

# Load APP_NAME / APP_VERSION from version.go (single source of truth).
source ./scripts/getAppInfo.sh
tag="v${APP_VERSION}"

if [[ -z "${APP_VERSION}" ]]; then
  echo "## 💥 ERROR: unable to read Version from pkg/version/version.go" >&2
  exit 1
fi
if [[ "${CONFIRM_RELEASE:-}" != "${tag}" ]]; then
  echo "## 💥 ERROR: set CONFIRM_RELEASE=${tag} to confirm this external operation" >&2
  exit 1
fi
if [[ "$(git branch --show-current)" != "main" ]]; then
  echo "## 💥 ERROR: releases must be made from main" >&2
  exit 1
fi
if [[ -n "$(git status --porcelain)" ]]; then
  echo "## 💥 ERROR: working tree is DIRTY — commit the version and changelog first" >&2
  git status --short
  exit 1
fi
if git rev-parse -q --verify "refs/tags/${tag}" >/dev/null; then
  echo "## 💥 ERROR: local tag ${tag} already exists" >&2
  exit 1
fi
set +e
git ls-remote --exit-code --tags origin "refs/tags/${tag}" >/dev/null 2>&1
remote_tag_status=$?
set -e
case "${remote_tag_status}" in
  0)
    echo "## 💥 ERROR: remote tag ${tag} already exists" >&2
    exit 1
    ;;
  2)
    ;;
  *)
    echo "## 💥 ERROR: unable to verify remote tag ${tag}; refusing to publish" >&2
    exit 1
    ;;
esac

make release-check
git tag -a "${tag}" -m "${tag}"

echo "## ✓ pushing main and ${tag} atomically to origin ..."
if ! git push --atomic origin main "refs/tags/${tag}"; then
  echo "## 💥 ERROR: push failed; local annotated tag ${tag} was kept for inspection" >&2
  exit 1
fi
echo "## ✓ ${tag} pushed; GitHub Actions will publish the release and the image"
