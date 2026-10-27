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

# Posts wrong passwords for e2e-authn-lock-user until Dex locks the account. With
# passwordPolicy.lockout.maxAttempts=3, the first two attempts must return the login form
# (HTTP 401) and the third must lock the account (HTTP 429 "Account temporarily locked").
set -Eeuo pipefail
source ../../tests/common/scripts/lib.sh

dex_discover
client_id="$(dex_client_id "${NS}" e2e-authn-lock-client)"
max_attempts=3

for attempt in $(seq 1 "${max_attempts}"); do
  dex_authorize e2e-authn-lock-user@example.com "wrong-$(gen_password)" "${client_id}" "${TEST_REDIRECT_URI}" "${TEST_SCOPE}"
  [[ -z "${AUTH_CODE}" ]] || die "a wrong password returned an authorization code"
  if (( attempt < max_attempts )); then
    [[ "${AUTH_STATUS}" == "401" ]] || die "wrong attempt ${attempt} of ${max_attempts} must return HTTP 401: $(auth_describe)"
    log "Wrong attempt ${attempt}: HTTP 401"
  else
    [[ "${AUTH_STATUS}" == "429" ]] && grep -q "Account temporarily locked" "${AUTH_BODY}" \
      || die "wrong attempt ${attempt} must lock the account with HTTP 429: $(auth_describe)"
    log "Wrong attempt ${attempt}: HTTP 429, the account is locked"
  fi
done
