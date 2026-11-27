#!/usr/bin/env bash
#
# install.sh — install (or upgrade) the Goéland synthetic demo on a Linux server with systemd
# (deployments/systemd/README.md). Run as root from a copy of this directory:
#
#   sudo ./install.sh [-b <binary>] [-l fr|en] [-t] [-n]
#
#   -b <binary>  the release binary to install (default: ./goeland-server-linux-amd64 next to
#                this script, else the one already in /usr/local/bin)
#   -l fr|en     demo language, used only when the env files are created (default: fr)
#   -t           also enable the nightly reset timer (03:30)
#   -n           install only: do not generate the demo nor start the server
#
# Idempotent: an existing user, directory, role or env file is kept. A database role created
# here gets a random password, written into its env file and never printed. Without -n, it
# ends with `systemctl start goeland-demo-reset`, which generates the demo then starts the server.

set -euo pipefail

readonly SERVICE_USER=goeland
readonly BIN_NAME=goeland-server-linux-amd64
readonly BIN_PATH="/usr/local/bin/${BIN_NAME}"
readonly STATE_DIR=/var/lib/goeland
readonly DOCS_ROOT="${STATE_DIR}/demo_documents"
readonly CONF_DIR=/etc/goeland
readonly UNIT_DIR=/etc/systemd/system
readonly OWNER_ROLE=goeland_owner
readonly APP_ROLE=goeland_app
HERE="$(dirname "$(readlink -f "$0")")"
readonly HERE

binary=""
lang=fr
enable_timer=false
start_demo=true

log() {
    local message="$1"
    echo "==> ${message}"
}

fail() {
    local message="$1"
    echo "install.sh: ${message}" >&2
    exit 1
}

usage() {
    sed -n '3,16p' "$0" | sed 's/^# \{0,1\}//' >&2
    exit 2
}

parse_args() {
    local opt
    while getopts ':b:l:tnh' opt; do
        case "$opt" in
            b) binary="$OPTARG" ;;
            l) lang="$OPTARG" ;;
            t) enable_timer=true ;;
            n) start_demo=false ;;
            *) usage ;;
        esac
    done
    [[ "$lang" == fr || "$lang" == en ]] || fail "-l takes fr or en, not '${lang}'"
}

check_prerequisites() {
    [[ "$(id -u)" -eq 0 ]] || fail "run as root (sudo ./install.sh)"
    local tool
    for tool in systemctl runuser psql openssl install; do
        command -v "$tool" >/dev/null || fail "'${tool}' not found"
    done
    id postgres >/dev/null 2>&1 || fail "no postgres OS user: install PostgreSQL with PostGIS first"
    local file
    for file in goeland.service goeland-demo-reset.service goeland-demo-reset.timer \
        goeland-demo-recreate-db config.env.sample seed.env.sample; do
        [[ -f "${HERE}/${file}" ]] || fail "missing ${HERE}/${file}"
    done
}

install_binary() {
    if [[ -z "$binary" && -f "${HERE}/${BIN_NAME}" ]]; then
        binary="${HERE}/${BIN_NAME}"
    fi
    if [[ -n "$binary" ]]; then
        [[ -f "$binary" ]] || fail "binary not found: ${binary}"
        log "installing ${binary} as ${BIN_PATH}"
        install -o root -g root -m 755 "$binary" "$BIN_PATH"
    fi
    [[ -x "$BIN_PATH" ]] || fail "no binary: download ${BIN_NAME} from the GitHub release and pass -b"
    log "binary: $("$BIN_PATH" --version 2>&1 | head -n 1)"
}

create_service_user() {
    if ! getent group "$SERVICE_USER" >/dev/null; then
        log "creating group ${SERVICE_USER}"
        groupadd --system "$SERVICE_USER"
    fi
    if ! id "$SERVICE_USER" >/dev/null 2>&1; then
        log "creating system user ${SERVICE_USER}"
        useradd --system --no-create-home --shell /usr/sbin/nologin \
            --gid "$SERVICE_USER" --home-dir "$STATE_DIR" "$SERVICE_USER"
    fi
    install -d -o "$SERVICE_USER" -g "$SERVICE_USER" -m 750 "$STATE_DIR" "$DOCS_ROOT"
    install -d -o root -g "$SERVICE_USER" -m 750 "$CONF_DIR"
}

# role_exists succeeds when the PostgreSQL role exists.
role_exists() {
    local role="$1"
    [[ "$(runuser -u postgres -- psql -X -tAq -c \
        "SELECT 1 FROM pg_roles WHERE rolname = '${role}'")" == 1 ]]
}

# ensure_role creates the login role with a random password and prints that password,
# or prints nothing when the role already exists.
ensure_role() {
    local role="$1"
    if role_exists "$role"; then
        return 0
    fi
    local password
    password="$(openssl rand -hex 24)"
    # The password goes through stdin, never on a command line visible in ps.
    printf "CREATE ROLE %s LOGIN PASSWORD '%s';\n" "$role" "$password" |
        runuser -u postgres -- psql -X -q -v ON_ERROR_STOP=1 >/dev/null
    printf '%s' "$password"
}

# install_env_file installs the sample as target when target is absent (language applied),
# then writes password as DB_PASSWORD when one was generated for a new role.
install_env_file() {
    local sample="$1" target="$2" mode="$3" group="$4" password="$5"
    if [[ ! -f "$target" ]]; then
        log "creating ${target}"
        install -o root -g "$group" -m "$mode" "${HERE}/${sample}" "$target"
        sed -i -e "s|^GOELAND_DEMO_LANG=.*|GOELAND_DEMO_LANG=${lang}|" \
            -e "s|^GOELAND_DOCUMENT_PATH=.*|GOELAND_DOCUMENT_PATH=${DOCS_ROOT}/${lang}|" "$target"
    else
        log "keeping ${target}"
    fi
    if [[ -n "$password" ]]; then
        sed -i "s|^DB_PASSWORD=.*|DB_PASSWORD=${password}|" "$target"
    fi
}

# require_passwords stops when an env file still holds the sample's password placeholder: its
# role existed before, so its password is not known here. Checked once both files are written,
# so a password generated for the other role is never lost.
require_passwords() {
    local target missing=false
    for target in "$@"; do
        if grep -q '^DB_PASSWORD=<' "$target"; then
            echo "install.sh: ${target}: set DB_PASSWORD (its role existed already), then rerun" >&2
            missing=true
        fi
    done
    [[ "$missing" == false ]] || exit 1
}

configure_database_and_settings() {
    log "database roles ${OWNER_ROLE} and ${APP_ROLE}"
    local owner_password app_password
    owner_password="$(ensure_role "$OWNER_ROLE")"
    app_password="$(ensure_role "$APP_ROLE")"
    install_env_file seed.env.sample "${CONF_DIR}/seed.env" 600 root "$owner_password"
    install_env_file config.env.sample "${CONF_DIR}/config.env" 640 "$SERVICE_USER" "$app_password"
    require_passwords "${CONF_DIR}/seed.env" "${CONF_DIR}/config.env"
}

install_units() {
    log "installing the units and the reset's root step"
    install -o root -g root -m 750 "${HERE}/goeland-demo-recreate-db" /usr/local/sbin/
    install -o root -g root -m 644 "${HERE}/goeland.service" "${HERE}/goeland-demo-reset.service" \
        "${HERE}/goeland-demo-reset.timer" "$UNIT_DIR/"
    systemctl daemon-reload
    systemctl enable goeland.service
    if [[ "$enable_timer" == true ]]; then
        systemctl enable --now goeland-demo-reset.timer
    fi
}

start() {
    if [[ "$start_demo" != true ]]; then
        log "done; generate the demo with: systemctl start goeland-demo-reset"
        return 0
    fi
    log "generating the demo (a few minutes), then starting the server"
    if ! systemctl start goeland-demo-reset.service; then
        echo "install.sh: the demo reset failed; read: journalctl -u goeland-demo-reset" >&2
        exit 1
    fi
    systemctl --no-pager status goeland.service || true
    log "done; the server listens on the address of ${CONF_DIR}/config.env (GOELAND_LISTEN_ADDRESS)"
}

main() {
    parse_args "$@"
    check_prerequisites
    install_binary
    create_service_user
    configure_database_and_settings
    install_units
    start
}

main "$@"
