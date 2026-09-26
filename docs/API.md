# API docs — where to look

Every client talks to one **gateway** (`Server/app/cmd/gateway`), which fronts the seven microservices.

- **The Swagger UI** (`http://localhost:9081/swagger/index.html` with the dev stack running, the gateway's port) — the generated REST reference for every service: routes, request/response shapes, auth requirements. Always in sync with the code. Start here for anything about what the REST API does.
- **[`Server/app/docs/API.md`](../Server/app/docs/API.md)** — conventions the Swagger UI doesn't cover per endpoint: the envelope, pagination, **API versioning** (REST, GraphQL, gRPC, events), idempotency, GraphQL errors, and the behaviour changes from the microservices split.
- **GraphQL** — `POST /graphql` on the gateway, with an interactive playground at `GET /playground` in dev. Schema: [`Server/app/internal/gateway/graph/schema.graphqls`](../Server/app/internal/gateway/graph/schema.graphqls).
- **Internal contracts** (service to service, not client-facing): gRPC in [`Server/app/proto/`](../Server/app/proto/), event types in `Server/app/internal/events/types.go`. [`docs/interfaces/`](interfaces/) holds the superseded mock-up drafts.
- **[`Client/docs/API.md`](../Client/docs/API.md)** — which `Client/` app talks to what, and the base URL per environment. Doesn't repeat the endpoint list — start here only for the frontend-integration angle.
