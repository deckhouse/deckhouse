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

# Signs a local user in for the audience `kubernetes` and calls `kubectl auth whoami` with the
# ID token only. When the suite has just enabled user-authn, control-plane-manager still has to
# reconfigure kube-apiserver with the OIDC issuer, so the call is retried for up to 10 minutes,
# each time with a freshly issued token (an ID token lives for idTokenTTL, 10 minutes by default).
# On success the kubeconfig is left in <state dir>/<USER_NAME>.kubeconfig and the answer of
# whoami in <state dir>/<USER_NAME>.whoami.json.
#
# Usage: whoami-as.sh <TEST_NAME> <NAMESPACE> <DEXCLIENT_NAME> <USER_NAME> <EMAIL>
set -Eeuo pipefail
script_dir="$(dirname "${BASH_SOURCE[0]}")"
source "${script_dir}/lib.sh"

test_name="$1" ns="$2" client="$3" user="$4" email="$5"
dir="$(state_dir "${test_name}")"
kubeconfig="${dir}/${user}.kubeconfig"

deadline=$(( $(date +%s) + 600 ))
while :; do
  bash "${script_dir}/get-token.sh" "${test_name}" "${ns}" "${client}" "${user}" "${email}" kubernetes
  token_kubeconfig "$(jq -r .id_token "${dir}/${user}.tokens.json")" "${kubeconfig}"
  if out="$(kubectl --kubeconfig "${kubeconfig}" auth whoami -o json 2>&1)"; then
    printf '%s' "${out}" > "${dir}/${user}.whoami.json"
    log "API server identity: $(jq -c .status.userInfo <<<"${out}")"
    exit 0
  fi
  (( $(date +%s) < deadline )) || die "the API server does not accept the Dex ID token: ${out}"
  echo "API server does not accept the token yet: ${out}"
  sleep 15
done
