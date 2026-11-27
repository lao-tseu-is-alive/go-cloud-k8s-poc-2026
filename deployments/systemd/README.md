# Goéland on a Linux server with systemd (demo)

These files serve the **synthetic public demo** ([docs/DEMO_SCRIPT.md](../../docs/DEMO_SCRIPT.md))
on a single Linux server, from the release binary only: no Go toolchain, no repository checkout.
The server runs as a systemd service; a second, one-shot service regenerates the demo, by hand
(the equivalent of `scripts/demo_seed.sh -r`) or every night with a timer.

| File | Installed as | Role |
| --- | --- | --- |
| `install.sh` | (run from this directory) | Installs or upgrades everything below, then generates the demo |
| `goeland.service` | `/etc/systemd/system/goeland.service` | The server, as the runtime role, in demo mode |
| `goeland-demo-reset.service` | `/etc/systemd/system/goeland-demo-reset.service` | Stops the server, recreates the demo, starts the server again |
| `goeland-demo-reset.timer` | `/etc/systemd/system/goeland-demo-reset.timer` | Optional: the reset every night at 03:30 |
| `goeland-demo-recreate-db` | `/usr/local/sbin/goeland-demo-recreate-db` | Root step of the reset: drop and recreate the demo database, empty its documents |
| `config.env.sample` | `/etc/goeland/config.env` | Settings of the server (runtime role) |
| `seed.env.sample` | `/etc/goeland/seed.env` | Settings of the reset (schema owner) |

## How it works

- **Two database roles** (GLD-046, [PRODUCTION_READINESS.md](../../docs/PRODUCTION_READINESS.md)):
  the server runs as `goeland_app`, which reads and writes rows but changes no schema
  (`GOELAND_DB_MIGRATE=verify`); the reset runs `demo-seed` and `migrate` as `goeland_owner`, the
  schema owner. `demo-seed` must be the owner: it applies the migrations and re-dates the generated
  history to the story times.
- **No PostgreSQL password for root:** `goeland-demo-recreate-db` creates the database as the
  `postgres` OS user over the local socket (peer authentication), owned by `goeland_owner`, with the
  extensions the migrations need. Only `goeland_owner` and `goeland_app` have passwords, in the two
  env files.
- **The reset** (`systemctl start goeland-demo-reset`): `Conflicts=goeland.service` stops the
  server; the root step drops the database (only if it carries the demo marker `demo_dataset`, like
  `scripts/demo_seed.sh -r`) and empties the documents (only under `/var/lib/goeland/demo_documents`);
  `demo-seed` generates the demo as the `goeland` user, so the server owns its documents; `migrate`
  grants the runtime role; `OnSuccess=goeland.service` starts the server again (systemd 249 or
  later). A failed reset leaves the server stopped: read `journalctl -u goeland-demo-reset`.
- **Demo mode** (`GOELAND_AUTH_MODE=demo`): the sign-in screen lists the demo users and needs no
  password; the server refuses to start on a database `demo-seed` did not generate, so real data is
  never served this way.

## Install

Prerequisites: PostgreSQL with PostGIS (`postgresql-16-postgis-3` or similar) on the same server,
systemd 249 or later, and the release binary from
[GitHub Releases](https://github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/releases).

Copy this directory to the server (a clone of the repository, or just these files), download the
release binary next to `install.sh` and check it against the release's `SHA256.txt`:

```bash
VERSION=v0.26.0   # the release to install
BASE=https://github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/releases/download/${VERSION}
curl -fsSLO "${BASE}/goeland-server-linux-amd64.tar.gz" -O "${BASE}/SHA256.txt"
sha256sum --check --ignore-missing SHA256.txt
tar -xzf goeland-server-linux-amd64.tar.gz

sudo ./install.sh            # French demo, profile from seed.env (demo)
journalctl -u goeland-demo-reset -f   # in another terminal, to follow the generation
```

Options of `install.sh`:

| Option | Effect |
| --- | --- |
| `-b <binary>` | The binary to install (default: `goeland-server-linux-amd64` next to the script, else the one already installed) |
| `-l fr\|en` | Demo language, written only into env files the script creates (default `fr`) |
| `-t` | Also enable the nightly reset timer (03:30) |
| `-n` | Install only: do not generate the demo nor start the server |

`install.sh` is idempotent; it:

- installs the binary;
- creates the `goeland` user and its directories;
- creates the two database roles with random passwords. They are written into `/etc/goeland/*.env` and never printed;
- installs the env files and the units;
- runs the reset, which generates the demo and starts the server.

Whatever already exists is kept: a user, a directory, a role, or an env file. A role that existed
before the script ran has a password the script cannot know. In that case, it stops and asks you to
set that password in the env file. `-n` installs without generating the demo.

The same steps by hand:

```bash
# 1. The binary and the service user
install -m 755 goeland-server-linux-amd64 /usr/local/bin/
groupadd --system goeland
useradd --system --no-create-home --shell /usr/sbin/nologin --gid goeland goeland
install -d -o goeland -g goeland -m 750 /var/lib/goeland /var/lib/goeland/demo_documents

# 2. The database roles (once)
sudo -u postgres psql -c "CREATE ROLE goeland_owner LOGIN PASSWORD '<owner-password>';"
sudo -u postgres psql -c "CREATE ROLE goeland_app LOGIN PASSWORD '<app-password>';"

# 3. The settings: copy the samples, replace the placeholders
install -d -m 750 -g goeland /etc/goeland
install -m 640 -g goeland config.env.sample /etc/goeland/config.env
install -m 600 seed.env.sample /etc/goeland/seed.env
editor /etc/goeland/config.env /etc/goeland/seed.env

# 4. The units and the root step
install -m 750 goeland-demo-recreate-db /usr/local/sbin/
install -m 644 goeland.service goeland-demo-reset.service goeland-demo-reset.timer /etc/systemd/system/
systemctl daemon-reload
systemctl enable goeland.service

# 5. Generate the demo; the server starts once it is ready
systemctl start goeland-demo-reset.service
journalctl -u goeland-demo-reset -f
```

## Reverse proxy

The server listens on `127.0.0.1:8080` only: put a TLS reverse proxy in front of it. It sets its own
security headers (CSP, `X-Frame-Options`, ...); the proxy adds TLS, HSTS and allows uploads as large
as `GOELAND_MAX_UPLOAD_BYTES` (100 MiB by default, nginx refuses more than 1 MB unless told). With
nginx and a Let's Encrypt certificate (`certbot --nginx`):

```nginx
server {
    listen 443 ssl;
    server_name demo.example.org;
    server_tokens off;
    add_header Strict-Transport-Security "max-age=31536000" always;
    client_max_body_size 100m;

    location / {
        proxy_pass http://127.0.0.1:8080/;   # not localhost: it may resolve to ::1 first
    }
    # ssl_certificate ... (managed by certbot)
}
```

## Daily use

```bash
systemctl start goeland-demo-reset           # regenerate the demo now (the "-r")
systemctl enable --now goeland-demo-reset.timer  # ... or every night at 03:30
systemctl status goeland                     # the server
journalctl -u goeland -f                     # its logs
```

Regenerate the demo the day before showing it at the latest: its work in progress (DEMO_SCRIPT
§4.8 to §4.12) is dated from the generation day, so the deadlines (« en retard », « aujourd'hui »)
stay true; a nightly timer also wipes what visitors changed.

**Amount of data:** `GOELAND_DEMO_PROFILE` in `seed.env` is `demo` (the stories and their
background of about 400 cases, [DEMO_SCRIPT.md §5](../../docs/DEMO_SCRIPT.md), a few minutes) or
`small` (the stories only, a few seconds); a `seed.env` without it generates `small`.

**English demo:** set `GOELAND_DEMO_LANG=en` in `seed.env` (and, to keep both, a second database
and document path), then run the reset.

## Upgrade to a new release

1. Read the release's section of [CHANGELOG.md](../../CHANGELOG.md), above all its « Upgrading »
   notes.
2. Update this directory (`git pull` in the clone, or the files of the new tag): the units, the
   samples and `install.sh` may have changed with the release.
3. Download and check the new binary as in [Install](#install).
4. Compare your settings with the samples: `install.sh` keeps existing env files, so a variable a
   release added is not in them yet (`GOELAND_DEMO_PROFILE` came with v0.26.0; a missing one takes
   its default):

   ```bash
   diff <(sed -n 's/=.*//p' seed.env.sample | sort) <(sudo sed -n 's/=.*//p' /etc/goeland/seed.env | sort)
   diff <(sed -n 's/=.*//p' config.env.sample | sort) <(sudo sed -n 's/=.*//p' /etc/goeland/config.env | sort)
   ```

5. Install and regenerate: `sudo ./install.sh -b goeland-server-linux-amd64`. It replaces the
   binary, refreshes the units and runs the reset, which applies the new migrations as the schema
   owner (the server in `verify` mode refuses to start while one is pending) and regenerates the
   demo with the stories of the new version.
6. Check the version served: `curl -s 127.0.0.1:8080/goAppInfo`.

A demo is always regenerated, never migrated in place: what visitors changed is not kept.

## Uninstall

```bash
systemctl disable --now goeland.service goeland-demo-reset.timer
rm /etc/systemd/system/goeland{.service,-demo-reset.service,-demo-reset.timer} /usr/local/sbin/goeland-demo-recreate-db
systemctl daemon-reload
runuser -u postgres -- dropdb goeland_demo
runuser -u postgres -- psql -c 'DROP ROLE goeland_app;' -c 'DROP ROLE goeland_owner;'
rm -rf /etc/goeland /var/lib/goeland /usr/local/bin/goeland-server-linux-amd64
userdel goeland
```

## Simpler variant: one role

For a throw-away demo server, one role may own the schema and run the server: put `goeland_owner`
in both env files, set `GOELAND_DB_MIGRATE=auto` in `config.env`, leave `GOELAND_DB_APP_ROLE`
empty, and remove the `ExecStartPost=` line of `goeland-demo-reset.service`. The server then
migrates at startup; the two-role setup above is the one production uses.

## Troubleshooting

- **The server logs « database not reachable yet, retrying »:** the database does not exist yet, or
  `DATABASE_URL` is set in `config.env` (it wins over `DB_*`: leave it empty). Run the reset.
- **The server refuses to start in demo mode:** the database was not generated by `demo-seed`
  (no `demo_dataset` row). Run the reset.
- **« must connect as the schema owner »:** `seed.env` names the runtime role; it must name
  `goeland_owner`, the owner of the database `goeland-demo-recreate-db` created.
- **« ... has no demo marker: refusing to drop it »:** `DB_NAME` names a database `demo-seed` did
  not generate (perhaps a real one). The reset never drops it; drop it yourself if it is disposable.
- **Password prompts from `psql`:** the root step uses the `postgres` OS user over the socket; if
  `pg_hba.conf` does not allow `peer` for `local` connections of `postgres`, restore that line.
- **Uploads fail with « 413 Request Entity Too Large »:** the reverse proxy limits the request size
  (`client_max_body_size` with nginx, see [Reverse proxy](#reverse-proxy)).
- **The reset is slow:** profile `demo` writes some 8,000 requests (about two minutes on a laptop);
  `TimeoutStartSec=15min` in `goeland-demo-reset.service` leaves room for a small server.

## See also

- [docs/DEMO_SCRIPT.md](../../docs/DEMO_SCRIPT.md): the demo users, the stories to show (§6 gives an
  order) and the background.
- [.env_sample](../../.env_sample): every variable the server reads, with its default.
- [docs/PRODUCTION_READINESS.md](../../docs/PRODUCTION_READINESS.md): what a real deployment needs
  beyond this demo (authentication, backups, object storage, ...).
- [deployments/k8s/README.md](../k8s/README.md): the same server on Kubernetes.
- [scripts/demo_seed.sh](../../scripts/demo_seed.sh): the demo on a development machine, from the
  sources.
