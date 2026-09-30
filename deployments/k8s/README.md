# Local Kubernetes smoke deployment

Minimal manifests to check that the published image really runs on a cluster
(Rancher Desktop / k3s by default). **Not a production chart:** PostGIS is
disposable (`emptyDir`), authentication is in `dev` mode and there is one replica
because blob storage is node-local (see [PRODUCTION_READINESS.md](../../docs/PRODUCTION_READINESS.md)).

| File | Content |
|------|---------|
| `00-namespace.yaml` | Namespace `goeland-poc` |
| `10-postgis.yaml` | Disposable PostGIS (`postgis/postgis:17-3.5`) and its Service |
| `20-goeland.yaml` | ConfigMap, hardened Deployment (non-root, read-only root filesystem, no capabilities, probes) and Service |

## Prerequisites

- A local cluster reachable by `kubectl`: Rancher Desktop with Kubernetes enabled
  (context `rancher-desktop`), or any k3s/kind cluster selected with `KUBE_CONTEXT`.
- `kubectl`, `curl`, `openssl` and `base64` on the `PATH`.
- Network access to `ghcr.io` (the image is public) and Docker Hub (PostGIS).

## Deploy and check

```bash
scripts/k8s_smoke_test.sh            # deploy, wait, check probes, version, SPA and a few API calls
scripts/k8s_smoke_test.sh --delete   # same, then delete the namespace
KUBE_CONTEXT=my-k3s scripts/k8s_smoke_test.sh
```

The script applies the manifests, restarts the Goéland Deployment, waits for both
rollouts, opens a temporary port-forward and checks liveness, readiness, version, the
SPA fallback, `/api/me`, the seeded catalogues, and a case creation + search. It ends
with the running version and the pod restart count (expected: `0`).

## Secrets and the dev token

Secrets are never committed. On the first run the script creates the Secret
`goeland-secrets` in namespace `goeland-poc` with two random values
(`openssl rand -hex 24`):

| Key | Used by |
|-----|---------|
| `DB_PASSWORD` | PostGIS (`POSTGRES_PASSWORD`) and the Goéland server |
| `GOELAND_DEV_TOKEN` | the bearer token accepted by the server in `dev` auth mode |

The values are stored only in the cluster and never printed. Later runs **keep** the
existing Secret, because PostGIS keeps the password of its first initialization.

Read the dev token when you need it (prefer the clipboard to your terminal history):

```bash
kubectl --context rancher-desktop -n goeland-poc get secret goeland-secrets \
  -o jsonpath='{.data.GOELAND_DEV_TOKEN}' | base64 -d; echo
# or straight to the clipboard: … | base64 -d | wl-copy   (xclip -selection clipboard on X11)
```

To choose your own token, create the Secret **before the first run** (the script then
keeps it):

```bash
kubectl --context rancher-desktop apply -f deployments/k8s/00-namespace.yaml
kubectl --context rancher-desktop -n goeland-poc create secret generic goeland-secrets \
  --from-literal=DB_PASSWORD="$(openssl rand -hex 24)" \
  --from-literal=GOELAND_DEV_TOKEN='<your-dev-token>'
scripts/k8s_smoke_test.sh
```

The dev user is set by the ConfigMap in `20-goeland.yaml` (`GOELAND_DEV_USER_*`,
administrator enabled) — edit it there, then rerun the script.

## Browse the deployed SPA

```bash
kubectl --context rancher-desktop -n goeland-poc port-forward svc/goeland 18090:80
```

Open `http://127.0.0.1:18090` and sign in with the dev token read above. The REST API
answers on the same port, e.g.
`curl -H "Authorization: Bearer <dev-token>" http://127.0.0.1:18090/api/me`.

## Reset and clean up

Changing only `DB_PASSWORD` in an existing namespace breaks the database connection
(PostGIS keeps its first password). To start over, delete the namespace, wait until it
is gone, then deploy again:

```bash
kubectl --context rancher-desktop delete namespace goeland-poc --wait
scripts/k8s_smoke_test.sh
```

All data (database and uploaded documents) lives in `emptyDir` volumes and disappears
with the pods.

## Troubleshooting

| Symptom | Check |
|---------|-------|
| `goeland` pod restarting | `kubectl -n goeland-poc logs deploy/goeland` — the server waits for the database up to `GOELAND_DB_CONNECT_TIMEOUT_SECONDS` (60 s) before giving up |
| `password authentication failed` in the logs | the Secret changed after PostGIS was initialized: reset (above) |
| `401` from the API or the SPA | wrong token: read it again from the Secret |
| image pull errors | network access to `ghcr.io`, and that the tag in `20-goeland.yaml` is published |
