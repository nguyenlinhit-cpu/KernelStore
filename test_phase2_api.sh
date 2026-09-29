#!/usr/bin/env bash
# Phase 2 (Shop & Seller System) API tests — mirrors the "Test cases" checklist
# in kernelstore_prompt.md §PHASE 2. Requires the API running on :5000 + Postgres.
# Run inside nix develop (needs curl + jq):  nix develop -c ./test_phase2_api.sh
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
login() { req POST /auth/login "" "{\"email\":\"$1\",\"password\":\"$2\"}" | jq -r '.data.accessToken'; }
register() { req POST /auth/register "" "{\"fullName\":\"$1\",\"email\":\"$2\",\"userName\":\"$3\",\"password\":\"Passw0rd!\"}" >/dev/null; }

echo "=== Phase 2 API tests (ts=${TS}) ==="

echo "--- setup ---"
ADMIN_TOKEN="$(login admin@ks.com Admin@12345)"
[ -n "$ADMIN_TOKEN" ] && [ "$ADMIN_TOKEN" != "null" ] && ok "admin login" || { bad "admin login"; exit 1; }

SELLER_EMAIL="seller${TS}@t.com"
register "Seller ${TS}" "$SELLER_EMAIL" "seller${TS}"
SELLER_TOKEN="$(login "$SELLER_EMAIL" Passw0rd!)"

# ── TC1: register seller → shop created with Pending ──────────────────────
echo "--- TC1: create shop → status Pending ---"
R="$(req POST /shops "$SELLER_TOKEN" "{\"name\":\"Shop ${TS}\",\"slug\":\"shop-${TS}\",\"description\":\"test shop\"}")"
check "create shop success" "$(echo "$R" | jq -r '.success')" "true"
SHOP_ID="$(echo "$R" | jq -r '.data.id')"
check "new shop status == Pending" "$(echo "$R" | jq -r '.data.status')" "Pending"
check "GET /shops/me returns the shop" "$(req GET /shops/me "$SELLER_TOKEN" | jq -r '.data.id')" "$SHOP_ID"
# a user can only own one shop
check "second shop for same owner rejected" "$(req POST /shops "$SELLER_TOKEN" "{\"name\":\"Dup\",\"slug\":\"shop-dup-${TS}\",\"description\":\"x\"}" | jq -r '.success')" "false"

# ── TC3: seller not approved → cannot add product (403) ───────────────────
echo "--- TC3: unapproved seller cannot add product → 403 ---"
# (no Seller role yet because shop is still Pending)
check "unapproved create product → HTTP 403" "$(req_code POST /products "$SELLER_TOKEN" "{\"name\":\"P ${TS}\",\"slug\":\"p-${TS}\",\"description\":\"x\",\"price\":10,\"stockQuantity\":1,\"sku\":\"S-${TS}\",\"images\":[]}")" "403"

# ── TC2: admin approve → shop Approved, seller can add product ────────────
echo "--- TC2: admin approve → Approved + seller can add product ---"
R="$(req POST "/admin/shops/${SHOP_ID}/approve" "$ADMIN_TOKEN")"
check "admin approve success" "$(echo "$R" | jq -r '.success')" "true"
check "shop now Approved (my-shop)" "$(req GET /shops/me "$SELLER_TOKEN" | jq -r '.data.status')" "Approved"
# re-login so JWT carries the Seller role granted on approval
SELLER_TOKEN="$(login "$SELLER_EMAIL" Passw0rd!)"
check "approved seller can add product" "$(req POST /products "$SELLER_TOKEN" "{\"name\":\"P ${TS}\",\"slug\":\"p-${TS}\",\"description\":\"x\",\"price\":10,\"stockQuantity\":1,\"sku\":\"S-${TS}\",\"images\":[]}" | jq -r '.success')" "true"

# ── TC (2.3): update my shop ──────────────────────────────────────────────
echo "--- 2.3: seller updates own shop ---"
R="$(req PUT /shops/me "$SELLER_TOKEN" "{\"name\":\"Shop ${TS} v2\",\"slug\":\"shop-${TS}\",\"description\":\"updated\"}")"
check "update shop success" "$(echo "$R" | jq -r '.success')" "true"
check "shop name updated" "$(req GET /shops/me "$SELLER_TOKEN" | jq -r '.data.name')" "Shop ${TS} v2"

# ── TC4: normal customer cannot act as seller (403) ───────────────────────
echo "--- TC4: normal user blocked from seller-only actions → 403 ---"
CUST_EMAIL="cust${TS}@t.com"
register "Cust ${TS}" "$CUST_EMAIL" "cust${TS}"
CUST_TOKEN="$(login "$CUST_EMAIL" Passw0rd!)"
check "customer create product → HTTP 403" "$(req_code POST /products "$CUST_TOKEN" "{\"name\":\"X\",\"slug\":\"x-${TS}\",\"description\":\"x\",\"price\":1,\"stockQuantity\":1,\"sku\":\"X-${TS}\",\"images\":[]}")" "403"
check "customer my-shop → null (no shop)" "$(req GET /shops/me "$CUST_TOKEN" | jq -r '.data')" "null"
# admin-only shop list blocked for customer
check "customer admin shop list → HTTP 403" "$(req_code GET '/admin/shops?status=Pending' "$CUST_TOKEN")" "403"

echo ""
echo "=== RESULT: ${PASS} passed, ${FAIL} failed ==="
[ "$FAIL" -eq 0 ]
