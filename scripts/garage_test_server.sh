#!/usr/bin/env bash
# Starts or stops a disposable single-node Garage (S3-compatible store) for the
# S3 blob store tests (GLD-020), locally and in CI. Its RPC secret and access
# key are generated for this container only and written to an env file (mode
# 600), never printed.
#
#   scripts/garage_test_server.sh start <env-file>   # then: set -a; . <env-file>; set +a
#   scripts/garage_test_server.sh stop
#
# CONTAINER_CLI picks docker or podman (default: docker, else podman);
# GARAGE_TEST_PORT is the loopback port of its S3 API (default 3910, clear of
# a local Garage on 3900).
set -euo pipefail

readonly IMAGE="docker.io/dxflrs/garage:v2.3.0@sha256:866bd13ed2038ba7e7190e840482bc27234c4afaf77be8cfa439ae088c1e4690"
readonly NAME="goeland-garage-test"
readonly BUCKET="goeland-test"
readonly REGION="garage"
readonly PORT="${GARAGE_TEST_PORT:-3910}"

die() {
  echo "garage_test_server: $*" >&2
  exit 1
}

container_cli() {
  if [[ -n "${CONTAINER_CLI:-}" ]]; then
    echo "${CONTAINER_CLI}"
  elif command -v docker >/dev/null 2>&1 && docker info >/dev/null 2>&1; then
    echo docker
  elif command -v podman >/dev/null 2>&1; then
    echo podman
  else
    die "neither docker nor podman is available"
  fi
}

garage() {
  "${CLI}" exec -e RUST_LOG=warn "${NAME}" /garage "$@"
}

write_config() {
  local config_file="$1"
  cat >"${config_file}" <<EOF
metadata_dir = "/var/lib/garage/meta"
data_dir = "/var/lib/garage/data"
db_engine = "sqlite"
replication_factor = 1
rpc_bind_addr = "[::]:3901"
rpc_public_addr = "127.0.0.1:3901"
rpc_secret = "$(openssl rand -hex 32)"

[s3_api]
s3_region = "${REGION}"
api_bind_addr = "[::]:3900"
EOF
  chmod 644 "${config_file}"
}

wait_ready() {
  local attempt
  for attempt in $(seq 1 30); do
    if garage status >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  "${CLI}" logs "${NAME}" >&2 || true
  die "Garage did not start after ${attempt} s"
}

start() {
  local env_file="$1"
  local work_dir node_id key_id key_secret
  [[ -n "${env_file}" ]] || die "usage: $0 start <env-file>"
  work_dir="$(mktemp -d)"
  write_config "${work_dir}/garage.toml"
  "${CLI}" rm -f "${NAME}" >/dev/null 2>&1 || true
  "${CLI}" run -d --name "${NAME}" -p "127.0.0.1:${PORT}:3900" \
    -v "${work_dir}/garage.toml:/etc/garage.toml:ro" "${IMAGE}" >/dev/null
  wait_ready

  node_id="$(garage node id -q 2>/dev/null | cut -d@ -f1)"
  garage layout assign -z dc1 -c 1G "${node_id}" >/dev/null
  garage layout apply --version 1 >/dev/null
  key_id="GK$(openssl rand -hex 12)"
  key_secret="$(openssl rand -hex 32)"
  garage key import --yes -n "${BUCKET}" "${key_id}" "${key_secret}" >/dev/null
  garage bucket create "${BUCKET}" >/dev/null
  garage bucket allow --read --write --owner "${BUCKET}" --key "${BUCKET}" >/dev/null

  (
    umask 077
    cat >"${env_file}" <<EOF
GOELAND_TEST_S3_ENDPOINT=http://127.0.0.1:${PORT}
GOELAND_TEST_S3_REGION=${REGION}
GOELAND_TEST_S3_BUCKET=${BUCKET}
GOELAND_TEST_S3_ACCESS_KEY_ID=${key_id}
GOELAND_TEST_S3_SECRET_ACCESS_KEY=${key_secret}
EOF
  )
  echo "Garage test server ready on 127.0.0.1:${PORT}, bucket ${BUCKET}; settings in ${env_file}"
}

stop() {
  "${CLI}" rm -f "${NAME}" >/dev/null 2>&1 || true
  echo "Garage test server removed"
}

CLI="$(container_cli)"
readonly CLI
case "${1:-}" in
  start) start "${2:-}" ;;
  stop) stop ;;
  *) die "usage: $0 start <env-file> | stop" ;;
esac
