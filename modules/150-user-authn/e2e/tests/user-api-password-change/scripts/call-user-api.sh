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

# Calls the self-service endpoints of user-api with the ID token of a user who has just signed in:
#   1. POST /api/v1/password/reset is gone (404): it set a password hash for whoever the token named;
#   2. POST /api/v1/self/password-change refuses a request without a token (401);
#   3. it refuses the token of a device signed in less than devices.trustedAfter ago or of no device (403),
#      and no UserOperation of the user appears.
# The suite does not change devices.trustedAfter; with 0s every device is trusted, and step 3 is skipped.
set -Eeuo pipefail
source ../../tests/common/scripts/lib.sh

dir="$(state_dir user-api-password-change)"
ip="$(kubectl -n d8-user-authn get service user-api -o jsonpath='{.spec.clusterIP}')"
[[ -n "${ip}" ]] || die "Service d8-user-authn/user-api has no ClusterIP"
[[ "${ip}" == *:* ]] && ip="[${ip}]"
auth_header="$(secret_file "Authorization: Bearer $(jq -r .id_token "${dir}/e2e-authn-api-user.tokens.json")")"
body="${dir}/response.json"

call() {
  local path="$1"
  shift
  curl -sS -k --max-time 30 -o "${body}" -w '%{http_code}' -X POST -H 'Content-Type: application/json' "$@" "https://${ip}${path}"
}

code="$(call /api/v1/password/reset -H "@${auth_header}" -d '{"newPasswordHash":"$2a$10$abc"}')"
[[ "${code}" == "404" ]] || die "the removed POST /api/v1/password/reset must get HTTP 404, got ${code}: $(head -c 200 "${body}")"
log "POST /api/v1/password/reset: HTTP 404"

code="$(call /api/v1/self/password-change)"
[[ "${code}" == "401" ]] && jq -e '.error == "unauthorized"' "${body}" >/dev/null \
  || die "a request without a token must get HTTP 401 unauthorized, got ${code}: $(head -c 200 "${body}")"
log "Without a token: HTTP 401 $(jq -c . "${body}")"

trusted_after="$(kubectl get moduleconfig user-authn --ignore-not-found -o json \
  | jq -r '.spec.settings.devices.trustedAfter // "24h"')"
if [[ "${trusted_after}" =~ ^(0h)?(0m)?(0s)?$ ]]; then
  log "devices.trustedAfter is ${trusted_after}: every device is trusted, the refusal of a new device is not checked"
  exit 0
fi

# The token names its device when Dex keeps a browser session (sid) or issued a refresh token (d8_gid);
# then the refusal is the age of the device (too_early), otherwise the token itself (forbidden).
code="$(call /api/v1/self/password-change -H "@${auth_header}")"
[[ "${code}" == "403" ]] && jq -e '.error == "too_early" or .message == "The token does not name its device"' "${body}" >/dev/null \
  || die "a device signed in less than ${trusted_after} ago must get HTTP 403, got ${code}: $(head -c 200 "${body}")"
log "A device signed in less than ${trusted_after} ago: HTTP 403 $(jq -c . "${body}")"

ops="$(kubectl get useroperations.deckhouse.io -o json \
  | jq -r '[.items[] | select(.spec.user == "e2e-authn-api-user") | .metadata.name] | join(" ")')"
[[ -z "${ops}" ]] || die "a refused request must create no UserOperation, found: ${ops}"
log "No UserOperation of e2e-authn-api-user was created"
