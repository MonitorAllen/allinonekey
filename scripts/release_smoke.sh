#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DB_FILE="${ALLINONEKEY_SMOKE_DB:-/tmp/allinonekey-release-smoke.db}"
BIN_FILE="/tmp/allinonekey-release-smoke-server"
PORT="${ALLINONEKEY_SMOKE_PORT:-8080}"
BASE_URL="http://127.0.0.1:${PORT}/api"
MASTER_KEY="ReleaseSmokeKey123!"
ADMIN_USER="smoke_admin_$(date +%s)"
USER_NAME="smoke_user_$(date +%s)"
USER_MASTER_KEY="ReleaseSmokeUserKey123!"
SERVER_PID=""

cleanup() {
  if [[ -n "${SERVER_PID}" ]]; then
    kill "${SERVER_PID}" >/dev/null 2>&1 || true
    wait "${SERVER_PID}" >/dev/null 2>&1 || true
  fi
  rm -f "${BIN_FILE}" "${DB_FILE}" "${DB_FILE}-wal" "${DB_FILE}-shm" /tmp/allinonekey-export.json /tmp/allinonekey-export.csv
}
trap cleanup EXIT

cd "${ROOT_DIR}"
rm -f "${DB_FILE}" "${DB_FILE}-wal" "${DB_FILE}-shm"
GO_BIN="${GO_BIN:-$(command -v go)}"
if [[ -z "${GO_BIN}" ]]; then
  echo "go binary not found" >&2
  exit 1
fi
export GOPROXY=https://goproxy.cn,direct
"${GO_BIN}" build -o "${BIN_FILE}" ./cmd/server/main.go

if command -v make >/dev/null 2>&1; then
  make -s free-port-8080 >/dev/null 2>&1 || true
fi

ALLINONEKEY_DB_PATH="${DB_FILE}" \
ALLINONEKEY_JWT_SECRET="release-smoke-jwt-secret-change-me" \
ALLINONEKEY_SESSION_SECRET="release-smoke-session-secret-change-me" \
PORT="${PORT}" \
"${BIN_FILE}" >/tmp/allinonekey-release-smoke.log 2>&1 &
SERVER_PID=$!

for _ in $(seq 1 40); do
  if curl -fsS "${BASE_URL}/keys/list" >/tmp/allinonekey-smoke-unauth.json 2>/dev/null; then
    break
  fi
  if grep -q "Server running" /tmp/allinonekey-release-smoke.log 2>/dev/null; then
    break
  fi
  sleep 0.25
done

curl_json() {
  local method="$1"
  local url="$2"
  local token="${3:-}"
  local body="${4:-}"
  if [[ -n "${token}" && -n "${body}" ]]; then
    curl -fsS -X "${method}" "${url}" -H "Authorization: Bearer ${token}" -H 'Content-Type: application/json' -d "${body}"
  elif [[ -n "${token}" ]]; then
    curl -fsS -X "${method}" "${url}" -H "Authorization: Bearer ${token}"
  elif [[ -n "${body}" ]]; then
    curl -fsS -X "${method}" "${url}" -H 'Content-Type: application/json' -d "${body}"
  else
    curl -fsS -X "${method}" "${url}"
  fi
}

assert_json() {
  python3 - "$@"
}

curl_json POST "${BASE_URL}/register" "" "{\"username\":\"${ADMIN_USER}\",\"master_key\":\"${MASTER_KEY}\"}" >/tmp/register-admin.json
ADMIN_LOGIN=$(curl_json POST "${BASE_URL}/login" "" "{\"username\":\"${ADMIN_USER}\",\"master_key\":\"${MASTER_KEY}\"}")
ADMIN_TOKEN=$(printf '%s' "${ADMIN_LOGIN}" | python3 -c 'import json,sys; data=json.load(sys.stdin); assert data["role"] == "admin"; print(data["token"])')
python3 - "${ADMIN_TOKEN}" "${MASTER_KEY}" <<'PY'
import base64, sys

token, master_key = sys.argv[1], sys.argv[2]
assert token.count('.') != 2, 'token must not be a readable JWT'
try:
    raw = base64.urlsafe_b64decode(token + '=' * (-len(token) % 4))
except Exception:
    raw = token.encode()
assert master_key.encode() not in raw, 'sealed token leaks master key'
PY

curl_json GET "${BASE_URL}/keys/list" "${ADMIN_TOKEN}" >/tmp/keys-empty.json
python3 - <<'PY'
import json
with open('/tmp/keys-empty.json') as f:
    assert isinstance(json.load(f), list)
PY

INVITE=$(curl_json POST "${BASE_URL}/admin/invites" "${ADMIN_TOKEN}" '{"expires_in_hours":24}')
INVITE_CODE=$(printf '%s' "${INVITE}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["code"])')
curl_json POST "${BASE_URL}/register" "" "{\"username\":\"${USER_NAME}\",\"master_key\":\"${USER_MASTER_KEY}\",\"invite_code\":\"${INVITE_CODE}\"}" >/tmp/register-user.json
USER_LOGIN=$(curl_json POST "${BASE_URL}/login" "" "{\"username\":\"${USER_NAME}\",\"master_key\":\"${USER_MASTER_KEY}\"}")
USER_TOKEN=$(printf '%s' "${USER_LOGIN}" | python3 -c 'import json,sys; data=json.load(sys.stdin); assert data["role"] == "user"; print(data["token"])')
ADMIN_STATUS=$(curl -s -o /tmp/nonadmin-admin.json -w '%{http_code}' -H "Authorization: Bearer ${USER_TOKEN}" "${BASE_URL}/admin/invites")
test "${ADMIN_STATUS}" = "403"

KEY_CREATE=$(curl_json POST "${BASE_URL}/keys/create" "${ADMIN_TOKEN}" '{"provider":"Custom","pool_group":"release","base_url":"http://127.0.0.1:1","keys":[{"key_name":"smoke-key","key_value":"sk-smoke-value"}]}')
printf '%s' "${KEY_CREATE}" | python3 -c 'import json,sys; data=json.load(sys.stdin); assert data["message"] == "success" and data["count"] == 1'
KEY_ID=$(curl_json GET "${BASE_URL}/keys/list" "${ADMIN_TOKEN}" | python3 -c 'import json,sys; data=json.load(sys.stdin); assert len(data) == 1; print(data[0]["id"])')
curl_json GET "${BASE_URL}/keys/${KEY_ID}/decrypt" "${ADMIN_TOKEN}" >/tmp/key-decrypt.json
python3 - <<'PY'
import json
with open('/tmp/key-decrypt.json') as f:
    assert json.load(f)['key'] == 'sk-smoke-value'
PY
HEALTH=$(curl_json POST "${BASE_URL}/keys/${KEY_ID}/check-quota" "${ADMIN_TOKEN}")
printf '%s' "${HEALTH}" | python3 -c 'import json,sys; assert json.load(sys.stdin)["status"] in {"quota_error","auth_error","rate_limited","active","quota_unsupported"}'

ACCOUNT_CREATE=$(curl_json POST "${BASE_URL}/accounts/create" "${ADMIN_TOKEN}" '{"platform":"GitHub","url":"https://github.com","account":"smoke@example.test","password":"SmokePassword123!","totp_secret":"JBSWY3DPEHPK3PXP"}')
ACCOUNT_ID=$(printf '%s' "${ACCOUNT_CREATE}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')
curl_json GET "${BASE_URL}/accounts/${ACCOUNT_ID}/decrypt" "${ADMIN_TOKEN}" >/tmp/account-decrypt.json
python3 - <<'PY'
import json
with open('/tmp/account-decrypt.json') as f:
    assert json.load(f)['password'] == 'SmokePassword123!'
PY
curl_json GET "${BASE_URL}/accounts/${ACCOUNT_ID}/totp" "${ADMIN_TOKEN}" >/tmp/account-totp.json
python3 - <<'PY'
import json, re
with open('/tmp/account-totp.json') as f:
    data = json.load(f)
assert re.fullmatch(r'\d{6}', data['code'])
PY

curl -fsS -H "Authorization: Bearer ${ADMIN_TOKEN}" "${BASE_URL}/export/json" -o /tmp/allinonekey-export.json
curl -fsS -H "Authorization: Bearer ${ADMIN_TOKEN}" "${BASE_URL}/export/csv" -o /tmp/allinonekey-export.csv
python3 - <<'PY'
import json
with open('/tmp/allinonekey-export.json') as f:
    data = json.load(f)
assert data['version'] == '0.2.0'
assert data['api_keys'] and data['accounts']
raw = open('/tmp/allinonekey-export.csv').read()
assert 'api_key' in raw and 'account' in raw
PY

curl_json POST "${BASE_URL}/import/keys/json" "${ADMIN_TOKEN}" "$(cat /tmp/allinonekey-export.json)" >/tmp/import-keys.json
PLAINTEXT_IMPORT_STATUS=$(curl -s -o /tmp/plaintext-import.json -w '%{http_code}' -X POST "${BASE_URL}/import/keys/json" -H "Authorization: Bearer ${ADMIN_TOKEN}" -H 'Content-Type: application/json' -d '{"api_keys":[{"provider":"Custom","key_name":"bad","key_value":"plaintext"}]}')
test "${PLAINTEXT_IMPORT_STATUS}" = "400"

for i in 1 2 3 4 5; do
  curl -s -o /tmp/login-fail-${i}.json -X POST "${BASE_URL}/login" -H 'Content-Type: application/json' -d "{\"username\":\"${ADMIN_USER}\",\"master_key\":\"WrongKey123!\"}" >/dev/null || true
done
LOCK_STATUS=$(curl -s -o /tmp/login-lock.json -w '%{http_code}' -X POST "${BASE_URL}/login" -H 'Content-Type: application/json' -d "{\"username\":\"${ADMIN_USER}\",\"master_key\":\"WrongKey123!\"}")
test "${LOCK_STATUS}" = "429"

"${GO_BIN}" run scripts/decrypt.go /tmp/allinonekey-export.json "${MASTER_KEY}" >/tmp/offline-decrypt.txt
python3 - <<'PY'
raw = open('/tmp/offline-decrypt.txt').read()
assert 'sk-smoke-value' in raw
assert 'SmokePassword123!' in raw
PY

echo "release smoke ok"
