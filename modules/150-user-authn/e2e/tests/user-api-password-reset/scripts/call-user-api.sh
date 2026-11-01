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

# Calls the self-service password reset endpoint of user-api with the ID token of the user.
# The endpoint must refuse a request without a token and a hash that is not bcrypt, and accept
# a bcrypt hash by creating a UserOperation ResetPassword with initiatorType self.
set -Eeuo pipefail
source ../../tests/common/scripts/lib.sh

dir="$(state_dir user-api-password-reset)"
ip="$(kubectl -n d8-user-authn get service user-api -o jsonpath='{.spec.clusterIP}')"
[[ -n "${ip}" ]] || die "Service d8-user-authn/user-api has no ClusterIP"
[[ "${ip}" == *:* ]] && ip="[${ip}]"
url="https://${ip}/api/v1/password/reset"
auth_header="$(secret_file "Authorization: Bearer $(jq -r .id_token "${dir}/e2e-authn-api-user.tokens.json")")"
body="${dir}/response.json"

call() {
  curl -sS -k --max-time 30 -o "${body}" -w '%{http_code}' -X POST -H 'Content-Type: application/json' "$@" "${url}"
}

code="$(call -d '{"newPasswordHash":"$2a$10$abc"}')"
[[ "${code}" == "401" ]] && jq -e '.error == "unauthorized"' "${body}" >/dev/null \
  || die "a request without a token must get HTTP 401 unauthorized, got ${code}: $(head -c 200 "${body}")"
log "Without a token: HTTP 401 $(jq -c . "${body}")"

code="$(call -H "@${auth_header}" -d '{"newPasswordHash":"plain-text"}')"
[[ "${code}" == "400" ]] && jq -e '.error == "invalid_request" and .message == "newPasswordHash must be a bcrypt hash (starting with $2)"' "${body}" >/dev/null \
  || die "a hash that is not bcrypt must get HTTP 400, got ${code}: $(head -c 200 "${body}")"
log "A hash that is not bcrypt: HTTP 400 $(jq -c . "${body}")"

new_password="$(gen_password)"
printf '%s' "${new_password}" > "${dir}/new.password"
hash="$(hash_password "${new_password}")"
[[ -n "${hash}" ]] || die "the password hash is empty"
# The hash goes to jq through stdin and to curl through a file, not through argv.
request="$(tmp_file)"
printf '%s' "${hash}" | jq -Rsc '{newPasswordHash: .}' > "${request}"
code="$(call -H "@${auth_header}" -d "@${request}")"
[[ "${code}" == "202" ]] && jq -e '.status == "accepted" and (.operationName | startswith("self-password-reset-"))' "${body}" >/dev/null \
  || die "a valid request must get HTTP 202 with an operation name, got ${code}: $(head -c 200 "${body}")"
jq -r .operationName "${body}" > "${dir}/operation"
log "A valid request: HTTP 202, UserOperation $(cat "${dir}/operation")"
