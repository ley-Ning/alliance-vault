#!/usr/bin/env bash
set -euo pipefail

API_BASE_URL="${API_BASE_URL:-http://localhost:8088}"
ADMIN_USERNAME="${ADMIN_USERNAME:-admin}"
ADMIN_PASSWORD="${ADMIN_PASSWORD:-12345678}"

if ! command -v curl >/dev/null 2>&1; then
  echo "[ERROR] curl is required" >&2
  exit 1
fi

if ! command -v python3 >/dev/null 2>&1; then
  echo "[ERROR] python3 is required" >&2
  exit 1
fi

workdir="$(mktemp -d)"
cleanup() {
  rm -rf "$workdir"
}
trap cleanup EXIT

stamp="$(date +%s)"
upload_file="${workdir}/upload.txt"
download_file="${workdir}/download.txt"
printf 's3-compatible-check-%s\n' "$stamp" > "$upload_file"
size_bytes="$(wc -c < "$upload_file" | tr -d ' ')"

auth_token=""
refresh_token=""
document_id=""
attachment_id=""

json_get() {
  local json="$1"
  local path="$2"
  python3 - "$json" "$path" <<'PY'
import json
import sys

payload = json.loads(sys.argv[1])
value = payload
for part in sys.argv[2].split('.'):
    if not part:
        continue
    value = value[part]

if isinstance(value, (dict, list)):
    print(json.dumps(value, ensure_ascii=False))
else:
    print(value)
PY
}

api_request() {
  local method="$1"
  local path="$2"
  local expected_status="$3"
  local body="${4:-}"
  local token="${5:-}"

  local url="${API_BASE_URL}${path}"
  local response
  local status
  local payload

  if [ -n "$token" ]; then
    if [ -n "$body" ]; then
      response="$(curl -sS -w '\n%{http_code}' -X "$method" "$url" -H 'Content-Type: application/json' -H "Authorization: Bearer ${token}" -d "$body")"
    else
      response="$(curl -sS -w '\n%{http_code}' -X "$method" "$url" -H 'Content-Type: application/json' -H "Authorization: Bearer ${token}")"
    fi
  else
    if [ -n "$body" ]; then
      response="$(curl -sS -w '\n%{http_code}' -X "$method" "$url" -H 'Content-Type: application/json' -d "$body")"
    else
      response="$(curl -sS -w '\n%{http_code}' -X "$method" "$url" -H 'Content-Type: application/json')"
    fi
  fi

  status="${response##*$'\n'}"
  payload="${response%$'\n'*}"

  if [ "$status" != "$expected_status" ]; then
    echo "[ERROR] ${method} ${path} expected ${expected_status} but got ${status}" >&2
    echo "[ERROR] response: ${payload}" >&2
    exit 1
  fi

  printf '%s' "$payload"
}

echo "[1/9] health check"
api_request GET "/api/v1/health" "200" >/dev/null

echo "[2/9] login admin"
login_payload="$(printf '{"username":"%s","password":"%s"}' "$ADMIN_USERNAME" "$ADMIN_PASSWORD")"
login_response="$(api_request POST "/api/v1/auth/login" "200" "$login_payload")"
auth_token="$(json_get "$login_response" "accessToken")"
refresh_token="$(json_get "$login_response" "refreshToken")"
must_change_password="$(json_get "$login_response" "user.mustChangePassword")"

if [ "$must_change_password" = "true" ]; then
  echo "[ERROR] admin account must change password before running checks." >&2
  echo "        please login once in UI and update password, then re-run script with:" >&2
  echo "        ADMIN_USERNAME=<admin> ADMIN_PASSWORD=<new-password> $0" >&2
  exit 1
fi

echo "[3/9] create temp document"
doc_payload='{"title":"RustFS compatibility check","content":"<p>temporary check</p>","tags":["storage"],"status":"草稿"}'
doc_response="$(api_request POST "/api/v1/documents" "201" "$doc_payload" "$auth_token")"
document_id="$(json_get "$doc_response" "document.id")"

echo "[4/9] request presigned upload URL"
presign_payload="$(printf '{"documentId":"%s","fileName":"%s","contentType":"text/plain","sizeBytes":%s}' "$document_id" "storage-check.txt" "$size_bytes")"
presign_response="$(api_request POST "/api/v1/uploads/presign" "200" "$presign_payload" "$auth_token")"
upload_url="$(json_get "$presign_response" "uploadUrl")"
object_key="$(json_get "$presign_response" "objectKey")"

echo "[5/9] upload file via presigned URL"
upload_status="$(curl -sS -o /dev/null -w '%{http_code}' -X PUT "$upload_url" -H 'Content-Type: text/plain' --upload-file "$upload_file")"
if [ "$upload_status" != "200" ] && [ "$upload_status" != "204" ]; then
  echo "[ERROR] upload failed, status=${upload_status}" >&2
  exit 1
fi

echo "[6/9] complete upload metadata"
complete_payload="$(printf '{"documentId":"%s","objectKey":"%s","fileName":"%s","contentType":"text/plain","sizeBytes":%s,"owner":"%s"}' "$document_id" "$object_key" "storage-check.txt" "$size_bytes" "$ADMIN_USERNAME")"
complete_response="$(api_request POST "/api/v1/uploads/complete" "201" "$complete_payload" "$auth_token")"
attachment_id="$(json_get "$complete_response" "attachment.id")"

echo "[7/9] request presigned download URL"
download_response="$(api_request GET "/api/v1/attachments/${attachment_id}/download-url" "200" "" "$auth_token")"
download_url="$(json_get "$download_response" "downloadUrl")"

echo "[8/9] download file and compare"
curl -sS -fL "$download_url" -o "$download_file"
if ! cmp -s "$upload_file" "$download_file"; then
  echo "[ERROR] downloaded file content differs from uploaded file" >&2
  exit 1
fi

echo "[9/9] cleanup temp document"
api_request DELETE "/api/v1/documents/${document_id}" "200" "" "$auth_token" >/dev/null
if [ -n "$refresh_token" ]; then
  logout_payload="$(printf '{"refreshToken":"%s"}' "$refresh_token")"
  api_request POST "/api/v1/auth/logout" "200" "$logout_payload" "$auth_token" >/dev/null || true
fi

echo "[OK] object storage compatibility check passed"
echo "      API_BASE_URL=${API_BASE_URL}"
echo "      user=${ADMIN_USERNAME}"
