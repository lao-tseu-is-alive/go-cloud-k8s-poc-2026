#!/usr/bin/env bash
#
# get_jwt_token.sh
# Fetch a JWT from a running go-cloud-k8s-auth server, using the ADMIN_USER /
# ADMIN_PASSWORD found in an .env file. Handy for exercising the server in
# jwt auth mode:
#
#   TOKEN=$(./scripts/get_jwt_token.sh)
#   curl -H "Authorization: Bearer $TOKEN" -H 'Connect-Protocol-Version: 1' ...
#
# Usage: ./scripts/get_jwt_token.sh [/path/to/.env]   # defaults to .env
set -euo pipefail

ENV_FILE="${1:-.env}"
if [[ ! -f "$ENV_FILE" ]]; then
  echo "Error: environment file '$ENV_FILE' not found." >&2
  exit 1
fi

# Load and export the dotenv variables.
set -o allexport
# shellcheck disable=SC1090
source "$ENV_FILE"
set +o allexport

ADMIN_USER="${ADMIN_USER:-}"
ADMIN_PASSWORD="${ADMIN_PASSWORD:-}"
AUTH_PORT="${PORT:-9090}"
AUTH_HOST="${AUTH_HOST:-localhost}"

# The login carries credentials: plain HTTP is only acceptable on the loopback
# interface (local dev auth server); any other host must use HTTPS unless the
# operator explicitly opts out with AUTH_ALLOW_INSECURE=1.
case "$AUTH_HOST" in
  localhost|127.0.0.1|::1) AUTH_SCHEME="${AUTH_SCHEME:-http}" ;;
  *) AUTH_SCHEME="${AUTH_SCHEME:-https}" ;;
esac
if [[ "$AUTH_SCHEME" != "https" ]]; then
  case "$AUTH_HOST" in
    localhost|127.0.0.1|::1) ;;
    *)
      if [[ "${AUTH_ALLOW_INSECURE:-0}" != "1" ]]; then
        echo "Error: refusing to send credentials over clear text to '$AUTH_HOST' (set AUTH_SCHEME=https)." >&2
        exit 1
      fi
      ;;
  esac
fi
AUTH_URL="${AUTH_SCHEME}://${AUTH_HOST}:${AUTH_PORT}/login"

if [[ -z "$ADMIN_USER" ]] || [[ -z "$ADMIN_PASSWORD" ]]; then
  echo "Error: ADMIN_USER and ADMIN_PASSWORD must be defined in '$ENV_FILE'." >&2
  exit 1
fi

command -v jq >/dev/null || { echo "Error: jq is required" >&2; exit 1; }

# The auth server expects the SHA-256 hash of the password, not the plaintext.
PASSWORD_HASH=$(printf '%s' "$ADMIN_PASSWORD" | sha256sum | awk '{print $1}')

RESPONSE=$(curl -s -X POST \
  -H "Content-Type: application/json" \
  -d "{\"username\": \"$ADMIN_USER\", \"password_hash\": \"$PASSWORD_HASH\"}" \
  "$AUTH_URL")

TOKEN=$(echo "$RESPONSE" | jq -r '.token // empty')
if [[ -z "$TOKEN" ]] || [[ "$TOKEN" == "null" ]]; then
  echo "Error: failed to fetch token from auth server." >&2
  echo "Server response: $RESPONSE" >&2
  exit 1
fi

echo "$TOKEN"
