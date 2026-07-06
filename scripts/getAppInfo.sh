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

# Each grep isolates the "Name = \"value\"" line, awk takes the 3rd field
# (the quoted value) and tr strips the surrounding double quotes.
APP_NAME=$(grep -E 'AppName\s+=' "$SOURCE_CODE" | awk '{ print $3 }' | tr -d '"')
APP_VERSION=$(grep -E 'Version\s+=' "$SOURCE_CODE" | awk '{ print $3 }' | tr -d '"')
APP_REVISION=$(grep -E 'Revision\s+=' "$SOURCE_CODE" | awk '{ print $3 }' | tr -d '"')
APP_REPOSITORY=$(grep -E 'Repository\s+=' "$SOURCE_CODE" | awk '{ print $3 }' | tr -d '"')
APP_NAME_SNAKE=$(grep -E 'AppNameSnake\s+=' "$SOURCE_CODE" | awk '{ print $3 }' | tr -d '"')

echo "## Found APP: ${APP_NAME}, VERSION: ${APP_VERSION}, REVISION: ${APP_REVISION} in source file ${SOURCE_CODE}"
export APP_NAME APP_NAME_SNAKE APP_VERSION APP_REVISION APP_REPOSITORY
