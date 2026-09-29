#!/usr/bin/env bash
# End-to-end smoke test across all roles: buyer/customer, seller, admin.
# Covers auth, browse, cart, orders, seller promotion + shop moderation,
# products, seller dashboard/sales, the ship→confirm-receipt→review lifecycle,
# and RBAC checks. Requires the API running on :5000 + Postgres.
# Run inside nix develop (needs curl + jq):  nix develop -c ./test_full_api.sh
# (Cần bật flakes: nix.settings.experimental-features = [ "nix-command" "flakes" ];)
#
# Seeded accounts used: admin@ks.com / Admin@12345
set -uo pipefail
B="http://localhost:5000/api"
TS="$(date +%s)"
PASS=0; FAIL=0
ok(){ echo "  [PASS] $1"; PASS=$((PASS+1)); }
bad(){ echo "  [FAIL] $1 ${2:-}"; FAIL=$((FAIL+1)); }
chk(){ [ "$2" = "$3" ] && ok "$1" || bad "$1" "(expected '$3' got '$2')"; }
# req METHOD PATH TOKEN BODY  -> response body
req(){ local m="$1" p="$2" t="${3:-}" d="${4:-}"; local a=(-s -X "$m" "${B}${p}" -H "Content-Type: application/json"); [ -n "$t" ] && a+=(-H "Authorization: Bearer $t"); [ -n "$d" ] && a+=(-d "$d"); curl "${a[@]}"; }
# code METHOD PATH TOKEN BODY -> http status code
code(){ local m="$1" p="$2" t="${3:-}" d="${4:-}"; local a=(-s -o /dev/null -w '%{http_code}' -X "$m" "${B}${p}" -H "Content-Type: application/json"); [ -n "$t" ] && a+=(-H "Authorization: Bearer $t"); [ -n "$d" ] && a+=(-d "$d"); curl "${a[@]}"; }

echo "================ AUTH ================"
CE="cust${TS}@t.com"
REG=$(req POST /auth/register "" "{\"fullName\":\"Cust $TS\",\"email\":\"$CE\",\"userName\":\"cust${TS}\",\"password\":\"Passw0rd!\"}")
CT=$(echo "$REG" | jq -r '.data.accessToken // empty'); CR=$(echo "$REG" | jq -r '.data.refreshToken // empty')
[ -n "$CT" ] && ok "register customer" || bad "register customer"
chk "login ok" "$(code POST /auth/login "" "{\"email\":\"$CE\",\"password\":\"Passw0rd!\"}")" "200"
chk "login wrong pw -> 401" "$(code POST /auth/login "" "{\"email\":\"$CE\",\"password\":\"WRONG\"}")" "401"
chk "me role Customer" "$(req GET /auth/me "$CT" | jq -r '.data.info.role')" "Customer"
NT=$(req POST /auth/refresh "" "{\"refreshToken\":\"$CR\"}" | jq -r '.data.accessToken // empty'); [ -n "$NT" ] && ok "refresh token" || bad "refresh token"
CT="$NT"

echo "================ PUBLIC / BROWSE ================"
chk "categories 200" "$(code GET /categories)" "200"
[ "$(req GET /categories | jq -r '.data|length>0')" = "true" ] && ok "categories non-empty" || bad "categories non-empty"
PL=$(req GET "/products?pageSize=5")
[ "$(echo "$PL" | jq -r '.data.items|length>0')" = "true" ] && ok "products list" || bad "products list"
PSLUG=$(echo "$PL" | jq -r '.data.items[0].slug'); PID0=$(echo "$PL" | jq -r '.data.items[0].id')
chk "product detail 200" "$(code GET /products/$PSLUG)" "200"
chk "featured 200" "$(code GET /products/featured?take=4)" "200"
chk "reviews public 200" "$(code GET "/reviews?productId=$PID0")" "200"
chk "unknown product 404" "$(code GET /products/khong-ton-tai-$TS)" "404"

echo "================ CART (buyer) ================"
BUYP=$(req GET "/products?pageSize=50" "$CT" | jq -r '.data.items[]|select(.stockQuantity>1)|.id' | head -1)
chk "cart add" "$(code POST /cart "$CT" "{\"productId\":\"$BUYP\",\"quantity\":1}")" "200"
[ "$(req GET /cart "$CT" | jq -r '.data.items|length')" = "1" ] && ok "cart has 1 item" || bad "cart has 1 item"
req PUT "/cart/$BUYP" "$CT" '{"quantity":2}' >/dev/null
chk "cart qty updated to 2" "$(req GET /cart "$CT" | jq -r '.data.items[0].quantity')" "2"
chk "cart delete" "$(code DELETE "/cart/$BUYP" "$CT")" "200"
chk "cart empty after delete" "$(req GET /cart "$CT" | jq -r '.data.items|length')" "0"

echo "================ ORDER (buyer) ================"
chk "order with empty cart -> 400" "$(code POST /orders "$CT" '{"fullName":"C","phone":"0900000000","street":"1","ward":"w","district":"d","city":"HCM","note":""}')" "400"
req POST /cart "$CT" "{\"productId\":\"$BUYP\",\"quantity\":1}" >/dev/null
ORD=$(req POST /orders "$CT" '{"fullName":"Cust","phone":"0900000000","street":"1 St","ward":"W","district":"D","city":"HCM","note":""}')
OID=$(echo "$ORD" | jq -r '.data.id'); [ "$(echo "$ORD" | jq -r '.success')" = "true" ] && ok "create order" || bad "create order"
[ "$(req GET /orders "$CT" | jq -r --arg id "$OID" '[.data[]|select(.id==$id)]|length')" = "1" ] && ok "order in history" || bad "order in history"
chk "get order by id 200" "$(code GET /orders/$OID "$CT")" "200"
chk "review before delivered -> 403" "$(code POST /reviews "$CT" "{\"productId\":\"$BUYP\",\"rating\":5,\"comment\":\"x\"}")" "403"

echo "================ SELLER: promotion + shop ================"
SE="seller${TS}@t.com"
SREG=$(req POST /auth/register "" "{\"fullName\":\"Seller $TS\",\"email\":\"$SE\",\"userName\":\"seller${TS}\",\"password\":\"Passw0rd!\"}")
ST=$(echo "$SREG" | jq -r '.data.accessToken'); SR=$(echo "$SREG" | jq -r '.data.refreshToken')
chk "create shop 200" "$(code POST /shops "$ST" "{\"name\":\"Test Shop $TS\",\"slug\":\"test-shop-$TS\",\"description\":\"d\"}")" "200"
ST=$(req POST /auth/refresh "" "{\"refreshToken\":\"$SR\"}" | jq -r '.data.accessToken')  # refresh -> Seller role
chk "role now Seller" "$(req GET /auth/me "$ST" | jq -r '.data.info.role')" "Seller"
chk "shop status Pending" "$(req GET /shops/me "$ST" | jq -r '.data.status')" "Pending"
chk "product create while Pending -> 400" "$(code POST /products "$ST" "{\"name\":\"Prod\",\"slug\":\"p-$TS\",\"description\":\"d\",\"price\":10,\"stockQuantity\":5,\"sku\":\"S$TS\",\"images\":[]}")" "400"
SHOPID=$(req GET /shops/me "$ST" | jq -r '.data.id')

echo "================ ADMIN: moderation ================"
AT=$(req POST /auth/login "" '{"email":"admin@ks.com","password":"Admin@12345"}' | jq -r '.data.accessToken')
[ -n "$AT" ] && ok "admin login" || bad "admin login"
chk "admin dashboard 200" "$(code GET /admin/dashboard "$AT")" "200"
chk "admin shops list 200" "$(code GET "/admin/shops?status=Pending" "$AT")" "200"
chk "admin orders list 200" "$(code GET /admin/orders "$AT")" "200"
chk "admin users list 200" "$(code GET /admin/users "$AT")" "200"
chk "admin approve shop 200" "$(code POST /admin/shops/$SHOPID/approve "$AT")" "200"
chk "shop now Approved" "$(req GET /shops/me "$ST" | jq -r '.data.status')" "Approved"

echo "================ SELLER: products + dashboard + sales ================"
ST=$(req POST /auth/login "" "{\"email\":\"$SE\",\"password\":\"Passw0rd!\"}" | jq -r '.data.accessToken')
NEWP=$(req POST /products "$ST" "{\"name\":\"KTest $TS\",\"slug\":\"ktest-$TS\",\"description\":\"nice\",\"price\":100,\"salePrice\":80,\"stockQuantity\":10,\"sku\":\"K$TS\",\"images\":[\"https://picsum.photos/seed/k$TS/400\"]}")
NPID=$(echo "$NEWP" | jq -r '.data.id'); [ "$(echo "$NEWP" | jq -r '.success')" = "true" ] && ok "seller create product" || bad "seller create product"
chk "product in my list" "$(req GET /products/my "$ST" | jq -r --arg id "$NPID" '[.data[]|select(.id==$id)]|length')" "1"
chk "seller update product 200" "$(code PUT /products/$NPID "$ST" "{\"name\":\"KTest $TS v2\",\"slug\":\"ktest-$TS\",\"description\":\"nicer\",\"price\":110,\"stockQuantity\":9,\"sku\":\"K$TS\",\"isActive\":true,\"images\":[]}")" "200"
chk "seller dashboard 200" "$(code GET /seller/dashboard "$ST")" "200"
chk "seller sales 200" "$(code GET /orders/sales "$ST")" "200"

echo "================ FULL LIFECYCLE: buy -> ship -> confirm -> review ================"
BE="buyer${TS}@t.com"
BT=$(req POST /auth/register "" "{\"fullName\":\"Buyer $TS\",\"email\":\"$BE\",\"userName\":\"buyer${TS}\",\"password\":\"Passw0rd!\"}" | jq -r '.data.accessToken')
req POST /cart "$BT" "{\"productId\":\"$NPID\",\"quantity\":1}" >/dev/null
BOID=$(req POST /orders "$BT" '{"fullName":"Buyer","phone":"0900000000","street":"1 St","ward":"W","district":"D","city":"HCM","note":""}' | jq -r '.data.id')
[ -n "$BOID" ] && ok "buyer places order for seller product" || bad "buyer places order"
[ "$(req GET /orders/sales "$ST" | jq -r --arg id "$BOID" '[.data[]|select(.id==$id)]|length')" = "1" ] && ok "order shows in seller sales" || bad "order in seller sales"
for s in Confirmed Processing Shipped; do req PUT /orders/$BOID/status "$ST" "{\"status\":\"$s\"}" >/dev/null; done
chk "order Shipped" "$(req GET /orders/$BOID "$BT" | jq -r '.data.status')" "Shipped"
chk "seller cannot set Delivered -> 400" "$(code PUT /orders/$BOID/status "$ST" '{"status":"Delivered"}')" "400"
chk "buyer confirm received" "$(req POST /orders/$BOID/confirm-received "$BT" | jq -r '.data.status')" "Delivered"
chk "buyer review after delivered 200" "$(code POST /reviews "$BT" "{\"productId\":\"$NPID\",\"rating\":5,\"comment\":\"great $TS\"}")" "200"
chk "duplicate review -> 400" "$(code POST /reviews "$BT" "{\"productId\":\"$NPID\",\"rating\":4,\"comment\":\"again\"}")" "400"
chk "review visible on product" "$(req GET "/reviews?productId=$NPID" | jq -r '.data.reviewCount')" "1"

echo "================ RBAC / SECURITY ================"
chk "customer -> admin dashboard blocked" "$([ "$(code GET /admin/dashboard "$CT")" != "200" ] && echo blocked || echo open)" "blocked"
chk "customer -> seller dashboard blocked" "$([ "$(code GET /seller/dashboard "$CT")" != "200" ] && echo blocked || echo open)" "blocked"
chk "customer -> seller sales blocked" "$([ "$(code GET /orders/sales "$CT")" != "200" ] && echo blocked || echo open)" "blocked"
chk "no-token cart -> 401" "$(code GET /cart "")" "401"
chk "customer cannot update order status -> !=200" "$([ "$(code PUT /orders/$BOID/status "$CT" '{"status":"Cancelled"}')" != "200" ] && echo blocked || echo open)" "blocked"
chk "seller cannot approve shop -> !=200" "$([ "$(code POST /admin/shops/$SHOPID/approve "$ST")" != "200" ] && echo blocked || echo open)" "blocked"

echo ""
echo "================================================="
echo " RESULT: PASS=$PASS  FAIL=$FAIL"
echo "================================================="
[ "$FAIL" -eq 0 ]
