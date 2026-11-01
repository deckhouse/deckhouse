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

# Checks the UserOperation that user-api created, that Dex refuses the old password and that the
# new one leads to the forced password change page.
set -Eeuo pipefail
source ../../tests/common/scripts/lib.sh

dir="$(state_dir user-api-password-reset)"
op="$(cat "${dir}/operation")"
wait_user_operation "${op}" Succeeded
kubectl get "useroperations.deckhouse.io/${op}" -o json \
  | jq -e '.spec.user == "e2e-authn-api-user" and .spec.type == "ResetPassword" and .spec.initiatorType == "self" and .spec.resetPassword == null' >/dev/null \
  || die "UserOperation ${op} has an unexpected spec: $(kubectl get "useroperations.deckhouse.io/${op}" -o jsonpath='{.spec}')"
log "UserOperation ${op}: ResetPassword of e2e-authn-api-user initiated by self, hash wiped, Succeeded"

dex_discover
client_id="$(dex_client_id "${NS}" e2e-authn-api-client)"
dex_authorize e2e-authn-api-user@example.com "$(user_password user-api-password-reset e2e-authn-api-user)" \
  "${client_id}" "${TEST_REDIRECT_URI}" "${TEST_SCOPE}"
[[ "${AUTH_STATUS}" == "401" ]] || die "the old password must get HTTP 401: $(auth_describe)"
dex_authorize e2e-authn-api-user@example.com "$(cat "${dir}/new.password")" \
  "${client_id}" "${TEST_REDIRECT_URI}" "${TEST_SCOPE}"
# Every ResetPassword sets requireResetHashOnNextSuccLogin (useroperation/execute.go), so the
# accepted new password leads to the forced password change page.
[[ "${AUTH_STATUS}" == "303" && "${AUTH_LOCATION}" == *"/password/change?"* ]] \
  || die "Dex must accept the new password and redirect to the forced password change page: $(auth_describe)"
log "Dex refuses the old password, accepts the new one and asks to change it"
