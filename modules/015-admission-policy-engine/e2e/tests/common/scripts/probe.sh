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

# Sends one workload to the API server with `kubectl create --dry-run=server` and checks how
# admission answers. Nothing is persisted, so a probe leaves no garbage behind.
#
# Usage:
#   probe.sh --expect <deny|allow|warn|webhook-error> --namespace <NS> --name <NAME>
#            [--kind pod|deployment] [--patch <JQ_FILTER>] [--match <ERE>]...
#            [--timeout <SECONDS>] [--as-deckhouse]
#
# The workload is the Restricted-compliant Pod of lib.sh, changed by the jq filter in --patch and,
# for --kind deployment, wrapped into a Deployment.
#
# Expectations:
#   deny           the request is rejected and the output matches every --match;
#   allow          the request is admitted;
#   warn           the request is admitted with a `Warning:` and the output matches every --match;
#   webhook-error  the request is rejected because the API server could not call a webhook.
#
# Admission becomes consistent only after Deckhouse renders the constraints and Gatekeeper loads
# them, so the probe is repeated every 5 seconds until it holds or --timeout (default 300) expires.
# --timeout 0 makes a single attempt.
set -Eeuo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

expect="" namespace="" name="" kind="pod" patch="." timeout=300
as_args=()
matches=()
while (( $# > 0 )); do
  case "$1" in
    --expect) expect="$2"; shift 2 ;;
    --namespace) namespace="$2"; shift 2 ;;
    --name) name="$2"; shift 2 ;;
    --kind) kind="$2"; shift 2 ;;
    --patch) patch="$2"; shift 2 ;;
    --match) matches+=("$2"); shift 2 ;;
    --timeout) timeout="$2"; shift 2 ;;
    --as-deckhouse) as_args=("${AS_DECKHOUSE[@]}"); shift ;;
    *) fail "unknown argument $1" ;;
  esac
done
[[ -n "${expect}" && -n "${namespace}" && -n "${name}" ]] || fail "--expect, --namespace and --name are required"

object="$(compliant_pod "${name}" "${namespace}" | jq "${patch}")"
if [[ "${kind}" == "deployment" ]]; then
  object="$(as_deployment <<<"${object}")"
fi

matches_all() {
  local m
  for m in "${matches[@]+"${matches[@]}"}"; do
    grep -Eq -- "${m}" <<<"$1" || return 1
  done
}

holds() {
  local rc="$1" out="$2"
  case "${expect}" in
    deny) (( rc != 0 )) && ! grep -q "failed calling webhook" <<<"${out}" && matches_all "${out}" ;;
    allow) (( rc == 0 )) ;;
    warn) (( rc == 0 )) && grep -q "Warning:" <<<"${out}" && matches_all "${out}" ;;
    webhook-error) (( rc != 0 )) && grep -q "failed calling webhook" <<<"${out}" && matches_all "${out}" ;;
    *) fail "unknown expectation ${expect}" ;;
  esac
}

deadline=$(( $(date +%s) + timeout ))
attempt=0
while true; do
  attempt=$(( attempt + 1 ))
  rc=0
  out="$(kubectl create --dry-run=server -o name "${as_args[@]+"${as_args[@]}"}" -f - <<<"${object}" 2>&1)" || rc=$?
  if holds "${rc}" "${out}"; then
    log "OK ${kind} ${namespace}/${name}: expected ${expect}, attempt ${attempt}"
    echo "${out}"
    exit 0
  fi
  if (( $(date +%s) >= deadline )); then
    >&2 echo "Object sent:"
    >&2 echo "${object}"
    fail "${kind} ${namespace}/${name}: expected ${expect} matching [${matches[*]-}], got exit code ${rc} after ${attempt} attempts: ${out}"
  fi
  sleep 5
done
