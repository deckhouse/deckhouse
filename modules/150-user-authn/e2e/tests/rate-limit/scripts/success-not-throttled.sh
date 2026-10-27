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

# Signs e2e-authn-rate-user in successfully more times in a row than the burst of the per-IP
# limit (2) times the possible number of Dex replicas, from one client address and without
# retries. The limiter reserves a token before the handler and refunds it for a response
# below 400, so none of the sign-ins may be throttled.
set -Eeuo pipefail
source ../../tests/common/scripts/lib.sh

dex_discover
client_id="$(dex_client_id "${NS}" e2e-authn-rate-client)"
password="$(user_password rate-limit e2e-authn-rate-user)"
ip="203.0.113.$(( RANDOM % 250 + 1 ))"

for attempt in $(seq 1 7); do
  dex_authorize e2e-authn-rate-user@example.com "${password}" "${client_id}" "${TEST_REDIRECT_URI}" "${TEST_SCOPE}" \
    -H "X-Real-IP: ${ip}"
  [[ -n "${AUTH_CODE}" ]] || die "successful sign-in ${attempt} was not completed: $(auth_describe)"
  log "Successful sign-in ${attempt}: authorization code issued"
done
