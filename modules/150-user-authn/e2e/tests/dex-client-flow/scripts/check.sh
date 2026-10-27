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

# Checks the tokens a DexClient gets for a local user and that a DexClient with allowedGroups
# refuses a user outside the groups.
set -Eeuo pipefail
source ../../tests/common/scripts/lib.sh

dex_discover
dir="$(state_dir dex-client-flow)"
client_id="$(dex_client_id "${NS}" e2e-authn-flow-client)"
claims="$(jwt_claims "$(jq -r '.id_token' "${dir}/e2e-authn-client-user.tokens.json")")"

jq -e --arg id "${client_id}" '(.aud | if type == "array" then . else [.] end) == [$id]' <<<"${claims}" >/dev/null \
  || die "aud is $(jq -c .aud <<<"${claims}"), expected [\"${client_id}\"]"
jq -e '.email == "e2e-authn-client-user@example.com"' <<<"${claims}" >/dev/null \
  || die "email is $(jq -c .email <<<"${claims}")"
jq -e '.access_token and .refresh_token and .token_type == "bearer"' "${dir}/e2e-authn-client-user.tokens.json" >/dev/null \
  || die "the token response lacks an access token, a refresh token or the bearer type"
log "The ID token is issued for ${client_id} only and the response has access and refresh tokens"

dex_authorize e2e-authn-client-user@example.com "$(user_password dex-client-flow e2e-authn-client-user)" \
  "$(dex_client_id "${NS}" e2e-authn-flow-restricted)" "${TEST_REDIRECT_URI}" "${TEST_SCOPE}"
[[ -z "${AUTH_CODE}" && "${AUTH_STATUS}" == "401" ]] && grep -q "Access denied" "${AUTH_BODY}" \
  || die "a user outside allowedGroups must get HTTP 401 'Access denied': $(auth_describe)"
log "The client with allowedGroups refuses the user: HTTP 401 Access denied"

dex_token --data-urlencode "grant_type=authorization_code" --data-urlencode "code=invalid-code" \
  --data-urlencode "redirect_uri=${TEST_REDIRECT_URI}" --data-urlencode "client_id=${client_id}" \
  --data-urlencode "client_secret=wrong-secret"
[[ "${TOKEN_STATUS}" == "401" ]] && jq -e '.error == "invalid_client"' "${TOKEN_BODY}" >/dev/null \
  || die "a wrong client secret must get HTTP 401 invalid_client, got ${TOKEN_STATUS}: $(jq -c "{error, error_description}" "${TOKEN_BODY}" 2>/dev/null || echo "not JSON")"
log "A wrong client secret gets HTTP 401 invalid_client"
