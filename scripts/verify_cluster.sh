#!/usr/bin/env bash
set -euo pipefail

BASE_DN="${BASE_DN:-dc=example,dc=com}"
COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.cluster.yml}"
NODES=(8081 8082 8083)

log() {
  printf '[verify-cluster] %s\n' "$*"
}

fail() {
  printf '[verify-cluster] ERROR: %s\n' "$*" >&2
  exit 1
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "required command not found: $1"
}

urlenc() {
  jq -rn --arg v "$1" '$v|@uri'
}

service_for_port() {
  case "$1" in
    8081) echo "oba-node1" ;;
    8082) echo "oba-node2" ;;
    8083) echo "oba-node3" ;;
    *) fail "unknown node port: $1" ;;
  esac
}

api() {
  local method="$1"
  local path="$2"
  local body="${3:-}"
  local port="${4:-8081}"
  TOKEN="$TOKEN" scripts/cluster_api.sh "$method" "$path" "$body" "$port"
}

api_write_on_current_leader() {
  local method="$1"
  local path="$2"
  local body="$3"
  local retries="${4:-20}"
  local i
  local lp
  local payload
  local err_code

  for i in $(seq 1 "$retries"); do
    lp="$(leader_port || true)"
    if [[ -z "$lp" ]]; then
      sleep 1
      continue
    fi

    TOKEN="$(scripts/cluster_api.sh auth "$lp")"
    payload="$(api "$method" "$path" "$body" "$lp")"
    err_code="$(jq -r '.error // empty' <<<"$payload" 2>/dev/null || true)"

    if [[ "$err_code" == "not_leader" || "$err_code" == "no_leader" ]]; then
      sleep 1
      continue
    fi

    printf '%s' "$payload"
    return 0
  done

  fail "failed to execute write on current leader after ${retries} retries: ${method} ${path}"
}

assert_no_error() {
  local payload="$1"
  local context="$2"
  local code
  local msg
  code="$(jq -r '.error // empty' <<<"$payload" 2>/dev/null || true)"
  msg="$(jq -r '.message // empty' <<<"$payload" 2>/dev/null || true)"
  if [[ -n "$code" ]]; then
    fail "$context failed: $code ${msg}"
  fi
}

extract_total_count() {
  local payload="$1"
  jq -r '(.totalCount // .total_count // 0)' <<<"$payload"
}

wait_node() {
  local port="$1"
  local retries="${2:-40}"
  local i
  for i in $(seq 1 "$retries"); do
    if curl -sS --max-time 3 "http://localhost:${port}/api/v1/health" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  return 1
}

leader_port() {
  local p
  local status
  local state
  for p in "${NODES[@]}"; do
    status="$(scripts/cluster_api.sh GET /api/v1/cluster/status "" "$p" 2>/dev/null || true)"
    state="$(jq -r '.state // empty' <<<"$status" 2>/dev/null || true)"
    if [[ "$state" == "leader" ]]; then
      echo "$p"
      return 0
    fi
  done
  return 1
}

check_raft_lag() {
  local p
  local status
  local commit
  local applied
  local lag
  for p in "${NODES[@]}"; do
    status="$(scripts/cluster_api.sh GET /api/v1/cluster/status "" "$p")"
    commit="$(jq -r '.commitIndex // 0' <<<"$status")"
    applied="$(jq -r '.lastApplied // 0' <<<"$status")"
    lag=$((commit - applied))
    log "node ${p} raft lag: commit=${commit} applied=${applied} lag=${lag}"
    if (( lag < 0 || lag > 2 )); then
      fail "node ${p} raft lag too high: ${lag}"
    fi
  done
}

snapshot_ldap() {
  local port="$1"
  local out="$2"
  local path
  local payload
  path="/api/v1/search?baseDN=$(urlenc "$BASE_DN")&scope=sub&filter=$(urlenc "(objectClass=*)")&limit=10000"
  payload="$(api GET "$path" "" "$port")"
  assert_no_error "$payload" "ldap snapshot node ${port}"
  jq -r '.entries[]? | .dn' <<<"$payload" | sort >"$out"
}

snapshot_uid_index() {
  local port="$1"
  local out="$2"
  local path
  local payload
  path="/api/v1/search?baseDN=$(urlenc "ou=users,$BASE_DN")&scope=sub&filter=$(urlenc "(uid=*)")&limit=10000"
  payload="$(api GET "$path" "" "$port")"
  assert_no_error "$payload" "uid snapshot node ${port}"
  jq -r '.entries[]? | [(.dn // ""), ((.attributes.uid[0] // "") | ascii_downcase)] | @tsv' <<<"$payload" | sort >"$out"
}

snapshot_logs_marker() {
  local port="$1"
  local marker="$2"
  local out="$3"
  local path
  local payload
  path="/api/v1/logs?search=$(urlenc "$marker")&limit=5000"
  payload="$(api GET "$path" "" "$port")"
  assert_no_error "$payload" "log snapshot node ${port}"
  jq -r '.entries[]? | [(.level // ""), (.source // ""), (.message // ""), (.fields.dn // ""), (.fields.path // ""), ((.fields.status // "")|tostring)] | @tsv' <<<"$payload" | sort >"$out"
}

compare_marker_logs() {
  local marker="$1"
  local prefix="$2"
  local retries="${3:-30}"
  local i

  for i in $(seq 1 "$retries"); do
    snapshot_logs_marker 8081 "$marker" "/tmp/${prefix}_8081.txt"
    snapshot_logs_marker 8082 "$marker" "/tmp/${prefix}_8082.txt"
    snapshot_logs_marker 8083 "$marker" "/tmp/${prefix}_8083.txt"

    if [[ -s "/tmp/${prefix}_8081.txt" ]] &&
      cmp -s "/tmp/${prefix}_8081.txt" "/tmp/${prefix}_8082.txt" &&
      cmp -s "/tmp/${prefix}_8081.txt" "/tmp/${prefix}_8083.txt"; then
      return 0
    fi

    sleep 2
  done

  fail "marker log snapshot mismatch after ${retries} retries"
}

compare_all_nodes() {
  local label="$1"
  local prefix="$2"
  local snapshot_fn="$3"
  local retries="${4:-30}"
  local base="/tmp/${prefix}_8081.txt"
  local i

  for i in $(seq 1 "$retries"); do
    "$snapshot_fn" 8081 "$base"
    "$snapshot_fn" 8082 "/tmp/${prefix}_8082.txt"
    "$snapshot_fn" 8083 "/tmp/${prefix}_8083.txt"

    if cmp -s "$base" "/tmp/${prefix}_8082.txt" && cmp -s "$base" "/tmp/${prefix}_8083.txt"; then
      return 0
    fi

    sleep 2
  done

  fail "${label} mismatch after ${retries} retries"
}

require_cmd jq
require_cmd docker
require_cmd scripts/cluster_api.sh

for port in "${NODES[@]}"; do
  wait_node "$port" 50 || fail "node ${port} is not healthy"
done

LEADER_PORT="$(leader_port || true)"
[[ -n "$LEADER_PORT" ]] || fail "leader not found"
TOKEN="$(scripts/cluster_api.sh auth "$LEADER_PORT")"
[[ -n "$TOKEN" ]] || fail "auth token empty"
log "leader detected on port ${LEADER_PORT}"

check_raft_lag

TS="$(date +%s)"
MARKER="e2e-${TS}"
GROUP1="grp-${MARKER}-a"
GROUP2="grp-${MARKER}-b"
USER1="usr-${MARKER}-a"
USER2="usr-${MARKER}-b"
USER_DEL="usr-${MARKER}-del"
USER_DOWNUP="usr-${MARKER}-downup"

GROUP1_DN="cn=${GROUP1},ou=groups,${BASE_DN}"
GROUP2_DN="cn=${GROUP2},ou=groups,${BASE_DN}"
USER1_DN="uid=${USER1},ou=users,${BASE_DN}"
USER2_DN="uid=${USER2},ou=users,${BASE_DN}"
USER_DEL_DN="uid=${USER_DEL},ou=users,${BASE_DN}"
USER_DOWNUP_DN="uid=${USER_DOWNUP},ou=users,${BASE_DN}"
ADMIN_DN="cn=admin,${BASE_DN}"

log "creating groups and users"
resp="$(api POST /api/v1/entries "$(jq -nc --arg dn "$GROUP1_DN" --arg cn "$GROUP1" --arg admin "$ADMIN_DN" '{dn:$dn,attributes:{objectClass:["groupOfNames","top"],cn:[$cn],member:[$admin],description:["cluster verification group"]}}')" "$LEADER_PORT")"
assert_no_error "$resp" "add group1"

resp="$(api POST /api/v1/entries "$(jq -nc --arg dn "$GROUP2_DN" --arg cn "$GROUP2" --arg admin "$ADMIN_DN" '{dn:$dn,attributes:{objectClass:["groupOfNames","top"],cn:[$cn],member:[$admin],description:["cluster verification group"]}}')" "$LEADER_PORT")"
assert_no_error "$resp" "add group2"

resp="$(api POST /api/v1/entries "$(jq -nc --arg dn "$USER1_DN" --arg uid "$USER1" --arg marker "$MARKER" '{dn:$dn,attributes:{objectClass:["inetOrgPerson","organizationalPerson","person","top"],uid:[$uid],givenName:["Node"],sn:["One"],cn:["Node One"],mail:[($uid+"@example.com")],description:["verify "+$marker],userPassword:["admin"]}}')" "$LEADER_PORT")"
assert_no_error "$resp" "add user1"

resp="$(api POST /api/v1/entries "$(jq -nc --arg dn "$USER2_DN" --arg uid "$USER2" --arg marker "$MARKER" '{dn:$dn,attributes:{objectClass:["inetOrgPerson","organizationalPerson","person","top"],uid:[$uid],givenName:["Node"],sn:["Two"],cn:["Node Two"],mail:[($uid+"@example.com")],description:["verify "+$marker],userPassword:["admin"]}}')" "$LEADER_PORT")"
assert_no_error "$resp" "add user2"

resp="$(api POST /api/v1/entries "$(jq -nc --arg dn "$USER_DEL_DN" --arg uid "$USER_DEL" --arg marker "$MARKER" '{dn:$dn,attributes:{objectClass:["inetOrgPerson","organizationalPerson","person","top"],uid:[$uid],givenName:["Delete"],sn:["Me"],cn:["Delete Me"],mail:[($uid+"@example.com")],description:["verify "+$marker],userPassword:["admin"]}}')" "$LEADER_PORT")"
assert_no_error "$resp" "add user_del"

log "assigning users to groups"
resp="$(api PATCH "/api/v1/entries/$(urlenc "$GROUP1_DN")" "$(jq -nc --arg admin "$ADMIN_DN" --arg u1 "$USER1_DN" '{changes:[{operation:"replace",attribute:"member",values:[$admin,$u1]}]}')" "$LEADER_PORT")"
assert_no_error "$resp" "assign group1 members"

resp="$(api PATCH "/api/v1/entries/$(urlenc "$GROUP2_DN")" "$(jq -nc --arg admin "$ADMIN_DN" --arg u2 "$USER2_DN" '{changes:[{operation:"replace",attribute:"member",values:[$admin,$u2]}]}')" "$LEADER_PORT")"
assert_no_error "$resp" "assign group2 members"

log "modifying user attributes"
resp="$(api PATCH "/api/v1/entries/$(urlenc "$USER1_DN")" "$(jq -nc --arg marker "$MARKER" '{changes:[{operation:"replace",attribute:"mail",values:["updated-"+$marker+"@example.com"]},{operation:"replace",attribute:"telephoneNumber",values:["555-0101"]}] }')" "$LEADER_PORT")"
assert_no_error "$resp" "modify user1"

log "deleting one user"
resp="$(api DELETE "/api/v1/entries/$(urlenc "$USER_DEL_DN")" "" "$LEADER_PORT")"
assert_no_error "$resp" "delete user_del"

log "duplicate uid test"
DUP_DN="uid=${USER1}-dup,ou=users,${BASE_DN}"
resp="$(api POST /api/v1/entries "$(jq -nc --arg dn "$DUP_DN" --arg uid "$USER1" '{dn:$dn,attributes:{objectClass:["inetOrgPerson","organizationalPerson","person","top"],uid:[$uid],givenName:["Dup"],sn:["Dup"],cn:["Duplicate"],userPassword:["admin"]}}')" "$LEADER_PORT")"
dup_err="$(jq -r '.error // empty' <<<"$resp" 2>/dev/null || true)"
if [[ "$dup_err" != "uid_not_unique" ]]; then
  fail "duplicate uid test failed: expected uid_not_unique, got payload: $resp"
fi

log "verifying ldap sync across all nodes"
compare_all_nodes "LDAP snapshot" "ldap_snapshot" snapshot_ldap
compare_all_nodes "UID index snapshot" "uid_snapshot" snapshot_uid_index

log "verifying marker logs across all nodes"
compare_marker_logs "$MARKER" "log_snapshot"

check_raft_lag

LEADER_PORT="$(leader_port || true)"
[[ -n "$LEADER_PORT" ]] || fail "leader not found before down/up test"
FOLLOWER_PORT=""
for p in "${NODES[@]}"; do
  if [[ "$p" != "$LEADER_PORT" ]]; then
    FOLLOWER_PORT="$p"
    break
  fi
done
[[ -n "$FOLLOWER_PORT" ]] || fail "no follower found for down/up test"
FOLLOWER_SERVICE="$(service_for_port "$FOLLOWER_PORT")"

log "down/up follower test on ${FOLLOWER_SERVICE} (port ${FOLLOWER_PORT})"
docker compose -f "$COMPOSE_FILE" stop "$FOLLOWER_SERVICE" >/dev/null
sleep 2

resp="$(api_write_on_current_leader POST /api/v1/entries "$(jq -nc --arg dn "$USER_DOWNUP_DN" --arg uid "$USER_DOWNUP" --arg marker "$MARKER" '{dn:$dn,attributes:{objectClass:["inetOrgPerson","organizationalPerson","person","top"],uid:[$uid],givenName:["Downup"],sn:["Test"],cn:["Downup Test"],description:["verify "+$marker],userPassword:["admin"]}}')" 25)"
assert_no_error "$resp" "add downup user while follower down"

docker compose -f "$COMPOSE_FILE" start "$FOLLOWER_SERVICE" >/dev/null
wait_node "$FOLLOWER_PORT" 60 || fail "follower ${FOLLOWER_PORT} did not recover"
sleep 2

TOKEN="$(scripts/cluster_api.sh auth "$FOLLOWER_PORT")"
search_path="/api/v1/search?baseDN=$(urlenc "ou=users,$BASE_DN")&scope=sub&filter=$(urlenc "(uid=$USER_DOWNUP)")&limit=5"
downup_count="0"
for _ in $(seq 1 45); do
  resp="$(api GET "$search_path" "" "$FOLLOWER_PORT")"
  assert_no_error "$resp" "search downup user on recovered follower"
  downup_count="$(extract_total_count "$resp")"
  if [[ "$downup_count" == "1" ]]; then
    break
  fi
  sleep 2
done
if [[ "$downup_count" != "1" ]]; then
  fail "recovered follower missing downup user after catch-up window: count=${downup_count}"
fi

log "full cluster restart test"
docker compose -f "$COMPOSE_FILE" restart oba-node1 oba-node2 oba-node3 >/dev/null
for port in "${NODES[@]}"; do
  wait_node "$port" 80 || fail "node ${port} did not recover after restart"
done

LEADER_PORT="$(leader_port || true)"
[[ -n "$LEADER_PORT" ]] || fail "leader not found after restart"
TOKEN="$(scripts/cluster_api.sh auth "$LEADER_PORT")"

compare_all_nodes "LDAP snapshot after restart" "ldap_snapshot_after_restart" snapshot_ldap
compare_all_nodes "UID index snapshot after restart" "uid_snapshot_after_restart" snapshot_uid_index
compare_marker_logs "$MARKER" "log_snapshot_restart"

search_path="/api/v1/search?baseDN=$(urlenc "ou=users,$BASE_DN")&scope=sub&filter=$(urlenc "(uid=$USER_DOWNUP)")&limit=5"
resp="$(api GET "$search_path" "" "$LEADER_PORT")"
assert_no_error "$resp" "search downup user after full restart"
downup_count="$(extract_total_count "$resp")"
if [[ "$downup_count" != "1" ]]; then
  fail "downup user not persistent after restart: count=${downup_count}"
fi

check_raft_lag
log "PASS: cluster LDAP/log sync and UID uniqueness checks succeeded"
log "created entities remain persisted:"
log "  ${GROUP1_DN}"
log "  ${GROUP2_DN}"
log "  ${USER1_DN}"
log "  ${USER2_DN}"
log "  ${USER_DOWNUP_DN}"
