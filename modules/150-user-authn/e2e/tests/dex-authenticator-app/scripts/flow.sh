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

# Walks through the sign-in of a browser to an application protected by a DexAuthenticator:
#   1. an anonymous request to the application is redirected to /dex-authenticator/sign_in;
#   2. sign_in redirects to Dex;
#   3. the user posts the password form of the local connector, Dex redirects to the callback;
#   4. the callback sets the session cookie and redirects back to the application;
#   5. with the cookie, the application answers 200 and gets the email in X-Auth-Request-Email.
# Requests to the application go to a Ready ingress-nginx controller pod, requests to Dex go to
# the ClusterIP of the dex Service; --resolve keeps the real host names, so cookies work.
set -Eeuo pipefail
source ../../tests/common/scripts/lib.sh

dex_discover
ingress_discover
dir="$(state_dir dex-authenticator-app)"
app_host="$(cat "${dir}/app-host")"
email="e2e-authn-app-user@example.com"
password="$(user_password dex-authenticator-app e2e-authn-app-user)"
jar="${dir}/cookies"
body="${dir}/body"
: > "${jar}"

app_curl() {
  curl -sS -k --max-time 30 "${ING_CURL_ARGS[@]}" --resolve "${app_host}:443:${ING_IP}" \
    -b "${jar}" -c "${jar}" -o "${body}" -w '%{http_code} %{redirect_url}' "$@"
}

# 1. Anonymous request. Retried while the controller loads the Ingress and the authenticator starts.
deadline=$(( $(date +%s) + 300 ))
while :; do
  out="$(app_curl "https://${app_host}/hostname" || true)"
  [[ "${out%% *}" == "302" && "${out#* }" == *"/dex-authenticator/sign_in"* ]] && break
  (( $(date +%s) < deadline )) || die "an anonymous request must be redirected to /dex-authenticator/sign_in, got '${out}': $(html_text "${body}" | head -c 300)"
  sleep 5
done
sign_in_url="${out#* }"
log "Anonymous request: HTTP 302 to ${sign_in_url%%\?*}"

# 2. sign_in redirects to the Dex authorization endpoint. /dex-authenticator is served by the
#    Ingress of the authenticator through the endpoints of its Service, which ingress-nginx picks
#    up some seconds after the pod is Ready; until then it answers 503 (the anonymous request above
#    does not show it: its auth subrequest goes to the Service host name). 502, 503 and 404 are
#    retried for at most 3 minutes, and the last answer is reported if they persist.
deadline=$(( $(date +%s) + 180 ))
while :; do
  out="$(app_curl "${sign_in_url}" || true)"
  case "${out%% *}" in
    502|503|404) ;;
    *) break ;;
  esac
  (( $(date +%s) < deadline )) || die "sign_in kept answering '${out%% *}' for 3 minutes: $(html_text "${body}" | head -c 300)"
  echo "sign_in answered ${out%% *}, the controller has not picked up the authenticator endpoints yet"
  sleep 5
done
dex_auth_url="${out#* }"
[[ "${out%% *}" == "302" && "${dex_auth_url}" == "${DEX_ISSUER}/auth?"* ]] \
  || die "sign_in must redirect to ${DEX_ISSUER}/auth, got '${out%%\?*}'"
redirect_uri="$(query_param "${dex_auth_url}" redirect_uri)"
[[ "${redirect_uri}" == "https://${app_host}/dex-authenticator/callback" ]] \
  || die "redirect_uri is '${redirect_uri}', expected https://${app_host}/dex-authenticator/callback"
log "sign_in: HTTP 302 to Dex with redirect_uri ${redirect_uri}"

# 3. Dex: open the local connector directly (skips the connector choice when there are several)
#    and post the password form.
dex_jar="${dir}/dex-cookies"
out="$(dex_curl -b "${dex_jar}" -c "${dex_jar}" -o "${body}" -w '%{http_code} %{redirect_url}' "${dex_auth_url/\/auth\?//auth/local?}")"
login_url="${out#* }"
[[ "${login_url}" == *"/auth/local/login"* ]] || die "Dex did not open the login form: '${out%%\?*}': $(html_text "${body}" | head -c 300)"
out="$(dex_curl -b "${dex_jar}" -c "${dex_jar}" -o "${body}" -w '%{http_code} %{redirect_url}' \
  --data-urlencode "login=${email}" --data-urlencode "password@$(secret_file "${password}")" "${login_url}")"
callback_url="${out#* }"
[[ "${out%% *}" == "303" && "${callback_url}" == "${redirect_uri}?"*"code="* ]] \
  || die "Dex must redirect to the callback with a code, got '${out%%\?*}': $(html_text "${body}" | head -c 300)"
log "Dex: HTTP 303 to the callback with an authorization code"

# 4. The callback redeems the code and sets the session cookie.
out="$(app_curl "${callback_url}")"
[[ "${out%% *}" == "302" ]] || die "the callback must redirect back to the application, got '${out}': $(html_text "${body}" | head -c 300)"
log "Callback: HTTP 302 to ${out#* }"

# 5. Authenticated requests reach the application.
out="$(app_curl "https://${app_host}/hostname")"
[[ "${out%% *}" == "200" ]] || die "an authenticated request must get HTTP 200, got '${out}'"
grep -q '^e2e-authn-app-' "${body}" || die "the answer is not from the application: $(head -c 200 "${body}")"
log "Authenticated request: HTTP 200 from pod $(cat "${body}")"

out="$(app_curl "https://${app_host}/header?key=X-Auth-Request-Email")"
[[ "${out%% *}" == "200" && "$(tr -d '\r\n' < "${body}")" == "${email}" ]] \
  || die "the application must get X-Auth-Request-Email ${email}, got '${out}' '$(head -c 200 "${body}")'"
log "The application received X-Auth-Request-Email: ${email}"

# A request without the session cookie is still redirected.
out="$(curl -sS -k --max-time 30 "${ING_CURL_ARGS[@]}" --resolve "${app_host}:443:${ING_IP}" -o /dev/null -w '%{http_code}' "https://${app_host}/hostname")"
[[ "${out}" == "302" ]] || die "a request without the session cookie must get HTTP 302, got ${out}"
log "A request without the session cookie: HTTP 302"
