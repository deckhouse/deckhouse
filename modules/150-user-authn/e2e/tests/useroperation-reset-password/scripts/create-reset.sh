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

# Creates a UserOperation of type ResetPassword with the bcrypt hash of a new random password.
# Usage: create-reset.sh <OPERATION_NAME> <USER_NAME>
# The new password is kept in <state dir>/<OPERATION_NAME>.password.
set -Eeuo pipefail
source ../../tests/common/scripts/lib.sh

op="$1" user="$2"
dir="$(state_dir useroperation-reset-password)"
password="$(gen_password)"
printf '%s' "${password}" > "${dir}/${op}.password"
hash="$(hash_password "${password}")"
[[ -n "${hash}" ]] || die "the password hash is empty"
# The hash goes to jq through stdin, not through argv.
spec="$(printf '%s' "${hash}" | jq -Rsc --arg user "${user}" \
  '{user: $user, type: "ResetPassword", initiatorType: "admin", resetPassword: {newPasswordHash: .}}')"
create_user_operation "${op}" "${spec}"
