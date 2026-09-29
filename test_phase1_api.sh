#!/usr/bin/env bash
# Phase 1 (Authentication) API tests — mirrors the "Test cases" checklist in
# kernelstore_prompt.md §PHASE 1. Requires the API running on :5000 + Postgres.
# Run inside nix develop (needs curl + jq):  nix develop -c ./test_phase1_api.sh
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

echo "=== Phase 1 API tests (ts=${TS}) ==="

EMAIL="user${TS}@t.com"

# ── TC1: register → user saved with role Customer ─────────────────────────
echo "--- TC1: register → role Customer ---"
R="$(req POST /auth/register "" "{\"fullName\":\"User ${TS}\",\"email\":\"${EMAIL}\",\"userName\":\"user${TS}\",\"password\":\"Passw0rd!\"}")"
check "register success" "$(echo "$R" | jq -r '.success')" "true"
check "new user role == Customer" "$(echo "$R" | jq -r '.data.user.role')" "Customer"
check "register returns access token" "$([ -n "$(echo "$R" | jq -r '.data.accessToken')" ] && [ "$(echo "$R" | jq -r '.data.accessToken')" != "null" ] && echo yes)" "yes"
# duplicate email rejected
check "duplicate email rejected" "$(req POST /auth/register "" "{\"fullName\":\"Dup\",\"email\":\"${EMAIL}\",\"userName\":\"dup${TS}\",\"password\":\"Passw0rd!\"}" | jq -r '.success')" "false"

# ── TC2: login correct → JWT ──────────────────────────────────────────────
echo "--- TC2: login correct → JWT ---"
R="$(req POST /auth/login "" "{\"email\":\"${EMAIL}\",\"password\":\"Passw0rd!\"}")"
check "login success" "$(echo "$R" | jq -r '.success')" "true"
TOKEN="$(echo "$R" | jq -r '.data.accessToken')"
REFRESH="$(echo "$R" | jq -r '.data.refreshToken')"
check "login returns JWT" "$([ -n "$TOKEN" ] && [ "$TOKEN" != "null" ] && echo yes)" "yes"
check "JWT is 3-part token" "$(echo "$TOKEN" | awk -F. '{print NF}')" "3"

# ── TC3: login wrong → error (proper format) ──────────────────────────────
echo "--- TC3: login wrong → error ---"
R="$(req POST /auth/login "" "{\"email\":\"${EMAIL}\",\"password\":\"WRONGpass!\"}")"
check "wrong password → success=false" "$(echo "$R" | jq -r '.success')" "false"
check "wrong password → HTTP 401" "$(req_code POST /auth/login "" "{\"email\":\"${EMAIL}\",\"password\":\"WRONGpass!\"}")" "401"
check "wrong password → message present" "$([ "$(echo "$R" | jq -r '.message')" != "null" ] && [ -n "$(echo "$R" | jq -r '.message')" ] && echo yes)" "yes"
check "unknown email → HTTP 401" "$(req_code POST /auth/login "" '{"email":"nobody@nope.com","password":"Passw0rd!"}')" "401"

# ── TC4: protected (GetMe) with valid token → user info ───────────────────
echo "--- TC4: GetMe with valid token ---"
R="$(req GET /auth/me "$TOKEN")"
check "me success" "$(echo "$R" | jq -r '.success')" "true"
check "me returns matching email" "$(echo "$R" | jq -r '.data.info.email')" "$EMAIL"
check "me roles include Customer" "$(echo "$R" | jq -r '.data.roles | index("Customer") != null')" "true"

# ── TC5: protected without token → 401 ────────────────────────────────────
echo "--- TC5: GetMe without token → 401 ---"
check "no token → HTTP 401" "$(req_code GET /auth/me "")" "401"
check "garbage token → HTTP 401" "$(req_code GET /auth/me "not.a.real.jwt")" "401"

# ── TC6: refresh token works ──────────────────────────────────────────────
echo "--- TC6: refresh token ---"
R="$(req POST /auth/refresh "" "{\"refreshToken\":\"${REFRESH}\"}")"
check "refresh success" "$(echo "$R" | jq -r '.success')" "true"
NEWTOKEN="$(echo "$R" | jq -r '.data.accessToken')"
check "refresh returns new access token" "$([ -n "$NEWTOKEN" ] && [ "$NEWTOKEN" != "null" ] && echo yes)" "yes"
check "new token usable on GetMe" "$(req GET /auth/me "$NEWTOKEN" | jq -r '.data.info.email')" "$EMAIL"
# refresh token is single-use (rotated)
check "reused refresh token rejected" "$(req POST /auth/refresh "" "{\"refreshToken\":\"${REFRESH}\"}" | jq -r '.success')" "false"
check "invalid refresh token → HTTP 401" "$(req_code POST /auth/refresh "" '{"refreshToken":"bogus-token"}')" "401"

echo ""
echo "=== RESULT: ${PASS} passed, ${FAIL} failed ==="
[ "$FAIL" -eq 0 ]
