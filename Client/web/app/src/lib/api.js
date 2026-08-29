// Server-only: relies on next/headers' cookies(), which only works inside
// Server Components, Server Actions, and Route Handlers. Never import this
// from a Client Component.
import { cookies } from "next/headers";
import {
  ACCESS_TOKEN_COOKIE,
  REFRESH_TOKEN_COOKIE,
  setSession,
  clearSession,
} from "./session";

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

// The Go API's refresh tokens are one-time-use and rotate on every refresh
// (Server/app/internal/modules/auth) — if two requests both hit a 401 at
// once and each independently calls /auth/refresh, the second call arrives
// with an already-invalidated token and fails. Sharing one in-flight
// promise across concurrent callers in this process avoids that race.
let refreshPromise = null;

// Exported for src/app/api/books/[id]/file/route.js, which streams the raw
// file response itself rather than going through request() above but still
// needs the same one-shot refresh-and-retry on an expired access token.
export function refreshSession() {
  if (!refreshPromise) {
    refreshPromise = performRefresh().finally(() => {
      refreshPromise = null;
    });
  }
  return refreshPromise;
}

async function performRefresh() {
  const cookieStore = await cookies();
  const refreshToken = cookieStore.get(REFRESH_TOKEN_COOKIE)?.value;
  if (!refreshToken) return false;

  const res = await fetch(`${API_INTERNAL_URL}/api/v1/auth/refresh`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ refresh_token: refreshToken }),
    cache: "no-store",
  });

  if (!res.ok) {
    await clearSession();
    return false;
  }

  const payload = await res.json().catch(() => null);
  const token = payload?.data?.token;
  if (!token) {
    await clearSession();
    return false;
  }

  await setSession({
    accessToken: token.access_token,
    refreshToken: token.refresh_token,
    expiresIn: token.expires_in,
  });
  return true;
}

async function request(path, { method = "GET", body, headers, _retried = false } = {}) {
  const cookieStore = await cookies();
  const accessToken = cookieStore.get(ACCESS_TOKEN_COOKIE)?.value;

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

  // A 401 on a request that already carried a token means it expired —
  // try one transparent refresh-and-retry before giving up. A 401 with no
  // token to begin with just means "not logged in," not an expired one, so
  // there's nothing to refresh.
  if (res.status === 401 && accessToken && !_retried && (await refreshSession())) {
    return request(path, { method, body, headers, _retried: true });
  }

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
