# API docs — where to look

Two hand-written references, not generated (see `Server/docs/Steps.md` →
Step 18 for the planned swaggo/OpenAPI pipeline, still future work):

- **[`Server/docs/API.md`](../Server/docs/API.md)** — the actual endpoint
  reference: routes, request/response shapes, auth requirements. Start here
  for anything about what the API does.
- **[`Client/docs/API.md`](../Client/docs/API.md)** — which `Client/` app
  talks to what, and the base URL per environment. Doesn't repeat the
  endpoint list — start here only for the frontend-integration angle.
