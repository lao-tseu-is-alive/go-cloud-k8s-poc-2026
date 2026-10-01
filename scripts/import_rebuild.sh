#!/usr/bin/env bash
#
# import_rebuild.sh
# Rebuild a local import database from scratch and load it from the legacy
# Goéland replica (GLD-051, docs/IMPORT_MAPPING.md): drop and recreate the
# target (owned by the .env application role, extensions enabled as the
# PostgreSQL superuser), then run cmd/goeland-import, which migrates it and
# imports. Pass --dry-run to roll the import back (counts only).
#
# The target name must start with goeland_import, so neither the development
# database nor the replica can be dropped by mistake. No credential is printed.
#
# Usage:
#   GOELAND_IMPORT_SOURCE_URL='postgres://goeland_read:<password>@localhost/goeland' \
#   GOELAND_IMPORT_ADMIN_URL='postgres://postgres@127.0.0.1:5432/postgres' \
#       scripts/import_rebuild.sh <goeland_import...> [--dry-run]

set -euo pipefail

fail() {
    local message="$1"
    echo "import_rebuild: ${message}" >&2
    exit 1
}

# env_value prints the value of a key of the .env file (never echoed elsewhere).
env_value() {
    local key="$1"
    sed -n "s/^${key}=//p" .env | tail -n 1
}

main() {
    local target="${1:-}"
    local mode="${2:-}"
    [[ "$target" =~ ^goeland_import[a-z0-9_]*$ ]] || fail "the target database name must start with goeland_import"
    [[ -z "$mode" || "$mode" == "--dry-run" ]] || fail "unknown option: $mode"
    [[ -r .env ]] || fail "run from the repository root, next to .env"
    [[ -n "${GOELAND_IMPORT_SOURCE_URL:-}" ]] || fail "GOELAND_IMPORT_SOURCE_URL is required"
    [[ -n "${GOELAND_IMPORT_ADMIN_URL:-}" ]] || fail "GOELAND_IMPORT_ADMIN_URL is required"
    command -v psql >/dev/null || fail "psql is required"

    local app_user app_password db_host db_port
    app_user="$(env_value DB_USER)"
    app_password="$(env_value DB_PASSWORD)"
    db_host="$(env_value DB_HOST)"
    db_port="$(env_value DB_PORT)"
    [[ -n "$app_user" ]] || fail "DB_USER is missing from .env"

    echo "## recreating database ${target} (owner: the .env application role)"
    psql "$GOELAND_IMPORT_ADMIN_URL" -v ON_ERROR_STOP=1 -q \
        -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '${target}' AND pid <> pg_backend_pid();" \
        -c "DROP DATABASE IF EXISTS ${target};" \
        -c "CREATE DATABASE ${target} OWNER \"${app_user}\";" >/dev/null
    local admin_target="${GOELAND_IMPORT_ADMIN_URL%/*}/${target}"
    psql "$admin_target" -v ON_ERROR_STOP=1 -q \
        -c "CREATE EXTENSION IF NOT EXISTS pgcrypto; CREATE EXTENSION IF NOT EXISTS pg_trgm;" \
        -c "CREATE EXTENSION IF NOT EXISTS unaccent; CREATE EXTENSION IF NOT EXISTS postgis;"

    local apply="-apply"
    [[ "$mode" == "--dry-run" ]] && apply=""
    echo "## importing into ${target} ${mode:-(committed)}"
    GOELAND_IMPORT_TARGET_URL="host=${db_host:-127.0.0.1} port=${db_port:-5432} dbname=${target} user=${app_user} password=${app_password} sslmode=disable" \
        go run ./cmd/goeland-import ${apply}
}

main "$@"
