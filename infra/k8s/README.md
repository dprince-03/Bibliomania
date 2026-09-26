# Bibliomania on Kubernetes

The whole stack as Kubernetes manifests: the gateway + seven Go services,
their databases and brokers, the observability stack, the web clients,
Umami. The same system as the Compose setup (`infra/docker/`), with the
same service names, ports, images, SQL init files and observability
configs.

```text
infra/k8s/
├── base/                      # everything, environment-neutral
│   ├── kustomization.yaml     # resource list + configMapGenerators (see "Shared files")
│   ├── config.yaml            # bibliomania-common ConfigMap (non-secret env for every Go service)
│   ├── services/              # one file per Go service: Deployment + Service (+ headless *-grpc Service)
│   ├── data/                  # mysql, postgres, mongo (+ rs-init Job), redis, seaweedfs, db-backup CronJob
│   ├── brokers/               # rabbitmq, kafka (KRaft), nats (JetStream)
│   ├── observability/         # prometheus (k8s pod discovery) + alertmanager, loki, tempo, alloy, grafana, exporters
│   ├── clients/               # web-app, web-main, admin-web, admin, umami
│   └── edge/ingress.yaml      # host routing (Traefik), /api prefix strip for the admin backend
├── components/                # opt-in pieces an overlay can include
│   ├── ha/                    # replication for every stateful piece (see "High availability")
│   ├── mesh/                  # Linkerd mTLS + gRPC policy (see "Service mesh")
│   └── virus-scan/            # ClamAV + catalog's CLAMAV_ADDR
├── overlays/
│   ├── local/                 # kind: :dev images, generated secrets, Mailpit
│   ├── local-ha/              # local + components/ha + components/mesh
│   └── prod/                  # GHCR images, HPAs, real hostnames + TLS, ha + mesh, SealedSecrets (sealed/)
├── argocd/                    # Argo CD values + the production Application (GitOps)
├── kind/
│   ├── cluster.yaml           # one-node kind cluster; ingress on host ports 9097/9098
│   └── traefik-values.yaml    # ingress controller Helm values
└── Makefile                   # up | up-ha | seed | smoke | status | down, and the targets below
```

## Why these tools

- **Kustomize** (built into `kubectl`) for the manifests: plain YAML, one
  base with small environment overlays, no templating language, no extra
  binary.
- **kind** for the local cluster: Kubernetes-in-Docker, same engine as the
  Compose stack, created and destroyed in about two minutes.
- **Terraform** (`infra/terraform/local`) owns the cluster lifecycle
  declaratively — the plan's Terraform-first, local-first platform layer.
  Moving to a real provider swaps one resource, and the manifests stay the same.
- **Traefik** as the ingress controller. The community ingress-nginx
  controller was retired in March 2026.
- **No Helm chart for the app.** Most third-party infra charts (Bitnami's
  in particular) moved their images behind a paid tier in 2025. Owning
  about 20 plain manifests is simpler and more predictable than depending
  on that.

## Run it locally

Needs `docker`, `kind`, `kubectl`, `helm`, `openssl` (and `terraform` for
the Terraform path).

```bash
# Either — imperative, step by step:
make -C infra/k8s up        # cluster → Traefik → build + load 12 images → secrets → apply → wait
make -C infra/k8s up-ha     # the same, replicated + meshed (overlays/local-ha; needs ~10 GB RAM)

# Or — declarative, same result as `up`:
cd infra/terraform/local && terraform init && terraform apply

make -C infra/k8s seed      # admin@bibliomania.local + sample catalog
make -C infra/k8s smoke     # the same end-to-end test as Compose (infra/docker/smoke-test.sh)
make -C infra/k8s status
make -C infra/k8s down      # or: terraform destroy

# After a backend change — rebuild just the eight Go images, not the clients:
make -C infra/k8s images-go load-go && kubectl -n bibliomania rollout restart deploy
```

`make preload` copies third-party images from the host into the kind node,
so pods don't pull them over the network. `kind load` fails on multi-arch
images when Docker's containerd store only has this machine's platform;
the target then imports the linux/amd64 image directly.

Traefik listens on host port **9097** (HTTP) and **9098** (HTTPS). Routing
is by hostname, so either add the `.local` names to `/etc/hosts` (same
list as `infra/README.md`) and open `http://api.bibliomania.local:9097`,
or send the header: `curl -H 'Host: api.bibliomania.local' localhost:9097/health`.
`seed`/`smoke` use temporary `kubectl port-forward`s instead, so they need
neither.

Secrets for the local cluster come from `overlays/local/secrets.env`
(git-ignored), which `make secrets` creates from `secrets.env.example`
with a random `JWT_SECRET`. After editing it, re-apply and
`kubectl -n bibliomania rollout restart deployment,statefulset`.

## Shared files (why the load restrictor is off)

`base/kustomization.yaml` loads these through `configMapGenerator`, so
Compose and Kubernetes use literally the same files:

- `infra/docker/postgres/init/*.sql`, `infra/docker/mysql/init/*.sql` — database init
- `infra/docker/rabbitmq/enabled_plugins`
- `infra/observability/{loki,tempo}/config.yaml`, Grafana datasources, providers and dashboard

Kustomize refuses files outside the kustomization's directory by default,
so render with `kubectl kustomize --load-restrictor LoadRestrictionsNone`
(the Makefile and Terraform do). Prometheus and Alloy have
Kubernetes-specific configs in `base/observability/`, because they discover
pods through the Kubernetes API instead of fixed hostnames or the Docker socket.

## How it differs from Compose

- **gRPC load balancing**: each gRPC-serving service also has a headless
  `<svc>-grpc` Service, and clients dial `dns:///<svc>-grpc:9090` with
  round-robin (`Server/app/internal/grpcx`), so calls spread across replicas.
  With the mesh on, Linkerd balances per request on top of that.
- **Files**: e-library files live in SeaweedFS (`base/data/seaweedfs.yaml`,
  self-hosted, S3 protocol), so catalog has no volume and scales freely.
- **Probes**: readiness = `/health` (stop traffic while a dependency is
  down); liveness = TCP only (a database blip must not restart pods).
- **Hardening**: Go services run as UID 10001 with a read-only root
  filesystem, no capabilities and a writable `/tmp` volume (the prod
  Dockerfile now uses that fixed numeric UID).
- **MongoDB**: the replica set is initiated by the `mongo-rs-init` Job
  instead of a healthcheck.
- **Metrics**: Prometheus scrapes every Go pod found through the Kubernetes
  API (`prometheus.io/scrape` annotation), not one address per service.

## High availability (`components/ha`, overlay `local-ha`)

| Piece | HA form | Failover |
| --- | --- | --- |
| Postgres (catalog, borrow, user, payment) | CloudNativePG cluster, 3 instances; `postgres` Service follows the primary, `pg-ro` = replicas (catalog's public reads) | automatic — measured **17s** after deleting the primary |
| MySQL (auth) | GTID primary + read-only replica (`mysql-replica`) | manual: `STOP REPLICA; RESET REPLICA ALL; SET PERSIST super_read_only = OFF; SET PERSIST read_only = OFF;` then point the `mysql` Service at it |
| MongoDB (reading) | 3-member replica set | automatic (election) |
| Kafka | 3 KRaft brokers (each also a controller), RF 3 | automatic |
| NATS JetStream | 3-server cluster, streams replicated ×3 | automatic |
| RabbitMQ | RabbitMQ Cluster Operator, 3 nodes, quorum queues | automatic |

Needs operators: `make -C infra/k8s operators` (cert-manager, CloudNativePG,
RabbitMQ Cluster Operator). Verified 2026-09-26: the 55-check smoke test
passed while the Postgres primary and one member each of Kafka, Mongo,
NATS and RabbitMQ were killed at once.

Things learned while verifying it, now encoded in the manifests:
- Quorum StatefulSets use `podManagementPolicy: Parallel`. With the
  default OrderedReady, member 0 waits to be Ready before member 1 exists,
  and a quorum member can't be Ready alone.
- CNPG `smartShutdownTimeout: 15`: the default 180s let pooled
  connections hold a dying primary for 3 minutes.
- The MySQL replica turns read-only on *after* its first-boot setup.
  `--super-read-only` as a flag blocked the entrypoint's own init.
- An env var that references another (`$(VAR)`) must come after it. Add
  such vars with a JSON-patch append, not a strategic merge.

## Service mesh (`components/mesh`)

Linkerd (`make -C infra/k8s mesh` installs it). The first run generates
the mesh CA and issuer into `mesh-certs/` (git-ignored).

- **mTLS** between the eight Go services, with identities from their
  ServiceAccount.
- **gRPC ports (9090) accept meshed clients only** (`policy.yaml`). A pod
  outside the mesh gets `PermissionDenied`. HTTP ports stay open to
  Traefik, Prometheus and kubelet probes.
- **Per-request, latency-aware gRPC balancing** that sees new pods at once.
- Databases and brokers are **not** meshed
  (`config.linkerd.io/skip-outbound-ports`).
- In production, let cert-manager issue and rotate the identity issuer
  instead of the one-year one from `make mesh-certs`.

## Secrets (Sealed Secrets)

Production Secrets are committed **encrypted**, as `SealedSecret`s that
only the target cluster's controller can decrypt:

```bash
make -C infra/k8s sealed-secrets SEAL_CONTEXT=<prod-context>   # controller, once
# fill in overlays/prod/secrets/*.env (git-ignored; see its README.md), then:
make -C infra/k8s seal SEAL_CONTEXT=<prod-context>             # → overlays/prod/sealed/*.yaml — commit these
```

## GitOps (Argo CD)

`make -C infra/k8s argocd ARGOCD_CONTEXT=<prod-context>` installs Argo CD
and `argocd/application-prod.yaml`, which keeps the cluster equal to
`overlays/prod` on `main` (auto-sync, prune, self-heal). After CD pushes
images, `app-cd.yml` → `promote` pins their tags in `overlays/prod` to the
commit SHA and commits that. Argo CD rolls it out, and `git revert` rolls
back. Argo CD runs kustomize with `--load-restrictor LoadRestrictionsNone`
(`argocd/values.yaml`), like the Makefile. Status: verified by server-side
dry run only; a real sync needs this branch on `main` and a production
cluster.

## Pod security

Every workload runs non-root, with a read-only root filesystem, no
privilege escalation, all capabilities dropped and the RuntimeDefault
seccomp profile; writable paths are emptyDirs. CI's Trivy scan fails on
any new HIGH finding. To add a workload, find its writable paths by
running the image with `docker run --read-only --user <uid> --cap-drop ALL`
first.

On an **existing** cluster from before 2026-09-26, volumes written by
containers that used to run as root (NATS, SeaweedFS) need a one-time
`chown -R 1000:1000`, since kind's local-path storage ignores `fsGroup`.
Completed Jobs (`mongo-rs-init`) must be deleted before re-applying. Also
raise the host's inotify limit (`sudo sysctl fs.inotify.max_user_instances=512`)
if kind runs alongside Compose — at 128, kube-proxy fails with "too many
open files".

## Production

`overlays/prod` is a starting point. No production cluster exists yet.
Before using it:

1. Secrets: `make sealed-secrets seal` (above).
2. Replace `bibliomania.example.com` (the ingress patch and the ConfigMap
   patch) and install cert-manager (the ingress expects a `letsencrypt`
   ClusterIssuer).
3. Install metrics-server (the HPAs need it), the HA operators
   (`make operators`), Linkerd (`make mesh`) and Argo CD (`make argocd`).
4. Alertmanager routes to Slack: its webhook URL is the
   `alertmanager-secrets` Secret.
5. Managed databases are still worth weighing against the in-cluster HA
   setup, above all for MySQL, which has no automatic failover here.
