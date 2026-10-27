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

# Signs a local user in through the authorization code flow of Dex and stores the token
# response in <state dir>/<USER_NAME>.tokens.json.
#
# Usage: get-token.sh <TEST_NAME> <NAMESPACE> <DEXCLIENT_NAME> <USER_NAME> <EMAIL> [kubernetes]
#
# With `kubernetes`, the ID token is requested for the audience of the `kubernetes` client
# (cross-client scope), so that the API server accepts it.
set -Eeuo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

test_name="$1" ns="$2" client="$3" user="$4" email="$5" audience="${6:-}"

dex_discover
client_id="$(dex_client_id "${ns}" "${client}")"
client_secret="$(dex_client_secret "${ns}" "${client}")"
scope="${TEST_SCOPE}"
[[ "${audience}" == "kubernetes" ]] && scope+=" ${KUBERNETES_AUDIENCE_SCOPE}"

dex_login_code "${email}" "$(user_password "${test_name}" "${user}")" "${client_id}" "${TEST_REDIRECT_URI}" "${scope}" \
  || die "user ${user} could not sign in"
dex_exchange_code "${AUTH_CODE}" "${client_id}" "${client_secret}" || die "the authorization code of ${user} was not exchanged"
cp "${TOKEN_BODY}" "$(state_dir "${test_name}")/${user}.tokens.json"
log "User ${user} signed in through client ${client_id}"
