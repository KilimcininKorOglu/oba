#!/usr/bin/env bash
set -euo pipefail

NODE_PORT="${NODE_PORT:-8082}"
BASE_URL="http://localhost:${NODE_PORT}"
ADMIN_DN="${ADMIN_DN:-cn=admin,dc=example,dc=com}"
ADMIN_PASSWORD="${ADMIN_PASSWORD:-admin}"

usage() {
  cat <<'EOF'
Usage:
  scripts/cluster_api.sh auth [node_port]
  scripts/cluster_api.sh METHOD PATH [JSON_BODY] [node_port]

Examples:
  scripts/cluster_api.sh auth 8082
  scripts/cluster_api.sh GET "/api/v1/cluster/status" "" 8081
  scripts/cluster_api.sh POST "/api/v1/entries" '{"dn":"uid=a,ou=users,dc=example,dc=com","attributes":{"objectClass":["inetOrgPerson"],"uid":["a"],"cn":["A"],"sn":["A"],"userPassword":["admin"]}}' 8082
EOF
}

get_token() {
  local port="$1"
  curl -sS -X POST "http://localhost:${port}/api/v1/auth/bind" \
    -H 'Content-Type: application/json' \
    -d "{\"dn\":\"${ADMIN_DN}\",\"password\":\"${ADMIN_PASSWORD}\"}" |
    sed -n 's/.*"token":"\([^"]*\)".*/\1/p'
}

if [[ $# -lt 1 ]]; then
  usage
  exit 1
fi

if [[ "$1" == "auth" ]]; then
  if [[ $# -ge 2 ]]; then
    NODE_PORT="$2"
  fi
  get_token "${NODE_PORT}"
  exit 0
fi

if [[ $# -lt 2 ]]; then
  usage
  exit 1
fi

METHOD="$1"
PATH_ONLY="$2"
BODY="${3:-}"
if [[ $# -ge 4 ]]; then
  NODE_PORT="$4"
  BASE_URL="http://localhost:${NODE_PORT}"
fi

TOKEN="${TOKEN:-}"
if [[ -z "${TOKEN}" ]]; then
  TOKEN="$(get_token "${NODE_PORT}")"
fi

if [[ -n "${BODY}" ]]; then
  curl -sS -X "${METHOD}" "${BASE_URL}${PATH_ONLY}" \
    -H "Authorization: Bearer ${TOKEN}" \
    -H 'Content-Type: application/json' \
    -d "${BODY}"
else
  curl -sS -X "${METHOD}" "${BASE_URL}${PATH_ONLY}" \
    -H "Authorization: Bearer ${TOKEN}"
fi
