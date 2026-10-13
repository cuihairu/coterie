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
PROVGET=$(api GET "/api/v1/providers/$PID" "")
expect "$(jget "['id']" "$PROVGET")" "$PID" "provider detail read back"
PRODGET=$(api GET "/api/v1/products/$PRID" "")
expect "$(jget "['id']" "$PRODGET")" "$PRID" "product detail read back"
# Presence checks pass even when a filter is ignored, so assert the
# narrowing: this video provider is listed unfiltered and hidden by
# category=ai.
case "$(api GET /api/v1/providers "")" in
  *"$PID"*) echo "ok  unfiltered catalog lists the new provider" ;;
  *) echo "FAIL new provider missing from the unfiltered catalog" >&2; exit 1 ;;
esac
expect "$(python3 -c "import sys,json;print(any(p['id']=='$PID' for p in json.load(sys.stdin)['items']))" <<<"$(api GET "/api/v1/providers?category=ai" "")")" False "category filter hides other categories"

step "subscription with seat capacity"
SUB=$(api POST /api/v1/subscriptions "$TOK" "{\"product_id\":\"$PRID\",\"owner_user_id\":\"$(jget "['user']['id']" "$REG")\",\"billing_cycle\":\"monthly\",\"price\":\"12.00\",\"currency\":\"USD\",\"start_date\":\"2026-10-01\",\"max_seats\":3}")
SID=$(jget "['id']" "$SUB")
[ -n "$SID" ] || { echo "FAIL subscription" >&2; exit 1; }
echo "ok  subscription $SID"
expect "$(code_of -H "Authorization: Bearer $TOK" "$BASE/api/v1/subscriptions?owner_user_id=00000000-0000-0000-0000-000000000000")" 403 "foreign owner filter refused"
expect "$(python3 -c "import sys,json;print(any(s['id']=='$SID' for s in json.load(sys.stdin)['items']))" <<<"$(api GET /api/v1/subscriptions "$TOK")")" True "owner lists the subscription"

step "coterie with capacity 2, publish open"
COT=$(api POST /api/v1/coteries "$TOK" "{\"subscription_id\":\"$SID\",\"name\":\"Smoke Circle\",\"capacity\":2}")
CID=$(jget "['id']" "$COT")
expect "$(jget "['status']" "$COT")" draft "coterie starts in draft"
api PATCH "/api/v1/coteries/$CID" "$TOK" '{"status":"open"}' >/dev/null
expect "$(jget "['status']" "$(api GET "/api/v1/coteries/$CID" "$TOK")")" open "published open"
expect "$(code_of -X PATCH -H "Authorization: Bearer $TOK" -H 'Content-Type: application/json' -d '{"name":"   "}' "$BASE/api/v1/coteries/$CID")" 422 "blank rename rejected"
expect "$(python3 -c "import sys,json;print(any(c['id']=='$CID' for c in json.load(sys.stdin)['items']))" <<<"$(api GET /api/v1/coteries "$TOK")")" True "owner lists the coterie"

step "invite and join a member"
INV=$(api POST "/api/v1/coteries/$CID/invitations" "$TOK" '{"role":"member"}')
ITOK=$(jget "['token']" "$INV")
REG2=$(api POST /api/v1/auth/register "" "{\"username\":\"$TAG-m\",\"email\":\"$TAG-m@example.com\",\"password\":\"password-123\"}")
TOK2=$(jget "['token']" "$REG2")
# A 429 body has no token: the auth rate limiter rejects fast reruns.
[ -n "$TOK2" ] || { echo "FAIL register member: $REG2" >&2; exit 1; }
MEM=$(api POST /api/v1/invitations/accept "$TOK2" "{\"token\":\"$ITOK\"}")
MID=$(jget "['id']" "$MEM")
expect "$(jget "['member_count']" "$(api GET "/api/v1/coteries/$CID" "$TOK")")" 2 "owner + member"
INVS=$(api GET "/api/v1/coteries/$CID/invitations" "$TOK")
expect "$(python3 -c "import sys,json;print(len([i for i in json.load(sys.stdin)['items'] if i.get('accepted_at')]))" <<<"$INVS")" 1 "invitation history marks the accepted one"
expect "$(code_of "$BASE/api/v1/coteries/$CID/invitations")" 401 "invitation history rejects anonymous"

step "marketplace listing: public lists, private hides"
api PATCH "/api/v1/coteries/$CID" "$TOK" '{"listing":"public"}' >/dev/null
# The directory sorts oldest-first; repeated runs push new circles off
# page 1. Filter to this run's product for a deterministic read, and
# check the total so an ignored filter cannot pass as "empty".
DIR=$(api GET "/api/v1/marketplace/coteries?product_id=$PRID" "")
case "$DIR" in
  *"$CID"*) echo "ok  listed circle in the directory" ;;
  *) echo "FAIL listed circle missing from directory" >&2; exit 1 ;;
esac
expect "$(python3 -c "import sys,json;print(json.load(sys.stdin)['meta']['total'])" <<<"$DIR")" 1 "directory filter narrows to this run"
api PATCH "/api/v1/coteries/$CID" "$TOK" '{"listing":"private"}' >/dev/null
DIR=$(api GET "/api/v1/marketplace/coteries?product_id=$PRID" "")
case "$DIR" in
  *"$CID"*) echo "FAIL private circle still in directory" >&2; exit 1 ;;
  *) echo "ok  private circle hidden" ;;
esac
expect "$(python3 -c "import sys,json;print(json.load(sys.stdin)['meta']['total'])" <<<"$DIR")" 0 "hidden circle drops the total"

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
expect "$(jget "['id']" "$(api GET "/api/v1/billing-periods/$PERID" "$TOK")")" "$PERID" "billing period detail read back"
GEN=$(api POST "/api/v1/billing-periods/$PERID/contributions/generate" "$TOK" '{}')
expect "$(python3 -c "import sys,json;d=json.load(sys.stdin);print(sorted(c['amount'] for c in d['items']))" <<<"$GEN")" "['6.00', '6.00']" "12.00 split over 2 members"

step "manual settlement: record the member's payment"
CONTRIB=$(python3 -c "import sys,json;d=json.load(sys.stdin);print([c['id'] for c in d['items'] if c['member_id']=='$MID'][0])" <<<"$GEN")
PAID=$(api POST "/api/v1/contributions/$CONTRIB/payments" "$TOK" "{\"external_ref\":\"cash on Friday smoke-$TAG\"}")
expect "$(jget "['status']" "$PAID")" succeeded "payment recorded via manual adapter" "$PAID"
expect "$(jget "['amount']" "$PAID")" 6.00 "payment echoes the contribution amount"
STATUS=$(api GET "/api/v1/billing-periods/$PERID/contributions" "$TOK")
expect "$(python3 -c "import sys,json;d=json.load(sys.stdin);print([c['status'] for c in d['items'] if c['id']=='$CONTRIB'][0])" <<<"$STATUS")" paid "contribution settled"
PAYID=$(jget "['id']" "$PAID")
expect "$(jget "['id']" "$(api GET "/api/v1/payments/$PAYID" "$TOK")")" "$PAYID" "payment detail read back"
case "$(api GET /api/v1/payments/methods "$TOK")" in
  *manual*) echo "ok  payment methods list manual" ;;
  *) echo "FAIL manual missing from payment methods" >&2; exit 1 ;;
esac

step "public webhook gate: signature is the only way in"
expect "$(code_of "$BASE/healthz")" 200 "healthz is public"
expect "$(code_of -X POST -H 'Content-Type: application/json' -d '{}' "$BASE/api/v1/payments/webhooks/nosuch")" 404 "unknown payment channel refused"
expect "$(code_of -X POST -H 'Content-Type: application/json' -d '{}' "$BASE/api/v1/payments/webhooks/manual")" 422 "sync-only channel cannot confirm"
expect "$(code_of -X POST -H 'Content-Type: application/json' -d '{}' "$BASE/api/v1/marketplace/webhooks/nosuch")" 404 "unknown gate channel refused"
expect "$(code_of -X POST -H 'Content-Type: application/json' -d '{}' "$BASE/api/v1/marketplace/webhooks/manual")" 422 "sync-only gate channel cannot confirm"

step "single-port web app: one server for API and client"
# The Docker build embeds the real Vite output; a local build serves
# the committed placeholder. Both are HTML, and both fall back to
# index.html for client-side routes.
expect "$(code_of "$BASE/")" 200 "root serves the embedded client"
expect "$(curl -s -o /dev/null -w '%{content_type}' "$BASE/")" "text/html; charset=utf-8" "root is html"
expect "$(code_of "$BASE/circles/deep/link")" 200 "spa fallback resolves client routes"
expect "$(code_of "$BASE/api/v1/nope")" 404 "api paths stay on the api mux"

step "usage metering and usage-based split"
OWNM=$(python3 -c "import sys,json;print([m['id'] for m in json.load(sys.stdin)['items'] if m['role']=='owner'][0])" <<<"$(api GET "/api/v1/coteries/$CID/members" "$TOK")")
api POST "/api/v1/subscriptions/$SID/usage-records" "$TOK" "{\"member_id\":\"$OWNM\",\"amount\":\"10\",\"unit\":\"credits\",\"recorded_at\":\"2026-11-05T12:00:00Z\"}" >/dev/null
api POST "/api/v1/subscriptions/$SID/usage-records" "$TOK" "{\"member_id\":\"$MID\",\"amount\":\"30\",\"unit\":\"credits\",\"recorded_at\":\"2026-11-05T12:00:00Z\"}" >/dev/null
UML=$(api GET "/api/v1/subscriptions/$SID/usage-records" "$TOK")
expect "$(python3 -c "import sys,json;print(sum(float(r['amount']) for r in json.load(sys.stdin)['items']))" <<<"$UML")" 40.0 "usage ledger sums to 40 credits"
expect "$(python3 -c "import sys,json;i=json.load(sys.stdin)['items'];print(len(i)==1 and i[0]['member_id']=='$MID')" <<<"$(api GET "/api/v1/subscriptions/$SID/usage-records?member_id=$MID" "$TOK")")" True "member filter narrows the ledger"
ROW=$(python3 -c "import sys,json;i=json.load(sys.stdin)['items'][0];print(i['id'],i['member_id'])" <<<"$UML")
URID=${ROW% *}; URMEMBER=${ROW#* }
expect "$(jget "['member_id']" "$(api GET "/api/v1/usage-records/$URID" "$TOK")")" "$URMEMBER" "usage record detail read back"
PER2=$(api POST "/api/v1/subscriptions/$SID/billing-periods" "$TOK" '{"start_date":"2026-11-01","end_date":"2026-12-01"}')
PER2ID=$(jget "['id']" "$PER2")
GEN2=$(api POST "/api/v1/billing-periods/$PER2ID/contributions/generate" "$TOK" '{"mode":"usage"}')
expect "$(python3 -c "import sys,json;print(sorted(c['amount'] for c in json.load(sys.stdin)['items']))" <<<"$GEN2")" "['3.00', '9.00']" "usage split is proportional"
# When the server registers the sandbox demo channel, prove the
# sync-adapter seam: the charge settles instantly, like manual.
case "$(api GET /api/v1/payments/methods "$TOK")" in
  *sandbox*)
    SBXOWN=$(python3 -c "import sys,json;print([c['id'] for c in json.load(sys.stdin)['items'] if c['member_id']=='$OWNM'][0])" <<<"$GEN2")
    SBX=$(api POST "/api/v1/contributions/$SBXOWN/payments" "$TOK" "{\"method\":\"sandbox\",\"external_ref\":\"sandbox smoke-$TAG\"}")
    expect "$(jget "['status']" "$SBX")" succeeded "sandbox channel settles instantly"
    expect "$(jget "['method']" "$SBX")" sandbox "payment echoes the channel"
    ;;
esac

step "quota seat and the per-period usage limit (D21)"
SP=$(api PATCH "/api/v1/subscriptions/$SID" "$TOK" '{"sharing_policy":{"usage_limit":{"unit":"credits","per_period":"50"}}}')
expect "$(jget "['sharing_policy']['usage_limit']['per_period']" "$SP")" 50 "sharing policy carries the limit"
QS=$(api PATCH "/api/v1/seats/$SEATID" "$TOK" '{"metadata":{"quota":100,"unit":"credits"}}')
expect "$(jget "['metadata']['unit']" "$QS")" credits "seat marked as a quota seat"
api POST "/api/v1/seats/$SEATID/assign" "$TOK" "{\"member_id\":\"$MID\"}" >/dev/null
api POST "/api/v1/subscriptions/$SID/usage-records" "$TOK" "{\"member_id\":\"$MID\",\"amount\":\"15\",\"unit\":\"credits\",\"recorded_at\":\"2026-11-06T12:00:00Z\"}" >/dev/null
expect "$(jget "['metadata']['used']" "$(api GET "/api/v1/seats/$SEATID" "$TOK")")" 15 "seat usage projected from the ledger"
expect "$(code_of -X POST -H "Authorization: Bearer $TOK" -H 'Content-Type: application/json' -d "{\"member_id\":\"$MID\",\"amount\":\"10\",\"unit\":\"credits\",\"recorded_at\":\"2026-11-06T12:00:00Z\"}" "$BASE/api/v1/subscriptions/$SID/usage-records")" 409 "record past the period limit refused"
api POST "/api/v1/subscriptions/$SID/usage-records" "$TOK" "{\"member_id\":\"$MID\",\"amount\":\"-10\",\"unit\":\"credits\",\"recorded_at\":\"2026-11-06T12:00:00Z\"}" >/dev/null
expect "$(jget "['metadata']['used']" "$(api GET "/api/v1/seats/$SEATID" "$TOK")")" 5 "correction lowers the projected usage"

step "fixed split: owner assigns exact shares"
PER3=$(api POST "/api/v1/subscriptions/$SID/billing-periods" "$TOK" '{"start_date":"2026-12-01","end_date":"2027-01-01"}')
PER3ID=$(jget "['id']" "$PER3")
expect "$(code_of -X POST -H "Authorization: Bearer $TOK" -H 'Content-Type: application/json' -d '{"mode":"fixed"}' "$BASE/api/v1/billing-periods/$PER3ID/contributions/generate")" 422 "fixed mode without items refused"
FIX=$(api POST "/api/v1/billing-periods/$PER3ID/contributions/generate" "$TOK" "{\"mode\":\"fixed\",\"items\":[{\"member_id\":\"$OWNM\",\"amount\":\"4.00\"},{\"member_id\":\"$MID\",\"amount\":\"8.00\"}]}")
expect "$(python3 -c "import sys,json;print(sorted(c['amount'] for c in json.load(sys.stdin)['items']))" <<<"$FIX")" "['4.00', '8.00']" "fixed shares land as assigned"
FIXOWN=$(python3 -c "import sys,json;print([c['id'] for c in json.load(sys.stdin)['items'] if c['member_id']=='$OWNM'][0])" <<<"$FIX")
ADJ=$(api PATCH "/api/v1/contributions/$FIXOWN" "$TOK" '{"amount":"5.00"}')
expect "$(jget "['amount']" "$ADJ")" 5.00 "owner adjusted a pending share"
expect "$(code_of -X PATCH -H "Authorization: Bearer $TOK2" -H 'Content-Type: application/json' -d '{"amount":"9.99"}' "$BASE/api/v1/contributions/$FIXOWN")" 403 "members cannot adjust contributions"
expect "$(jget "['status']" "$(api PATCH "/api/v1/contributions/$FIXOWN" "$TOK" '{"status":"waived"}')")" waived "owner waived the share"
expect "$(code_of -X PATCH -H "Authorization: Bearer $TOK" -H 'Content-Type: application/json' -d '{"amount":"6.00"}' "$BASE/api/v1/contributions/$FIXOWN")" 409 "amount frozen once waived"
expect "$(jget "['status']" "$(api POST "/api/v1/billing-periods/$PER3ID/close" "$TOK" "")")" closed "owner closed the period"
expect "$(code_of -X POST -H "Authorization: Bearer $TOK" "$BASE/api/v1/billing-periods/$PER3ID/close")" 409 "closing a closed period refused"
expect "$(code_of -X POST -H "Authorization: Bearer $TOK2" "$BASE/api/v1/billing-periods/$PER3ID/close")" 403 "members cannot close periods"

step "dispute: member raises, owner decides"
MEMCONTRIB=$(python3 -c "import sys,json;print([c['id'] for c in json.load(sys.stdin)['items'] if c['member_id']=='$MID'][0])" <<<"$GEN2")
DISP=$(api POST "/api/v1/contributions/$MEMCONTRIB/disputes" "$TOK2" '{"reason":"I streamed less than that"}')
expect "$(jget "['status']" "$DISP")" open "dispute raised by the member"
DISPID=$(jget "['id']" "$DISP")
expect "$(code_of -X POST -H "Authorization: Bearer $TOK2" -H 'Content-Type: application/json' -d '{"decision":"rejected"}' "$BASE/api/v1/disputes/$DISPID/decide")" 403 "members cannot decide disputes"
DECIDED=$(api POST "/api/v1/disputes/$DISPID/decide" "$TOK" '{"decision":"rejected","note":"ledger matches the provider export"}')
expect "$(jget "['status']" "$DECIDED")" rejected "owner rejected the dispute"
expect "$(jget "['status']" "$(api GET "/api/v1/disputes/$DISPID" "$TOK2")")" rejected "raiser still sees the dispute"
expect "$(python3 -c "import sys,json;d=json.load(sys.stdin)['items'];print([x['status'] for x in d if x['id']=='$DISPID'][0])" <<<"$(api GET "/api/v1/subscriptions/$SID/disputes" "$TOK")")" rejected "owner dispute list carries the decision"

step "member notifications"
NOTIF=$(api GET /api/v1/notifications "$TOK2")
COUNT=$(python3 -c "import sys,json;print(json.load(sys.stdin)['meta']['total'])" <<<"$NOTIF")
[ "$COUNT" -ge 2 ] || { echo "FAIL notifications: expected payment_due + seat_assigned, got $COUNT" >&2; exit 1; }
echo "ok  member inbox has $COUNT notifications"
NID=$(python3 -c "import sys,json;print(json.load(sys.stdin)['items'][0]['id'])" <<<"$NOTIF")
RD=$(api POST "/api/v1/notifications/$NID/read" "$TOK2" "")
[ -n "$(jget "['read_at']" "$RD")" ] && echo "ok  marking read stamps read_at"
expect "$(python3 -c "import sys,json;print(all(n['id']!='$NID' for n in json.load(sys.stdin)['items']))" <<<"$(api GET "/api/v1/notifications?unread=true" "$TOK2")")" True "unread filter hides the read one"

step "push subscription registry"
case "$(code_of -H "Authorization: Bearer $TOK" "$BASE/api/v1/push/vapid-public-key")" in
  404) echo "ok  vapid key absent (push not configured)" ;;
  200) echo "ok  vapid key served (push configured)" ;;
  *) echo "FAIL vapid-public-key: unexpected status" >&2; exit 1 ;;
esac
P256DH=BAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
PAUTH=AAECAwQFBgcICQoLDA0ODw
expect "$(code_of -X POST -H "Authorization: Bearer $TOK2" -H 'Content-Type: application/json' -d "{\"endpoint\":\"http://push.example.com/x\",\"keys\":{\"p256dh\":\"$P256DH\",\"auth\":\"$PAUTH\"}}" "$BASE/api/v1/push/subscriptions")" 422 "push endpoint must be https"
PSUB=$(api POST /api/v1/push/subscriptions "$TOK2" "{\"endpoint\":\"https://push.example.com/send/smoke-$TAG\",\"keys\":{\"p256dh\":\"$P256DH\",\"auth\":\"$PAUTH\"}}")
PSID=$(jget "['id']" "$PSUB")
expect "$(jget "['endpoint']" "$PSUB")" "https://push.example.com/send/smoke-$TAG" "push subscription registered"
expect "$(python3 -c "import sys,json;print(any(s['id']=='$PSID' for s in json.load(sys.stdin)['items']))" <<<"$(api GET /api/v1/push/subscriptions "$TOK2")")" True "push list shows the registration"
expect "$(code_of -X DELETE -H "Authorization: Bearer $TOK" "$BASE/api/v1/push/subscriptions/$PSID")" 404 "another user cannot delete foreign subscriptions"
expect "$(code_of -X DELETE -H "Authorization: Bearer $TOK2" "$BASE/api/v1/push/subscriptions/$PSID")" 204 "owner removed the subscription"

step "marketplace admission: join request, accept, leave, decline"
api PATCH "/api/v1/coteries/$CID" "$TOK" '{"listing":"public"}' >/dev/null
REG3=$(api POST /api/v1/auth/register "" "{\"username\":\"$TAG-j\",\"email\":\"$TAG-j@example.com\",\"password\":\"password-123\"}")
TOK3=$(jget "['token']" "$REG3")
[ -n "$TOK3" ] || { echo "FAIL register requester: $REG3" >&2; exit 1; }
expect "$(code_of -X POST -H "Authorization: Bearer $TOK3" "$BASE/api/v1/notifications/$NID/read")" 404 "another user cannot read foreign notifications"
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
expect "$(code_of -X DELETE -H "Authorization: Bearer $TOK" "$BASE/api/v1/join-requests/$(jget "['id']" "$JR4")")" 403 "owner cannot withdraw another's request"
api DELETE "/api/v1/join-requests/$(jget "['id']" "$JR4")" "$TOK3" "" >/dev/null
expect "$(python3 -c "import sys,json;print(json.load(sys.stdin)['items'][0]['status'])" <<<"$(api GET /api/v1/me/join-requests "$TOK3")")" cancelled "requester withdrew the request"

step "owner removes the member: seat freed, audit stamped"
expect "$(code_of -X DELETE -H "Authorization: Bearer $TOK2" "$BASE/api/v1/members/$MID")" 403 "members cannot remove each other"
api DELETE "/api/v1/members/$MID" "$TOK" "" >/dev/null
expect "$(jget "['member_count']" "$(api GET "/api/v1/coteries/$CID" "$TOK")")" 1 "owner removed the member"
expect "$(code_of -X DELETE -H "Authorization: Bearer $TOK" "$BASE/api/v1/members/$MID")" 409 "already-removed member cannot be removed again"

step "owner audit trail"
case "$(api GET "/api/v1/coteries/$CID/audit-logs" "$TOK")" in
  *member_left*) echo "ok  audit trail records the leave" ;;
  *) echo "FAIL member_left missing from the coterie audit trail" >&2; exit 1 ;;
esac
case "$(api GET "/api/v1/coteries/$CID/audit-logs" "$TOK")" in
  *member_removed*) echo "ok  audit trail records the removal" ;;
  *) echo "FAIL member_removed missing from the coterie audit trail" >&2; exit 1 ;;
esac
case "$(api GET "/api/v1/subscriptions/$SID/audit-logs" "$TOK")" in
  *seat_assigned*) echo "ok  subscription audit trail carries the seat actions" ;;
  *) echo "FAIL seat_assigned missing from the subscription audit trail" >&2; exit 1 ;;
esac
expect "$(python3 -c "import sys,json;d=json.load(sys.stdin)['items'];print(bool(d) and all(i['action']=='seat_assigned' for i in d))" <<<"$(api GET "/api/v1/subscriptions/$SID/audit-logs?action=seat_assigned" "$TOK")")" True "action filter narrows the subscription trail"
expect "$(code_of -H "Authorization: Bearer $TOK2" "$BASE/api/v1/coteries/$CID/audit-logs")" 403 "members cannot read the audit trail"

step "logout invalidates the session"
expect "$(code_of -X POST -H "Authorization: Bearer $TOK3" "$BASE/api/v1/auth/logout")" 204 "logout accepted"
expect "$(code_of -H "Authorization: Bearer $TOK3" "$BASE/api/v1/auth/me")" 401 "logged-out token rejected"

step "user directory"
DIRUSER=$(api POST /api/v1/users "$TOK" "{\"username\":\"$TAG-dir\",\"email\":\"$TAG-dir@example.com\",\"password\":\"password-123\"}")
DIRID=$(jget "['id']" "$DIRUSER")
expect "$(jget "['username']" "$DIRUSER")" "$TAG-dir" "owner provisioned a user without a session"
DIRLIST=$(api GET /api/v1/users "$TOK")
expect "$(python3 -c "import sys,json;print(any(u['id']=='$DIRID' for u in json.load(sys.stdin)['items']))" <<<"$DIRLIST")" True "directory lists the new user"
DIRBODY=$(api GET "/api/v1/users/$DIRID" "$TOK")
expect "$(jget "['id']" "$DIRBODY")" "$DIRID" "directory entry reads back"
# D29: the directory is a display projection only — contact and
# authorization fields must not cross to another session.
case "$DIRBODY" in
  *'"email"'*|*'"role"'*|*'"locale"'*) echo "FAIL directory leaks contact fields" >&2; exit 1 ;;
  *) echo "ok  directory serves the summary projection only" ;;
esac
case "$DIRLIST" in
  *'"email"'*|*'"role"'*) echo "FAIL directory list leaks contact fields" >&2; exit 1 ;;
  *) echo "ok  directory list serves the projection too" ;;
esac

step "abuse reports: flag a listed circle"
REP=$(api POST "/api/v1/coteries/$CID/report" "$TOK2" '{"reason":"suspected fake subscription"}')
expect "$(jget "['status']" "$REP")" open "member flagged the listed circle"
RID=$(jget "['id']" "$REP")
expect "$(code_of -X POST -H "Authorization: Bearer $TOK2" -H 'Content-Type: application/json' -d '{"reason":"again"}' "$BASE/api/v1/coteries/$CID/report")" 409 "duplicate open report refused"
expect "$(code_of -X POST -H "Authorization: Bearer $TOK2" -H 'Content-Type: application/json' -d '{"reason":"   "}' "$BASE/api/v1/coteries/$CID/report")" 422 "blank report reason rejected"

step "settlement reputation"
USERID2=$(jget "['user']['id']" "$REG2")
REP=$(api GET "/api/v1/users/$USERID2/reputation" "$TOK")
expect "$(python3 -c "import sys,json;d=json.load(sys.stdin)['contributions'];print(d['paid'],d['pending'])" <<<"$REP")" "1 2" "reputation counts settled and pending shares"
expect "$(code_of -H "Authorization: Bearer $TOK" "$BASE/api/v1/users/00000000-0000-0000-0000-000000000000/reputation")" 404 "unknown user has no reputation"

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
  REP2=$(api POST "/api/v1/coteries/$CID/report" "$TOK" '{"reason":"smoke resolve walk"}')
  RID2=$(jget "['id']" "$REP2")
  expect "$(jget "['status']" "$(api POST "/api/v1/admin/reports/$RID2/resolve" "$ATOK" '{"note":"handled in smoke"}')")" resolved "admin resolved the second report"
  case "$(api GET "/api/v1/admin/audit-logs?action=report_decided&limit=20&offset=0" "$ATOK")" in
    *"$RID"*) echo "ok  admin audit ledger records the decision" ;;
    *) echo "FAIL decision missing from the audit ledger" >&2; exit 1 ;;
  esac
else
  echo "note: admin walk not run (set SMOKE_ADMIN_EMAIL and SMOKE_ADMIN_PASSWORD to cover it)"
fi

echo
echo "SMOKE PASS — full MVP journey OK on $BASE"
