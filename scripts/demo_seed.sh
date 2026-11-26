#!/usr/bin/env bash
#
# demo_seed.sh
# Fill a local database with the synthetic public demo (GLD-079,
# docs/DEMO_SCRIPT.md): create the database when it does not exist (owned by
# the application role DB_USER, extensions enabled as the PostgreSQL
# superuser), then run `goeland-server demo-seed`, which migrates it and writes
# the demo through the API. An existing database is used as it is: the
# generator refuses one that already holds data.
#
# Settings come from the environment, else from the demo env file given with
# -e (e.g. .env_demo_small_fr, see .env_demo.sample), else from .env: the demo
# file only holds what differs (database, blob store, document path, language)
# and no secret. Serve the demo later with the same settings, so its documents
# are found. No credential is printed.
#
# The database is the argument, else DB_NAME of the -e file itself (never the
# one of .env: that is the development database). The language is the
# argument, else GOELAND_DEMO_LANG, else fr. The superuser connection is
# GOELAND_DEMO_ADMIN_URL, by default postgres@DB_HOST:DB_PORT (local trust
# authentication).
#
# With -s the script creates and writes nothing: it serves the existing demo
# database with the same settings (`go run ./cmd/goeland-server`), as the demo
# user of the GOELAND_DEV_USER_* settings (docs/DEMO_SCRIPT.md §7).
#
# Usage:
#   scripts/demo_seed.sh [-s] [-e <env-file>] [<database>] [fr|en]
#   scripts/demo_seed.sh goeland_demo en
#   scripts/demo_seed.sh -e .env_demo_small_fr
#   scripts/demo_seed.sh -s -e .env_demo_small_fr

set -euo pipefail

fail() {
    local message="$1"
    echo "demo_seed: ${message}" >&2
    exit 1
}

# load_env exports the variables of the dotenv file that are not set yet, so
# the environment wins over the files and the first file loaded over the next.
# Values in these files are never quoted (AGENTS.md): each line is exported as it is.
load_env() {
    local file="$1"
    local line key
    while IFS= read -r line || [[ -n "$line" ]]; do
        [[ "$line" =~ ^([A-Za-z_][A-Za-z0-9_]*)=(.*)$ ]] || continue
        key="${BASH_REMATCH[1]}"
        [[ -n "${!key+set}" ]] && continue
        export "${key}=${BASH_REMATCH[2]}"
    done <"$file"
}

# file_value prints the value of key in the dotenv file (never echoed elsewhere).
file_value() {
    local file="$1"
    local key="$2"
    sed -n "s/^${key}=//p" "$file" | tail -n 1
}

# database_exists succeeds when the database exists.
database_exists() {
    local admin_url="$1"
    local database="$2"
    [[ "$(psql "$admin_url" -Atq -v ON_ERROR_STOP=1 -c "SELECT 1 FROM pg_database WHERE datname = '${database}'")" == "1" ]]
}

# ensure_database creates the database owned by owner when missing, and
# enables the extensions the migrations need (a no-op when present).
ensure_database() {
    local admin_url="$1"
    local database="$2"
    local owner="$3"
    if database_exists "$admin_url" "$database"; then
        echo "## using the existing database ${database}"
    else
        echo "## creating the database ${database} (owner: the application role DB_USER)"
        psql "$admin_url" -q -v ON_ERROR_STOP=1 -c "CREATE DATABASE ${database} OWNER \"${owner}\";" >/dev/null
    fi
    psql "${admin_url%/*}/${database}" -q -v ON_ERROR_STOP=1 \
        -c "CREATE EXTENSION IF NOT EXISTS pgcrypto; CREATE EXTENSION IF NOT EXISTS pg_trgm;" \
        -c "CREATE EXTENSION IF NOT EXISTS unaccent; CREATE EXTENSION IF NOT EXISTS postgis;"
}

main() {
    local env_file="" serve=false positional=()
    while [[ $# -gt 0 ]]; do
        case "$1" in
            -s)
                serve=true
                shift
                ;;
            -e)
                [[ $# -ge 2 ]] || fail "-e needs an env file"
                env_file="$2"
                shift 2
                ;;
            -*) fail "unknown option: $1 (usage: $0 [-s] [-e <env-file>] [<database>] [fr|en])" ;;
            *)
                positional+=("$1")
                shift
                ;;
        esac
    done
    [[ ${#positional[@]} -le 2 ]] || fail "usage: $0 [-s] [-e <env-file>] [<database>] [fr|en]"
    [[ -r .env ]] || fail "run from the repository root, next to .env"
    [[ -z "$env_file" || -r "$env_file" ]] || fail "cannot read the env file ${env_file}"
    command -v psql >/dev/null || fail "psql is required"

    local database="${positional[0]:-}"
    if [[ -z "$database" && -n "$env_file" ]]; then
        database="$(file_value "$env_file" DB_NAME)"
    fi
    [[ -n "$env_file" ]] && load_env "$env_file"
    load_env .env
    local lang="${positional[1]:-${GOELAND_DEMO_LANG:-fr}}"
    [[ "$database" =~ ^[a-z][a-z0-9_]{0,62}$ ]] || fail "give the database as argument or as DB_NAME in the -e file (lower-case letters, digits and _)"
    [[ "$lang" =~ ^(fr|en)$ ]] || fail "language must be fr or en"
    local owner="${DB_USER:-}"
    local host="${DB_HOST:-127.0.0.1}"
    local port="${DB_PORT:-5432}"
    [[ -n "$owner" ]] || fail "DB_USER is missing"
    local admin_url="${GOELAND_DEMO_ADMIN_URL:-postgres://postgres@${host}:${port}/postgres}"

    if [[ "$serve" == true ]]; then
        database_exists "$admin_url" "$database" || fail "the database ${database} does not exist: generate the demo first (without -s)"
    else
        ensure_database "$admin_url" "$database" "$owner"
    fi
    if [[ ! -f cmd/goeland-server/goeland-front/dist/index.html ]]; then
        echo "## building the SPA (embedded by the server)"
        make front-build >/dev/null
    fi
    # The server assembles its DSN from the DB_* settings when DATABASE_URL is unset.
    unset DATABASE_URL
    export DB_NAME="$database"
    export GOELAND_DB_MIGRATE=auto
    if [[ "$serve" == true ]]; then
        echo "## serving the demo database ${database} as user ${GOELAND_DEV_USER_ID:-1} (${GOELAND_DEV_USER_NAME:-dev user}), documents in the ${GOELAND_BLOB_STORE:-file} store"
        exec go run ./cmd/goeland-server
    fi
    echo "## generating the demo (profile small, language ${lang}) into ${database}, documents in the ${GOELAND_BLOB_STORE:-file} store"
    go run ./cmd/goeland-server demo-seed -profile small -lang "$lang"
    echo "## done: serve it with DB_NAME=${database}, DATABASE_URL unset and the same blob store settings;"
    echo "## log in as a demo user with GOELAND_DEV_USER_ID / _NAME / _EMAIL (docs/DEMO_SCRIPT.md §7)"
}

main "$@"
