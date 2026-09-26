// k6 load test for the gateway (REST + GraphQL). Plain JavaScript, like
// every JS file in this repo. Needs a seeded stack (catalog has books).
//
//   docker run --rm -i --network host -e BASE_URL=http://localhost:9081 \
//     grafana/k6:1.3.0 run - < infra/loadtest/load.js
//
// Two scenarios:
//   browse  — anonymous catalog reads (REST list/search + GraphQL) at a fixed
//             arrival rate, the bulk of real traffic;
//   members — logged-in readers doing the full write path: borrow (Saga,
//             with an Idempotency-Key) → reading progress → return.
//
// Rate limits: the gateway allows RATE_LIMIT_RPS (100) per client IP and 5
// logins/registrations per burst, and every k6 VU shares one IP. The
// defaults stay under both; to push harder, raise RATE_LIMIT_RPS on the
// target (never in production) — otherwise you're measuring the limiter.
import http from "k6/http";
import { check, fail, sleep } from "k6";
import { Counter, Trend } from "k6/metrics";

const BASE = __ENV.BASE_URL || "http://localhost:9081";
const API = `${BASE}/api/v1`;
const RATE = Number(__ENV.RATE || 40); // browse requests/s
const DURATION = __ENV.DURATION || "1m";
const MEMBERS = Number(__ENV.MEMBERS || 3); // ≤ 5 (auth burst limit)
const JSON_HDR = { "Content-Type": "application/json" };

const rateLimited = new Counter("rate_limited");
const borrowCycle = new Trend("borrow_cycle_duration", true);

// 409 is a legitimate answer under concurrency (e.g. already borrowed), not
// a failure of the system under test.
http.setResponseCallback(http.expectedStatuses({ min: 200, max: 299 }, 409));

export const options = {
  scenarios: {
    browse: {
      executor: "constant-arrival-rate",
      rate: RATE,
      timeUnit: "1s",
      duration: DURATION,
      preAllocatedVUs: 20,
      maxVUs: 60,
      exec: "browse",
    },
    members: {
      executor: "constant-vus",
      vus: MEMBERS,
      duration: DURATION,
      exec: "member",
    },
  },
  thresholds: {
    http_req_failed: ["rate<0.01"],
    "http_req_duration{kind:read}": ["p(95)<500"],
    "http_req_duration{kind:write}": ["p(95)<1500"],
    rate_limited: ["count==0"],
    checks: ["rate>0.99"],
  },
};

function track(res) {
  if (res.status === 429) rateLimited.add(1);
  return res;
}

export function setup() {
  const books = http.get(`${API}/books?limit=20`).json("data.items");
  if (!books || books.length === 0) fail("no books — seed the stack first (make seed)");
  const users = [];
  for (let i = 0; i < MEMBERS; i++) {
    const email = `k6-${Date.now()}-${i}@example.com`;
    const res = http.post(`${API}/auth/register`,
      JSON.stringify({ first_name: "Load", last_name: "Test", email, password: "Password123!" }),
      { headers: JSON_HDR });
    if (res.status !== 201 && res.status !== 200) fail(`register ${email}: ${res.status} ${res.body}`);
    users.push(res.json("data.token.access_token"));
  }
  // user-service learns about new accounts from an event (eventually
  // consistent), so `me` 404s for a moment after registering. Wait for it
  // rather than counting that window as errors.
  for (const token of users) {
    let ready = false;
    for (let i = 0; i < 30 && !ready; i++) {
      ready = http.get(`${API}/users/me`, { headers: { Authorization: `Bearer ${token}` } }).status === 200;
      if (!ready) sleep(0.5);
    }
    if (!ready) fail("user-service never saw the registered users (is auth → user event flow working?)");
  }
  return { bookIds: books.map((b) => b.id), users };
}

const QUERIES = ["dune", "hobbit", "the", "science", "history"];

export function browse(data) {
  const r = Math.random();
  let res;
  if (r < 0.4) {
    res = http.get(`${API}/books?page=${1 + Math.floor(Math.random() * 3)}&limit=10`, { tags: { kind: "read", name: "GET /books" } });
  } else if (r < 0.7) {
    const q = QUERIES[Math.floor(Math.random() * QUERIES.length)];
    res = http.get(`${API}/search?q=${q}`, { tags: { kind: "read", name: "GET /search" } });
  } else {
    const id = data.bookIds[Math.floor(Math.random() * data.bookIds.length)];
    res = http.post(`${BASE}/graphql`,
      JSON.stringify({ query: `{ book(id: "${id}") { title availableCopies authors { lastName } } }` }),
      { headers: JSON_HDR, tags: { kind: "read", name: "POST /graphql book" } });
  }
  track(res);
  check(res, { "browse 2xx": (x) => x.status >= 200 && x.status < 300 });
}

export function member(data) {
  const token = data.users[(__VU - 1) % data.users.length];
  const auth = { ...JSON_HDR, Authorization: `Bearer ${token}` };
  const bookId = data.bookIds[Math.floor(Math.random() * data.bookIds.length)];
  const start = Date.now();

  const borrow = track(http.post(`${API}/borrows`, JSON.stringify({ book_id: bookId }), {
    headers: { ...auth, "Idempotency-Key": `k6-${__VU}-${__ITER}-${Date.now()}` },
    tags: { kind: "write", name: "POST /borrows" },
  }));
  // 409: no copies left, or this member already holds it — both fine.
  check(borrow, { "borrow 201/409": (x) => x.status === 201 || x.status === 409 });

  const now = new Date().toISOString();
  const progress = track(http.patch(`${API}/reading/${bookId}/sync`,
    JSON.stringify({ current_page: 1 + (__ITER % 300), total_pages: 300, client_updated_at: now }),
    { headers: auth, tags: { kind: "write", name: "PATCH /reading/{id}/sync" } }));
  check(progress, { "progress 2xx": (x) => x.status >= 200 && x.status < 300 });

  const gql = track(http.post(`${BASE}/graphql`,
    JSON.stringify({ query: "{ me { email } myBorrows(limit: 5) { items { id status } } }" }),
    { headers: auth, tags: { kind: "read", name: "POST /graphql me" } }));
  check(gql, { "graphql no errors": (x) => x.status === 200 && !x.json("errors") });

  if (borrow.status === 201) {
    const id = borrow.json("data.public_id") || borrow.json("data.id");
    const ret = track(http.patch(`${API}/borrows/${id}/return`, null, {
      headers: auth, tags: { kind: "write", name: "PATCH /borrows/{id}/return" },
    }));
    check(ret, { "return 200": (x) => x.status === 200 });
    borrowCycle.add(Date.now() - start);
  }
  // Think time: a person reads between actions. Without it three VUs loop
  // ~1,500 times/s and all you measure is the rate limiter.
  sleep(1 + Math.random());
}

// Give back anything still borrowed (an iteration cut off at the end of the
// run can leave one open), so the load test never drains the catalog for
// whoever uses the stack next — the smoke test expects copies available.
export function teardown(data) {
  for (const token of data.users) {
    const auth = { Authorization: `Bearer ${token}` };
    const mine = http.get(`${API}/borrows/my?limit=100`, { headers: auth }).json("data.items") || [];
    for (const b of mine) {
      if (b.status !== "returned") {
        http.patch(`${API}/borrows/${b.public_id || b.id}/return`, null, { headers: auth });
      }
    }
  }
}

