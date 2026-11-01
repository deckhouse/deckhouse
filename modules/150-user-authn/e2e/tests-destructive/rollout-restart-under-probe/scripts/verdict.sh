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

# Summarizes the probe log and fails when a request did not get HTTP 200.
#
# In HA mode every failed request fails the case.
#
# In single-replica mode one more disruption is possible that the rollout restarts do not cause:
# the VerticalPodAutoscaler of a component (updateMode InPlaceOrRecreate) may evict its only pod
# to apply new resources, and the component is unavailable until the replacement is ready.
# Run 4 showed exactly that (events EvictedByVPA of user-api and the authenticator 5 seconds
# before the failures). In this mode a failed request is attributed to such an eviction when an
# EvictedByVPA event of a pod of the same component happened between 60 seconds before and
# 5 seconds after it; these requests are reported but do not fail the case. Every other failed
# request still fails it.
set -Eeuo pipefail
source ../../tests/common/scripts/lib.sh

dir="$(state_dir rollout-restart-under-probe)"
touch "${dir}/probe.stop"
sleep 3
log_file="${dir}/probe.log"
[[ -s "${log_file}" ]] || die "the probe wrote nothing"
mode="$(cat "${dir}/mode" 2>/dev/null || echo ha)"

# evictions <NAMESPACE> <POD_NAME_PREFIX> prints the Unix times of EvictedByVPA events of the pods.
evictions() {
  kubectl -n "$1" get events --field-selector reason=EvictedByVPA -o json 2>/dev/null \
    | jq -r --arg p "$2" '.items[]
        | select(.involvedObject.kind == "Pod" and (.involvedObject.name | startswith($p)))
        | (.lastTimestamp // .eventTime // .firstTimestamp)' \
    | while read -r ts; do [[ -n "${ts}" && "${ts}" != "null" ]] && date -d "${ts}" +%s; done || true
}

declare -A prefix=([dex]="d8-user-authn dex-" [authenticator]="${NS} e2e-authn-ha-dex-authenticator-" [user-api]="d8-user-authn user-api-")

failed=0
for target in dex authenticator user-api; do
  total="$(awk -v t="${target}" '$2 == t' "${log_file}" | wc -l)"
  bad_file="${dir}/${target}.bad"
  awk -v t="${target}" '$2 == t && $3 != "200" {print $1, $3}' "${log_file}" > "${bad_file}"
  echo "${target}: ${total} requests, $(wc -l < "${bad_file}") not HTTP 200"
  (( total > 100 )) || { echo "  too few requests, the probe did not run through the restarts"; failed=1; }
  [[ -s "${bad_file}" ]] || continue

  evicted=""
  if [[ "${mode}" == "single" ]]; then
    # shellcheck disable=SC2086
    evicted="$(evictions ${prefix[${target}]} | tr '\n' ' ')"
    [[ -n "${evicted}" ]] && echo "  EvictedByVPA events of ${target} pods at: ${evicted}"
  fi
  unexplained="$(awk -v ev="${evicted}" 'BEGIN { n = split(ev, e, " ") }
      { t = int($1); ok = 0
        for (i = 1; i <= n; i++) if (t >= e[i] - 5 && t <= e[i] + 60) ok = 1
        if (!ok) print "  " $1 " HTTP " $2 }' "${bad_file}")"
  excused=$(( $(wc -l < "${bad_file}") - $(grep -c . <<<"${unexplained}" || true) ))
  if (( excused > 0 )); then
    echo "  ${excused} failed requests follow a VPA eviction of the only ${target} pod (single-replica mode), not a restart"
  fi
  if [[ -n "${unexplained}" ]]; then
    failed=1
    echo "  failed requests not explained by a VPA eviction:"
    head -n 20 <<<"${unexplained}"
  fi
done
(( failed == 0 )) || die "requests failed during the rollout restarts (HTTP 000 is a connection error or a timeout)"
log "No request failed because of the rollout restarts"
