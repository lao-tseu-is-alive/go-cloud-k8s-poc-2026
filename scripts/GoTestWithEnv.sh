#!/bin/bash
#
# GoTestWithEnv.sh
# Run the full Go test suite with the race detector and coverage, loading a
# dotenv file first. Useful once DB-backed integration tests exist and need
# DATABASE_URL from a dedicated test .env.
#
# Usage:
#   ./scripts/GoTestWithEnv.sh [env-file]   # defaults to .env

echo "## $0 received NUM ARGS : " $#
APP_REPOSITORY=github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026
NOW=$(date -u '+%Y-%m-%d_%I:%M:%S%p')
REVISION="$(git describe --dirty --always 2>/dev/null || echo unknown)"
LDFLAGS="-X ${APP_REPOSITORY}/pkg/version.BuildStamp=${NOW} -X ${APP_REPOSITORY}/pkg/version.Revision=${REVISION}"

ENV_FILENAME=${1:-.env}

echo "## will run go test (race + coverage) with env variables in ${ENV_FILENAME} ..."
if [[ -r "$ENV_FILENAME" ]]; then
  set -a
  source <(sed -e '/^#/d;/^\s*$/d' -e "s/'/'\\\''/g" -e "s/=\(.*\)/='\1'/g" "$ENV_FILENAME")
  go test -race -coverprofile=coverage.txt -coverpkg=./cmd/...,./pkg/... -ldflags "$LDFLAGS" ./...
  set +a
  go tool cover -func=coverage.txt | tail -1
else
  echo "## 💥💥 env path argument : ${ENV_FILENAME} was not found"
  exit 1
fi
