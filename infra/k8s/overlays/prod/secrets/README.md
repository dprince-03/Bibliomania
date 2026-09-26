# Production secrets (plaintext — never committed)

`*.env` files here are git-ignored. `make -C infra/k8s seal SEAL_CONTEXT=<prod-context>`
encrypts each one against the production cluster's Sealed Secrets key into
`../sealed/<name>.yaml` (safe to commit — only that cluster can decrypt).
Install the controller there first: `make -C infra/k8s sealed-secrets SEAL_CONTEXT=<prod-context>`.

One file per Secret, named after it:

| File | Secret | Contents |
| --- | --- | --- |
| `bibliomania-secrets.env` | app secrets | every key in `../../local/secrets.env.example`, with real values (`openssl rand -hex 32` for JWT_SECRET / INTERNAL_API_TOKEN) |
| `pg-superuser.env` | CloudNativePG superuser | `username=postgres`, `password=` (= POSTGRES_PASSWORD) |
| `pg-catalog.env`, `pg-borrow.env`, `pg-users.env`, `pg-payment.env` | per-service Postgres roles | `username=catalog` / `borrow` / `users` / `payment`, `password=` (= the matching *_DB_PASSWORD) |
| `alertmanager-secrets.env` | Alertmanager's Slack webhook | `slack-webhook-url=https://hooks.slack.com/services/...` |

`pg-*` files become `kubernetes.io/basic-auth` Secrets (what CloudNativePG
expects); the rest are `Opaque`. Re-run `make seal` after changing any value,
commit `../sealed/`, and Argo CD rolls it out.
