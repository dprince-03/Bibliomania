# API docs — where to look

- **The Swagger UI** (`http://localhost:8080/swagger/index.html` when the server is running) — the actual, generated endpoint reference: routes, request/response shapes, auth requirements. Always in sync with the code (see `Server/docs/Steps.md` → Step 18). Start here for anything about what the API does.
- **[`Server/docs/API.md`](../Server/docs/API.md)** — cross-cutting conventions the Swagger UI doesn't cover per-endpoint (the response envelope, pagination format), plus how to regenerate the spec after changing a handler.
- **[`Client/docs/API.md`](../Client/docs/API.md)** — which `Client/` app talks to what, and the base URL per environment. Doesn't repeat the endpoint list — start here only for the frontend-integration angle.
