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
if [ ! -f "$ENV_FILE" ]; then
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

if [ -z "$ADMIN_USER" ] || [ -z "$ADMIN_PASSWORD" ]; then
  echo "Error: ADMIN_USER and ADMIN_PASSWORD must be defined in '$ENV_FILE'." >&2
  exit 1
fi

command -v jq >/dev/null || { echo "Error: jq is required" >&2; exit 1; }

# The auth server expects the SHA-256 hash of the password, not the plaintext.
PASSWORD_HASH=$(printf '%s' "$ADMIN_PASSWORD" | sha256sum | awk '{print $1}')

RESPONSE=$(curl -s -X POST \
  -H "Content-Type: application/json" \
  -d "{\"username\": \"$ADMIN_USER\", \"password_hash\": \"$PASSWORD_HASH\"}" \
  "http://$AUTH_HOST:$AUTH_PORT/login")

TOKEN=$(echo "$RESPONSE" | jq -r '.token // empty')
if [ -z "$TOKEN" ] || [ "$TOKEN" = "null" ]; then
  echo "Error: failed to fetch token from auth server." >&2
  echo "Server response: $RESPONSE" >&2
  exit 1
fi

echo "$TOKEN"
