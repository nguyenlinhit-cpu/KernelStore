#!/usr/bin/env bash
# Phase 4 (Cart & Checkout) API tests — mirrors the "Test cases" checklist in
# kernelstore_prompt.md §PHASE 4. Requires the API running on :5000 + Postgres.
# Run inside nix develop (needs curl + jq):  nix develop -c ./test_phase4_api.sh
# (Cần bật flakes: nix.settings.experimental-features = [ "nix-command" "flakes" ];)
set -uo pipefail

BASE="http://localhost:5000/api"
TS="$(date +%s)"
PASS=0
FAIL=0

ok()   { echo "  [PASS] $1"; PASS=$((PASS+1)); }
bad()  { echo "  [FAIL] $1 ${2:-}"; FAIL=$((FAIL+1)); }
check() { [ "$2" = "$3" ] && ok "$1" || bad "$1" "(expected '$3' got '$2')"; }

# POST/PUT/GET/DELETE helpers: $1=method $2=path $3=token(optional) $4=json(optional)
req() {
  local method="$1" path="$2" token="${3:-}" data="${4:-}"
  local args=(-s -X "$method" "${BASE}${path}" -H "Content-Type: application/json")
  [ -n "$token" ] && args+=(-H "Authorization: Bearer ${token}")
  [ -n "$data" ] && args+=(-d "$data")
  curl "${args[@]}"
}

# Same as req but prints only the HTTP status code.
req_code() {
  local method="$1" path="$2" token="${3:-}" data="${4:-}"
  local args=(-s -o /dev/null -w '%{http_code}' -X "$method" "${BASE}${path}" -H "Content-Type: application/json")
  [ -n "$token" ] && args+=(-H "Authorization: Bearer ${token}")
  [ -n "$data" ] && args+=(-d "$data")
  curl "${args[@]}"
}

echo "=== Phase 4 API tests (ts=${TS}) ==="

# ── Setup ────────────────────────────────────────────────────────────────
echo "--- setup ---"
ADMIN_TOKEN="$(req POST /auth/login "" '{"email":"admin@ks.com","password":"Admin@12345"}' | jq -r '.data.accessToken')"
[ -n "$ADMIN_TOKEN" ] && [ "$ADMIN_TOKEN" != "null" ] && ok "admin login" || { bad "admin login"; echo "cannot continue"; exit 1; }

SELLER_EMAIL="seller${TS}@t.com"
req POST /auth/register "" "{\"fullName\":\"Seller ${TS}\",\"email\":\"${SELLER_EMAIL}\",\"userName\":\"seller${TS}\",\"password\":\"Passw0rd!\"}" >/dev/null
SELLER_TOKEN="$(req POST /auth/login "" "{\"email\":\"${SELLER_EMAIL}\",\"password\":\"Passw0rd!\"}" | jq -r '.data.accessToken')"

SHOP_ID="$(req POST /shops "$SELLER_TOKEN" "{\"name\":\"Shop ${TS}\",\"slug\":\"shop-${TS}\",\"description\":\"test shop\"}" | jq -r '.data.id')"
[ -n "$SHOP_ID" ] && [ "$SHOP_ID" != "null" ] && ok "seller created shop (Pending)" || bad "seller created shop"

req POST "/admin/shops/${SHOP_ID}/approve" "$ADMIN_TOKEN" >/dev/null
ok "admin approved shop"

# re-login so the JWT carries the Seller role
SELLER_TOKEN="$(req POST /auth/login "" "{\"email\":\"${SELLER_EMAIL}\",\"password\":\"Passw0rd!\"}" | jq -r '.data.accessToken')"

PRODUCT_ID="$(req POST /products "$SELLER_TOKEN" "{\"name\":\"Widget ${TS}\",\"slug\":\"widget-${TS}\",\"description\":\"a widget\",\"price\":100,\"stockQuantity\":5,\"sku\":\"SKU-${TS}\",\"isActive\":true,\"images\":[]}" | jq -r '.data.id')"
[ -n "$PRODUCT_ID" ] && [ "$PRODUCT_ID" != "null" ] && ok "seller created product (stock=5, price=100)" || bad "seller created product"

CUST_EMAIL="cust${TS}@t.com"
req POST /auth/register "" "{\"fullName\":\"Cust ${TS}\",\"email\":\"${CUST_EMAIL}\",\"userName\":\"cust${TS}\",\"password\":\"Passw0rd!\"}" >/dev/null
CUST_TOKEN="$(req POST /auth/login "" "{\"email\":\"${CUST_EMAIL}\",\"password\":\"Passw0rd!\"}" | jq -r '.data.accessToken')"
[ -n "$CUST_TOKEN" ] && [ "$CUST_TOKEN" != "null" ] && ok "customer registered + logged in" || bad "customer login"

# ── TC1: add to cart → cart updates ───────────────────────────────────────
echo "--- TC1: add product to cart → cart updates ---"
R="$(req POST /cart "$CUST_TOKEN" "{\"productId\":\"${PRODUCT_ID}\",\"quantity\":2}")"
check "add-to-cart success" "$(echo "$R" | jq -r '.success')" "true"
check "cart totalItems == 2" "$(echo "$R" | jq -r '.data.totalItems')" "2"

# ── TC2: increase qty beyond stock → error ────────────────────────────────
echo "--- TC2: qty beyond stock → error ---"
R="$(req PUT "/cart/${PRODUCT_ID}" "$CUST_TOKEN" '{"quantity":99}')"
check "update beyond stock rejected" "$(echo "$R" | jq -r '.success')" "false"
R="$(req POST /cart "$CUST_TOKEN" "{\"productId\":\"${PRODUCT_ID}\",\"quantity\":99}")"
check "add beyond stock rejected" "$(echo "$R" | jq -r '.success')" "false"
# cart unchanged (still 2)
check "cart still totalItems == 2" "$(req GET /cart "$CUST_TOKEN" | jq -r '.data.totalItems')" "2"

# ── TC6: total = sum(items) + shipping ────────────────────────────────────
echo "--- TC6: totals correct ---"
check "cart subtotal == 200" "$(req GET /cart "$CUST_TOKEN" | jq -r '.data.subtotal * 1')" "200"

# ── TC3: create order → decrement stock, create order + details ───────────
echo "--- TC3: create order → stock decrement + order/details ---"
R="$(req POST /orders "$CUST_TOKEN" '{"fullName":"Cust","phone":"0901234567","street":"123 Le Loi","ward":"Ben Nghe","district":"Q1","city":"HCMC","note":"leave at door"}')"
check "order created" "$(echo "$R" | jq -r '.success')" "true"
ORDER_ID="$(echo "$R" | jq -r '.data.id')"
check "order has 1 detail line" "$(echo "$R" | jq -r '.data.items | length')" "1"
check "order total == 200 (200 items + 0 ship)" "$(echo "$R" | jq -r '.data.totalAmount * 1')" "200"
check "order shippingFee == 0" "$(echo "$R" | jq -r '.data.shippingFee * 1')" "0"
# stock 5 - 2 = 3
check "product stock decremented to 3" "$(req GET "/products/${PRODUCT_ID}" | jq -r '.data.stockQuantity')" "3"
# cart cleared after checkout
check "cart cleared after order" "$(req GET /cart "$CUST_TOKEN" | jq -r '.data.totalItems')" "0"
# order shows in history
check "order visible in customer history" "$(req GET /orders "$CUST_TOKEN" | jq -r "[.data[] | select(.id==\"${ORDER_ID}\")] | length")" "1"

# ── TC4: create order with empty cart → error ─────────────────────────────
echo "--- TC4: checkout empty cart → error ---"
R="$(req POST /orders "$CUST_TOKEN" '{"fullName":"Cust","phone":"0901234567","street":"123 Le Loi","ward":"Ben Nghe","district":"Q1","city":"HCMC","note":""}')"
check "empty-cart order rejected" "$(echo "$R" | jq -r '.success')" "false"

# ── TC5: seller updates status → customer sees update ─────────────────────
echo "--- TC5: seller updates status → customer sees it ---"
R="$(req PUT "/orders/${ORDER_ID}/status" "$SELLER_TOKEN" '{"status":"Confirmed"}')"
check "seller status update success" "$(echo "$R" | jq -r '.success')" "true"
check "customer sees status = Confirmed" "$(req GET "/orders/${ORDER_ID}" "$CUST_TOKEN" | jq -r '.data.status')" "Confirmed"
# a random customer (not Seller/Admin role) cannot update → 403 Forbidden
check "customer cannot update status (HTTP 403)" "$(req_code PUT "/orders/${ORDER_ID}/status" "$CUST_TOKEN" '{"status":"Delivered"}')" "403"

# ── Summary ───────────────────────────────────────────────────────────────
echo ""
echo "=== RESULT: ${PASS} passed, ${FAIL} failed ==="
[ "$FAIL" -eq 0 ]
