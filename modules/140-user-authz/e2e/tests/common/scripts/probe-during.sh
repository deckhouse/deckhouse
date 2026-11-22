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

# Runs an action while a probe keeps asking kube-apiserver for authorization
# decisions, then reports every decision that differed from the expectation.
#
# Usage: probe-during.sh PROBE_FILE SETTLE_SECONDS MAX_FALSE_DENIES MAX_ERRORS -- COMMAND...
#
# PROBE_FILE has the matrix format of sar-check.sh. The probe evaluates its
# lines in a loop; every review carries a unique resource name, so that the
# authorizer cache of kube-apiserver (authorizedTTL 5m, unauthorizedTTL 30s)
# cannot hide an outage of the webhook. COMMAND runs in the foreground; the
# probe keeps going for SETTLE_SECONDS after it returns.
#
# Outcomes per review (decisions as classified by sar_decide in lib.sh):
#   error       - the review could not be created, or it carries
#                 status.evaluationError. An unreachable authorization webhook
#                 lands here: kube-apiserver denies by failurePolicy Deny and
#                 reports the connection error as evaluationError. Counted
#                 against MAX_ERRORS (the 4th argument);
#   false deny  - EXPECT is yes and the webhook or RBAC decided "not allowed"
#                 without an evaluation error. Counted against MAX_FALSE_DENIES
#                 (the 3rd argument);
#   false allow - EXPECT is no or deny, the decision is yes (always a failure).
# The report gives the counts and the length of the false deny window and of
# the error window (first to last affected review), which is the outage of the
# webhook as seen by kube-apiserver. The script fails when COMMAND fails, when
# there is any false allow, or when the false denies or errors exceed the given
# maximums.
#
# The probe stops on its own when the script that started it is gone (for
# example, killed by a Chainsaw timeout) or after PROBE_MAX_SECONDS (default 1800).
set -Eeuo pipefail

# shellcheck source=lib.sh
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

probe_file="${1:?probe file is required}"
settle="${2:?settle seconds are required}"
max_false_denies="${3:?max false denies is required}"
max_errors="${4:?max errors is required}"
shift 4
[[ "${1:-}" == "--" ]] && shift
(( $# > 0 )) || { >&2 echo "a command is required after --"; exit 1; }

work_dir="$(mktemp -d "${TMPDIR:-/tmp}/e2e-authz-probe.XXXXXX")"
results="${work_dir}/results"
stop_file="${work_dir}/stop"
: > "${results}"

parent_pid=$$
probe_deadline=$(( $(date +%s) + ${PROBE_MAX_SECONDS:-1800} ))

probe_loop() {
  local counter=0 user verb group resource sub ns expect result decision
  while [[ ! -e "${stop_file}" ]] && kill -0 "${parent_pid}" 2>/dev/null \
      && (( $(date +%s) < probe_deadline )); do
    while read -r user verb group resource sub ns expect _; do
      [[ -z "${user}" || "${user}" == \#* ]] && continue
      counter=$(( counter + 1 ))
      result="$(sar_decide "${user}" "${verb}" "$(field "${group}")" "$(field "${resource}")" \
        "$(field "${sub}")" "$(field "${ns}")" "e2e-authz-probe-${counter}-$(date +%s%N)")"
      decision="${result%% *}"
      printf '%s\t%s\t%s\t%s\t%s\t%s\n' "$(date +%s.%N)" "${user}" "$(field "${ns}")" \
        "${expect}" "${decision}" "${result#* }" >> "${results}"
    done < "${probe_file}"
    sleep 0.2
  done
}

# The probe writes to a file only, so that it never holds the stdout of the script open.
probe_loop </dev/null >"${work_dir}/probe.log" 2>&1 &
probe_pid=$!
trap 'touch "${stop_file}"; wait "${probe_pid}" 2>/dev/null || true' EXIT

sleep 3
log "Probe started (pid ${probe_pid}), running: $*"
action_rc=0
"$@" || action_rc=$?
log "Action finished with exit code ${action_rc}, probing for another ${settle}s"
sleep "${settle}"
touch "${stop_file}"
wait "${probe_pid}" 2>/dev/null || true
trap - EXIT

total="$(( $(wc -l < "${results}") ))"
summary="$(awk -F'\t' '
  {
    expect = $4; decision = $5
    if (decision == "error") { errors++; if (!fe) fe = $1; le = $1; next }
    if (expect == "yes" && decision != "yes") { fd++; if (!ffd) ffd = $1; lfd = $1; next }
    if (expect != "yes" && decision == "yes") { fa++; next }
  }
  END {
    printf "false_denies=%d false_allows=%d errors=%d", fd, fa, errors
    if (fd) printf " false_deny_window=%.1fs", lfd - ffd
    if (errors) printf " error_window=%.1fs", le - fe
    printf "\n"
  }' "${results}")"

log "Probe made ${total} reviews: ${summary}"

# utc EPOCH_WITH_FRACTION: the time of a review as an ISO 8601 UTC timestamp.
utc() {
  date -u -d "@${1}" +%Y-%m-%dT%H:%M:%S.%3NZ 2>/dev/null || echo "${1}"
}
# First and last affected review of each kind, to line up with the webhook Pod events.
first_last() {
  local filter="$1" what="$2" first last
  first="$(awk -F'\t' "${filter}"' { print $1; exit }' "${results}")"
  last="$(awk -F'\t' "${filter}"' { t = $1 } END { if (t != "") print t }' "${results}")"
  [[ -n "${first}" ]] && log "${what}: first at $(utc "${first}"), last at $(utc "${last}")"
  return 0
}
first_last '$5 == "error"' "Errors"
first_last '$4 == "yes" && $5 != "yes" && $5 != "error"' "False denies"
first_last '$4 != "yes" && $5 == "yes"' "False allows"
if awk -F'\t' '($4 == "yes" && $5 != "yes") || ($4 != "yes" && $5 == "yes") || $5 == "error"' "${results}" | grep -q .; then
  echo "Unexpected decisions (time, user, namespace, expected, got, reason):"
  awk -F'\t' '($4 == "yes" && $5 != "yes") || ($4 != "yes" && $5 == "yes") || $5 == "error"' "${results}" | head -n 50
fi

false_denies="$(sed -E 's/.*false_denies=([0-9]+).*/\1/' <<<"${summary}")"
false_allows="$(sed -E 's/.*false_allows=([0-9]+).*/\1/' <<<"${summary}")"
errors="$(sed -E 's/.*errors=([0-9]+).*/\1/' <<<"${summary}")"
rm -rf "${work_dir}"

rc=0
(( total > 0 )) || { >&2 log "The probe made no reviews"; rc=1; }
(( action_rc == 0 )) || { >&2 log "The action failed"; rc=1; }
(( false_allows == 0 )) || { >&2 log "${false_allows} reviews were allowed that must not be"; rc=1; }
(( false_denies <= max_false_denies )) || { >&2 log "${false_denies} false denies, at most ${max_false_denies} allowed"; rc=1; }
(( errors <= max_errors )) || { >&2 log "${errors} failed reviews, at most ${max_errors} allowed"; rc=1; }
exit "${rc}"
