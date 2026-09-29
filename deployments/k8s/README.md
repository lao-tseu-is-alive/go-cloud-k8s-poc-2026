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

Secrets are never committed: `scripts/k8s_smoke_test.sh` creates the
`goeland-secrets` Secret (`DB_PASSWORD`, `GOELAND_DEV_TOKEN`) with random values on
the first run and keeps it afterwards (PostGIS keeps the password of its first
initialization).

```bash
scripts/k8s_smoke_test.sh            # deploy, wait, check probes, version, SPA and a few API calls
scripts/k8s_smoke_test.sh --delete   # same, then delete the namespace
KUBE_CONTEXT=my-k3s scripts/k8s_smoke_test.sh
```

To browse the deployed SPA afterwards:
`kubectl --context rancher-desktop -n goeland-poc port-forward svc/goeland 18090:80`, then
open `http://127.0.0.1:18090` and sign in with the dev token read from the Secret
(`kubectl -n goeland-poc get secret goeland-secrets -o jsonpath='{.data.GOELAND_DEV_TOKEN}' | base64 -d`).
