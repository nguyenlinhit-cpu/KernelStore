#!/usr/bin/env bash
# Real-time chat (Phase 8.1) API + WebSocket tests.
# Requires the API running on :5000 + Postgres. Run inside nix develop:  nix develop -c ./test_chat_api.sh
# (Cần bật flakes: nix.settings.experimental-features = [ "nix-command" "flakes" ];)
set -uo pipefail

BASE="http://localhost:5000/api"
WS="http://localhost:5000/ws/chat"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
WSCLIENT_SRC="${SCRIPT_DIR}/test/wsclient"
WSCLIENT="${WSCLIENT_SRC}/wsclient"
if [ ! -f "$WSCLIENT" ]; then
  echo "  (building wsclient WebSocket probe...)"
  (cd "$WSCLIENT_SRC" && CGO_ENABLED=0 go build -o wsclient . >/dev/null 2>&1) || { echo "cannot build wsclient"; exit 1; }
fi
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

echo "=== Chat real-time tests (ts=${TS}) ==="

# ── Setup ────────────────────────────────────────────────────────────────
echo "--- setup ---"
SELLER_TOKEN="$(req POST /auth/login "" '{"email":"seller1@demo.ks","password":"Seller@12345"}' | jq -r '.data.accessToken')"
[ -n "$SELLER_TOKEN" ] && [ "$SELLER_TOKEN" != "null" ] && ok "seller1 login" || { bad "seller1 login"; exit 1; }

SHOP_ID="$(req GET /shops/me "$SELLER_TOKEN" | jq -r '.data.id')"
[ -n "$SHOP_ID" ] && [ "$SHOP_ID" != "null" ] && ok "seller1 has shop ($SHOP_ID)" || { bad "seller shop"; exit 1; }

BUYER_EMAIL="buyer${TS}@t.com"
req POST /auth/register "" "{\"fullName\":\"Buyer ${TS}\",\"email\":\"${BUYER_EMAIL}\",\"userName\":\"buyer${TS}\",\"password\":\"Passw0rd!\"}" >/dev/null
BUYER_TOKEN="$(req POST /auth/login "" "{\"email\":\"${BUYER_EMAIL}\",\"password\":\"Passw0rd!\"}" | jq -r '.data.accessToken')"
[ -n "$BUYER_TOKEN" ] && [ "$BUYER_TOKEN" != "null" ] && ok "buyer registered + logged in" || { bad "buyer"; exit 1; }

THIRD_EMAIL="third${TS}@t.com"
req POST /auth/register "" "{\"fullName\":\"Third ${TS}\",\"email\":\"${THIRD_EMAIL}\",\"userName\":\"third${TS}\",\"password\":\"Passw0rd!\"}" >/dev/null
THIRD_TOKEN="$(req POST /auth/login "" "{\"email\":\"${THIRD_EMAIL}\",\"password\":\"Passw0rd!\"}" | jq -r '.data.accessToken')"

# ── Auth ─────────────────────────────────────────────────────────────────
echo "--- auth ---"
check "no token → conversations 401" "$(req_code GET /chat/conversations)" "401"
check "no token → send message 401" "$(req_code POST /chat/conversations/00000000-0000-0000-0000-000000000000/messages "" '{"content":"hi"}')" "401"
check "invalid ws token → reject" "$(timeout 6 "$WSCLIENT" "not-a-jwt" 3 | grep -c connected || true)" "0"
check "missing ws token → reject" "$(timeout 6 "$WSCLIENT" "" 3 | grep -c connected || true)" "0"

# ── Conversations ────────────────────────────────────────────────────────
echo "--- start conversation ---"
CONVO_JSON="$(req POST /chat/conversations "$BUYER_TOKEN" "{\"shopId\":\"${SHOP_ID}\"}")"
CONVO_ID="$(echo "$CONVO_JSON" | jq -r '.data.id')"
[ -n "$CONVO_ID" ] && [ "$CONVO_ID" != "null" ] && ok "buyer starts conversation with shop" || { bad "start conversation: $CONVO_JSON"; exit 1; }

CONVO2="$(req POST /chat/conversations "$BUYER_TOKEN" "{\"shopId\":\"${SHOP_ID}\"}")"
check "re-open conversation is idempotent (same id)" "$(echo "$CONVO2" | jq -r '.data.id')" "$CONVO_ID"

check "seller cannot chat with own shop → 400" "$(req_code POST /chat/conversations "$SELLER_TOKEN" "{\"shopId\":\"${SHOP_ID}\"}")" "400"
check "chat with unknown shop → 404" "$(req_code POST /chat/conversations "$BUYER_TOKEN" '{"shopId":"00000000-0000-0000-0000-000000000000"}')" "404"

check "buyer conversation list contains convo" "$(req GET /chat/conversations "$BUYER_TOKEN" | jq -r '.data | length')" "1"
SELLER_HAS="$(req GET /chat/conversations "$SELLER_TOKEN" | jq -r --arg id "$CONVO_ID" '.data | any(.id == $id)')"
check "seller conversation list contains convo" "$SELLER_HAS" "true"

# ── Send / receive messages ──────────────────────────────────────────────
echo "--- messages ---"
check "send empty content → 400" "$(req_code POST /chat/conversations/${CONVO_ID}/messages "$BUYER_TOKEN" '{"content":"   "}')" "400"
LONG_MSG="$(printf 'x%.0s' $(seq 1 2001))"
check "send >2000 chars → 400" "$(req_code POST /chat/conversations/${CONVO_ID}/messages "$BUYER_TOKEN" "{\"content\":\"${LONG_MSG}\"}")" "400"

check "buyer sends message → 200" "$(req_code POST /chat/conversations/${CONVO_ID}/messages "$BUYER_TOKEN" '{"content":"hello seller"}' | head -c 3)" "200"
MSG1="$(req POST /chat/conversations/${CONVO_ID}/messages "$BUYER_TOKEN" '{"content":"xin chào, có hàng không?"}' | jq -r '.data.content')"
check "buyer message persisted" "$MSG1" "xin chào, có hàng không?"

check "third user (not participant) send → 403" "$(req_code POST /chat/conversations/${CONVO_ID}/messages "$THIRD_TOKEN" '{"content":"hack"}')" "403"
check "third user (not participant) read → 403" "$(req_code GET /chat/conversations/${CONVO_ID}/messages "$THIRD_TOKEN")" "403"

MSGS_BUYER="$(req GET /chat/conversations/${CONVO_ID}/messages "$BUYER_TOKEN")"
check "buyer history has 2 messages" "$(echo "$MSGS_BUYER" | jq -r '.data | length')" "2"

# ── Unread counts ────────────────────────────────────────────────────────
echo "--- unread ---"
SELLER_UNREAD="$(req GET /chat/conversations "$SELLER_TOKEN" | jq -r '.data[0].unreadCount')"
[ "$SELLER_UNREAD" -ge 1 ] && ok "seller sees unread ≥1 (buyer's messages)" || bad "seller unread" "(got $SELLER_UNREAD)"
check "seller history marks read → 200" "$(req_code GET /chat/conversations/${CONVO_ID}/messages "$SELLER_TOKEN")" "200"
check "seller unread now 0" "$(req GET /chat/conversations "$SELLER_TOKEN" | jq -r '.data[0].unreadCount')" "0"
BUYER_UNREAD="$(req GET /chat/conversations "$BUYER_TOKEN" | jq -r '.data[0].unreadCount')"
check "buyer unread 0 (own messages not counted)" "$BUYER_UNREAD" "0"

# ── Real-time via WebSocket ──────────────────────────────────────────────
echo "--- websocket realtime ---"
rm -f /tmp/opencode/ws_buyer.log
timeout 12 "$WSCLIENT" "$BUYER_TOKEN" 10 > /tmp/opencode/ws_buyer.log 2>&1 &
WSPID=$!
sleep 2
grep -q "\[ws\] connected" /tmp/opencode/ws_buyer.log && ok "buyer ws connected" || bad "buyer ws connected"
req POST /chat/conversations/${CONVO_ID}/messages "$SELLER_TOKEN" '{"content":"có chứ bạn, còn hàng nhé!"}' >/dev/null
sleep 3
grep -q 'c\\u00F3 ch\\u1EE9 b\\u1EA1n' /tmp/opencode/ws_buyer.log && ok "seller message pushed to buyer ws in realtime" || bad "ws push to buyer"
wait $WSPID

rm -f /tmp/opencode/ws_seller.log
timeout 12 "$WSCLIENT" "$SELLER_TOKEN" 10 > /tmp/opencode/ws_seller.log 2>&1 &
WSPID=$!
sleep 2
req POST /chat/conversations/${CONVO_ID}/messages "$BUYER_TOKEN" '{"content":"ok đặt nhé"}' >/dev/null
sleep 3
grep -q 'ok \\u0111\\u1EB7t nh\\u00E9' /tmp/opencode/ws_seller.log && ok "buyer message pushed to seller ws in realtime" || bad "ws push to seller"
wait $WSPID

# ── Summary ──────────────────────────────────────────────────────────────
echo ""
echo "=== RESULT: ${PASS} passed, ${FAIL} failed ==="
[ "$FAIL" -eq 0 ]
