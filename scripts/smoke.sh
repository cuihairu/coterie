#!/usr/bin/env bash
# Smoke test: walks the full Phase 1 MVP journey against a running
# Coterie server. Usage: scripts/smoke.sh [base_url]
# Requires curl and python3.
set -euo pipefail

BASE="${1:-http://localhost:8080}"
J="import sys,json;d=json.load(sys.stdin);print(d{})"
step() { printf '\n== %s\n' "$1"; }
expect() { # expect <actual> <want> <label> [raw_body]
  if [ "$1" != "$2" ]; then
    echo "FAIL $3: got '$1', want '$2'" >&2
    [ -n "${4:-}" ] && echo "  body: $4" >&2
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

step "catalog: anonymous browse of the seeded registry"
expect "$(code_of "$BASE/api/v1/providers?category=ai")" 200 "public catalog read"
CAT=$(api GET "/api/v1/providers?category=ai" "")
case "$CAT" in
  *ChatGPT*) ;;
  *) echo "FAIL seeded catalog missing ChatGPT: $CAT" >&2; exit 1 ;;
esac
echo "ok  seeded providers present"

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
expect "$(code_of -X PATCH -H "Authorization: Bearer $TOK" -H 'Content-Type: application/json' -d '{"name":"   "}' "$BASE/api/v1/coteries/$CID")" 422 "blank rename rejected"

step "invite and join a member"
INV=$(api POST "/api/v1/coteries/$CID/invitations" "$TOK" '{"role":"member"}')
ITOK=$(jget "['token']" "$INV")
REG2=$(api POST /api/v1/auth/register "" "{\"username\":\"$TAG-m\",\"email\":\"$TAG-m@example.com\",\"password\":\"password-123\"}")
TOK2=$(jget "['token']" "$REG2")
# A 429 body has no token: the auth rate limiter rejects fast reruns.
[ -n "$TOK2" ] || { echo "FAIL register member: $REG2" >&2; exit 1; }
[ -n "$TOK2" ] || { echo "FAIL register member: $REG2" >&2; exit 1; }
MEM=$(api POST /api/v1/invitations/accept "$TOK2" "{\"token\":\"$ITOK\"}")
MID=$(jget "['id']" "$MEM")
expect "$(jget "['member_count']" "$(api GET "/api/v1/coteries/$CID" "$TOK")")" 2 "owner + member"
INVS=$(api GET "/api/v1/coteries/$CID/invitations" "$TOK")
expect "$(python3 -c "import sys,json;print(len([i for i in json.load(sys.stdin)['items'] if i.get('accepted_at')]))" <<<"$INVS")" 1 "invitation history marks the accepted one"
expect "$(code_of "$BASE/api/v1/coteries/$CID/invitations")" 401 "invitation history rejects anonymous"

step "marketplace listing: public lists, private hides"
api PATCH "/api/v1/coteries/$CID" "$TOK" '{"listing":"public"}' >/dev/null
case "$(api GET /api/v1/marketplace/coteries "")" in
  *"$CID"*) echo "ok  listed circle in the directory" ;;
  *) echo "FAIL listed circle missing from directory" >&2; exit 1 ;;
esac
api PATCH "/api/v1/coteries/$CID" "$TOK" '{"listing":"private"}' >/dev/null
case "$(api GET /api/v1/marketplace/coteries "")" in
  *"$CID"*) echo "FAIL private circle still in directory" >&2; exit 1 ;;
  *) echo "ok  private circle hidden" ;;
esac

step "assign a seat to the member"
SEAT=$(api POST "/api/v1/subscriptions/$SID/seats" "$TOK" '{"count":1}')
SEATID=$(python3 -c "import sys,json;print(json.load(sys.stdin)['items'][0]['id'])" <<<"$SEAT")
expect "$(code_of -X POST -H "Authorization: Bearer $TOK" -H 'Content-Type: application/json' -d "{\"member_id\":\"$MID\"}" "$BASE/api/v1/seats/$SEATID/assign")" 200 "seat assigned"

step "seat management: relabel, disable, release"
REL=$(api PATCH "/api/v1/seats/$SEATID" "$TOK" '{"label":"Living Room"}')
expect "$(jget "['label']" "$REL")" "Living Room" "seat relabeled"
expect "$(code_of -X PATCH -H "Authorization: Bearer $TOK" -H 'Content-Type: application/json' -d '{"label":"   "}' "$BASE/api/v1/seats/$SEATID")" 422 "blank seat label rejected"
expect "$(code_of -X PATCH -H "Authorization: Bearer $TOK" -H 'Content-Type: application/json' -d '{"status":"disabled"}' "$BASE/api/v1/seats/$SEATID")" 409 "occupied seat refuses disable"
REL=$(api POST "/api/v1/seats/$SEATID/release" "$TOK" "")
expect "$(jget "['status']" "$REL")" free "released seat is free"
REL=$(api PATCH "/api/v1/seats/$SEATID" "$TOK" '{"status":"disabled"}')
expect "$(jget "['status']" "$REL")" disabled "free seat disabled"
REL=$(api PATCH "/api/v1/seats/$SEATID" "$TOK" '{"status":"free"}')
expect "$(jget "['status']" "$REL")" free "seat re-enabled"

step "billing period + equal split"
PER=$(api POST "/api/v1/subscriptions/$SID/billing-periods" "$TOK" '{"start_date":"2026-10-01","end_date":"2026-11-01"}')
PERID=$(jget "['id']" "$PER")
GEN=$(api POST "/api/v1/billing-periods/$PERID/contributions/generate" "$TOK" '{}')
expect "$(python3 -c "import sys,json;d=json.load(sys.stdin);print(sorted(c['amount'] for c in d['items']))" <<<"$GEN")" "['6.00', '6.00']" "12.00 split over 2 members"

step "manual settlement: record the member's payment"
CONTRIB=$(python3 -c "import sys,json;d=json.load(sys.stdin);print([c['id'] for c in d['items'] if c['member_id']=='$MID'][0])" <<<"$GEN")
PAID=$(api POST "/api/v1/contributions/$CONTRIB/payments" "$TOK" "{\"external_ref\":\"cash on Friday smoke-$TAG\"}")
expect "$(jget "['status']" "$PAID")" succeeded "payment recorded via manual adapter" "$PAID"
expect "$(jget "['amount']" "$PAID")" 6.00 "payment echoes the contribution amount"
STATUS=$(api GET "/api/v1/billing-periods/$PERID/contributions" "$TOK")
expect "$(python3 -c "import sys,json;d=json.load(sys.stdin);print([c['status'] for c in d['items'] if c['id']=='$CONTRIB'][0])" <<<"$STATUS")" paid "contribution settled"

step "usage metering and usage-based split"
OWNM=$(python3 -c "import sys,json;print([m['id'] for m in json.load(sys.stdin)['items'] if m['role']=='owner'][0])" <<<"$(api GET "/api/v1/coteries/$CID/members" "$TOK")")
api POST "/api/v1/subscriptions/$SID/usage-records" "$TOK" "{\"member_id\":\"$OWNM\",\"amount\":\"10\",\"unit\":\"credits\",\"recorded_at\":\"2026-11-05T12:00:00Z\"}" >/dev/null
api POST "/api/v1/subscriptions/$SID/usage-records" "$TOK" "{\"member_id\":\"$MID\",\"amount\":\"30\",\"unit\":\"credits\",\"recorded_at\":\"2026-11-05T12:00:00Z\"}" >/dev/null
UML=$(api GET "/api/v1/subscriptions/$SID/usage-records" "$TOK")
expect "$(python3 -c "import sys,json;print(sum(float(r['amount']) for r in json.load(sys.stdin)['items']))" <<<"$UML")" 40.0 "usage ledger sums to 40 credits"
PER2=$(api POST "/api/v1/subscriptions/$SID/billing-periods" "$TOK" '{"start_date":"2026-11-01","end_date":"2026-12-01"}')
PER2ID=$(jget "['id']" "$PER2")
GEN2=$(api POST "/api/v1/billing-periods/$PER2ID/contributions/generate" "$TOK" '{"mode":"usage"}')
expect "$(python3 -c "import sys,json;print(sorted(c['amount'] for c in json.load(sys.stdin)['items']))" <<<"$GEN2")" "['3.00', '9.00']" "usage split is proportional"

step "dispute: member raises, owner decides"
MEMCONTRIB=$(python3 -c "import sys,json;print([c['id'] for c in json.load(sys.stdin)['items'] if c['member_id']=='$MID'][0])" <<<"$GEN2")
DISP=$(api POST "/api/v1/contributions/$MEMCONTRIB/disputes" "$TOK2" '{"reason":"I streamed less than that"}')
expect "$(jget "['status']" "$DISP")" open "dispute raised by the member"
DISPID=$(jget "['id']" "$DISP")
expect "$(code_of -X POST -H "Authorization: Bearer $TOK2" -H 'Content-Type: application/json' -d '{"decision":"rejected"}' "$BASE/api/v1/disputes/$DISPID/decide")" 403 "members cannot decide disputes"
DECIDED=$(api POST "/api/v1/disputes/$DISPID/decide" "$TOK" '{"decision":"rejected","note":"ledger matches the provider export"}')
expect "$(jget "['status']" "$DECIDED")" rejected "owner rejected the dispute"
expect "$(jget "['status']" "$(api GET "/api/v1/disputes/$DISPID" "$TOK2")")" rejected "raiser still sees the dispute"

step "member notifications"
NOTIF=$(api GET /api/v1/notifications "$TOK2")
COUNT=$(python3 -c "import sys,json;print(json.load(sys.stdin)['meta']['total'])" <<<"$NOTIF")
[ "$COUNT" -ge 2 ] || { echo "FAIL notifications: expected payment_due + seat_assigned, got $COUNT" >&2; exit 1; }
echo "ok  member inbox has $COUNT notifications"

step "marketplace admission: join request, accept, leave, decline"
api PATCH "/api/v1/coteries/$CID" "$TOK" '{"listing":"public"}' >/dev/null
REG3=$(api POST /api/v1/auth/register "" "{\"username\":\"$TAG-j\",\"email\":\"$TAG-j@example.com\",\"password\":\"password-123\"}")
TOK3=$(jget "['token']" "$REG3")
[ -n "$TOK3" ] || { echo "FAIL register requester: $REG3" >&2; exit 1; }
[ -n "$TOK3" ] || { echo "FAIL register requester: $REG3" >&2; exit 1; }
expect "$(code_of -X POST -H "Authorization: Bearer $TOK2" -H 'Content-Type: application/json' -d '{}' "$BASE/api/v1/coteries/$CID/join-requests")" 409 "existing member cannot re-apply"
JR=$(api POST "/api/v1/coteries/$CID/join-requests" "$TOK3" '{"message":"count me in"}')
JRID=$(jget "['id']" "$JR")
MINE=$(api GET /api/v1/me/join-requests "$TOK3")
expect "$(python3 -c "import sys,json;i=json.load(sys.stdin)['items'][0];print(i['status'], i['coterie_name'])" <<<"$MINE")" "pending Smoke Circle" "my request visible with circle name"
expect "$(code_of -X POST -H "Authorization: Bearer $TOK2" "$BASE/api/v1/join-requests/$JRID/accept")" 403 "non-owner cannot accept"
ACC=$(api POST "/api/v1/join-requests/$JRID/accept" "$TOK" "")
expect "$(jget "['status']" "$ACC")" accepted "owner accepted the request"
expect "$(jget "['member_count']" "$(api GET "/api/v1/coteries/$CID" "$TOK")")" 3 "requester admitted"
api POST "/api/v1/coteries/$CID/leave" "$TOK3" "" >/dev/null
expect "$(jget "['member_count']" "$(api GET "/api/v1/coteries/$CID" "$TOK")")" 2 "leaver gone"
JR2=$(api POST "/api/v1/coteries/$CID/join-requests" "$TOK3" '{"message":"again"}')
JRID2=$(jget "['id']" "$JR2")
DEC=$(api POST "/api/v1/join-requests/$JRID2/decline" "$TOK" "")
expect "$(jget "['status']" "$DEC")" declined "owner declined the re-request"
MINE=$(api GET /api/v1/me/join-requests "$TOK3")
expect "$(python3 -c "import sys,json;print(json.load(sys.stdin)['items'][0]['status'])" <<<"$MINE")" declined "history shows the decline"

step "payment gate: accept holds at awaiting_payment until the charge lands"
api PATCH "/api/v1/coteries/$CID" "$TOK" '{"payment_gate":true}' >/dev/null
JR3=$(api POST "/api/v1/coteries/$CID/join-requests" "$TOK3" '{"message":"gate walk"}')
JRID3=$(jget "['id']" "$JR3")
GACC=$(api POST "/api/v1/join-requests/$JRID3/accept" "$TOK" "")
expect "$(jget "['status']" "$GACC")" awaiting_payment "gated accept holds for payment"
expect "$(code_of -X POST -H "Authorization: Bearer $TOK3" -H 'Content-Type: application/json' -d '{"method":"manual"}' "$BASE/api/v1/join-requests/$JRID3/payments")" 403 "requester cannot self-certify a manual charge"
CHARGE=$(api POST "/api/v1/join-requests/$JRID3/payments" "$TOK" "{\"method\":\"manual\",\"external_ref\":\"admission smoke-$TAG\"}")
expect "$(jget "['amount']" "$CHARGE")" 3.00 "admission amount is the share estimate"
expect "$(jget "['status']" "$CHARGE")" succeeded "manual admission charge settled"
expect "$(python3 -c "import sys,json;print(json.load(sys.stdin)['items'][0]['status'])" <<<"$(api GET /api/v1/me/join-requests "$TOK3")")" accepted "charge admitted the requester"
expect "$(jget "['member_count']" "$(api GET "/api/v1/coteries/$CID" "$TOK")")" 3 "gated member joined"

step "owner blocklist: blocked users cannot re-apply"
OWNERID=$(jget "['user']['id']" "$REG")
USERID3=$(jget "['user']['id']" "$REG3")
api POST "/api/v1/coteries/$CID/leave" "$TOK3" "" >/dev/null
expect "$(jget "['member_count']" "$(api GET "/api/v1/coteries/$CID" "$TOK")")" 2 "gate member left"
expect "$(code_of -X PUT -H "Authorization: Bearer $TOK" "$BASE/api/v1/coteries/$CID/blocks/$OWNERID")" 422 "owner cannot self-block"
expect "$(code_of -X PUT -H "Authorization: Bearer $TOK3" "$BASE/api/v1/coteries/$CID/blocks/$USERID3")" 403 "members cannot block"
BLOCK=$(api PUT "/api/v1/coteries/$CID/blocks/$USERID3" "$TOK" "")
expect "$(jget "['username']" "$BLOCK")" "$TAG-j" "owner blocked the leaver"
expect "$(code_of -X PUT -H "Authorization: Bearer $TOK" "$BASE/api/v1/coteries/$CID/blocks/$USERID3")" 200 "replay block is idempotent"
case "$(api GET "/api/v1/coteries/$CID/blocks" "$TOK")" in
  *"$USERID3"*) echo "ok  blocklist names the user" ;;
  *) echo "FAIL blocklist missing the user" >&2; exit 1 ;;
esac
expect "$(code_of -X POST -H "Authorization: Bearer $TOK3" -H 'Content-Type: application/json' -d '{"message":"let me back"}' "$BASE/api/v1/coteries/$CID/join-requests")" 403 "blocked user cannot re-apply"
api DELETE "/api/v1/coteries/$CID/blocks/$USERID3" "$TOK" "" >/dev/null
JR4=$(api POST "/api/v1/coteries/$CID/join-requests" "$TOK3" '{"message":"round two"}')
expect "$(jget "['status']" "$JR4")" pending "unblocked user can apply again"

step "abuse reports: flag a listed circle"
REP=$(api POST "/api/v1/coteries/$CID/report" "$TOK2" '{"reason":"suspected fake subscription"}')
expect "$(jget "['status']" "$REP")" open "member flagged the listed circle"
RID=$(jget "['id']" "$REP")
expect "$(code_of -X POST -H "Authorization: Bearer $TOK2" -H 'Content-Type: application/json' -d '{"reason":"again"}' "$BASE/api/v1/coteries/$CID/report")" 409 "duplicate open report refused"
expect "$(code_of -X POST -H "Authorization: Bearer $TOK2" -H 'Content-Type: application/json' -d '{"reason":"   "}' "$BASE/api/v1/coteries/$CID/report")" 422 "blank report reason rejected"

# The admin walk needs an account the instance operator promoted via
# ADMIN_EMAILS before this run: pass its credentials to cover it.
if [ -n "${SMOKE_ADMIN_EMAIL:-}" ] && [ -n "${SMOKE_ADMIN_PASSWORD:-}" ]; then
  step "platform admin: reports inbox and audit ledger"
  AL=$(api POST /api/v1/auth/login "" "{\"email\":\"$SMOKE_ADMIN_EMAIL\",\"password\":\"$SMOKE_ADMIN_PASSWORD\"}")
  ATOK=$(jget "['token']" "$AL")
  [ -n "$ATOK" ] || { echo "FAIL admin login: $AL" >&2; exit 1; }
  expect "$(code_of -H "Authorization: Bearer $TOK" "$BASE/api/v1/admin/reports")" 403 "non-admin cannot read the inbox"
  case "$(api GET /api/v1/admin/reports "$ATOK")" in
    *"$CID"*) echo "ok  admin inbox shows the report" ;;
    *) echo "FAIL report missing from the admin inbox" >&2; exit 1 ;;
  esac
  DISM=$(api POST "/api/v1/admin/reports/$RID/dismiss" "$ATOK" '{"note":"smoke walk"}')
  expect "$(jget "['status']" "$DISM")" dismissed "admin dismissed the report"
  case "$(api GET "/api/v1/admin/audit-logs?action=report_decided&limit=20&offset=0" "$ATOK")" in
    *"$RID"*) echo "ok  admin audit ledger records the decision" ;;
    *) echo "FAIL decision missing from the audit ledger" >&2; exit 1 ;;
  esac
else
  echo "note: admin walk not run (set SMOKE_ADMIN_EMAIL and SMOKE_ADMIN_PASSWORD to cover it)"
fi

echo
echo "SMOKE PASS — full MVP journey OK on $BASE"
