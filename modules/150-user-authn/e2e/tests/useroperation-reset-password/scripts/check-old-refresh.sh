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

# The refresh token issued before the password reset must be gone: the reset deletes the
# RefreshToken and OfflineSessions objects of the user.
set -Eeuo pipefail
source ../../tests/common/scripts/lib.sh

dex_discover
dir="$(state_dir useroperation-reset-password)"
refresh_token="$(jq -r '.refresh_token // empty' "${dir}/e2e-authn-reset-user.tokens.json")"
[[ -n "${refresh_token}" ]] || die "the sign-in with offline_access returned no refresh token"
dex_refresh "${refresh_token}" "$(dex_client_id "${NS}" e2e-authn-reset-client)" "$(dex_client_secret "${NS}" e2e-authn-reset-client)"
[[ "${TOKEN_STATUS}" == "400" ]] \
  || die "refresh with a token issued before the reset must fail with HTTP 400, got ${TOKEN_STATUS}: $(jq -c '{error, error_description}' "${TOKEN_BODY}" 2>/dev/null || echo 'not JSON')"
log "The old refresh token is rejected: HTTP 400 $(jq -c '{error}' "${TOKEN_BODY}" 2>/dev/null || true)"
