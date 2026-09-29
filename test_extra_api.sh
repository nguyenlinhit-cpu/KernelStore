#!/usr/bin/env bash
# Consolidated test for the remaining functionality NOT covered by
# test_full_api.sh / test_chat_api.sh:
#   category CRUD + block-delete, shop settings update, admin shop ban/unban/delete,
#   admin user ban/unban, order cancel + return flow, product delete,
#   refresh-token rotation. Run inside nix develop: nix develop -c ./test_extra_api.sh
# (Cần bật flakes: nix.settings.experimental-features = [ "nix-command" "flakes" ];)
set -uo pipefail

B="http://localhost:5000/api"
TS="$(date +%s)"
PASS=0; FAIL=0
ok(){ echo "  [PASS] $1"; PASS=$((PASS+1)); }
bad(){ echo "  [FAIL] $1 ${2:-}"; FAIL=$((FAIL+1)); }
chk(){ [ "$2" = "$3" ] && ok "$1" || bad "$1" "(expected '$3' got '$2')"; }
req(){ local m="$1" p="$2" t="${3:-}" d="${4:-}"; local a=(-s -X "$m" "${B}${p}" -H "Content-Type: application/json"); [ -n "$t" ] && a+=(-H "Authorization: Bearer $t"); [ -n "$d" ] && a+=(-d "$d"); curl "${a[@]}"; }
code(){ local m="$1" p="$2" t="${3:-}" d="${4:-}"; local a=(-s -o /dev/null -w '%{http_code}' -X "$m" "${B}${p}" -H "Content-Type: application/json"); [ -n "$t" ] && a+=(-H "Authorization: Bearer $t"); [ -n "$d" ] && a+=(-d "$d"); curl "${a[@]}"; }

echo "================ EXTRA FEATURES (ts=$TS) ================"

AT=$(req POST /auth/login "" '{"email":"admin@ks.com","password":"Admin@12345"}' | jq -r '.data.accessToken')
S1=$(req POST /auth/login "" '{"email":"seller1@demo.ks","password":"Seller@12345"}' | jq -r '.data.accessToken')
S1SHOP=$(req GET /shops/me "$S1" | jq -r '.data.id')
[ -n "$AT" ] && [ -n "$S1" ] && [ -n "$S1SHOP" ] && ok "setup: admin + seller1 + shop" || { bad "setup"; exit 1; }

BUY=$(req POST /auth/register "" "{\"fullName\":\"Buyer $TS\",\"email\":\"exbuyer${TS}@t.com\",\"userName\":\"exbuyer${TS}\",\"password\":\"Passw0rd!\"}" | jq -r '.data.accessToken')

echo "--- categories CRUD (admin) ---"
CID=$(req POST /categories "$AT" "{\"name\":\"Cat $TS\",\"slug\":\"cat-$TS\",\"description\":\"root\"}" | jq -r '.data.id')
[ -n "$CID" ] && [ "$CID" != "null" ] && ok "admin create category" || bad "admin create category"
chk "non-admin create category -> 403" "$(code POST /categories "$BUY" "{\"name\":\"X\",\"slug\":\"x-$TS\",\"description\":\"\"}")" "403"
CHILD=$(req POST /categories "$AT" "{\"name\":\"Child $TS\",\"slug\":\"child-$TS\",\"description\":\"child\",\"parentId\":\"$CID\"}" | jq -r '.data.id')
[ -n "$CHILD" ] && [ "$CHILD" != "null" ] && ok "admin create child category" || bad "create child"
chk "admin update category 200" "$(code PUT /categories/$CID "$AT" "{\"name\":\"Cat $TS v2\",\"slug\":\"cat-$TS\",\"description\":\"u\",\"parentId\":null}")" "200"
chk "delete root with child -> blocked 400" "$(code DELETE /categories/$CID "$AT")" "400"
chk "delete child category 200" "$(code DELETE /categories/$CHILD "$AT")" "200"
CLEAN=$(req POST /categories "$AT" "{\"name\":\"Clean $TS\",\"slug\":\"clean-$TS\",\"description\":\"\"}" | jq -r '.data.id')
chk "delete empty category 200" "$(code DELETE /categories/$CLEAN "$AT")" "200"
chk "delete unknown category -> 404" "$(code DELETE /categories/00000000-0000-0000-0000-000000000000 "$AT")" "404"

echo "--- shop settings (PUT /shops/me) ---"
chk "update own shop 200" "$(code PUT /shops/me "$S1" "{\"name\":\"Kernel Gadgets\",\"slug\":\"kernel-gadgets\",\"description\":\"updated $TS\"}")" "200"
chk "shop description updated" "$(req GET /shops/me "$S1" | jq -r '.data.description')" "updated $TS"
S2=$(req POST /auth/register "" "{\"fullName\":\"S2 $TS\",\"email\":\"s2${TS}@t.com\",\"userName\":\"s2${TS}\",\"password\":\"Passw0rd!\"}" | jq -r '.data.accessToken')
req POST /shops "$S2" "{\"name\":\"S2 $TS\",\"slug\":\"s2-$TS\",\"description\":\"\"}" >/dev/null
chk "slug conflict -> 400" "$(code PUT /shops/me "$S2" "{\"name\":\"S2 $TS\",\"slug\":\"kernel-gadgets\",\"description\":\"\"}")" "400"
req PUT /shops/me "$S1" "{\"name\":\"Kernel Gadgets\",\"slug\":\"kernel-gadgets\",\"description\":\"Laptops, phones and dev gear.\"}" >/dev/null

echo "--- admin shop ban / unban / delete ---"
S3=$(req POST /auth/register "" "{\"fullName\":\"S3 $TS\",\"email\":\"s3${TS}@t.com\",\"userName\":\"s3${TS}\",\"password\":\"Passw0rd!\"}" | jq -r '.data.accessToken')
S3SHOP=$(req POST /shops "$S3" "{\"name\":\"S3 $TS\",\"slug\":\"s3-$TS\",\"description\":\"\"}" | jq -r '.data.id')
req POST "/admin/shops/$S3SHOP/approve" "$AT" >/dev/null
chk "ban shop -> Banned" "$(req POST "/admin/shops/$S3SHOP/ban" "$AT" | jq -r '.data.status')" "Banned"
chk "unban shop -> Approved" "$(req POST "/admin/shops/$S3SHOP/unban" "$AT" | jq -r '.data.status')" "Approved"
chk "delete shop -> 200" "$(code DELETE "/admin/shops/$S3SHOP" "$AT")" "200"
chk "deleted shop not visible in shops/me" "$(req GET /shops/me "$S3" | jq -r '.data')" "null"

echo "--- admin user ban / unban ---"
VICTIM=$(req POST /auth/register "" "{\"fullName\":\"V $TS\",\"email\":\"v${TS}@t.com\",\"userName\":\"v${TS}\",\"password\":\"Passw0rd!\"}" | jq -r '.data.user.id')
chk "ban user 200" "$(code POST "/admin/users/$VICTIM/ban" "$AT")" "200"
chk "banned user login -> 401" "$(code POST /auth/login "" "{\"email\":\"v${TS}@t.com\",\"password\":\"Passw0rd!\"}")" "401"
chk "unban user 200" "$(code POST "/admin/users/$VICTIM/unban" "$AT")" "200"
chk "unbanned user login -> 200" "$(code POST /auth/login "" "{\"email\":\"v${TS}@t.com\",\"password\":\"Passw0rd!\"}")" "200"
ADMIN_ID=$(req GET /auth/me "$AT" | jq -r '.data.info.id')
chk "ban self -> 400" "$(code POST "/admin/users/$ADMIN_ID/ban" "$AT")" "400"
chk "ban unknown user -> 404" "$(code POST /admin/users/00000000-0000-0000-0000-000000000000/ban "$AT")" "404"

echo "--- order cancel (restores stock) ---"
SPID=$(req GET /products/my "$S1" | jq -r '.data[]|select(.stockQuantity>1)|.id' | head -1)
STOCK0=$(req GET "/products/my" "$S1" | jq -r --arg id "$SPID" '.data[]|select(.id==$id)|.stockQuantity')
req POST /cart "$BUY" "{\"productId\":\"$SPID\",\"quantity\":2}" >/dev/null
COID=$(req POST /orders "$BUY" '{"fullName":"Buyer","phone":"0900000000","street":"1 St","ward":"W","district":"D","city":"HCM","note":""}' | jq -r '.data.id')
[ -n "$COID" ] && [ "$COID" != "null" ] && ok "cancel-flow order created" || { bad "cancel-flow order created"; exit 1; }
chk "cancel order -> Cancelled" "$(req POST "/orders/$COID/cancel" "$BUY" | jq -r '.data.status')" "Cancelled"
STOCK1=$(req GET "/products/my" "$S1" | jq -r --arg id "$SPID" '.data[]|select(.id==$id)|.stockQuantity')
chk "stock restored after cancel (back to original)" "$((STOCK1 - STOCK0))" "0"

echo "--- return flow ---"
req POST /cart "$BUY" "{\"productId\":\"$SPID\",\"quantity\":1}" >/dev/null
ROID=$(req POST /orders "$BUY" '{"fullName":"Buyer","phone":"0900000000","street":"1 St","ward":"W","district":"D","city":"HCM","note":""}' | jq -r '.data.id')
[ -n "$ROID" ] && [ "$ROID" != "null" ] && ok "return-flow order created" || { bad "return-flow order created"; exit 1; }
for s in Confirmed Processing Shipped; do req PUT "/orders/$ROID/status" "$S1" "{\"status\":\"$s\"}" >/dev/null; done
chk "buyer confirm received -> Delivered" "$(req POST "/orders/$ROID/confirm-received" "$BUY" | jq -r '.data.status')" "Delivered"
chk "return request -> ReturnRequested" "$(req POST "/orders/$ROID/return" "$BUY" | jq -r '.data.status')" "ReturnRequested"
chk "seller approve return -> Returned" "$(req POST "/orders/$ROID/return/approve" "$S1" | jq -r '.data.status')" "Returned"

echo "--- product delete ---"
S4REG=$(req POST /auth/register "" "{\"fullName\":\"S4 $TS\",\"email\":\"s4${TS}@t.com\",\"userName\":\"s4${TS}\",\"password\":\"Passw0rd!\"}")
S4=$(printf '%s' "$S4REG" | jq -r '.data.accessToken')
S4R=$(printf '%s' "$S4REG" | jq -r '.data.refreshToken')
S4SHOP=$(req POST /shops "$S4" "{\"name\":\"S4 $TS\",\"slug\":\"s4-$TS\",\"description\":\"\"}" | jq -r '.data.id')
req POST "/admin/shops/$S4SHOP/approve" "$AT" >/dev/null
S4=$(req POST /auth/refresh "" "{\"refreshToken\":\"$S4R\"}" | jq -r '.data.accessToken')  # pickup Seller role
chk "role now Seller" "$(req GET /auth/me "$S4" | jq -r '.data.info.role')" "Seller"
DELPID=$(req POST /products "$S4" "{\"name\":\"Del $TS\",\"slug\":\"del-$TS\",\"description\":\"\",\"price\":10,\"stockQuantity\":5,\"sku\":\"D$TS\",\"images\":[]}" | jq -r '.data.id')
[ -n "$DELPID" ] && [ "$DELPID" != "null" ] && ok "seller create product" || { bad "seller create product"; exit 1; }
chk "delete own product 200" "$(code DELETE "/products/$DELPID" "$S4")" "200"
chk "deleted product gone from my list" "$(req GET /products/my "$S4" | jq -r --arg id "$DELPID" '[.data[]|select(.id==$id)]|length')" "0"

echo "--- customer -> seller role promotion (register + open shop) ---"
PREG=$(req POST /auth/register "" "{\"fullName\":\"P $TS\",\"email\":\"p${TS}@t.com\",\"userName\":\"p${TS}\",\"password\":\"Passw0rd!\"}")
PT=$(printf '%s' "$PREG" | jq -r '.data.accessToken')
PR=$(printf '%s' "$PREG" | jq -r '.data.refreshToken')
chk "role after register -> Customer" "$(req GET /auth/me "$PT" | jq -r '.data.info.role')" "Customer"
PSHOP=$(req POST /shops "$PT" "{\"name\":\"P Shop $TS\",\"slug\":\"p-$TS\",\"description\":\"\"}" | jq -r '.data.id')
[ -n "$PSHOP" ] && [ "$PSHOP" != "null" ] && ok "customer open shop" || { bad "customer open shop"; exit 1; }
PT=$(req POST /auth/refresh "" "{\"refreshToken\":\"$PR\"}" | jq -r '.data.accessToken')
chk "role after open shop + refresh -> Seller" "$(req GET /auth/me "$PT" | jq -r '.data.info.role')" "Seller"
chk "shop of promoted user still Pending" "$(req GET /shops/me "$PT" | jq -r '.data.status')" "Pending"
chk "product create while Pending -> 400" "$(code POST /products "$PT" "{\"name\":\"Prod\",\"slug\":\"pp-$TS\",\"description\":\"\",\"price\":10,\"stockQuantity\":5,\"sku\":\"PP$TS\",\"images\":[]}")" "400"
req POST "/admin/shops/$PSHOP/approve" "$AT" >/dev/null
chk "shop Approved after admin approve" "$(req GET /shops/me "$PT" | jq -r '.data.status')" "Approved"
chk "product create after approve -> 200" "$(code POST /products "$PT" "{\"name\":\"Prod2\",\"slug\":\"pp2-$TS\",\"description\":\"\",\"price\":10,\"stockQuantity\":5,\"sku\":\"PP2$TS\",\"images\":[]}")" "200"

echo "--- temporary ban: product visibility hidden/restored ---"
TREG=$(req POST /auth/register "" "{\"fullName\":\"T $TS\",\"email\":\"t${TS}@t.com\",\"userName\":\"t${TS}\",\"password\":\"Passw0rd!\"}")
TT=$(printf '%s' "$TREG" | jq -r '.data.accessToken')
TR=$(printf '%s' "$TREG" | jq -r '.data.refreshToken')
TSHOP=$(req POST /shops "$TT" "{\"name\":\"T Shop $TS\",\"slug\":\"t-$TS\",\"description\":\"\"}" | jq -r '.data.id')
req POST "/admin/shops/$TSHOP/approve" "$AT" >/dev/null
TT=$(req POST /auth/refresh "" "{\"refreshToken\":\"$TR\"}" | jq -r '.data.accessToken')
TPID=$(req POST /products "$TT" "{\"name\":\"T Prod $TS\",\"slug\":\"tprod-$TS\",\"description\":\"\",\"price\":15,\"stockQuantity\":7,\"sku\":\"T$TS\",\"images\":[]}" | jq -r '.data.id')
chk "product public detail before ban -> 200" "$(code GET "/products/tprod-$TS")" "200"
chk "products?shop=t-... total before ban -> 1" "$(req GET "/products?shop=t-$TS" | jq -r '.data.total')" "1"
chk "admin ban (temporary) -> Banned" "$(req POST "/admin/shops/$TSHOP/ban" "$AT" | jq -r '.data.status')" "Banned"
chk "public detail after ban -> 404" "$(code GET "/products/tprod-$TS")" "404"
chk "products?shop=t-... total after ban -> 0" "$(req GET "/products?shop=t-$TS" | jq -r '.data.total')" "0"
chk "product isActive false in /products/my" "$(req GET /products/my "$TT" | jq -r --arg id "$TPID" '.data[]|select(.id==$id)|.isActive')" "false"
chk "product create while Banned -> 400" "$(code POST /products "$TT" "{\"name\":\"BanProd\",\"slug\":\"bp-$TS\",\"description\":\"\",\"price\":15,\"stockQuantity\":3,\"sku\":\"BP$TS\",\"images\":[]}")" "400"
chk "product update while Banned -> 400" "$(code PUT "/products/$TPID" "$TT" "{\"name\":\"T Prod $TS\",\"slug\":\"tprod-$TS\",\"description\":\"x\",\"price\":15,\"stockQuantity\":7,\"sku\":\"T$TS\",\"images\":[],\"isActive\":true}")" "400"
chk "unban -> Approved" "$(req POST "/admin/shops/$TSHOP/unban" "$AT" | jq -r '.data.status')" "Approved"
chk "public detail after unban -> 200" "$(code GET "/products/tprod-$TS")" "200"
chk "products?shop=t-... total after unban -> 1" "$(req GET "/products?shop=t-$TS" | jq -r '.data.total')" "1"

echo "--- permanent ban (soft delete) when shop has orders ---"
UBREG=$(req POST /auth/register "" "{\"fullName\":\"U $TS\",\"email\":\"u${TS}@t.com\",\"userName\":\"u${TS}\",\"password\":\"Passw0rd!\"}")
UT=$(printf '%s' "$UBREG" | jq -r '.data.accessToken')
UR=$(printf '%s' "$UBREG" | jq -r '.data.refreshToken')
USHOP=$(req POST /shops "$UT" "{\"name\":\"U Shop $TS\",\"slug\":\"u-$TS\",\"description\":\"\"}" | jq -r '.data.id')
req POST "/admin/shops/$USHOP/approve" "$AT" >/dev/null
UT=$(req POST /auth/refresh "" "{\"refreshToken\":\"$UR\"}" | jq -r '.data.accessToken')
UPID=$(req POST /products "$UT" "{\"name\":\"U Prod $TS\",\"slug\":\"uprod-$TS\",\"description\":\"\",\"price\":20,\"stockQuantity\":5,\"sku\":\"U$TS\",\"images\":[]}" | jq -r '.data.id')
req POST /cart "$BUY" "{\"productId\":\"$UPID\",\"quantity\":1}" >/dev/null
UOID=$(req POST /orders "$BUY" '{"fullName":"Buyer","phone":"0900000000","street":"1 St","ward":"W","district":"D","city":"HCM","note":""}' | jq -r '.data.id')
[ -n "$UOID" ] && [ "$UOID" != "null" ] && ok "order placed against shop" || { bad "order placed against shop"; exit 1; }
chk "admin delete (permanent) -> 200" "$(code DELETE "/admin/shops/$USHOP" "$AT")" "200"
chk "shop status -> Deleted" "$(req GET "/admin/shops?status=Deleted" "$AT" | jq -r --arg id "$USHOP" '.data[]|select(.id==$id)|.status')" "Deleted"
chk "product hidden after permanent ban -> 404" "$(code GET "/products/uprod-$TS")" "404"
chk "buyer still sees order in history" "$(req GET /orders "$BUY" | jq -r --arg id "$UOID" '[.data[]|select(.id==$id)]|length')" "1"
chk "order item product name preserved" "$(req GET "/orders/$UOID" "$BUY" | jq -r '.data.items[0].productName')" "U Prod $TS"

echo "--- refresh token rotation (reuse rejected) ---"
RR=$(req POST /auth/login "" "{\"email\":\"exbuyer${TS}@t.com\",\"password\":\"Passw0rd!\"}" | jq -r '.data.refreshToken')
req POST /auth/refresh "" "{\"refreshToken\":\"$RR\"}" >/dev/null
chk "refresh-token reuse -> 401" "$(code POST /auth/refresh "" "{\"refreshToken\":\"$RR\"}")" "401"

echo "--- product image upload (jpg/png/svg) ---"
UPSVG="$(mktemp --suffix=.svg)"
printf '<svg xmlns="http://www.w3.org/2000/svg" width="8" height="8"><rect width="8" height="8" fill="lime"/></svg>' > "$UPSVG"
UPTXT="$(mktemp --suffix=.txt)"; echo nope > "$UPTXT"
UPPNG="$(mktemp --suffix=.png)"
printf 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==' | base64 -d > "$UPPNG"
chk "upload without auth -> 401" "$(curl -s -o /dev/null -w '%{http_code}' -X POST "$B/uploads/image" -F "file=@$UPSVG;type=image/svg+xml")" "401"
chk "upload bad ext (.txt) -> 400" "$(curl -s -o /dev/null -w '%{http_code}' -X POST "$B/uploads/image" -H "Authorization: Bearer $S1" -F "file=@$UPTXT;type=text/plain")" "400"
IMGURL=$(curl -s -X POST "$B/uploads/image" -H "Authorization: Bearer $S1" -F "file=@$UPSVG;type=image/svg+xml" | jq -r '.data.url')
[ -n "$IMGURL" ] && [ "$IMGURL" != "null" ] && ok "upload svg returns url" || bad "upload svg returns url"
chk "uploaded file served -> 200" "$(curl -s -o /dev/null -w '%{http_code}' "$IMGURL")" "200"
chk "served file has nosniff header" "$(curl -s -D - -o /dev/null "$IMGURL" | grep -ci 'X-Content-Type-Options: nosniff')" "1"
IMGSLUG="img-prod-$TS"
IPID=$(req POST /products "$S1" "{\"name\":\"Img Prod $TS\",\"slug\":\"$IMGSLUG\",\"description\":\"d\",\"price\":30,\"stockQuantity\":5,\"sku\":\"IMG$TS\",\"isActive\":true,\"images\":[\"$IMGURL\"]}" | jq -r '.data.id')
[ -n "$IPID" ] && [ "$IPID" != "null" ] && ok "seller create product with uploaded image" || bad "seller create product with uploaded image"
chk "buyer sees uploaded image on detail" "$(req GET "/products/$IMGSLUG" | jq -r '.data.images[0].url')" "$IMGURL"
IMG2=$(curl -s -X POST "$B/uploads/image" -H "Authorization: Bearer $S1" -F "file=@$UPPNG;type=image/png" | jq -r '.data.url')
req PUT "/products/$IPID" "$S1" "{\"name\":\"Img Prod $TS\",\"slug\":\"$IMGSLUG\",\"description\":\"d\",\"price\":30,\"salePrice\":null,\"stockQuantity\":5,\"sku\":\"IMG$TS\",\"isActive\":true,\"images\":[\"$IMG2\"]}" >/dev/null
chk "seller edit replaces image; buyer sees new one" "$(req GET "/products/$IMGSLUG" | jq -r '.data.images[0].url')" "$IMG2"
req DELETE "/products/$IPID" "$S1" >/dev/null
rm -f "$UPSVG" "$UPTXT" "$UPPNG"

echo ""
echo "================================================="
echo " RESULT: PASS=$PASS  FAIL=$FAIL"
echo "================================================="
[ "$FAIL" -eq 0 ]
