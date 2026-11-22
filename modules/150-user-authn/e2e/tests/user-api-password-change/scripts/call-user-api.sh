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

# The release-1.77 endpoint requires authentication and validates the password hash.
set -Eeuo pipefail
source ../../tests/common/scripts/lib.sh

dir="$(state_dir user-api-password-change)"
ip="$(kubectl -n d8-user-authn get service user-api -o jsonpath='{.spec.clusterIP}')"
[[ -n "${ip}" ]] || die "Service d8-user-authn/user-api has no ClusterIP"
[[ "${ip}" == *:* ]] && ip="[${ip}]"
auth_header="$(secret_file "Authorization: Bearer $(jq -r .id_token "${dir}/e2e-authn-api-user.tokens.json")")"
body="${dir}/response.json"

call() {
  curl -sS -k --max-time 30 -o "${body}" -w '%{http_code}' -X POST -H 'Content-Type: application/json' "$@" "https://${ip}/api/v1/password/reset"
}

code="$(call -d '{}')"
[[ "${code}" == "401" ]] && jq -e '.error == "unauthorized"' "${body}" >/dev/null \
  || die "a request without a token must get HTTP 401 unauthorized, got ${code}"
log "Without a token: HTTP 401"

code="$(call -H "@${auth_header}" -d '{"newPasswordHash":"invalid"}')"
[[ "${code}" == "400" ]] && jq -e '.error == "invalid_request"' "${body}" >/dev/null \
  || die "an invalid password hash must get HTTP 400 invalid_request, got ${code}"
log "With an invalid password hash: HTTP 400"

ops="$(kubectl get useroperations.deckhouse.io -o json \
  | jq -r '[.items[] | select(.spec.user == "e2e-authn-api-user") | .metadata.name] | join(" ")')"
[[ -z "${ops}" ]] || die "a refused request must create no UserOperation, found: ${ops}"
log "No UserOperation of e2e-authn-api-user was created"
