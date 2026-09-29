#!/usr/bin/env bash
# k8s_smoke_test.sh — deploy the published image with a disposable PostGIS on a
# local cluster (Rancher Desktop / k3s by default) and check that it really runs:
# rollout, probes, migrations, version, SPA, and a few authenticated API calls.
#
# Usage: scripts/k8s_smoke_test.sh [--delete]
#   KUBE_CONTEXT (default rancher-desktop) selects the cluster; --delete removes
#   the namespace afterwards. Secrets (DB password, dev token) are generated for
#   the run, stored only in the cluster Secret and never printed.
set -euo pipefail

readonly CONTEXT="${KUBE_CONTEXT:-rancher-desktop}"
readonly NAMESPACE="goeland-poc"
readonly MANIFESTS="deployments/k8s"
readonly LOCAL_PORT="${SMOKE_LOCAL_PORT:-18090}"
readonly BASE_URL="http://127.0.0.1:${LOCAL_PORT}"
# FORWARD_PID is the background port-forward, stopped on exit.
FORWARD_PID=""

kc() { kubectl --context "${CONTEXT}" "$@"; }

fail() {
  local message="$1"
  echo "## 💥 ${message}" >&2
  exit 1
}

# create_secret stores freshly generated secrets in the cluster only. An existing
# Secret is kept: PostGIS keeps the password of its first initialization.
create_secret() {
  local db_password dev_token
  if kc -n "${NAMESPACE}" get secret goeland-secrets >/dev/null 2>&1; then
    return 0
  fi
  db_password="$(openssl rand -hex 24)"
  dev_token="$(openssl rand -hex 24)"
  kc -n "${NAMESPACE}" create secret generic goeland-secrets \
    --from-literal=DB_PASSWORD="${db_password}" \
    --from-literal=GOELAND_DEV_TOKEN="${dev_token}" \
    --dry-run=client -o yaml | kc apply -f - >/dev/null
}

# dev_token reads the token back from the Secret (never echoed).
dev_token() {
  kc -n "${NAMESPACE}" get secret goeland-secrets -o jsonpath='{.data.GOELAND_DEV_TOKEN}' | base64 -d
}

# wait_forward waits until the port-forward answers (at most 30s).
wait_forward() {
  local attempt
  for attempt in $(seq 1 30); do
    if curl -s -o /dev/null "${BASE_URL}/health"; then
      return 0
    fi
    sleep 1
  done
  fail "the port-forward to svc/goeland never answered (${attempt} attempts)"
}

# check runs one HTTP check against the port-forward and fails on a non-2xx status.
check() {
  local label="$1" method="$2" path="$3" body="${4:-}"
  local token status
  token="$(dev_token)"
  if [[ -n "${body}" ]]; then
    status="$(curl -s -o /dev/null -w '%{http_code}' -X "${method}" -H "Authorization: Bearer ${token}" \
      -H 'Content-Type: application/json' -d "${body}" "${BASE_URL}${path}" || true)"
  else
    status="$(curl -s -o /dev/null -w '%{http_code}' -X "${method}" -H "Authorization: Bearer ${token}" "${BASE_URL}${path}" || true)"
  fi
  [[ "${status}" == 2* ]] || fail "${label}: HTTP ${status} on ${method} ${path}"
  echo "## ✓ ${label} (${method} ${path} → ${status})"
}

main() {
  local delete="false"
  [[ "${1:-}" == "--delete" ]] && delete="true"
  command -v kubectl >/dev/null || fail "kubectl is required"
  kc get nodes >/dev/null || fail "cluster ${CONTEXT} is not reachable"

  kc apply -f "${MANIFESTS}/00-namespace.yaml" >/dev/null
  create_secret
  kc apply -f "${MANIFESTS}/10-postgis.yaml" -f "${MANIFESTS}/20-goeland.yaml" >/dev/null
  kc -n "${NAMESPACE}" rollout restart deployment/goeland >/dev/null
  echo "## … waiting for the rollouts"
  kc -n "${NAMESPACE}" rollout status deployment/postgis --timeout=180s
  kc -n "${NAMESPACE}" rollout status deployment/goeland --timeout=180s

  kc -n "${NAMESPACE}" port-forward svc/goeland "${LOCAL_PORT}:80" >/dev/null 2>&1 &
  FORWARD_PID=$!
  trap 'if [[ -n "${FORWARD_PID}" ]]; then kill "${FORWARD_PID}" 2>/dev/null || true; fi' EXIT
  wait_forward

  check "liveness" GET /health
  check "readiness (database reachable)" GET /readiness
  check "version" GET /goAppInfo
  check "SPA deep link" GET /cases/some/deep/link
  check "authenticated caller recorded" GET /api/me
  check "migrations seeded the catalogues" GET "/api/org-unit-types?onlyActive=true"
  check "create a case" POST /api/cases '{"caseTypeCode":"GENERIC_REQUEST","title":"Smoke test k8s"}'
  check "search cases" GET "/api/cases/search?query=smoke"
  echo "## ✓ app version: $(curl -s "${BASE_URL}/goAppInfo")"
  echo "## ✓ restarts: $(kc -n "${NAMESPACE}" get pods -l app.kubernetes.io/name=goeland -o jsonpath='{.items[0].status.containerStatuses[0].restartCount}')"

  if [[ "${delete}" == "true" ]]; then
    kc delete namespace "${NAMESPACE}" --wait=false >/dev/null
    echo "## ✓ namespace ${NAMESPACE} deleted"
  fi
}

main "$@"
