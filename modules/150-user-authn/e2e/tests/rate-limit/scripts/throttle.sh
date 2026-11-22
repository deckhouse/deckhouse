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

# Posts wrong credentials of a user that does not exist until the per-IP rate limit of Dex
# answers HTTP 429 "Too many requests" with a Retry-After header. The requests carry a
# random X-Real-IP from TEST-NET-2, the header Dex reads the client address from, so other
# clients are not throttled if Dex trusts it. With several Dex replicas each keeps its own
# buckets, hence the generous number of attempts.
set -Eeuo pipefail
source ../../tests/common/scripts/lib.sh

dex_discover
client_id="$(dex_client_id "${NS}" e2e-authn-rate-client)"
headers="$(state_dir rate-limit)/headers"
ip="198.51.100.$(( RANDOM % 250 + 1 ))"

for attempt in $(seq 1 30); do
  dex_authorize "e2e-authn-nobody@example.com" "wrong-$(gen_password)" "${client_id}" "${TEST_REDIRECT_URI}" "${TEST_SCOPE}" \
    -H "X-Real-IP: ${ip}" -D "${headers}"
  [[ -z "${AUTH_CODE}" ]] || die "wrong credentials returned an authorization code"
  if [[ "${AUTH_STATUS}" == "429" ]]; then
    grep -q "Too many requests" "${AUTH_BODY}" || die "HTTP 429 without 'Too many requests': $(head -c 200 "${AUTH_BODY}")"
    grep -qi '^Retry-After: [0-9]' "${headers}" || die "HTTP 429 without a Retry-After header"
    log "Attempt ${attempt}: HTTP 429 Too many requests, $(grep -i '^Retry-After' "${headers}" | tr -d '\r')"
    exit 0
  fi
  [[ "${AUTH_STATUS}" == "401" ]] || die "attempt ${attempt}: expected HTTP 401 or 429: $(auth_describe)"
  log "Attempt ${attempt}: HTTP 401"
done
die "30 failed sign-ins from one address were never throttled with HTTP 429"
