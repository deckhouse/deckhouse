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

# Calls the API server with the ID token of e2e-authn-login-user only. The API server must map
# the email claim to the user name and the groups claim to the groups.
set -Eeuo pipefail
source ../../tests/common/scripts/lib.sh

bash ../../tests/common/scripts/whoami-as.sh static-user-login "${NS}" e2e-authn-login-client \
  e2e-authn-login-user e2e-authn-login-user@example.com
out="$(cat "$(state_dir static-user-login)/e2e-authn-login-user.whoami.json")"
jq -e '.status.userInfo.username == "e2e-authn-login-user@example.com"' <<<"${out}" >/dev/null \
  || die "whoami user name is $(jq -c .status.userInfo.username <<<"${out}")"
jq -e '.status.userInfo.groups | (index("e2e-authn-login-group") != null) and (index("system:authenticated") != null)' <<<"${out}" >/dev/null \
  || die "whoami groups are $(jq -c .status.userInfo.groups <<<"${out}")"
log "The API server maps the email to the user name and the groups claim to the groups"
