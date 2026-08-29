// Server-only: relies on next/headers' cookies(), which only works inside
// Server Components, Server Actions, and Route Handlers. Never import this
// from a Client Component.
import { cookies } from "next/headers";

// Reachable by container name on bibliotheca_network in Docker; falls back
// to the Go API's default local port for bare `npm run dev`. Deliberately
// not NEXT_PUBLIC_* — this is never read in the browser, so it's never
// shipped to client JS. See Client/docs/web-app-plan.md's "Two API base
// URLs, not one".
const API_INTERNAL_URL = process.env.API_INTERNAL_URL || "http://localhost:8080";

export class ApiError extends Error {
  constructor(message, status, code) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
  }
}

// Token attachment is a no-op until Step 2 adds the access_token cookie —
// this wrapper's shape already accounts for it so Step 2 doesn't have to
// touch every call site.
async function request(path, { method = "GET", body, headers } = {}) {
  const cookieStore = await cookies();
  const accessToken = cookieStore.get("access_token")?.value;

  const res = await fetch(`${API_INTERNAL_URL}${path}`, {
    method,
    headers: {
      "Content-Type": "application/json",
      ...(accessToken ? { Authorization: `Bearer ${accessToken}` } : {}),
      ...headers,
    },
    body: body !== undefined ? JSON.stringify(body) : undefined,
    cache: "no-store",
  });

  const payload = await res.json().catch(() => null);

  if (!res.ok) {
    const message = payload?.error || `Request failed with status ${res.status}`;
    throw new ApiError(message, res.status, payload?.code);
  }

  return payload?.data;
}

export function apiGet(path) {
  return request(path);
}

export function apiPost(path, body) {
  return request(path, { method: "POST", body });
}

export function apiPatch(path, body) {
  return request(path, { method: "PATCH", body });
}

export function apiDelete(path) {
  return request(path, { method: "DELETE" });
}
