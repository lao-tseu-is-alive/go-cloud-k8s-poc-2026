#!/bin/bash
#
# getAppInfo.sh
# Extract the application name, version, revision and repository from the single
# source of truth (pkg/version/version.go) and export them as environment
# variables. Other scripts (build, deploy, release) source this file so the
# values never drift from the Go code.
#
# Usage:
#   source ./scripts/getAppInfo.sh   # exports APP_NAME, APP_VERSION, ...

SOURCE_CODE=pkg/version/version.go
echo "## Extracting app name and version from code in ${SOURCE_CODE}"

# go_value NAME prints the quoted value of `NAME = "value"` in SOURCE_CODE: grep
# isolates the line, awk takes the 3rd field and tr strips the double quotes.
go_value() {
  local name="$1"
  grep -E "${name}\s+=" "$SOURCE_CODE" | awk '{ print $3 }' | tr -d '"'
}

APP_NAME=$(go_value AppName)
APP_VERSION=$(go_value Version)
APP_REVISION=$(go_value Revision)
APP_REPOSITORY=$(go_value Repository)
APP_NAME_SNAKE=$(go_value AppNameSnake)

echo "## Found APP: ${APP_NAME}, VERSION: ${APP_VERSION}, REVISION: ${APP_REVISION} in source file ${SOURCE_CODE}"
export APP_NAME APP_NAME_SNAKE APP_VERSION APP_REVISION APP_REPOSITORY
