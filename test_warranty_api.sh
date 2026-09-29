#!/usr/bin/env bash
# End-to-end test for the product warranty feature (bảo hành).
# Covers: warranty period on products, claim creation with eligibility checks,
# customer listing/cancel, seller/admin approve->process->complete lifecycle.
# Run inside nix develop (needs curl + jq), backend on :5000:
#   nix develop -c ./test_warranty_api.sh
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

echo "================ WARRANTY (ts=$TS) ================"
AT=$(req POST /auth/login "" '{"email":"admin@ks.com","password":"Admin@12345"}' | jq -r '.data.accessToken')
S1=$(req POST /auth/login "" '{"email":"seller1@demo.ks","password":"Seller@12345"}' | jq -r '.data.accessToken')
[ -n "$S1" ] && [ "$S1" != "null" ] && ok "seller login" || { bad "seller login"; exit 1; }

# Products with & without warranty
WPID=$(req POST /products "$S1" "{\"name\":\"WrtP $TS\",\"slug\":\"wrtp-$TS\",\"description\":\"\",\"price\":100,\"stockQuantity\":10,\"sku\":\"W$TS\",\"warrantyMonths\":12,\"images\":[]}" | jq -r '.data.id')
chk "product created with warrantyMonths=12" "$(req GET /products/my "$S1" | jq -r --arg id "$WPID" '.data[]|select(.id==$id)|.warrantyMonths')" "12"
NPID=$(req POST /products "$S1" "{\"name\":\"NoWrt $TS\",\"slug\":\"nowrt-$TS\",\"description\":\"\",\"price\":50,\"stockQuantity\":10,\"sku\":\"N$TS\",\"warrantyMonths\":0,\"images\":[]}" | jq -r '.data.id')

# Buyer
BUY=$(req POST /auth/register "" "{\"fullName\":\"WBuyer $TS\",\"email\":\"wbuyer${TS}@t.com\",\"userName\":\"wbuyer${TS}\",\"password\":\"Passw0rd!\"}" | jq -r '.data.accessToken')

# Order with the warranty product, drive to Delivered
req POST /cart "$BUY" "{\"productId\":\"$WPID\",\"quantity\":1}" >/dev/null
OID=$(req POST /orders "$BUY" '{"fullName":"Buyer","phone":"0900000000","street":"1 St","ward":"W","district":"D","city":"HCM","note":""}' | jq -r '.data.id')
[ -n "$OID" ] && [ "$OID" != "null" ] && ok "order created" || { bad "order created"; exit 1; }
for s in Confirmed Processing Shipped; do req PUT "/orders/$OID/status" "$S1" "{\"status\":\"$s\"}" >/dev/null; done
chk "buyer confirm received -> Delivered" "$(req POST "/orders/$OID/confirm-received" "$BUY" | jq -r '.data.status')" "Delivered"

# OrderDetail id for the warranty product line
DETAIL=$(req GET "/orders/$OID" "$BUY" | jq -r --arg pid "$WPID" '.data.items[]|select(.productId==$pid)|.id')
[ -n "$DETAIL" ] && [ "$DETAIL" != "null" ] && ok "got orderDetailId" || { bad "got orderDetailId"; exit 1; }

# Negative: description too short -> 400
chk "short description rejected -> 400" "$(code POST /warranty "$BUY" "{\"orderDetailId\":\"$DETAIL\",\"description\":\"short\"}")" "400"

# Create warranty claim
CREATE=$(req POST /warranty "$BUY" "{\"orderDetailId\":\"$DETAIL\",\"description\":\"Bàn phím bị liệt phím Enter sau 1 tuần dùng.\",\"imageUrl\":\"\"}")
CLAIM=$(printf '%s' "$CREATE" | jq -r '.data.id')
chk "warranty claim created -> Pending" "$(printf '%s' "$CREATE" | jq -r '.data.status')" "Pending"
chk "claim has code" "$(printf '%s' "$CREATE" | jq -r '.data.claimCode' | grep -c '^WR-')" "1"
chk "claim shows warrantyMonths" "$(printf '%s' "$CREATE" | jq -r '.data.warrantyMonths')" "12"
chk "claim has expiry date" "$(printf '%s' "$CREATE" | jq -r '.data.warrantyExpiresAt|length>0')" "true"

# Duplicate open claim -> 400
chk "duplicate open claim -> 400" "$(code POST /warranty "$BUY" "{\"orderDetailId\":\"$DETAIL\",\"description\":\"Yêu cầu trùng lặp nội dung.\"}")" "400"

# Buyer sees it in /warranty/mine
chk "claim in buyer /warranty/mine" "$(req GET /warranty/mine "$BUY" | jq -r --arg id "$CLAIM" '[.data[]|select(.id==$id)]|length')" "1"

# Seller sees it in /warranty/shop
chk "claim in seller /warranty/shop" "$(req GET /warranty/shop "$S1" | jq -r --arg id "$CLAIM" '[.data[]|select(.id==$id)]|length')" "1"
chk "seller sees canManage=true" "$(req GET /warranty/shop "$S1" | jq -r --arg id "$CLAIM" '.data[]|select(.id==$id)|.canManage')" "true"

# Buyer (non-owner-seller) cannot approve -> 403
chk "buyer cannot approve -> 403" "$(code POST "/warranty/$CLAIM/approve" "$BUY" '{"resolution":"Repair"}')" "403"

# Seller approves with invalid resolution -> 400
chk "approve invalid resolution -> 400" "$(code POST "/warranty/$CLAIM/approve" "$S1" '{"resolution":"Bogus"}')" "400"

# Seller approves -> Approved + Repair
APP=$(req POST "/warranty/$CLAIM/approve" "$S1" '{"resolution":"Repair","note":"Nhận máy sửa trong 3 ngày"}')
chk "seller approve -> Approved" "$(printf '%s' "$APP" | jq -r '.data.status')" "Approved"
chk "resolution recorded -> Repair" "$(printf '%s' "$APP" | jq -r '.data.resolution')" "Repair"

# Seller starts processing -> Processing
chk "seller process -> Processing" "$(req POST "/warranty/$CLAIM/process" "$S1" | jq -r '.data.status')" "Processing"

# Seller completes -> Completed
chk "seller complete -> Completed" "$(req POST "/warranty/$CLAIM/complete" "$S1" '{"note":"Đã sửa xong, bàn giao"}' | jq -r '.data.status')" "Completed"

# Cannot cancel a completed claim (buyer) -> 400
chk "cancel completed claim -> 400" "$(code POST "/warranty/$CLAIM/cancel" "$BUY")" "400"

# Warranty on a product without coverage -> 400
req POST /cart "$BUY" "{\"productId\":\"$NPID\",\"quantity\":1}" >/dev/null
OID2=$(req POST /orders "$BUY" '{"fullName":"Buyer","phone":"0900000000","street":"1 St","ward":"W","district":"D","city":"HCM","note":""}' | jq -r '.data.id')
for s in Confirmed Processing Shipped; do req PUT "/orders/$OID2/status" "$S1" "{\"status\":\"$s\"}" >/dev/null; done
req POST "/orders/$OID2/confirm-received" "$BUY" >/dev/null
DETAIL2=$(req GET "/orders/$OID2" "$BUY" | jq -r --arg pid "$NPID" '.data.items[]|select(.productId==$pid)|.id')
chk "no-warranty product claim -> 400" "$(code POST /warranty "$BUY" "{\"orderDetailId\":\"$DETAIL2\",\"description\":\"Sản phẩm không có bảo hành nên bị từ chối.\"}")" "400"

# Cancel-a-pending flow: new claim then buyer cancels
req POST /cart "$BUY" "{\"productId\":\"$WPID\",\"quantity\":1}" >/dev/null
OID3=$(req POST /orders "$BUY" '{"fullName":"Buyer","phone":"0900000000","street":"1 St","ward":"W","district":"D","city":"HCM","note":""}' | jq -r '.data.id')
for s in Confirmed Processing Shipped; do req PUT "/orders/$OID3/status" "$S1" "{\"status\":\"$s\"}" >/dev/null; done
req POST "/orders/$OID3/confirm-received" "$BUY" >/dev/null
DETAIL3=$(req GET "/orders/$OID3" "$BUY" | jq -r --arg pid "$WPID" '.data.items[]|select(.productId==$pid)|.id')
CLAIM3=$(req POST /warranty "$BUY" "{\"orderDetailId\":\"$DETAIL3\",\"description\":\"Muốn bảo hành nhưng sẽ hủy để test.\"}" | jq -r '.data.id')
chk "buyer cancel pending claim -> Cancelled" "$(req POST "/warranty/$CLAIM3/cancel" "$BUY" | jq -r '.data.status')" "Cancelled"

echo "================ RESULT: PASS=$PASS FAIL=$FAIL ================"
[ "$FAIL" -eq 0 ]
