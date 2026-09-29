#!/usr/bin/env bash
# Phase 6 (Admin Panel) API tests — mirrors the "Test cases" checklist in
# kernelstore_prompt.md §PHASE 6. Requires the API running on :5000 + Postgres.
# Run inside nix develop (needs curl + jq):  nix develop -c ./test_phase6_api.sh
# (Cần bật flakes: nix.settings.experimental-features = [ "nix-command" "flakes" ];)
set -uo pipefail

BASE="http://localhost:5000/api"
TS="$(date +%s)"
PASS=0
FAIL=0

ok()   { echo "  [PASS] $1"; PASS=$((PASS+1)); }
bad()  { echo "  [FAIL] $1 ${2:-}"; FAIL=$((FAIL+1)); }
check() { [ "$2" = "$3" ] && ok "$1" || bad "$1" "(expected '$3' got '$2')"; }
check_ge() { [ "$2" -ge "$3" ] 2>/dev/null && ok "$1" || bad "$1" "(expected >= '$3' got '$2')"; }

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

echo "=== Phase 6 API tests (ts=${TS}) ==="

# ── Setup ────────────────────────────────────────────────────────────────
echo "--- setup ---"
ADMIN_LOGIN="$(req POST /auth/login "" '{"email":"admin@ks.com","password":"Admin@12345"}')"
ADMIN_TOKEN="$(echo "$ADMIN_LOGIN" | jq -r '.data.accessToken')"
ADMIN_ID="$(echo "$ADMIN_LOGIN" | jq -r '.data.user.id')"
[ -n "$ADMIN_TOKEN" ] && [ "$ADMIN_TOKEN" != "null" ] && ok "admin login" || { bad "admin login"; exit 1; }

# customer to be banned
CUST_EMAIL="ban${TS}@t.com"
CUST_REG="$(req POST /auth/register "" "{\"fullName\":\"Ban ${TS}\",\"email\":\"${CUST_EMAIL}\",\"userName\":\"ban${TS}\",\"password\":\"Passw0rd!\"}")"
CUST_ID="$(echo "$CUST_REG" | jq -r '.data.user.id')"
CUST_TOKEN="$(echo "$CUST_REG" | jq -r '.data.accessToken')"
ok "customer registered"

# seller + pending shop
SELLER_EMAIL="s6${TS}@t.com"
req POST /auth/register "" "{\"fullName\":\"S ${TS}\",\"email\":\"${SELLER_EMAIL}\",\"userName\":\"s6${TS}\",\"password\":\"Passw0rd!\"}" >/dev/null
SELLER_TOKEN="$(req POST /auth/login "" "{\"email\":\"${SELLER_EMAIL}\",\"password\":\"Passw0rd!\"}" | jq -r '.data.accessToken')"
SHOP_ID="$(req POST /shops "$SELLER_TOKEN" "{\"name\":\"Shop6 ${TS}\",\"slug\":\"shop6-${TS}\",\"description\":\"d\"}" | jq -r '.data.id')"
[ -n "$SHOP_ID" ] && [ "$SHOP_ID" != "null" ] && ok "seller created pending shop" || bad "seller shop"

# ── 6.1: dashboard stats ──────────────────────────────────────────────────
echo "--- 6.1: admin dashboard ---"
D="$(req GET /admin/dashboard "$ADMIN_TOKEN")"
check "dashboard success" "$(echo "$D" | jq -r '.success')" "true"
check_ge "totalUsers >= 3" "$(echo "$D" | jq -r '.data.totalUsers')" "3"
check_ge "totalShops >= 1" "$(echo "$D" | jq -r '.data.totalShops')" "1"
check_ge "pendingShops >= 1" "$(echo "$D" | jq -r '.data.pendingShops')" "1"
check "ordersByStatus has 8 buckets" "$(echo "$D" | jq -r '.data.ordersByStatus | length')" "8"
check "totalRevenue present" "$([ "$(echo "$D" | jq -r '.data.totalRevenue')" != "null" ] && echo yes || echo no)" "yes"

# ── test case: non-admin → 403 ────────────────────────────────────────────
echo "--- non-admin access → 403 ---"
check "customer dashboard → 403" "$(req_code GET /admin/dashboard "$CUST_TOKEN")" "403"
check "customer admin/users → 403" "$(req_code GET /admin/users "$CUST_TOKEN")" "403"

# ── test case: admin approves shop → shop active + owner Seller ────────────
echo "--- 6.5: approve shop → active ---"
check "approve success" "$(req POST "/admin/shops/${SHOP_ID}/approve" "$ADMIN_TOKEN" | jq -r '.success')" "true"
check "shop now Approved" \
  "$(req GET '/admin/shops?status=Approved' "$ADMIN_TOKEN" | jq -r "[.data[] | select(.id==\"${SHOP_ID}\")] | length")" "1"
check "owner became Seller" \
  "$(req GET "/admin/users?search=s6${TS}" "$ADMIN_TOKEN" | jq -r '.data[0].role')" "Seller"

# ── 6.2: ban user → cannot login, unban → can login ───────────────────────
echo "--- 6.2: ban / unban user ---"
check "ban success" "$(req POST "/admin/users/${CUST_ID}/ban" "$ADMIN_TOKEN" | jq -r '.success')" "true"
check "banned user cannot login (HTTP 401)" \
  "$(req_code POST /auth/login "" "{\"email\":\"${CUST_EMAIL}\",\"password\":\"Passw0rd!\"}")" "401"
check "unban success" "$(req POST "/admin/users/${CUST_ID}/unban" "$ADMIN_TOKEN" | jq -r '.success')" "true"
check "unbanned user can login again (HTTP 200)" \
  "$(req_code POST /auth/login "" "{\"email\":\"${CUST_EMAIL}\",\"password\":\"Passw0rd!\"}")" "200"
check "admin cannot ban self" "$(req POST "/admin/users/${ADMIN_ID}/ban" "$ADMIN_TOKEN" | jq -r '.success')" "false"
check "ban nonexistent user → 404" \
  "$(req_code POST '/admin/users/00000000-0000-0000-0000-000000000000/ban' "$ADMIN_TOKEN")" "404"

# ── 6.3: admin orders (need an order) ─────────────────────────────────────
echo "--- 6.3: admin orders ---"
SELLER_TOKEN="$(req POST /auth/login "" "{\"email\":\"${SELLER_EMAIL}\",\"password\":\"Passw0rd!\"}" | jq -r '.data.accessToken')"
PRODUCT_ID="$(req POST /products "$SELLER_TOKEN" "{\"name\":\"P6 ${TS}\",\"slug\":\"p6-${TS}\",\"description\":\"d\",\"price\":70,\"stockQuantity\":10,\"sku\":\"S6-${TS}\",\"isActive\":true,\"images\":[]}" | jq -r '.data.id')"
req POST /cart "$CUST_TOKEN" "{\"productId\":\"${PRODUCT_ID}\",\"quantity\":1}" >/dev/null
ORD="$(req POST /orders "$CUST_TOKEN" '{"fullName":"Buyer Six","phone":"0900000000","street":"1 St","ward":"w","district":"d","city":"c","note":""}')"
ORDER_CODE="$(echo "$ORD" | jq -r '.data.orderCode')"
check "order placed (for admin-orders test)" "$(echo "$ORD" | jq -r '.success')" "true"
O="$(req GET '/admin/orders?pageSize=5' "$ADMIN_TOKEN")"
check "admin orders success" "$(echo "$O" | jq -r '.success')" "true"
check_ge "admin orders total >= 1" "$(echo "$O" | jq -r '.data.total')" "1"
check "admin orders search by code finds it" \
  "$(req GET "/admin/orders?search=${ORDER_CODE}" "$ADMIN_TOKEN" | jq -r "[.data.items[] | select(.orderCode==\"${ORDER_CODE}\")] | length")" "1"
check_ge "admin orders filter status=Pending >= 1" \
  "$(req GET '/admin/orders?status=Pending' "$ADMIN_TOKEN" | jq -r '.data.total')" "1"

# ── 6.6: category CRUD + block-delete ─────────────────────────────────────
echo "--- 6.6: category management ---"
PARENT_ID="$(req POST /categories "$ADMIN_TOKEN" "{\"name\":\"Cat ${TS}\",\"slug\":\"cat-${TS}\",\"description\":\"\"}" | jq -r '.data.id')"
check "create parent category" "$([ -n "$PARENT_ID" ] && [ "$PARENT_ID" != "null" ] && echo yes || echo no)" "yes"
check "update category" \
  "$(req PUT "/categories/${PARENT_ID}" "$ADMIN_TOKEN" "{\"name\":\"Cat ${TS} v2\",\"slug\":\"cat-${TS}\",\"description\":\"x\"}" | jq -r '.success')" "true"
CHILD_ID="$(req POST /categories "$ADMIN_TOKEN" "{\"name\":\"Child ${TS}\",\"slug\":\"child-${TS}\",\"description\":\"\",\"parentId\":\"${PARENT_ID}\"}" | jq -r '.data.id')"
check "create child category" "$([ -n "$CHILD_ID" ] && [ "$CHILD_ID" != "null" ] && echo yes || echo no)" "yes"
check "delete parent-with-children blocked" \
  "$(req DELETE "/categories/${PARENT_ID}" "$ADMIN_TOKEN" | jq -r '.success')" "false"
check "delete child success" "$(req DELETE "/categories/${CHILD_ID}" "$ADMIN_TOKEN" | jq -r '.success')" "true"
check "delete now-empty parent success" "$(req DELETE "/categories/${PARENT_ID}" "$ADMIN_TOKEN" | jq -r '.success')" "true"
check "non-admin create category → 403" \
  "$(req_code POST /categories "$CUST_TOKEN" "{\"name\":\"X ${TS}\",\"slug\":\"x-${TS}\",\"description\":\"\"}")" "403"

echo ""
echo "=== RESULT: ${PASS} passed, ${FAIL} failed ==="
[ "$FAIL" -eq 0 ]
