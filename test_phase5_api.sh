#!/usr/bin/env bash
# Phase 5 (Review & Rating) API tests — mirrors the "Test cases" checklist in
# kernelstore_prompt.md §PHASE 5. Requires the API running on :5000 + Postgres.
# Run inside nix develop (needs curl + jq):  nix develop -c ./test_phase5_api.sh
# (Cần bật flakes: nix.settings.experimental-features = [ "nix-command" "flakes" ];)
set -uo pipefail

BASE="http://localhost:5000/api"
TS="$(date +%s)"
PASS=0
FAIL=0

ok()   { echo "  [PASS] $1"; PASS=$((PASS+1)); }
bad()  { echo "  [FAIL] $1 ${2:-}"; FAIL=$((FAIL+1)); }
check() { [ "$2" = "$3" ] && ok "$1" || bad "$1" "(expected '$3' got '$2')"; }

req() {
  local method="$1" path="$2" token="${3:-}" data="${4:-}"
  local args=(-s -X "$method" "${BASE}${path}" -H "Content-Type: application/json")
  [ -n "$token" ] && args+=(-H "Authorization: Bearer ${token}")
  [ -n "$data" ] && args+=(-d "$data")
  curl "${args[@]}"
}
req_code() {
  local method="$1" path="$2" token="${3:-}" data="${4:-}"
  local args=(-s -o /dev/null -w '%{http_code}' -X "$method" "${BASE}${path}" -H "Content-Type: application/json")
  [ -n "$token" ] && args+=(-H "Authorization: Bearer ${token}")
  [ -n "$data" ] && args+=(-d "$data")
  curl "${args[@]}"
}

# Registers a user and returns their access token.
signup() {
  local tag="$1"
  local email="${tag}${TS}@t.com"
  req POST /auth/register "" "{\"fullName\":\"${tag} ${TS}\",\"email\":\"${email}\",\"userName\":\"${tag}${TS}\",\"password\":\"Passw0rd!\"}" >/dev/null
  req POST /auth/login "" "{\"email\":\"${email}\",\"password\":\"Passw0rd!\"}" | jq -r '.data.accessToken'
}

# Places an order for a single unit of $PRODUCT_ID with the given token and
# drives it all the way to Delivered (seller ships → buyer confirms receipt),
# because a review is only allowed once the buyer has received the item.
buy() {
  local token="$1" oid
  req POST /cart "$token" "{\"productId\":\"${PRODUCT_ID}\",\"quantity\":1}" >/dev/null
  oid="$(req POST /orders "$token" '{"fullName":"Buyer","phone":"0901234567","street":"1 St","ward":"W","district":"D","city":"C","note":""}' | jq -r '.data.id')"
  for s in Confirmed Processing Shipped; do req PUT "/orders/${oid}/status" "$SELLER_TOKEN" "{\"status\":\"$s\"}" >/dev/null; done
  req POST "/orders/${oid}/confirm-received" "$token" >/dev/null
}

echo "=== Phase 5 API tests (ts=${TS}) ==="

# ── Setup ────────────────────────────────────────────────────────────────
echo "--- setup ---"
ADMIN_TOKEN="$(req POST /auth/login "" '{"email":"admin@ks.com","password":"Admin@12345"}' | jq -r '.data.accessToken')"
[ -n "$ADMIN_TOKEN" ] && [ "$ADMIN_TOKEN" != "null" ] && ok "admin login" || { bad "admin login"; exit 1; }

SELLER_TOKEN="$(signup seller)"
SHOP_ID="$(req POST /shops "$SELLER_TOKEN" "{\"name\":\"Shop ${TS}\",\"slug\":\"shop-${TS}\",\"description\":\"d\"}" | jq -r '.data.id')"
req POST "/admin/shops/${SHOP_ID}/approve" "$ADMIN_TOKEN" >/dev/null
SELLER_TOKEN="$(req POST /auth/login "" "{\"email\":\"seller${TS}@t.com\",\"password\":\"Passw0rd!\"}" | jq -r '.data.accessToken')"
PRODUCT_ID="$(req POST /products "$SELLER_TOKEN" "{\"name\":\"Widget ${TS}\",\"slug\":\"widget-${TS}\",\"description\":\"d\",\"price\":50,\"stockQuantity\":10,\"sku\":\"SKU-${TS}\",\"isActive\":true,\"images\":[]}" | jq -r '.data.id')"
[ -n "$PRODUCT_ID" ] && [ "$PRODUCT_ID" != "null" ] && ok "setup: shop approved + product created" || bad "setup product"

CUST_A="$(signup custa)"
CUST_B="$(signup custb)"
CUST_C="$(signup custc)"   # never buys
ok "3 customers registered"

# ── TC1: not purchased → 403 ──────────────────────────────────────────────
echo "--- TC1: review without purchase → 403 ---"
check "unpurchased review rejected (HTTP 403)" \
  "$(req_code POST /reviews "$CUST_C" "{\"productId\":\"${PRODUCT_ID}\",\"rating\":5,\"comment\":\"nope\"}")" "403"

# ── TC2: purchased → can review 1-5 + comment ─────────────────────────────
echo "--- TC2: purchased → can review ---"
buy "$CUST_A"
R="$(req POST /reviews "$CUST_A" "{\"productId\":\"${PRODUCT_ID}\",\"rating\":4,\"comment\":\"pretty good\"}")"
check "purchased review accepted" "$(echo "$R" | jq -r '.success')" "true"
check "review rating stored = 4" "$(echo "$R" | jq -r '.data.rating')" "4"
check "review has reviewer name" "$([ -n "$(echo "$R" | jq -r '.data.userName')" ] && echo yes || echo no)" "yes"

# duplicate review blocked
check "duplicate review rejected" \
  "$(req POST /reviews "$CUST_A" "{\"productId\":\"${PRODUCT_ID}\",\"rating\":3,\"comment\":\"again\"}" | jq -r '.success')" "false"

# invalid rating rejected (validation → 400)
buy "$CUST_B"
check "invalid rating 6 rejected (HTTP 400)" \
  "$(req_code POST /reviews "$CUST_B" "{\"productId\":\"${PRODUCT_ID}\",\"rating\":6,\"comment\":\"x\"}")" "400"
R="$(req POST /reviews "$CUST_B" "{\"productId\":\"${PRODUCT_ID}\",\"rating\":2,\"comment\":\"meh\"}")"
check "second valid review accepted (rating 2)" "$(echo "$R" | jq -r '.success')" "true"

# ── TC3: average rating correct ───────────────────────────────────────────
echo "--- TC3: average rating + list ---"
R="$(req GET "/reviews?productId=${PRODUCT_ID}")"
check "review count == 2" "$(echo "$R" | jq -r '.data.reviewCount')" "2"
check "average == 3.0 ((4+2)/2)" "$(echo "$R" | jq -r '.data.averageRating * 1')" "3"
check "newest-first: first review rating == 2" "$(echo "$R" | jq -r '.data.reviews[0].rating')" "2"

# ── Guards ────────────────────────────────────────────────────────────────
echo "--- guards ---"
check "GET without productId → 400" "$(req_code GET '/reviews')" "400"
check "review non-existent product → 404" \
  "$(req_code POST /reviews "$CUST_A" '{"productId":"00000000-0000-0000-0000-000000000000","rating":5,"comment":"x"}')" "404"

echo ""
echo "=== RESULT: ${PASS} passed, ${FAIL} failed ==="
[ "$FAIL" -eq 0 ]
