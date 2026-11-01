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

# Signs a local user in with the password form and checks the outcome.
#
# Usage: login-expect.sh <TEST_NAME> <NAMESPACE> <DEXCLIENT_NAME> <USER_NAME> <EMAIL> <EXPECT> [PASSWORD_FILE]
#
# EXPECT:
#   code     Dex issues an authorization code (retried while Dex restarts);
#   locked   Dex answers 429 with "Account temporarily locked" even for the right password;
#   invalid  Dex answers 401 with the login form and no code.
# The password is the one create_user stored, or the content of PASSWORD_FILE.
set -Eeuo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

test_name="$1" ns="$2" client="$3" user="$4" email="$5" expect="$6" password_file="${7:-}"

dex_discover
client_id="$(dex_client_id "${ns}" "${client}")"
if [[ -n "${password_file}" ]]; then
  password="$(cat "${password_file}")"
else
  password="$(user_password "${test_name}" "${user}")"
fi

case "${expect}" in
  code)
    dex_login_code "${email}" "${password}" "${client_id}" "${TEST_REDIRECT_URI}" "${TEST_SCOPE}" \
      || die "user ${user} must be able to sign in"
    log "User ${user} signed in and got an authorization code"
    ;;
  locked)
    deadline=$(( $(date +%s) + 60 ))
    while :; do
      dex_authorize "${email}" "${password}" "${client_id}" "${TEST_REDIRECT_URI}" "${TEST_SCOPE}"
      if [[ "${AUTH_STATUS}" == "429" && -z "${AUTH_CODE}" ]] && grep -q "Account temporarily locked" "${AUTH_BODY}"; then
        break
      fi
      (( $(date +%s) < deadline )) || die "a locked user ${user} must get HTTP 429 'Account temporarily locked': $(auth_describe)"
      sleep 5
    done
    log "User ${user} is locked: HTTP 429 'Account temporarily locked' for the right password"
    ;;
  invalid)
    dex_authorize "${email}" "${password}" "${client_id}" "${TEST_REDIRECT_URI}" "${TEST_SCOPE}"
    [[ "${AUTH_STATUS}" == "401" && -z "${AUTH_CODE}" ]] \
      || die "user ${user} must get the login form back with HTTP 401: $(auth_describe)"
    log "User ${user} was refused with HTTP 401"
    ;;
  *)
    die "unknown expectation ${expect}"
    ;;
esac
