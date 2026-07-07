#!/usr/bin/env bash
#
# createLocalDBAndUser.sh
# Create a local PostgreSQL role and database for the Goéland POC, enable the
# required extensions (as the postgres admin), then write the connection
# settings to ./.env in the directory from which this script is invoked.
#
# Why enable extensions here? PostGIS/pgcrypto need elevated privileges to
# install. Installing them once as the postgres admin lets the unprivileged
# application role run the migrations: migration 0001's
# `CREATE EXTENSION IF NOT EXISTS ...` then becomes a privilege-free no-op.
#
# Usage:  ./scripts/createLocalDBAndUser.sh <app-or-db-name>
# Example: ./scripts/createLocalDBAndUser.sh goeland_poc_db

set -euo pipefail

usage() {
    echo "Usage: $0 <app-name>" >&2
    echo "Example: $0 GoelandInventory" >&2
}

fail() {
    echo "ERROR: $*" >&2
    exit 1
}

if [[ $# -ne 1 ]]; then
    usage
    exit 1
fi

ENV_FILE="${PWD}/.env"
if [[ -e "$ENV_FILE" ]]; then
    fail "$ENV_FILE already exists; refusing to overwrite it"
fi

command -v openssl >/dev/null || fail "openssl is required"
command -v psql >/dev/null || fail "psql is required"
command -v createdb >/dev/null || fail "createdb is required"
command -v su >/dev/null || fail "su is required"

APP_NAME="$1"
DB_NAME="$(
    printf '%s' "$APP_NAME" |
        sed --regexp-extended \
            --expression 's/([a-z0-9])([A-Z])/\1_\2/g' \
            --expression 's/[- ]/_/g' |
        tr '[:upper:]' '[:lower:]'
)"

if [[ ! "$DB_NAME" =~ ^[a-z][a-z0-9_]*$ ]]; then
    fail "app name must produce a PostgreSQL identifier matching ^[a-z][a-z0-9_]*$"
fi

run_as_postgres() {
    su -c "$1" postgres
}

postgres_value_exists() {
    local query="$1"
    [[ "$(run_as_postgres "psql --dbname=postgres --tuples-only --no-align --command \"$query\"")" == "1" ]]
}

if postgres_value_exists "SELECT 1 FROM pg_roles WHERE rolname = '${DB_NAME}'"; then
    fail "PostgreSQL role ${DB_NAME} already exists"
fi

if postgres_value_exists "SELECT 1 FROM pg_database WHERE datname = '${DB_NAME}'"; then
    fail "PostgreSQL database ${DB_NAME} already exists"
fi

# Hex avoids quoting issues in SQL and dotenv files while retaining strong entropy.
DB_PASSWORD="$(openssl rand -hex 24)"

umask 077
if ! (set -o noclobber; cat >"$ENV_FILE") <<EOF
DB_DRIVER=postgres
DB_HOST=127.0.0.1
DB_PORT=5432
DB_NAME=${DB_NAME}
DB_USER=${DB_NAME}
DB_PASSWORD=${DB_PASSWORD}
DB_SSL_MODE=prefer
DATABASE_URL=postgres://${DB_NAME}:${DB_PASSWORD}@127.0.0.1:5432/${DB_NAME}?sslmode=prefer
EOF
then
    fail "$ENV_FILE appeared while preparing the database; refusing to overwrite it"
fi

cleanup_env_on_failure() {
    rm -f -- "$ENV_FILE"
}
trap cleanup_env_on_failure EXIT

echo "Creating PostgreSQL role ${DB_NAME}"
run_as_postgres "psql --dbname=postgres --set=ON_ERROR_STOP=1 --command \"CREATE USER ${DB_NAME} WITH PASSWORD '${DB_PASSWORD}';\""

echo "Creating PostgreSQL database ${DB_NAME} owned by ${DB_NAME}"
if ! run_as_postgres "createdb --owner=${DB_NAME} ${DB_NAME}"; then
    echo "Database creation failed; removing the newly created role ${DB_NAME}" >&2
    run_as_postgres "psql --dbname=postgres --set=ON_ERROR_STOP=1 --command \"DROP USER ${DB_NAME};\""
    exit 1
fi

echo "Enabling required extensions (pgcrypto, pg_trgm, unaccent and postgis) in ${DB_NAME}"
run_as_postgres "psql --dbname=${DB_NAME} --set=ON_ERROR_STOP=1 \
    --command \"CREATE EXTENSION IF NOT EXISTS pgcrypto;\" \
    --command \"CREATE EXTENSION IF NOT EXISTS pg_trgm;\" \
    --command \"CREATE EXTENSION IF NOT EXISTS unaccent;\" \
    --command \"CREATE EXTENSION IF NOT EXISTS postgis;\""

trap - EXIT
echo "Created ${ENV_FILE} with permissions restricted by umask 077"
echo
echo "Next steps:"
echo "  make db-up      # apply migrations with dbmate (optional; the server self-migrates on start)"
echo "  make run        # start goeland-server"
