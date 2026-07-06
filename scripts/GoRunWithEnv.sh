#!/bin/bash
#
# GoRunWithEnv.sh
# `go run` the server (or any Go main) with the version LDFLAGS injected and a
# dotenv file loaded into the environment. This mirrors what `make run` does but
# lets you point at an arbitrary .env / main package.
#
# Usage:
#   ./scripts/GoRunWithEnv.sh [go-main-path] [env-file]
# Examples:
#   ./scripts/GoRunWithEnv.sh                                   # ./cmd/goeland-server + .env
#   ./scripts/GoRunWithEnv.sh ./cmd/goeland-server .env_dev

echo "## $0 received NUM ARGS : " $#
APP_REPOSITORY=github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026
NOW=$(date -u '+%Y-%m-%d_%I:%M:%S%p')
REVISION="$(git describe --dirty --always 2>/dev/null || echo unknown)"
# Match the ldflags targets used by the Makefile (pkg/version.Revision / .BuildStamp).
LDFLAGS="-X ${APP_REPOSITORY}/pkg/version.BuildStamp=${NOW} -X ${APP_REPOSITORY}/pkg/version.Revision=${REVISION}"

GO_MAIN_FILENAME='./cmd/goeland-server'
ENV_FILENAME='.env'
if [[ $# -eq 1 ]]; then
  GO_MAIN_FILENAME=${1}
elif [[ $# -eq 2 ]]; then
  GO_MAIN_FILENAME=${1}
  ENV_FILENAME=${2:-.env}
fi

echo "## will try to run : ${GO_MAIN_FILENAME} with env variables in ${ENV_FILENAME} ..."
if [[ -r "$ENV_FILENAME" ]]; then
  echo "## will do : go run $LDFLAGS $GO_MAIN_FILENAME"
  set -a
  source <(sed -e '/^#/d;/^\s*$/d' -e "s/'/'\\\''/g" -e "s/=\(.*\)/='\1'/g" "$ENV_FILENAME")
  go run -ldflags "$LDFLAGS" "${GO_MAIN_FILENAME}"
  set +a
else
  echo "## 💥💥 env path argument : ${ENV_FILENAME} was not found"
  exit 1
fi
