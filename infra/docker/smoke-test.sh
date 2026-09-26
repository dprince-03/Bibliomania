#!/usr/bin/env bash
# End-to-end smoke test for the microservices stack — every cross-service
# path, through the gateway, against a running dev stack that has been
# seeded (`make docker-up && make seed` from Server/app). Needs curl + jq.
#
#   infra/docker/smoke-test.sh [gateway-url] [mailpit-url]
#
# Exits non-zero on the first failed check. Creates a fresh member each
# run, so it can be re-run against the same stack.
set -euo pipefail

GW="${1:-http://localhost:9081}"
MAIL="${2:-http://localhost:9089}"
API="$GW/api/v1"
PASS=0

ok()   { PASS=$((PASS + 1)); echo "  ✓ $*"; }
fail() { echo "  ✗ $*" >&2; exit 1; }
check() { [[ "$2" == "$3" ]] && ok "$1" || fail "$1 — got '$2', want '$3'"; }
# Retry a jq expression against a GET until it matches — for eventually-
# consistent reads fed by events.
eventually() {
  local desc="$1" url="$2" expr="$3" want="$4" token="${5:-}" tries="${6:-30}" got=""
  for _ in $(seq 1 "$tries"); do
    got=$(curl -s "$url" ${token:+-H "Authorization: Bearer $token"} | jq -r "$expr" 2>/dev/null || true)
    [[ "$got" == "$want" ]] && { ok "$desc"; return; }
    sleep 0.5
  done
  fail "$desc — got '$got', want '$want'"
}
req() { # method path [json] [token] [extra header]
  curl -s -X "$1" "$API$2" -H "Content-Type: application/json" \
    ${4:+-H "Authorization: Bearer $4"} ${5:+-H "$5"} ${3:+-d "$3"}
}
code() { # same args, prints only the HTTP status
  curl -s -o /dev/null -w '%{http_code}' -X "$1" "$API$2" -H "Content-Type: application/json" \
    ${4:+-H "Authorization: Bearer $4"} ${5:+-H "$5"} ${3:+-d "$3"}
}

echo "API versioning"
check "unknown version → 404" "$(curl -s -o /dev/null -w '%{http_code}' "$GW/api/v2/books")" "404"
check "404 lists supported versions" "$(curl -s "$GW/api/v2/books" | jq -c .supported_versions)" '["v1"]'
check "GET /api/versions current" "$(curl -s "$GW/api/versions" | jq -r .data.current)" "v1"
check "API-Version header" "$(curl -sI "$API/books" | tr -d '\r' | awk -F': ' 'tolower($1)=="api-version"{print $2}')" "v1"

echo "auth → user (RabbitMQ: auth.user_registered)"
EMAIL="smoke-$(date +%s%N)@example.com"
REG=$(req POST /auth/register "{\"first_name\":\"Smoke\",\"last_name\":\"Test\",\"email\":\"$EMAIL\",\"password\":\"Password123!\"}")
TOKEN=$(jq -r .data.token.access_token <<<"$REG")
USER_ID=$(jq -r .data.user.id <<<"$REG")
[[ "$TOKEN" != "null" ]] && ok "registered $EMAIL (id $USER_ID)" || fail "register: $REG"
eventually "user-service has the new profile" "$API/users/me" '.data.email' "$EMAIL" "$TOKEN"
ADMIN=$(req POST /auth/login '{"email":"admin@bibliomania.local","password":"ChangeMe123!"}' | jq -r .data.token.access_token)
[[ "$ADMIN" != "null" ]] && ok "seed admin logs in" || fail "admin login"

echo "catalog"
BOOK=$(req GET "/search?q=dune" | jq '.data.items[0]')
BOOK_ID=$(jq -r .id <<<"$BOOK"); COPIES=$(jq -r .available_copies <<<"$BOOK")
check "full-text search finds Dune" "$(jq -r .title <<<"$BOOK")" "Dune"
check "partial title search" "$(req GET "/search?q=hobb" | jq -r '.data.items[0].title')" "The Hobbit"

echo "borrow Saga (borrow → catalog gRPC ReserveCopy)"
KEY="smoke-$RANDOM-$RANDOM"
B1=$(curl -s -w '\n%{http_code}' -X POST "$API/borrows" -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" -H "Idempotency-Key: $KEY" -d "{\"book_id\":$BOOK_ID}")
check "first borrow → 201" "$(tail -1 <<<"$B1")" "201"
BORROW_ID=$(head -1 <<<"$B1" | jq -r .data.id)
check "title snapshotted on the borrow" "$(head -1 <<<"$B1" | jq -r .data.book_title)" "Dune"
# The gateway's idempotency middleware replays the original response.
REPLAY_HDRS=$(curl -s -D - -o /dev/null -X POST "$API/borrows" -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" -H "Idempotency-Key: $KEY" -d "{\"book_id\":$BOOK_ID}" | tr -d '\r')
check "retry with same Idempotency-Key is a replay" "$(awk -F': ' 'tolower($1)=="idempotent-replayed"{print $2}' <<<"$REPLAY_HDRS")" "true"
check "…of the original 201" "$(head -1 <<<"$REPLAY_HDRS" | awk '{print $2}')" "201"
check "…and returns the same borrow" "$(req POST /borrows "{\"book_id\":$BOOK_ID}" "$TOKEN" "Idempotency-Key: $KEY" | jq -r .data.id)" "$BORROW_ID"
check "same key, different body → 422" "$(code POST /borrows '{"book_id":1}' "$TOKEN" "Idempotency-Key: $KEY")" "422"
check "exactly one copy reserved" "$(req GET "/books/$BOOK_ID" | jq -r .data.available_copies)" "$((COPIES - 1))"
check "second open borrow of same book → 409" "$(code POST /borrows "{\"book_id\":$BOOK_ID}" "$TOKEN")" "409"
check "unknown book → 404" "$(code POST /borrows '{"book_id":999999}' "$TOKEN")" "404"
check "my borrows lists it" "$(req GET /borrows/my '' "$TOKEN" | jq -r '.data.items[0].id')" "$BORROW_ID"

echo "user library (gateway aggregation: user + catalog)"
check "add to shelf" "$(req PATCH "/users/me/library/$BOOK_ID" '{"status":"reading"}' "$TOKEN" | jq -r .data.book_title)" "Dune"
check "shelf add validates book → 404" "$(code PATCH /users/me/library/999999 '{"status":"reading"}' "$TOKEN")" "404"
check "GET library resolves title live" "$(req GET /users/me/library '' "$TOKEN" | jq -r '.data.items[0].book_title')" "Dune"

echo "reading → user (NATS: reading.book_completed)"
check "progress update" "$(req PATCH "/reading/$BOOK_ID/progress" '{"current_page":50,"total_pages":100}' "$TOKEN" | jq -r .data.progress_pct)" "50"
OLD=$(date -u -d '-1 hour' +%Y-%m-%dT%H:%M:%S.000Z)
check "stale offline sync loses (last write wins)" "$(req PATCH "/reading/$BOOK_ID/sync" "{\"current_page\":10,\"total_pages\":100,\"client_updated_at\":\"$OLD\"}" "$TOKEN" | jq -r .data.current_page)" "50"
check "finish the book" "$(req PATCH "/reading/$BOOK_ID/progress" '{"current_page":100,"total_pages":100}' "$TOKEN" | jq -r .data.is_completed)" "true"
eventually "user-service counters updated from event" "$API/users/me" '.data.total_books_read' "1" "$TOKEN"
check "history served by reading-service" "$(req GET /users/me/history '' "$TOKEN" | jq -r '.data.items[0].book_title')" "Dune"
BM=$(req POST "/reading/$BOOK_ID/bookmarks" '{"page":12,"note":"smoke"}' "$TOKEN" | jq -r .data.id)
[[ "$BM" =~ ^[0-9]+$ ]] && ok "bookmark created with numeric id $BM" || fail "bookmark: $BM"
check "delete bookmark" "$(code DELETE "/reading/$BOOK_ID/bookmarks/$BM" '' "$TOKEN")" "200"

echo "GraphQL (gateway fan-out)"
GQL=$(curl -s "$GW/graphql" -H "Content-Type: application/json" -H "Authorization: Bearer $TOKEN" \
  -d "{\"query\":\"{ book(id: \\\"$BOOK_ID\\\") { title myBorrowStatus { id status } myLibraryStatus } me { email totalBooksRead } myHistory { items { bookTitle book { title } } } }\"}")
check "book.myBorrowStatus (borrow-service)" "$(jq -r .data.book.myBorrowStatus.id <<<"$GQL")" "$BORROW_ID"
check "book.myLibraryStatus (user-service)" "$(jq -r .data.book.myLibraryStatus <<<"$GQL")" "READING"
check "me (user-service)" "$(jq -r .data.me.email <<<"$GQL")" "$EMAIL"
check "myHistory.book (reading → catalog)" "$(jq -r '.data.myHistory.items[0].book.title' <<<"$GQL")" "Dune"
check "anonymous myBorrowStatus is null" "$(curl -s "$GW/graphql" -H "Content-Type: application/json" -d "{\"query\":\"{ book(id: \\\"$BOOK_ID\\\") { myBorrowStatus { id } } }\"}" | jq -r .data.book.myBorrowStatus)" "null"
check "auth-required query → UNAUTHENTICATED" "$(curl -s "$GW/graphql" -H "Content-Type: application/json" -d '{"query":"{ me { email } }"}' | jq -r '.errors[0].extensions.code')" "UNAUTHENTICATED"

echo "return (catalog gRPC ReleaseCopy)"
check "return → returned" "$(req PATCH "/borrows/$BORROW_ID/return" '' "$TOKEN" | jq -r .data.status)" "returned"
check "copy released" "$(req GET "/books/$BOOK_ID" | jq -r .data.available_copies)" "$COPIES"
check "second return → 409" "$(code PATCH "/borrows/$BORROW_ID/return" '' "$TOKEN")" "409"

echo "notification-service (RabbitMQ + Kafka → email)"
if curl -s -m 2 "$MAIL/api/v1/messages" >/dev/null; then
  eventually "welcome email (auth.user_registered)" "$MAIL/api/v1/search?query=to:$EMAIL%20subject:Welcome" '.messages_count' "1"
  # Up to 90s: right after notification-service restarts, its Kafka
  # consumer group waits out the old member's session timeout (~45s)
  # before it's assigned partitions again. Late, never lost.
  eventually "borrow receipt (borrow.book_borrowed via Kafka)" "$MAIL/api/v1/search?query=to:$EMAIL%20subject:borrowed" '.messages_count' "1" "" 180
else
  echo "  - mailpit not reachable at $MAIL, skipping"
fi

echo "input validation"
check "unknown JSON field → 400" "$(code POST /reading/$BOOK_ID/bookmarks '{"page":1,"colour":"red"}' "$TOKEN")" "400"
BIGF=$(mktemp); { printf '{"bio":"'; head -c 1100000 /dev/zero | tr '\0' 'a'; printf '"}'; } > "$BIGF"
check "JSON body over 1 MB → 413" "$(curl -s -o /dev/null -w '%{http_code}' -X PATCH "$API/users/me" -H "Content-Type: application/json" -H "Authorization: Bearer $TOKEN" --data-binary "@$BIGF")" "413"
rm -f "$BIGF"
check "javascript: URL rejected" "$(code PATCH /users/me '{"profile_picture":"javascript:alert(1)"}' "$TOKEN")" "422"
check "search query too long → 400" "$(curl -s -o /dev/null -w '%{http_code}' "$API/search?q=$(head -c 300 /dev/zero | tr '\0' 'q')")" "400"
check "rate-limit headers present" "$(curl -s -D - -o /dev/null "$API/books" | tr -d '\r' | awk -F': ' 'tolower($1)=="ratelimit-limit"{print "yes"}')" "yes"

echo "uploads (librarian/admin)"
TMPD=$(mktemp -d)
printf '<html><script>alert(1)</script>' > "$TMPD/fake.pdf"
printf '%%PDF-1.4\n1 0 obj<<>>endobj\ntrailer<<>>\n%%%%EOF\n' > "$TMPD/real.pdf"
up() { curl -s -o /dev/null -w '%{http_code}' -X POST "$API/books/$BOOK_ID/upload" -H "Authorization: Bearer $ADMIN" -F "file=@$1"; }
check "HTML disguised as .pdf → 400" "$(up "$TMPD/fake.pdf")" "400"
check "real PDF → 200" "$(up "$TMPD/real.pdf")" "200"
DL=$(curl -s -D "$TMPD/h" -o "$TMPD/dl.pdf" -w '%{http_code}' "$API/books/$BOOK_ID/download" -H "Authorization: Bearer $TOKEN")
check "download → 200" "$DL" "200"
check "downloaded bytes match upload" "$(cmp -s "$TMPD/real.pdf" "$TMPD/dl.pdf" && echo same)" "same"
check "served as application/pdf" "$(tr -d '\r' < "$TMPD/h" | awk -F': ' 'tolower($1)=="content-type"{print $2}')" "application/pdf"
check "range request → 206" "$(curl -s -o /dev/null -w '%{http_code}' -H 'Range: bytes=0-3' "$API/books/$BOOK_ID/download" -H "Authorization: Bearer $TOKEN")" "206"
rm -rf "$TMPD"

echo "user → auth (RabbitMQ: user.status_changed)"
check "admin deactivates the member" "$(code PATCH "/users/$USER_ID/status" '{"is_active":false}' "$ADMIN")" "200"
for _ in $(seq 1 30); do
  c=$(code POST /auth/login "{\"email\":\"$EMAIL\",\"password\":\"Password123!\"}")
  [[ "$c" == "401" ]] && break; sleep 0.5
done
check "deactivated member can't log in" "$c" "401"

echo "payments"
# Seeded books have no price — whatever providers are configured, an
# unpriced book can't be bought.
check "checkout of an unpriced book → 400" "$(req POST /payments/checkout "{\"book_id\":$BOOK_ID}" "$ADMIN" | jq -r .error)" "this book is not for sale"
check "unknown webhook provider → 404" "$(curl -s -o /dev/null -w '%{http_code}' -X POST "$API/payments/webhook/paypal" -d '{}')" "404"

echo
echo "All $PASS checks passed."
