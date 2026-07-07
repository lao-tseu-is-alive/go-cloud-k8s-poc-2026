#!/bin/bash
#
# create_k8s_configmap_from_env.sh
# Render a Kubernetes ConfigMap from ONLY the non-secret keys of .env, and print
# guidance (not values) for creating the Secret separately.
#
# Rationale: the previous version passed the whole .env (including DB_PASSWORD,
# JWT_SECRET, DATABASE_URL, dev/admin tokens) to `kubectl create configmap` and
# rendered every value as YAML — publishing secrets into a ConfigMap and to stdout.
# Secrets must live in a Kubernetes Secret, and this script never echoes their values.
#
# Usage:
#   ./scripts/create_k8s_configmap_from_env.sh [env-file] | kubectl apply -f -
# See https://kubernetes.io/docs/concepts/configuration/secret/
set -euo pipefail

ENV_FILE="${1:-.env}"
[ -f "$ENV_FILE" ] || { echo "ERROR: $ENV_FILE not found" >&2; exit 1; }

# Keys that are safe to expose in a ConfigMap.
NON_SECRET_KEYS=(
  DB_DRIVER DB_HOST DB_PORT DB_NAME DB_USER DB_SSL_MODE
  GOELAND_LISTEN_ADDRESS GOELAND_SHUTDOWN_TIMEOUT_SECONDS GOELAND_REQUEST_TIMEOUT_SECONDS
  GOELAND_DB_MAX_CONNECTIONS GOELAND_AUTH_MODE AUTH_SERVER_URL LOG_LEVEL
  GOELAND_DEV_USER_ID GOELAND_DEV_USER_EMAIL GOELAND_DEV_USER_NAME
  JWT_ISSUER_ID JWT_CONTEXT_KEY JWT_DURATION_MINUTES
)

# Keys that must go into a Secret — their VALUES are never read or printed here.
SECRET_KEYS=(DB_PASSWORD DATABASE_URL JWT_SECRET GOELAND_DEV_TOKEN ADMIN_PASSWORD)

# Build the ConfigMap from non-secret keys present in the env file.
cm_args=()
for key in "${NON_SECRET_KEYS[@]}"; do
  value="$(grep -E "^${key}=" "$ENV_FILE" | head -1 | cut -d= -f2-)"
  [ -n "$value" ] && cm_args+=(--from-literal="${key}=${value}")
done

if [ ${#cm_args[@]} -eq 0 ]; then
  echo "ERROR: no non-secret keys found in $ENV_FILE" >&2
  exit 1
fi

echo "# --- ConfigMap: non-secret configuration ---"
kubectl create configmap goeland-poc-config "${cm_args[@]}" --output yaml --dry-run=client

echo
echo "# --- Secret: create separately (values are NOT printed by this script) ---"
secret_flags=""
for key in "${SECRET_KEYS[@]}"; do secret_flags+=" --from-literal=${key}=\$${key}"; done
echo "#   set -a; . ${ENV_FILE}; set +a   # load secrets into your shell (not into git/logs)"
echo "#   kubectl create secret generic goeland-poc-secrets${secret_flags}"
