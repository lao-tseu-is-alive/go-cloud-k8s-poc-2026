#!/bin/bash
#
# 02_tag_new_release_github.sh
# Create and push a git tag "v<APP_VERSION>" using the version read from
# pkg/version/version.go. Refuses to tag a dirty working tree or an existing tag.
# Bump Version in pkg/version/version.go before running.

set -euo pipefail

# Load APP_NAME / APP_VERSION from version.go.
if [[ -f getAppInfo.sh ]]; then source getAppInfo.sh;
elif [[ -f ./scripts/getAppInfo.sh ]]; then source ./scripts/getAppInfo.sh;
else echo "ERROR: getAppInfo.sh not found" >&2; exit 1; fi

echo "## APP: ${APP_NAME}, version: ${APP_VERSION} (from pkg/version/version.go)"

if output=$(git status --porcelain) && [ -z "$output" ]; then
  if [ "$(git tag -l "v$APP_VERSION")" ]; then
    echo "## 💥 ERROR: tag v${APP_VERSION} already exists" >&2
    exit 1
  fi
  echo "## ✓ tagging v${APP_VERSION} and pushing to origin ..."
  git tag "v$APP_VERSION" -m "v$APP_VERSION bump"
  git push origin --tags
else
  echo "## 💥 ERROR: working tree is DIRTY — commit before tagging v${APP_VERSION}" >&2
  git status --short
  exit 1
fi
