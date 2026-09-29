#!/usr/bin/env bash
# Phase 3 (Product & Category) API tests — mirrors the "Test cases" checklist in
# kernelstore_prompt.md §PHASE 3. Requires the API running on :5000 + Postgres.
# Run inside nix develop (needs curl + jq):  nix develop -c ./test_phase3_api.sh
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

# make-seller EMAIL USER SHOPSLUG → echoes an approved-seller token
make_seller() {
  local email="$1" user="$2" slug="$3"
  register "$user" "$email" "$user"
  local tok; tok="$(login "$email" Passw0rd!)"
  local sid; sid="$(req POST /shops "$tok" "{\"name\":\"$user shop\",\"slug\":\"$slug\",\"description\":\"x\"}" | jq -r '.data.id')"
  req POST "/admin/shops/${sid}/approve" "$ADMIN_TOKEN" >/dev/null
  login "$email" Passw0rd!   # re-login for Seller role
}

echo "=== Phase 3 API tests (ts=${TS}) ==="

echo "--- setup ---"
ADMIN_TOKEN="$(login admin@ks.com Admin@12345)"
[ -n "$ADMIN_TOKEN" ] && [ "$ADMIN_TOKEN" != "null" ] && ok "admin login" || { bad "admin login"; exit 1; }

# ── 3.1: admin category CRUD ──────────────────────────────────────────────
echo "--- 3.1: admin creates categories ---"
CAT_A="$(req POST /categories "$ADMIN_TOKEN" "{\"name\":\"CatA ${TS}\",\"slug\":\"cata-${TS}\",\"description\":\"a\"}" | jq -r '.data.id')"
CAT_B="$(req POST /categories "$ADMIN_TOKEN" "{\"name\":\"CatB ${TS}\",\"slug\":\"catb-${TS}\",\"description\":\"b\"}" | jq -r '.data.id')"
check "category A created" "$([ -n "$CAT_A" ] && [ "$CAT_A" != "null" ] && echo yes)" "yes"
check "category appears in tree" "$(req GET /categories "" | jq -r "[.data[] | select(.id==\"${CAT_A}\")] | length")" "1"
check "anonymous create category → HTTP 401" "$(req_code POST /categories "" '{"name":"x","slug":"x","description":"x"}')" "401"

SELLER_A="$(make_seller "sa${TS}@t.com" "sellerA${TS}" "shopa-${TS}")"
SELLER_B="$(make_seller "sb${TS}@t.com" "sellerB${TS}" "shopb-${TS}")"
SHOP_A_SLUG="shopa-${TS}"
ok "two approved sellers ready"

# ── 3.2 + TC1: seller adds product → saved + displayed ────────────────────
echo "--- TC1: add product → saved & displayed ---"
R="$(req POST /products "$SELLER_A" "{\"name\":\"Alpha ${TS}\",\"slug\":\"alpha-${TS}\",\"description\":\"first widget\",\"price\":100,\"salePrice\":80,\"stockQuantity\":10,\"sku\":\"A-${TS}\",\"categoryId\":\"${CAT_A}\",\"images\":[\"http://img/1.png\",\"http://img/2.png\",\"http://img/3.png\"]}")"
check "product created" "$(echo "$R" | jq -r '.success')" "true"
PROD_A="$(echo "$R" | jq -r '.data.id')"
check "product carries categoryName" "$(echo "$R" | jq -r '.data.categoryName')" "CatA ${TS}"

# cheaper product in category B for filter/sort coverage
req POST /products "$SELLER_A" "{\"name\":\"Beta ${TS}\",\"slug\":\"beta-${TS}\",\"description\":\"cheap gadget\",\"price\":30,\"stockQuantity\":5,\"sku\":\"B-${TS}\",\"categoryId\":\"${CAT_B}\",\"images\":[]}" >/dev/null
# a pricey one, no category
req POST /products "$SELLER_A" "{\"name\":\"Gamma ${TS}\",\"slug\":\"gamma-${TS}\",\"description\":\"premium alpha thing\",\"price\":500,\"stockQuantity\":2,\"sku\":\"G-${TS}\",\"images\":[]}" >/dev/null

# ── TC2: multiple images → gallery ────────────────────────────────────────
echo "--- TC2: product with multiple images → gallery ---"
D="$(req GET "/products/alpha-${TS}" "")"
check "detail returns 3 images" "$(echo "$D" | jq -r '.data.images | length')" "3"
check "first image is primary" "$(echo "$D" | jq -r '.data.images[0].isPrimary')" "true"
check "images ordered by displayOrder" "$(echo "$D" | jq -r '[.data.images[].displayOrder] == [0,1,2]')" "true"

# ── TC3: filter by category / price ───────────────────────────────────────
echo "--- TC3: filter by category & price ---"
check "filter category=CatA → only Alpha" "$(req GET "/products?category=cata-${TS}" "" | jq -r "[.data.items[] | select(.slug==\"alpha-${TS}\")] | length")" "1"
check "filter category=CatA excludes Beta" "$(req GET "/products?category=cata-${TS}" "" | jq -r "[.data.items[] | select(.slug==\"beta-${TS}\")] | length")" "0"
# price filter uses effective price (salePrice ?? price): Alpha=80, Beta=30, Gamma=500
check "filter minPrice=100 excludes Alpha(80) & Beta(30)" "$(req GET "/products?minPrice=100&search=${TS}" "" | jq -r "[.data.items[] | select(.slug==\"alpha-${TS}\" or .slug==\"beta-${TS}\")] | length")" "0"
check "filter maxPrice=50 → only Beta(30)" "$(req GET "/products?maxPrice=50&search=${TS}" "" | jq -r '[.data.items[].slug] | map(select(. | test("beta")))| length')" "1"
check "filter shop=shopA returns this shop's products" "$(req GET "/products?shop=${SHOP_A_SLUG}" "" | jq -r '.data.total >= 3')" "true"

# ── TC6: search by name ───────────────────────────────────────────────────
echo "--- TC6: search by name ---"
check "search 'Alpha' finds Alpha" "$(req GET "/products?search=Alpha%20${TS}" "" | jq -r "[.data.items[] | select(.slug==\"alpha-${TS}\")] | length")" "1"
# 'alpha' also appears in Gamma's description → search matches name OR description
check "search matches name or description" "$(req GET "/products?search=alpha" "" | jq -r '.data.total >= 1')" "true"

# ── sort ──────────────────────────────────────────────────────────────────
echo "--- sort ---"
check "sort=price_asc → Beta(30) before Gamma(500)" "$(req GET "/products?search=${TS}&sort=price_asc&pageSize=50" "" | jq -r '[.data.items[] | (.salePrice // .price)] as $p | ($p == ($p | sort))')" "true"

# ── TC4: pagination ───────────────────────────────────────────────────────
echo "--- TC4: pagination ---"
P1="$(req GET "/products?pageSize=1&page=1" "")"
check "pageSize=1 → 1 item" "$(echo "$P1" | jq -r '.data.items | length')" "1"
check "page field == 1" "$(echo "$P1" | jq -r '.data.page')" "1"
check "totalPages > 1 (many products)" "$(echo "$P1" | jq -r '.data.totalPages > 1')" "true"
ID_P1="$(echo "$P1" | jq -r '.data.items[0].id')"
ID_P2="$(req GET "/products?pageSize=1&page=2" "" | jq -r '.data.items[0].id')"
check "page 2 differs from page 1" "$([ "$ID_P1" != "$ID_P2" ] && echo yes)" "yes"

# ── TC5: seller A cannot edit seller B's product (404) ────────────────────
echo "--- TC5: cross-seller edit blocked → 404 ---"
check "seller B cannot update A's product → HTTP 404" "$(req_code PUT "/products/${PROD_A}" "$SELLER_B" "{\"name\":\"hack\",\"slug\":\"alpha-${TS}\",\"description\":\"x\",\"price\":1,\"stockQuantity\":0,\"sku\":\"A-${TS}\",\"isActive\":true,\"images\":[]}")" "404"
check "seller B cannot delete A's product → HTTP 404" "$(req_code DELETE "/products/${PROD_A}" "$SELLER_B")" "404"
check "seller A CAN update own product" "$(req PUT "/products/${PROD_A}" "$SELLER_A" "{\"name\":\"Alpha ${TS} v2\",\"slug\":\"alpha-${TS}\",\"description\":\"first widget\",\"price\":100,\"salePrice\":80,\"stockQuantity\":10,\"sku\":\"A-${TS}\",\"isActive\":true,\"images\":[]}" | jq -r '.success')" "true"

# ── inactive products hidden from public list ─────────────────────────────
echo "--- inactive products hidden ---"
req PUT "/products/${PROD_A}" "$SELLER_A" "{\"name\":\"Alpha ${TS} v2\",\"slug\":\"alpha-${TS}\",\"description\":\"first widget\",\"price\":100,\"salePrice\":80,\"stockQuantity\":10,\"sku\":\"A-${TS}\",\"isActive\":false,\"images\":[]}" >/dev/null
check "inactive product hidden from public list" "$(req GET "/products?search=Alpha%20${TS}" "" | jq -r "[.data.items[] | select(.slug==\"alpha-${TS}\")] | length")" "0"
check "inactive product detail → HTTP 404" "$(req_code GET "/products/alpha-${TS}" "")" "404"

echo ""
echo "=== RESULT: ${PASS} passed, ${FAIL} failed ==="
[ "$FAIL" -eq 0 ]
