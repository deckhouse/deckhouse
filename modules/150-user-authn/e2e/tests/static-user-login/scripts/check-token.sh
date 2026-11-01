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

# Checks the claims of the ID token of e2e-authn-login-user and that a wrong password
# gets the login form back instead of an authorization code.
set -Eeuo pipefail
source ../../tests/common/scripts/lib.sh

dex_discover
tokens="$(state_dir static-user-login)/e2e-authn-login-user.tokens.json"
claims="$(jwt_claims "$(jq -r '.id_token' "${tokens}")")"

jq -e --arg iss "${DEX_ISSUER}" '(.iss | rtrimstr("/")) == $iss' <<<"${claims}" >/dev/null \
  || die "iss is $(jq -c .iss <<<"${claims}"), expected ${DEX_ISSUER}/"
jq -e '.email == "e2e-authn-login-user@example.com" and .email_verified == true' <<<"${claims}" >/dev/null \
  || die "email claims are $(jq -c '{email, email_verified}' <<<"${claims}")"
jq -e '.groups | index("e2e-authn-login-group") != null' <<<"${claims}" >/dev/null \
  || die "groups claim is $(jq -c .groups <<<"${claims}"), expected to contain e2e-authn-login-group"
jq -e '(.aud | if type == "array" then . else [.] end) | index("kubernetes") != null' <<<"${claims}" >/dev/null \
  || die "aud is $(jq -c .aud <<<"${claims}"), expected to contain kubernetes"
jq -e '.name == "e2e-authn-login-user" and .federated_claims.connector_id == "local"' <<<"${claims}" >/dev/null \
  || die "name and connector are $(jq -c '{name, federated_claims}' <<<"${claims}")"
log "ID token claims are correct: $(jq -c '{iss, aud, email, groups, name}' <<<"${claims}")"

client_id="$(dex_client_id "${NS}" e2e-authn-login-client)"
dex_authorize e2e-authn-login-user@example.com "wrong-$(gen_password)" "${client_id}" "${TEST_REDIRECT_URI}" "${TEST_SCOPE}"
[[ -z "${AUTH_CODE}" && "${AUTH_STATUS}" == "401" ]] \
  || die "a wrong password must return the login form with HTTP 401 and no code: $(auth_describe)"
log "Wrong password: HTTP 401, no authorization code"
