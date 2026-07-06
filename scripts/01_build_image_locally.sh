#!/bin/bash
#
# 01_build_image_locally.sh
# Build the server container image locally from the multi-stage Dockerfile,
# tagging it with the version read from pkg/version/version.go. Skips the build
# if that image:tag already exists (bump Version in version.go to rebuild).
#
# Uses nerdctl (Rancher Desktop) by default; change DOCKER_BIN for docker.
# If trivy is installed, the Dockerfile is scanned first and the build is
# aborted on MEDIUM/HIGH/CRITICAL findings; if trivy is absent the scan is skipped.

set -euo pipefail

# Load APP_NAME / APP_VERSION / APP_NAME_SNAKE from version.go if not already set.
if [[ -z "${APP_NAME:-}" ]]; then
  if [[ -f getAppInfo.sh ]]; then source getAppInfo.sh;
  elif [[ -f ./scripts/getAppInfo.sh ]]; then source ./scripts/getAppInfo.sh;
  else echo "ERROR: getAppInfo.sh not found" >&2; exit 1; fi
fi

DOCKER_BIN="${DOCKER_BIN:-nerdctl}"
CONTAINER_REGISTRY_ID="${CONTAINER_REGISTRY_ID:-laotseu}"
IMAGE="${CONTAINER_REGISTRY_ID}/${APP_NAME_SNAKE}"

if ! $DOCKER_BIN --version >/dev/null 2>&1; then
  echo "## 💥 ERROR: ${DOCKER_BIN} is not available (start Rancher Desktop, or set DOCKER_BIN=docker)" >&2
  exit 1
fi

echo "## APP: ${APP_NAME}, version: ${APP_VERSION}, image: ${IMAGE}:${APP_VERSION}"

# Refuse to silently overwrite an already-built image:tag.
if $DOCKER_BIN images --format '{{.Repository}}:{{.Tag}}' | grep -qx "${IMAGE}:${APP_VERSION}"; then
  echo "## 💥 ERROR: ${IMAGE}:${APP_VERSION} already exists."
  echo "##    Bump Version in pkg/version/version.go, or remove it: ${DOCKER_BIN} rmi ${IMAGE}:${APP_VERSION}" >&2
  exit 1
fi

# Optional security gate: scan the Dockerfile with trivy when available.
if command -v trivy >/dev/null 2>&1; then
  echo "## running trivy config scan on the Dockerfile ..."
  trivy config --exit-code 1 --severity MEDIUM,HIGH,CRITICAL . || {
    echo "## 💥 ERROR: fix the vulnerabilities reported by trivy before building" >&2; exit 1; }
else
  echo "## (trivy not installed — skipping Dockerfile security scan)"
fi

echo "## building ${IMAGE}:${APP_VERSION} + ${IMAGE}:latest ..."
$DOCKER_BIN build -t "${IMAGE}:latest" -t "${IMAGE}:${APP_VERSION}" .

echo "## ✓ built. Try it locally:"
echo "   ${DOCKER_BIN} run --rm -p 8080:8080 --env-file .env --name ${APP_NAME_SNAKE} ${IMAGE}:${APP_VERSION}"
echo "## push:"
echo "   ${DOCKER_BIN} push ${IMAGE}:${APP_VERSION}"
