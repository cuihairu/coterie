#!/usr/bin/env bash
# Smoke test: walks the full Phase 1 MVP journey against a running
# Coterie server. Usage: scripts/smoke.sh [base_url]
# Requires curl and python3.
set -euo pipefail

BASE="${1:-http://localhost:8080}"
J="import sys,json;d=json.load(sys.stdin);print(d{})"
step() { printf '\n== %s\n' "$1"; }
expect() { # expect <actual> <want> <label>
  if [ "$1" != "$2" ]; then
    echo "FAIL $3: got '$1', want '$2'" >&2
    exit 1
  fi
  echo "ok  $3"
}

code_of() { curl -s -o /dev/null -w '%{http_code}' "$@"; }
api() { # api <method> <path> [token] [body] -> body
  local method="$1" path="$2" token="${3:-}" body="${4:-}"
  local args=(-s -X "$method" "$BASE$path" -H 'Content-Type: application/json')
  [ -n "$token" ] && args+=(-H "Authorization: Bearer $token")
  [ -n "$body" ] && args+=(-d "$body")
  curl "${args[@]}"
}
jget() { python3 -c "${J/\{\}/$1}" <<<"$2"; }

step "register owner and login"
TAG="smoke-$(date +%s)"
REG=$(api POST /api/v1/auth/register "" "{\"username\":\"$TAG\",\"email\":\"$TAG@example.com\",\"password\":\"password-123\"}")
TOK=$(jget "['token']" "$REG")
[ -n "$TOK" ] || { echo "FAIL register: $REG" >&2; exit 1; }
expect "$(code_of -H "Authorization: Bearer $TOK" "$BASE/api/v1/auth/me")" 200 "auth/me with token"
expect "$(code_of "$BASE/api/v1/users")" 401 "protected route rejects anonymous"

step "catalog: provider → product"
PROV=$(api POST /api/v1/providers "$TOK" "{\"slug\":\"smoke-$TAG\",\"name\":\"Smoke Provider\",\"category\":\"video\"}")
PID=$(jget "['id']" "$PROV")
PROD=$(api POST /api/v1/products "$TOK" "{\"provider_id\":\"$PID\",\"name\":\"Premium\"}")
PRID=$(jget "['id']" "$PROD")
[ -n "$PRID" ] || { echo "FAIL catalog" >&2; exit 1; }
echo "ok  provider $PID / product $PRID"

step "subscription with seat capacity"
SUB=$(api POST /api/v1/subscriptions "$TOK" "{\"product_id\":\"$PRID\",\"owner_user_id\":\"$(jget "['user']['id']" "$REG")\",\"billing_cycle\":\"monthly\",\"price\":\"12.00\",\"currency\":\"USD\",\"start_date\":\"2026-10-01\",\"max_seats\":3}")
SID=$(jget "['id']" "$SUB")
[ -n "$SID" ] || { echo "FAIL subscription" >&2; exit 1; }
echo "ok  subscription $SID"

step "coterie with capacity 2, publish open"
COT=$(api POST /api/v1/coteries "$TOK" "{\"subscription_id\":\"$SID\",\"name\":\"Smoke Circle\",\"capacity\":2}")
CID=$(jget "['id']" "$COT")
expect "$(jget "['status']" "$COT")" draft "coterie starts in draft"
api PATCH "/api/v1/coteries/$CID" "$TOK" '{"status":"open"}' >/dev/null
expect "$(jget "['status']" "$(api GET "/api/v1/coteries/$CID" "$TOK")")" open "published open"

step "invite and join a member"
INV=$(api POST "/api/v1/coteries/$CID/invitations" "$TOK" '{"role":"member"}')
ITOK=$(jget "['token']" "$INV")
REG2=$(api POST /api/v1/auth/register "" "{\"username\":\"$TAG-m\",\"email\":\"$TAG-m@example.com\",\"password\":\"password-123\"}")
TOK2=$(jget "['token']" "$REG2")
MEM=$(api POST /api/v1/invitations/accept "$TOK2" "{\"token\":\"$ITOK\"}")
MID=$(jget "['id']" "$MEM")
expect "$(jget "['member_count']" "$(api GET "/api/v1/coteries/$CID" "$TOK")")" 2 "owner + member"

step "assign a seat to the member"
SEAT=$(api POST "/api/v1/subscriptions/$SID/seats" "$TOK" '{"count":1}')
SEATID=$(python3 -c "import sys,json;print(json.load(sys.stdin)['items'][0]['id'])" <<<"$SEAT")
expect "$(code_of -X POST -H "Authorization: Bearer $TOK" -H 'Content-Type: application/json' -d "{\"member_id\":\"$MID\"}" "$BASE/api/v1/seats/$SEATID/assign")" 200 "seat assigned"

step "billing period + equal split"
PER=$(api POST "/api/v1/subscriptions/$SID/billing-periods" "$TOK" '{"start_date":"2026-10-01","end_date":"2026-11-01"}')
PERID=$(jget "['id']" "$PER")
GEN=$(api POST "/api/v1/billing-periods/$PERID/contributions/generate" "$TOK" '{}')
expect "$(python3 -c "import sys,json;d=json.load(sys.stdin);print(sorted(c['amount'] for c in d['items']))" <<<"$GEN")" "['6.00', '6.00']" "12.00 split over 2 members"

step "manual settlement: mark the member's share paid"
CONTRIB=$(python3 -c "import sys,json;d=json.load(sys.stdin);print([c['id'] for c in d['items'] if c['member_id']=='$MID'][0])" <<<"$GEN")
PAID=$(api PATCH "/api/v1/contributions/$CONTRIB" "$TOK" '{"status":"paid"}')
expect "$(jget "['status']" "$PAID")" paid "contribution settled"

step "member notifications"
NOTIF=$(api GET /api/v1/notifications "$TOK2")
COUNT=$(python3 -c "import sys,json;print(json.load(sys.stdin)['meta']['total'])" <<<"$NOTIF")
[ "$COUNT" -ge 2 ] || { echo "FAIL notifications: expected payment_due + seat_assigned, got $COUNT" >&2; exit 1; }
echo "ok  member inbox has $COUNT notifications"

echo
echo "SMOKE PASS — full MVP journey OK on $BASE"
