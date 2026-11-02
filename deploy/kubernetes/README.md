# Kubernetes manifests for Argus

These manifests deploy Argus with a single Redis instance for template state. They are intended as a starting point: point `ALERT_WEBHOOK_URL` at your Tier-2 webhook, tune `meta.env` / `meta.service`, and switch log inputs for your workload (stdin sidecar or shared log files).

## Prerequisites

- Kubernetes 1.24+
- `kubectl` and `kustomize` (built into kubectl 1.14+)
- Container image published to GitHub Container Registry (see below)

## Image

After the [Publish container image](../../.github/workflows/publish-container.yml) workflow runs on `main`, pull:

```text
ghcr.io/orenvill/argus:latest
```

Pin a digest or tag (for example `ghcr.io/orenvill/argus:<git-sha>`) in production. Update `images.newTag` in `kustomization.yaml` or patch the Deployment.

Public packages: link the repo on GitHub under **Packages** and set package visibility to public if pulls fail with `403`.

## Quick apply

1. Create the webhook secret (required for `argus run`):

```bash
kubectl create namespace argus --dry-run=client -o yaml | kubectl apply -f -
kubectl -n argus create secret generic argus-secrets \
  --from-literal=alert-webhook-url='https://example.com/hook' \
  --dry-run=client -o yaml | kubectl apply -f -
```

2. Deploy the stack:

```bash
kubectl apply -k deploy/kubernetes
```

3. Check status:

```bash
kubectl -n argus get pods,svc
kubectl -n argus port-forward svc/argus-control 9090:9090
curl -sS http://127.0.0.1:9090/healthz
```

## Configuration

| Item | Location |
| --- | --- |
| Argus YAML | `configmap.yaml` → mounted at `/config/argus.yaml` |
| Webhook URL | Secret `argus-secrets` key `alert-webhook-url` |
| Env / service | Deployment env `ARGUS_ENV`, `ARGUS_SERVICE` (expanded into config) |
| Redis | In-cluster `redis` Service (replace address in ConfigMap for external Redis) |

### Sidecar / file logs

The default ConfigMap enables `inputs.stdin: true` so the pod starts without mounted log files. For file tailing next to an app container:

1. Set `inputs.stdin: false` and list paths under `inputs.files`.
2. Mount a shared `emptyDir` or `volume` at the same path in both containers.

See [`docs/SPEC.md`](../../docs/SPEC.md) for pipeline and control API details.

## Docker only (no Kubernetes)

```bash
docker build -t argus:local .
docker run --rm -e ARGUS_ENV=dev -e ARGUS_SERVICE=demo \
  -e ARGUS_REDIS_ADDRESS=host.docker.internal:6379 \
  -e ALERT_WEBHOOK_URL=https://example.com/hook \
  -p 9090:9090 argus:local
```

The image defaults to `config/argus.container.yaml` baked in at `/etc/argus/argus.yaml`; override with `-v $(pwd)/my.yaml:/etc/argus/argus.yaml` and `-c /etc/argus/argus.yaml`.
