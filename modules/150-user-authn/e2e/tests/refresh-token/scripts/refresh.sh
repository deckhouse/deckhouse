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

# Refreshes the tokens of e2e-authn-refresh-user and checks the outcome.
# Usage: refresh.sh ok|rotated|revoked
#   ok       HTTP 200 with a new ID token; within expiry.refreshTokens.reuseInterval (2m) of the last
#            use Dex may return the same refresh token. The returned token is kept for the next call;
#   rotated  as ok, and the refresh token must be a new one (the last use is older than reuseInterval);
#   revoked  HTTP 400: the refresh token no longer exists (an administrator Lock ends the sessions).
set -Eeuo pipefail
source ../../tests/common/scripts/lib.sh

expect="${1:?ok, rotated or revoked}"
dex_discover
tokens="$(state_dir refresh-token)/e2e-authn-refresh-user.tokens.json"
old_refresh="$(jq -r '.refresh_token // empty' "${tokens}")"
[[ -n "${old_refresh}" ]] || die "there is no refresh token to use"
old_id="$(jq -r '.id_token // empty' "${tokens}")"
[[ -n "${old_id}" ]] || die "there is no previous ID token to compare with"
dex_refresh "${old_refresh}" "$(dex_client_id "${NS}" e2e-authn-refresh-client)" "$(dex_client_secret "${NS}" e2e-authn-refresh-client)"

case "${expect}" in
  ok|rotated)
    [[ "${TOKEN_STATUS}" == "200" ]] || die "refresh must succeed, got HTTP ${TOKEN_STATUS}: $(jq -c '{error, error_description}' "${TOKEN_BODY}" 2>/dev/null || echo 'not JSON')"
    new_refresh="$(jq -r '.refresh_token // empty' "${TOKEN_BODY}")"
    [[ -n "${new_refresh}" ]] || die "refresh must return a refresh token"
    if [[ "${expect}" == "rotated" && "${new_refresh}" == "${old_refresh}" ]]; then
      die "a refresh after reuseInterval must rotate the refresh token"
    fi
    new_id="$(jq -r '.id_token // empty' "${TOKEN_BODY}")"
    [[ -n "${new_id}" ]] || die "refresh must return an ID token"
    # The tokens are compared, never printed.
    [[ "${new_id}" != "${old_id}" ]] || die "refresh must return a new ID token, it returned the previous one"
    claims="$(jwt_claims "${new_id}")"
    jq -e '.email == "e2e-authn-refresh-user@example.com"' <<<"${claims}" >/dev/null \
      || die "the refreshed ID token has the wrong email"
    cp "${TOKEN_BODY}" "${tokens}"
    if [[ "${new_refresh}" == "${old_refresh}" ]]; then
      log "Refresh: HTTP 200, new ID token, the same refresh token (within reuseInterval)"
    else
      log "Refresh: HTTP 200, new ID token, rotated refresh token"
    fi
    ;;
  revoked)
    [[ "${TOKEN_STATUS}" == "400" ]] \
      || die "refresh with a revoked token must get HTTP 400, got ${TOKEN_STATUS}: $(jq -c '{error, error_description}' "${TOKEN_BODY}" 2>/dev/null || echo 'not JSON')"
    log "Refresh with a revoked token: HTTP 400 $(jq -c '{error}' "${TOKEN_BODY}" 2>/dev/null || true)"
    ;;
  *)
    die "unknown expectation ${expect}"
    ;;
esac
