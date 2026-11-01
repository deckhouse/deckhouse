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

# Requests the endpoints of the module every 200 ms until <state dir>/probe.stop appears and
# writes one line per request to <state dir>/probe.log: "<unix time> <target> <HTTP code>".
# HTTP code 000 means a connection error or a timeout (2 seconds). The probe also stops after
# 30 minutes, or when the state directory is removed, so it never outlives the suite.
#   dex            GET /.well-known/openid-configuration of the dex Service;
#   authenticator  GET /ping of the Service of DexAuthenticator e2e-authn-ha;
#   user-api       GET /healthz of the user-api Service.
# Chainsaw deletes the temporary kubeconfig of the script that starts the probe when that script
# ends, so the probe copies it first, resolves the addresses with the copy and deletes the copy:
# the loop itself does not call the API.
set -Eeuo pipefail
source ../../tests/common/scripts/lib.sh

dir="$(state_dir rollout-restart-under-probe)"
log_file="${dir}/probe.log"
stop_file="${dir}/probe.stop"
rm -f "${stop_file}" "${log_file}"
echo $$ > "${dir}/probe.pid"

kubeconfig="${dir}/probe.kubeconfig"
(umask 077 && cp "${KUBECONFIG:?KUBECONFIG is required}" "${kubeconfig}")
export KUBECONFIG="${kubeconfig}"
trap 'rm -f "${kubeconfig}"' EXIT

dex_discover
svc_ip() {
  local ip
  ip="$(kubectl -n "$1" get service "$2" -o jsonpath='{.spec.clusterIP}')"
  [[ "${ip}" == *:* ]] && ip="[${ip}]"
  printf '%s' "${ip}"
}
auth_ip="$(svc_ip "${NS}" "$(kubectl -n "${NS}" get service -l deckhouse.io/dex-authenticator-for=e2e-authn-ha -o jsonpath='{.items[0].metadata.name}')")"
api_ip="$(svc_ip d8-user-authn user-api)"
[[ -n "${auth_ip}" && -n "${api_ip}" ]] || die "the Service addresses are empty: authenticator '${auth_ip}', user-api '${api_ip}'"
rm -f "${kubeconfig}"
: > "${log_file}"

probe() {
  local target="$1"
  shift
  local code
  code="$(curl -sk --max-time 2 -o /dev/null -w '%{http_code}' "$@" 2>/dev/null || true)"
  echo "$(date +%s.%N) ${target} ${code:-000}" >> "${log_file}"
}

deadline=$(( $(date +%s) + 1800 ))
while [[ ! -e "${stop_file}" && -d "${dir}" ]] && (( $(date +%s) < deadline )); do
  probe dex --resolve "${DEX_RESOLVE}" "${DEX_ISSUER}/.well-known/openid-configuration" &
  probe authenticator "https://${auth_ip}/ping" &
  probe user-api "https://${api_ip}/healthz" &
  sleep 0.2
done
wait
