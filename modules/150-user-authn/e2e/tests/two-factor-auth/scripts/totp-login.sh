#!/usr/bin/env bash

# Copyright 2026 Flant JSC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Signs e2e-authn-2fa-user in with staticUsers2FA enabled: password form, then the TOTP page,
# then the approval page, and finally the authorization code.
#
# Usage: totp-login.sh enroll|known
#   enroll  the TOTP page must show the enrollment QR code (no confirmed TOTP yet); a wrong
#           code is refused before the right one is accepted;
#   known   the TOTP page must ask for a code without a QR code.
#
# The TOTP secret is read from the OfflineSessions object of the user in d8-user-authn, the
# way an authenticator app reads it from the QR code, and the code is computed with openssl.
set -Eeuo pipefail
source ../../tests/common/scripts/lib.sh

mode="${1:?enroll or known}"
dex_discover
client_id="$(dex_client_id "${NS}" e2e-authn-2fa-client)"
email="e2e-authn-2fa-user@example.com"

deadline=$(( $(date +%s) + 300 ))
while :; do
  dex_authorize "${email}" "$(user_password two-factor-auth e2e-authn-2fa-user)" "${client_id}" "${TEST_REDIRECT_URI}" "${TEST_SCOPE}"
  [[ "${AUTH_STATUS}" == "303" && "${AUTH_LOCATION}" == *"/totp?"* ]] && break
  (( $(date +%s) < deadline )) || die "the password must lead to the TOTP page: $(auth_describe)"
  sleep 5
done
[[ -z "${AUTH_CODE}" ]] || die "Dex issued a code before the second factor"
totp_url="${AUTH_LOCATION}"
log "The password leads to the TOTP page"

page() {
  dex_curl -b "${AUTH_JAR}" -c "${AUTH_JAR}" -o "${AUTH_BODY}" -w '%{http_code} %{redirect_url}' "$@"
}

out="$(page "${totp_url}")"
[[ "${out%% *}" == "200" ]] || die "the TOTP page must answer 200, got '${out%%\?*}'"
case "${mode}" in
  enroll)
    grep -q "Scan the QR code" "${AUTH_BODY}" || die "the first TOTP page must show the enrollment QR code: $(html_text "${AUTH_BODY}" | head -c 300)"
    log "The TOTP page shows the enrollment QR code"
    ;;
  known)
    grep -q "Scan the QR code" "${AUTH_BODY}" && die "a confirmed TOTP must not be shown as a QR code again"
    grep -q "Open your authenticator app" "${AUTH_BODY}" || die "the TOTP page must ask for a code: $(html_text "${AUTH_BODY}" | head -c 300)"
    log "The TOTP page asks for a code without a QR code"
    ;;
esac

otpauth="$(kubectl -n d8-user-authn get offlinesessionses.dex.coreos.com -o json \
  | jq -r '[.items[] | select(.userID == "e2e-authn-2fa-user" and .connID == "local") | .totp] | first // empty')"
[[ "${otpauth}" == otpauth://* ]] || die "the OfflineSessions object of the user has no TOTP key"
secret="$(query_param "${otpauth}" secret)"
code="$(totp_code "${secret}")"

if [[ "${mode}" == "enroll" ]]; then
  wrong="$(printf '%06d' $(( (10#${code} + 500000) % 1000000 )))"
  out="$(page --data-urlencode "totp=${wrong}" "${totp_url}")"
  # Dex renders the TOTP page again with HTTP 401 (patch 004-2fa, w.WriteHeader(http.StatusUnauthorized)).
  [[ "${out%% *}" == "401" ]] && grep -q "Invalid or Expired User Code" "${AUTH_BODY}" \
    || die "a wrong TOTP code must be refused: '${out%%\?*}' $(html_text "${AUTH_BODY}" | head -c 300)"
  log "A wrong TOTP code is refused"
  code="$(totp_code "${secret}")"
fi

out="$(page --data-urlencode "totp=${code}" "${totp_url}")"
approval_url="${out#* }"
[[ "${out%% *}" == "303" ]] || die "the right TOTP code must be accepted with 303, got '${out%%\?*}': $(html_text "${AUTH_BODY}" | head -c 300)"
log "The TOTP code is accepted"

# With 2FA the module turns the approval screen on (skipApprovalScreen: false).
if [[ "${approval_url}" == *"/approval?"* ]]; then
  out="$(page "${approval_url}")"
  [[ "${out%% *}" == "200" ]] || die "the approval page must answer 200, got '${out%%\?*}'"
  out="$(page --data-urlencode "approval=approve" "${approval_url}")"
  AUTH_LOCATION="${out#* }"
else
  AUTH_LOCATION="${approval_url}"
fi
[[ "${AUTH_LOCATION}" == "${TEST_REDIRECT_URI}?"*"code="* ]] \
  || die "after the second factor Dex must redirect to the client with a code, got '${AUTH_LOCATION%%\?*}'"
dex_exchange_code "$(query_param "${AUTH_LOCATION}" code)" "${client_id}" "$(dex_client_secret "${NS}" e2e-authn-2fa-client)" \
  || die "the code issued after the second factor was not exchanged"
log "Signed in with the password and the TOTP code"
