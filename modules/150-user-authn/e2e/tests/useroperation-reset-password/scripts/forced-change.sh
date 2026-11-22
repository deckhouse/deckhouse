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

# After an administrator reset (requireResetHashOnNextSuccLogin), signing in with the new
# password redirects to the forced password change page. The user changes the password
# there and signs in with the changed one.
set -Eeuo pipefail
source ../../tests/common/scripts/lib.sh

dex_discover
dir="$(state_dir useroperation-reset-password)"
client_id="$(dex_client_id "${NS}" e2e-authn-reset-client)"
email="e2e-authn-reset-user@example.com"
reset_password="$(cat "${dir}/e2e-authn-reset-op.password")"

deadline=$(( $(date +%s) + 120 ))
while :; do
  dex_authorize "${email}" "${reset_password}" "${client_id}" "${TEST_REDIRECT_URI}" "${TEST_SCOPE}"
  [[ "${AUTH_STATUS}" == "303" && "${AUTH_LOCATION}" == *"/password/change?"* ]] && break
  (( $(date +%s) < deadline )) || die "the new password must lead to the password change page: $(auth_describe)"
  sleep 5
done
[[ -z "${AUTH_CODE}" ]] || die "Dex issued a code before the forced password change"
[[ "$(query_param "${AUTH_LOCATION}" changeReason)" == "forced" ]] \
  || die "changeReason is '$(query_param "${AUTH_LOCATION}" changeReason || true)', expected 'forced'"
log "The new password leads to the forced password change page"

changed_password="$(gen_password)"
printf '%s' "${changed_password}" > "${dir}/changed.password"
out="$(dex_curl -b "${AUTH_JAR}" -c "${AUTH_JAR}" -o "${AUTH_BODY}" -w '%{http_code}' \
  --data-urlencode "currentPassword@$(secret_file "${reset_password}")" --data-urlencode "newPassword@$(secret_file "${changed_password}")" \
  "${AUTH_LOCATION}")"
[[ "${out}" == "303" ]] || die "the password change must answer 303, got ${out}: $(html_text "${AUTH_BODY}" | head -c 300)"
log "The password was changed on the forced change page"
