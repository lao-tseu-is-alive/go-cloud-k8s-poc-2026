#!/bin/bash
#
# create_k8s_configmap_from_env.sh
# Render a Kubernetes ConfigMap from the local .env (dry-run, printed as YAML).
# Review the output, then apply it, e.g.:
#   ./scripts/create_k8s_configmap_from_env.sh | kubectl apply -f -
#
# ⚠️ .env contains secrets (DB_PASSWORD, JWT_SECRET). For real deployments put
# secret values in a Kubernetes Secret, not a ConfigMap.
# See https://kubernetes.io/docs/tasks/configure-pod-container/configure-pod-configmap/
set -euo pipefail

ENV_FILE="${1:-.env}"
[ -f "$ENV_FILE" ] || { echo "ERROR: $ENV_FILE not found" >&2; exit 1; }

kubectl create configmap goeland-poc-config \
  --from-env-file="$ENV_FILE" \
  --output yaml --dry-run=client
