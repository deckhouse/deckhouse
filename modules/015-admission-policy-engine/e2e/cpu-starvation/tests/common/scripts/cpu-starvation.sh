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

# Steps of the "CPU starvation and throttling" e2e scenarios of the admission-policy-engine module.
#
# Every Chainsaw step calls one subcommand. The steps share state through files in
# STATE_DIR (reports/state-<NAMESPACE> in the scenario directory): the settings written by
# `preflight`, the phase marks, the probe and watch logs, and the snapshots taken before the
# pressure. Background observers run from `start-observers` until `stop-observers`.
#
# The `cleanup` subcommand runs outside Chainsaw (`task cleanup`): it undoes what an interrupted
# run left in the cluster.
#
# The script needs only kubectl, the current kubeconfig context and POSIX tools. It is compatible
# with bash 3.2, the default shell of macOS.

set -euo pipefail

MODULE_NS="d8-admission-policy-engine"
VWC_NAME="d8-admission-policy-engine-config"
# Webhooks of the module: the ValidatingWebhookConfiguration and the MutatingWebhookConfiguration
# d8-admission-policy-engine-config, see templates/validatingwebhookconfiguration.yaml and
# templates/mutatingwebhookconfiguration.yaml of the module. A failed call to any other webhook is
# not an admission failure of the module.
MODULE_WEBHOOKS="admission-policy-engine.deckhouse.io
exec-attach.admission-policy-engine.deckhouse.io
system-namespaces.admission-policy-engine.deckhouse.io
system-namespaces-enforce.admission-policy-engine.deckhouse.io"
WEBHOOK_SELECTOR="app=gatekeeper,control-plane=controller-manager"
AUDIT_SELECTOR="app=gatekeeper,control-plane=audit-controller"
TARGET_CONTAINER="manager"
METRICS_PORT="8888"
# Annotations that let `cleanup` find what the test changed even without the state directory.
VPA_MODE_ANNOTATION="ape-cpu-starvation.e2e.deckhouse.io/update-mode"
RESIZED_ANNOTATION="ape-cpu-starvation.e2e.deckhouse.io/resized"

SCRIPT_PATH="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/$(basename "${BASH_SOURCE[0]}")"
COMMON_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MANIFESTS_DIR="${COMMON_DIR}/manifests"

# init_state sets STATE_DIR for the Chainsaw steps and the background observers.
init_state() {
  : "${NAMESPACE:?NAMESPACE is set by Chainsaw for every script}"
  STATE_DIR="${STATE_DIR:-$(pwd)/reports/state-${NAMESPACE}}"
  export STATE_DIR
  mkdir -p "${STATE_DIR}"
  CONFIG_FILE="${STATE_DIR}/config.env"
}

# ---------------------------------------------------------------------------------------------
# Helpers.
# ---------------------------------------------------------------------------------------------

log() {
  printf '%s %s\n' "$(date -u +%H:%M:%S)" "$*"
}

fail() {
  log "FAIL: $*"
  exit 1
}

now() {
  date +%s
}

# now_ms prints the current time in milliseconds: from EPOCHREALTIME in bash 5, from the perl
# Time::HiRes module in bash 3.2. It prints nothing when neither is available, and the probe
# latency is then recorded as NA instead of a value rounded to whole seconds.
now_ms() {
  if [ -n "${EPOCHREALTIME:-}" ]; then
    local t="${EPOCHREALTIME//[.,]/}"
    echo $((t / 1000))
  elif [ "${HAVE_PERL_HIRES:-}" = "yes" ]; then
    perl -MTime::HiRes=time -e 'printf "%d\n", time() * 1000'
  fi
}

detect_ms_clock() {
  HAVE_PERL_HIRES="no"
  if [ -z "${EPOCHREALTIME:-}" ] && perl -MTime::HiRes=time -e 1 >/dev/null 2>&1; then
    HAVE_PERL_HIRES="yes"
  fi
}

# ms_since <start in ms> prints the milliseconds elapsed since the start, or NA without a clock.
ms_since() {
  local t
  t="$(now_ms)"
  if [ -n "$1" ] && [ -n "${t}" ]; then
    echo $((t - $1))
  else
    echo NA
  fi
}

# fmt_time <epoch> prints the time of day in UTC with BSD date (-r) or GNU date (-d @).
fmt_time() {
  date -u -r "$1" +%H:%M:%S 2>/dev/null || date -u -d "@$1" +%H:%M:%S 2>/dev/null || echo "epoch $1"
}

# to_seconds converts a duration such as 90, 90s, 5m or 1h to seconds.
to_seconds() {
  local d="$1"
  case "${d}" in
    *h) d="$((${d%h} * 3600))" ;;
    *m) d="$((${d%m} * 60))" ;;
    *s) d="${d%s}" ;;
  esac
  case "${d}" in
    '' | *[!0-9]*) fail "cannot parse the duration '$1': use an integer with an optional s, m or h suffix" ;;
  esac
  echo "${d}"
}

# cpu_to_milli converts a Kubernetes CPU quantity (2, 1.5, 250m, 120000u, 5000000n) to millicores.
cpu_to_milli() {
  awk -v q="${1:-}" 'BEGIN {
    if (q == "") { print 0; exit }
    if (q ~ /m$/) { sub(/m$/, "", q); printf "%d\n", q + 0; exit }
    if (q ~ /u$/) { sub(/u$/, "", q); printf "%d\n", q / 1000; exit }
    if (q ~ /n$/) { sub(/n$/, "", q); printf "%d\n", q / 1000000; exit }
    printf "%d\n", q * 1000 + 0.5
  }'
}

# require_int <name> <value> stops the script unless the value is a non-negative integer.
require_int() {
  case "$2" in '' | *[!0-9]*) fail "$1 must be a non-negative integer, got '$2'" ;; esac
}

sum_lines() {
  awk '{ s += $1 } END { printf "%d\n", s }'
}

load_config() {
  [ -f "${CONFIG_FILE}" ] || fail "${CONFIG_FILE} is missing: the preflight step did not run"
  # shellcheck disable=SC1090
  . "${CONFIG_FILE}"
}

phase_mark() {
  printf '%s %s\n' "$(now)" "$1" >>"${STATE_DIR}/phases"
  log "phase: $1"
}

phase_time() {
  [ -f "${STATE_DIR}/phases" ] || return 0
  awk -v p="$1" '$2 == p { t = $1 } END { if (t != "") print t }' "${STATE_DIR}/phases"
}

# mark_ramp_up marks the start of the ramp-up: the first step that changes the target or its node
# after the baseline (the in-place resize or the start of the stress pods). The ramp-up lasts until
# the stress pods run and the pressure mark is set, and it never counts as the baseline.
mark_ramp_up() {
  [ -n "$(phase_time ramp-up)" ] || phase_mark ramp-up
}

target_selector() {
  # shellcheck disable=SC2153 # TARGET comes from config.env.
  case "${TARGET}" in
    controller-manager) echo "${WEBHOOK_SELECTOR}" ;;
    audit) echo "${AUDIT_SELECTOR}" ;;
    *) fail "unknown TARGET '${TARGET}': use controller-manager or audit" ;;
  esac
}

target_deployment() {
  case "${TARGET}" in
    controller-manager) echo "gatekeeper-controller-manager" ;;
    audit) echo "gatekeeper-audit" ;;
  esac
}

# running_pods prints "<pod> <node>" for every running pod that matches the selector.
running_pods() {
  kubectl -n "${MODULE_NS}" get pods -l "$1" --field-selector=status.phase=Running \
    -o jsonpath='{range .items[*]}{.metadata.name}{" "}{.spec.nodeName}{"\n"}{end}'
}

# module_pods_snapshot prints "<pod> <uid> <container>=<restarts>,..." for every running module pod.
module_pods_snapshot() {
  kubectl -n "${MODULE_NS}" get pods --field-selector=status.phase=Running \
    -o jsonpath='{range .items[*]}{.metadata.name}{" "}{.metadata.uid}{" "}{range .status.containerStatuses[*]}{.name}{"="}{.restartCount}{","}{end}{"\n"}{end}' |
    sort
}

leases_snapshot() {
  kubectl -n "${MODULE_NS}" get leases.coordination.k8s.io \
    -o jsonpath='{range .items[*]}{.metadata.name}{" "}{.spec.holderIdentity}{" "}{.spec.leaseTransitions}{"\n"}{end}' |
    sort
}

# node_requested_milli prints the CPU requested on the node the way the scheduler counts it:
# per pod, the larger of (containers + sidecars) and the largest regular init container, plus overhead.
node_requested_milli() {
  kubectl get pods -A --field-selector="spec.nodeName=$1,status.phase!=Succeeded,status.phase!=Failed" \
    -o jsonpath='{range .items[*]}{"P\n"}{range .spec.containers[*]}{"C "}{.resources.requests.cpu}{"\n"}{end}{range .spec.initContainers[*]}{"I "}{.restartPolicy}{" "}{.resources.requests.cpu}{"\n"}{end}{"O "}{.spec.overhead.cpu}{"\n"}{end}' |
    awk '
      function milli(q) {
        if (q == "") return 0
        if (q ~ /m$/) { sub(/m$/, "", q); return q + 0 }
        if (q ~ /u$/) { sub(/u$/, "", q); return q / 1000 }
        if (q ~ /n$/) { sub(/n$/, "", q); return q / 1000000 }
        return q * 1000
      }
      function flush() { total += (run > init ? run : init) + over; run = 0; init = 0; over = 0 }
      $1 == "P" { if (seen) flush(); seen = 1; next }
      $1 == "C" { run += milli($2); next }
      $1 == "I" && $2 == "Always" { run += milli($3); next }
      $1 == "I" { v = milli($2); if (v > init) init = v; next }
      $1 == "O" { over = milli($2) }
      END { if (seen) flush(); printf "%d\n", total }
    '
}

# module_health prints the problems of the module pods; empty output means healthy.
module_health() {
  kubectl -n "${MODULE_NS}" get pods --request-timeout=20s \
    -o jsonpath='{range .items[*]}{.metadata.name}{" "}{.status.phase}{" "}{.status.conditions[?(@.type=="Ready")].status}{" "}{.status.containerStatuses[*].state.waiting.reason}{"\n"}{end}' |
    awk '
      $2 == "Succeeded" { next }
      $2 != "Running" { print $1 " phase=" $2; next }
      $3 != "True" { print $1 " not Ready" }
      /CrashLoopBackOff/ { print $1 " CrashLoopBackOff" }
    '
}

# ---------------------------------------------------------------------------------------------
# Admission probes.
# ---------------------------------------------------------------------------------------------

# Probe results:
#   ok / denied  - the expected admission answer;
#   allowed      - the privileged Pod was admitted (fail-open);
#   error        - the API server answered, but admission failed: a call to a webhook of the module
#                  that failed or timed out, a 5xx; this is what the scenarios measure;
#   foreign-webhook - a call to a webhook of another module or application failed or timed out,
#                  or that webhook denied the request. The result is reported apart and never
#                  counts against the admission failure budget of the module;
#   unreachable  - the API server did not take the request: the connection was refused or reset,
#                  the client timed out, or the server answered 429. Under pressure this is a
#                  finding about the control plane, not about admission, and it is reported apart;
#   infra        - the probe itself is broken: no kubeconfig, authentication, RBAC, a wrong
#                  certificate or host name. Any infra result fails the test, but it is never
#                  counted as an admission failure.
# In the baseline both unreachable and infra mean that the probes cannot measure anything.
# A failure that names a webhook is logged with a "webhook=<name>" field before the reason.
#
# Client-side validation is disabled (--validate=false): it downloads the OpenAPI schema, which
# is not part of the admission path.

# probe_reason prints the first meaningful line of the kubectl output on one line.
probe_reason() {
  printf '%s\n' "$1" | awk '!/^Warning:/ && NF { gsub(/[ \t]+/, " "); print; exit }'
}

# webhook_name prints the name of the webhook that the output of a failed kubectl call names in
# 'failed calling webhook "<name>"' or 'admission webhook "<name>"', or nothing.
webhook_name() {
  printf '%s\n' "$1" | awk '
    match($0, /failed calling webhook "[^"]*"/) { print substr($0, RSTART + 24, RLENGTH - 25); exit }
    match($0, /admission webhook "[^"]*"/) { print substr($0, RSTART + 19, RLENGTH - 20); exit }'
}

# is_module_webhook <name> succeeds for a webhook of the module.
is_module_webhook() {
  [ -n "$1" ] && printf '%s\n' "${MODULE_WEBHOOKS}" | grep -qxF -- "$1"
}

# classify_failure prints "error", "foreign-webhook", "unreachable" or "infra" for the output of a
# failed kubectl call. An answer that names a webhook came through admission, whatever went wrong
# there (for example an x509 error of the webhook call), so it is an admission failure, not a probe
# failure: of the module for a webhook of the module, of someone else for any other webhook.
# An answer that mentions a webhook without its name is counted against the module.
classify_failure() {
  local hook
  hook="$(webhook_name "$1")"
  if [ -n "${hook}" ]; then
    if is_module_webhook "${hook}"; then echo error; else echo foreign-webhook; fi
  elif printf '%s\n' "$1" | grep -q 'webhook'; then
    echo error
  elif printf '%s\n' "$1" | grep -qE 'error validating|localhost:8080|no configuration has been provided|provide credentials|Unauthorized|cannot create resource|no such host|certificate signed by unknown authority|x509:'; then
    echo infra
  elif printf '%s\n' "$1" | grep -qE 'The connection to the server .* was refused|connection refused|connection reset by peer|Unable to connect to the server|couldn.t get current server API group list|Client.Timeout exceeded|TLS handshake timeout|i/o timeout|no route to host|http2: client connection lost|net/http: request canceled|unexpected EOF|TooManyRequests'; then
    echo unreachable
  else
    echo error
  fi
}

# save_probe_failure keeps the full output of the first probe of each class for the report.
save_probe_failure() {
  local file="${STATE_DIR}/probe-$1-error.txt"
  [ -f "${file}" ] || printf '%s\n' "$2" >"${file}"
}

probe_pod() {
  kubectl -n "${NAMESPACE}" create -f "${MANIFESTS_DIR}/$1" \
    --dry-run=server --validate=false -o name --request-timeout=30s 2>&1
}

# probe_failure prints "<class> [webhook=<name>] <reason>" for the output of a failed probe.
probe_failure() {
  local class hook
  class="$(classify_failure "$1")"
  hook="$(webhook_name "$1")"
  case "${class}" in
    infra | unreachable | foreign-webhook) save_probe_failure "${class}" "$1" ;;
  esac
  echo "${class} ${hook:+webhook=${hook} }$(probe_reason "$1")"
}

# probe_allowed prints "ok" or "<error|foreign-webhook|unreachable|infra> <reason>" for the compliant Pod.
probe_allowed() {
  local out
  if out="$(probe_pod probe-allowed-pod.yaml)"; then
    echo "ok"
  else
    probe_failure "${out}"
  fi
}

# probe_denied prints "denied", "allowed" or "<error|foreign-webhook|unreachable|infra> <reason>" for the
# privileged Pod.
probe_denied() {
  local out
  if out="$(probe_pod probe-privileged-pod.yaml)"; then
    echo "allowed"
  elif printf '%s' "${out}" | grep -q 'denied the request' &&
    printf '%s' "${out}" | grep -q '\[d8-pod-security-baseline-deny'; then
    echo "denied"
  else
    probe_failure "${out}"
  fi
}

# observing returns success while the background observers must keep running. They stop on the
# stop file or, when an interrupted run never wrote it, at the deadline that every step re-arms.
observing() {
  [ ! -f "${STATE_DIR}/stop" ] && [ "$(now)" -lt "$(cat "${STATE_DIR}/deadline" 2>/dev/null || echo 0)" ]
}

# arm_deadline <seconds> lets the observers run for the planned rest of the scenario plus
# DEADLINE_MARGIN. Every long step re-arms it, so the margin covers only one step overrunning.
arm_deadline() {
  echo $(($(now) + $1 + DEADLINE_MARGIN)) >"${STATE_DIR}/deadline"
}

# after_span prints how long the after-release phase lasts at most.
after_span() {
  # shellcheck disable=SC2153 # AFTER_WINDOW and AUDIT_RECOVERY come from config.env.
  if [ "${AUDIT_RECOVERY}" -gt "${AFTER_WINDOW}" ]; then echo "${AUDIT_RECOVERY}"; else echo "${AFTER_WINDOW}"; fi
}

# on_observer_exit removes the kubeconfig copy when an observer ends because the run is over:
# the stop file exists or the deadline has passed. An observer that fails while the others still
# run leaves the copy in place. At the deadline the reaper still needs the copy to remove the
# stress load, so the other observers wait for it up to 2 minutes.
on_observer_exit() {
  local i
  ! observing || return 0
  if [ ! -f "${STATE_DIR}/stop" ] && [ "${OBSERVER_ROLE:-}" != "reaper" ]; then
    for i in $(seq 1 24); do
      [ ! -f "${STATE_DIR}/reaped" ] || break
      sleep 5
    done
  fi
  remove_kubeconfig
}

# _probe_loop writes "<epoch> <probe> <result> <latency_ms|NA> [reason]" lines until the stop file appears.
_probe_loop() {
  load_config
  detect_ms_clock
  local log_file="${STATE_DIR}/probes.log" t0 r
  while observing; do
    t0="$(now_ms)"
    r="$(probe_allowed)"
    printf '%s allowed %s %s %s\n' "$(now)" "${r%% *}" "$(ms_since "${t0}")" "$(printf '%s' "${r}" | cut -s -d' ' -f2-)" >>"${log_file}"
    t0="$(now_ms)"
    r="$(probe_denied)"
    printf '%s denied %s %s %s\n' "$(now)" "${r%% *}" "$(ms_since "${t0}")" "$(printf '%s' "${r}" | cut -s -d' ' -f2-)" >>"${log_file}"
    sleep "${PROBE_INTERVAL}"
  done
}

# _reaper ends an interrupted run at the deadline: it deletes the stress Deployments and restores
# the VPA update mode. The EXIT trap then removes the kubeconfig copy.
_reaper() {
  while observing; do
    sleep 5
  done
  [ ! -f "${STATE_DIR}/stop" ] || return 0
  log "the deadline passed without stop-observers: the run was interrupted, removing the stress load"
  kubectl -n "${NAMESPACE}" delete deployments -l app=ape-cpu-stress --ignore-not-found --wait=false \
    --request-timeout=30s || true
  (cmd_restore_vpa) || true
  touch "${STATE_DIR}/reaped"
}

# ---------------------------------------------------------------------------------------------
# Background observers.
# ---------------------------------------------------------------------------------------------

# _watch_raw <name> <api path> writes "<epoch> <EVENT TYPE>" lines for every watch event.
# Each watch ends on the server side after 5 minutes and is reopened until the stop file appears.
_watch_raw() {
  local name="$1" path="$2" line type rc
  # Record failures of kubectl instead of exiting on them.
  set +e
  while observing; do
    kubectl get --raw "${path}?watch=1&timeoutSeconds=300" 2>>"${STATE_DIR}/watch-${name}.err" |
      while IFS= read -r line; do
        type="${line#\{\"type\":\"}"
        type="${type%%\"*}"
        printf '%s %s\n' "$(date +%s)" "${type}"
      done >>"${STATE_DIR}/watch-${name}.log"
    rc="${PIPESTATUS[0]}"
    # A watch ends normally with 0 after timeoutSeconds; anything else is a broken watch.
    printf '%s %s\n' "$(date +%s)" "${rc}" >>"${STATE_DIR}/watch-${name}.exits"
    sleep 1
  done
}

# _vwc_poll writes "<epoch> present <uid> <webhook count>", "<epoch> missing" or
# "<epoch> <error|foreign-webhook|unreachable|infra> <reason>" every 2 seconds until the stop file appears.
_vwc_poll() {
  local out
  while observing; do
    if out="$(kubectl get validatingwebhookconfiguration "${VWC_NAME}" --request-timeout=10s \
      -o jsonpath='{.metadata.uid}{" "}{range .webhooks[*]}{.name}{","}{end}' 2>&1)"; then
      printf '%s present %s %s\n' "$(now)" "${out%% *}" "$(printf '%s' "${out#* }" | tr ',' '\n' | grep -c .)"
    elif printf '%s' "${out}" | grep -q 'NotFound'; then
      printf '%s missing\n' "$(now)"
    else
      printf '%s %s %s\n' "$(now)" "$(classify_failure "${out}")" "$(probe_reason "${out}" | cut -c1-200)"
    fi >>"${STATE_DIR}/vwc.log"
    sleep 2
  done
}

# kubeconfig_path prints the path of the kubeconfig copy of the background observers, if any.
kubeconfig_path() {
  cat "${STATE_DIR}/kubeconfig-path" 2>/dev/null || true
}

# start_background runs a subcommand detached from the Chainsaw step. Chainsaw gives every script a
# temporary kubeconfig in KUBECONFIG and deletes it when the script ends, so the background
# processes use their own copy, written by save_kubeconfig.
start_background() {
  KUBECONFIG="$(kubeconfig_path)" nohup bash "${SCRIPT_PATH}" "$@" </dev/null >>"${STATE_DIR}/background.out" 2>&1 &
  echo "$!" >>"${STATE_DIR}/pids"
}

# save_kubeconfig copies the effective kubeconfig of the current step for the background observers
# and checks that it works. The copy holds credentials, so it goes to a private temporary
# directory outside the reports; STATE_DIR keeps only its path.
save_kubeconfig() {
  local dir
  remove_kubeconfig
  dir="$(mktemp -d "${TMPDIR:-/tmp}/ape-cpu-starvation.XXXXXX")"
  chmod 700 "${dir}"
  printf '%s\n' "${dir}/kubeconfig" >"${STATE_DIR}/kubeconfig-path"
  (
    umask 077
    kubectl config view --raw --minify --flatten >"${dir}/kubeconfig"
  )
  KUBECONFIG="${dir}/kubeconfig" kubectl get --raw /version --request-timeout=20s >/dev/null ||
    fail "the copy of the kubeconfig for the background observers does not work"
}

# remove_kubeconfig deletes the kubeconfig copy and its directory.
remove_kubeconfig() {
  local path
  path="$(kubeconfig_path)"
  [ -n "${path}" ] || return 0
  # Delete only a directory that save_kubeconfig created.
  case "${path}" in
    */ape-cpu-starvation.*/kubeconfig) rm -rf "$(dirname "${path}")" ;;
  esac
  rm -f "${STATE_DIR}/kubeconfig-path"
}

kill_tree() {
  local child
  for child in $(pgrep -P "$1" 2>/dev/null || true); do
    kill_tree "${child}"
  done
  kill "$1" 2>/dev/null || true
}

# ---------------------------------------------------------------------------------------------
# Audit and kubelet metrics.
# ---------------------------------------------------------------------------------------------

# audit_last_run prints the end time (epoch seconds) of the last completed audit cycle.
# It reads gatekeeper_audit_last_run_end_time through the API server pod proxy and falls back
# to the newest status.auditTimestamp of the constraints.
audit_last_run() {
  local pod value
  pod="$(running_pods "${AUDIT_SELECTOR}" | awk 'NR == 1 { print $1 }')"
  if [ -n "${pod}" ]; then
    value="$(kubectl get --raw "/api/v1/namespaces/${MODULE_NS}/pods/${pod}:${METRICS_PORT}/proxy/metrics" \
      --request-timeout=20s 2>/dev/null |
      awk '$1 == "gatekeeper_audit_last_run_end_time" { printf "%d\n", $2 }' || true)"
    if [ -n "${value}" ] && [ "${value}" != "0" ]; then
      echo "${value}"
      return 0
    fi
  fi
  local ts
  ts="$(kubectl get constraints --request-timeout=20s \
    -o jsonpath='{range .items[*]}{.status.auditTimestamp}{"\n"}{end}' 2>/dev/null | grep . | sort | tail -n 1 || true)"
  if [ -z "${ts}" ]; then
    echo 0
    return 0
  fi
  # RFC 3339 (UTC) to epoch seconds without GNU date: days from the civil calendar.
  printf '%s\n' "${ts}" | awk -F'[-T:Z]' '{
    y = $1; m = $2 + 0; d = $3 + 0
    if (m <= 2) { y -= 1; m += 12 }
    days = 365 * y + int(y / 4) - int(y / 100) + int(y / 400) + int((153 * (m - 3) + 2) / 5) + d - 719469
    printf "%d\n", days * 86400 + $4 * 3600 + $5 * 60 + $6
  }'
}

# record_cpu_sample <label> appends kubelet CPU counters of the target nodes and pods.
# The numbers are informational: they show how much CPU the node and the target container got.
record_cpu_sample() {
  local label="$1" pod node raw node_cpu pod_cpu wait_cpu
  while read -r pod node; do
    [ -n "${pod}" ] || continue
    raw="$(kubectl get --raw "/api/v1/nodes/${node}/proxy/metrics/resource" --request-timeout=20s 2>/dev/null || true)"
    node_cpu="$(printf '%s\n' "${raw}" | awk '$1 == "node_cpu_usage_seconds_total" { print $2 }')"
    pod_cpu="$(printf '%s\n' "${raw}" | awk -v p="pod=\"${pod}\"" -v c="container=\"${TARGET_CONTAINER}\"" \
      'index($0, "container_cpu_usage_seconds_total{") == 1 && index($0, p) && index($0, c) { print $2 }')"
    wait_cpu="$(kubectl get --raw "/api/v1/nodes/${node}/proxy/metrics/cadvisor" --request-timeout=30s 2>/dev/null |
      awk -v p="pod=\"${pod}\"" -v c="container=\"${TARGET_CONTAINER}\"" \
        'index($0, "container_pressure_cpu_waiting_seconds_total{") == 1 && index($0, p) && index($0, c) { print $2 }' || true)"
    printf '%s %s %s %s %s %s %s\n' "${label}" "$(now)" "${pod}" "${node}" \
      "${node_cpu:-NA}" "${pod_cpu:-NA}" "${wait_cpu:-NA}" >>"${STATE_DIR}/cpu-samples"
  done <"${STATE_DIR}/targets"
}

# ---------------------------------------------------------------------------------------------
# Subcommands.
# ---------------------------------------------------------------------------------------------

cmd_preflight() {
  local minor deny_constraints i r_allowed r_denied hold recovery baseline_window after_window audit_recovery
  # Assignments stop the script when a duration cannot be parsed.
  hold="$(to_seconds "${HOLD:-5m}")"
  recovery="$(to_seconds "${RECOVERY:-120s}")"
  baseline_window="$(to_seconds "${BASELINE_WINDOW:-3m}")"
  after_window="$(to_seconds "${AFTER_WINDOW:-5m}")"
  audit_recovery="$(to_seconds "${AUDIT_RECOVERY:-5m}")"
  {
    printf 'TARGET=%q\n' "${TARGET:-controller-manager}"
    printf 'MODE=%q\n' "${MODE:-noisy-neighbour}"
    printf 'CPU_LIMIT=%q\n' "${CPU_LIMIT:-false}"
    printf 'HOLD=%q\n' "${hold}"
    printf 'RECOVERY=%q\n' "${recovery}"
    printf 'BASELINE_WINDOW=%q\n' "${baseline_window}"
    printf 'AFTER_WINDOW=%q\n' "${after_window}"
    printf 'AUDIT_RECOVERY=%q\n' "${audit_recovery}"
    printf 'THREADS_FACTOR=%q\n' "${THREADS_FACTOR:-1}"
    printf 'STRESS_IMAGE=%q\n' "${STRESS_IMAGE:-mirror.gcr.io/library/busybox:1.36}"
    printf 'STRESS_MARGIN_MILLI=%q\n' "${STRESS_MARGIN_MILLI:-100}"
    printf 'LA_MIN_REQUEST_MILLI=%q\n' "${LA_MIN_REQUEST_MILLI:-100}"
    printf 'MAX_FAILURE_PERCENT=%q\n' "${MAX_FAILURE_PERCENT:-1}"
    printf 'MIN_PROBES=%q\n' "${MIN_PROBES:-30}"
    printf 'PROBE_INTERVAL=%q\n' "${PROBE_INTERVAL:-0.5}"
    printf 'MAX_STRESS_PODS=%q\n' "${MAX_STRESS_PODS:-100}"
    printf 'STRESS_POD_MIN_MILLI=%q\n' "${STRESS_POD_MIN_MILLI:-10}"
    printf 'STRESS_POD_SLOT_RESERVE=%q\n' "${STRESS_POD_SLOT_RESERVE:-10}"
    printf 'DEADLINE_MARGIN=%q\n' "$(to_seconds "${DEADLINE_MARGIN:-10m}")"
  } >"${CONFIG_FILE}"
  load_config
  target_selector >/dev/null
  require_int THREADS_FACTOR "${THREADS_FACTOR}"
  require_int MAX_STRESS_PODS "${MAX_STRESS_PODS}"
  require_int STRESS_POD_MIN_MILLI "${STRESS_POD_MIN_MILLI}"
  require_int STRESS_POD_SLOT_RESERVE "${STRESS_POD_SLOT_RESERVE}"
  require_int MIN_PROBES "${MIN_PROBES}"
  [ "${THREADS_FACTOR}" -ge 1 ] || fail "THREADS_FACTOR must be at least 1"
  [ "${STRESS_POD_MIN_MILLI}" -ge 1 ] || fail "STRESS_POD_MIN_MILLI must be at least 1"
  log "settings: $(tr '\n' ' ' <"${CONFIG_FILE}")"
  log "state directory: ${STATE_DIR}"

  minor="$(kubectl get --raw /version | sed -n 's/.*"minor": *"\([0-9]*\).*/\1/p')"
  log "Kubernetes minor version: ${minor}"
  if [ "${MODE}" = "high-load-average" ]; then
    [ "${minor:-0}" -ge 33 ] || fail "in-place Pod resize needs Kubernetes 1.33 or newer"
    kubectl patch --help 2>/dev/null | grep -q -- '--subresource' ||
      fail "kubectl does not support 'patch --subresource': use kubectl 1.32 or newer"
  fi

  kubectl -n "${MODULE_NS}" rollout status deployment/gatekeeper-controller-manager --timeout=120s
  kubectl -n "${MODULE_NS}" rollout status deployment/gatekeeper-audit --timeout=120s
  kubectl get validatingwebhookconfiguration "${VWC_NAME}" -o name
  # Admission failures count against the module only for the webhooks in MODULE_WEBHOOKS.
  for i in validatingwebhookconfiguration mutatingwebhookconfiguration; do
    kubectl get "${i}" "${VWC_NAME}" --ignore-not-found -o jsonpath='{range .webhooks[*]}{.name}{"\n"}{end}'
  done | while read -r i; do
    [ -z "${i}" ] || is_module_webhook "${i}" ||
      log "WARNING: webhook ${i} of ${VWC_NAME} is not in MODULE_WEBHOOKS: its failures would count as foreign-webhook"
  done

  # Make the Baseline standard with the deny action apply to the test namespace whatever
  # defaultPolicy the cluster uses. The label comes first: Deckhouse renders the
  # d8-pod-security-baseline-deny* constraints only while some namespace uses the deny action.
  kubectl label namespace "${NAMESPACE}" --overwrite \
    security.deckhouse.io/pod-policy=baseline \
    security.deckhouse.io/pod-policy-action=deny

  # The module hook has to notice the label and Deckhouse has to render the release; give it 5 minutes.
  deny_constraints=""
  for i in $(seq 1 30); do
    deny_constraints="$(kubectl get d8privilegedcontainer.constraints.gatekeeper.sh --request-timeout=20s \
      -o jsonpath='{range .items[*]}{.metadata.name}{" "}{.spec.enforcementAction}{"\n"}{end}' 2>/dev/null |
      awk '$1 ~ /^d8-pod-security-baseline-deny/ && $2 == "deny" { print $1 }' || true)"
    [ -z "${deny_constraints}" ] || break
    log "waiting for a d8-pod-security-baseline-deny* constraint (attempt ${i}/30)"
    sleep 10
  done
  [ -n "${deny_constraints}" ] ||
    fail "no d8-pod-security-baseline-deny* constraint with enforcementAction deny 5 minutes after labelling the namespace: the deny probe cannot work"
  log "deny constraints: $(printf '%s' "${deny_constraints}" | tr '\n' ' ')"

  running_pods "$(target_selector)" >"${STATE_DIR}/targets"
  [ -s "${STATE_DIR}/targets" ] || fail "no running pod matches $(target_selector) in ${MODULE_NS}"
  log "targets (pod node):"
  sed 's/^/  /' "${STATE_DIR}/targets"

  # Gatekeeper learns the namespace labels from its cache; give it up to 2 minutes.
  for i in $(seq 1 60); do
    r_allowed="$(probe_allowed)"
    r_denied="$(probe_denied)"
    if [ "${r_allowed}" = "ok" ] && [ "${r_denied}" = "denied" ]; then
      log "admission probes work: compliant Pod allowed, privileged Pod denied (attempt ${i})"
      return 0
    fi
    sleep 2
  done
  fail "admission probes do not work before the pressure: allowed probe '${r_allowed}', deny probe '${r_denied}'"
}

cmd_start_observers() {
  load_config
  rm -f "${STATE_DIR}/stop" "${STATE_DIR}/reaped"
  # A safety net for interrupted runs: without stop-observers, the observers stop and the reaper
  # removes the stress load DEADLINE_MARGIN after the planned end of the scenario.
  arm_deadline $((BASELINE_WINDOW + HOLD + RECOVERY + $(after_span)))
  rm -f "${STATE_DIR}/probe-infra-error.txt" "${STATE_DIR}/probe-unreachable-error.txt" \
    "${STATE_DIR}/probe-foreign-webhook-error.txt"
  save_kubeconfig
  module_pods_snapshot >"${STATE_DIR}/pods-before"
  leases_snapshot >"${STATE_DIR}/leases-before"
  kubectl get validatingwebhookconfiguration "${VWC_NAME}" \
    -o jsonpath='{.metadata.uid}{" "}{range .webhooks[*]}{.name}{","}{end}' |
    awk '{ n = split($2, a, ","); print $1, n - 1 }' >"${STATE_DIR}/vwc-before"
  log "module pods before the pressure:"
  sed 's/^/  /' "${STATE_DIR}/pods-before"
  log "VWC before the pressure (uid webhooks): $(cat "${STATE_DIR}/vwc-before")"

  start_background _vwc_poll
  start_background _watch_raw leases "/apis/coordination.k8s.io/v1/namespaces/${MODULE_NS}/leases"
  start_background _watch_raw constraintpodstatuses "/apis/status.gatekeeper.sh/v1beta1/namespaces/${MODULE_NS}/constraintpodstatuses"
  start_background _watch_raw constrainttemplatepodstatuses "/apis/status.gatekeeper.sh/v1beta1/namespaces/${MODULE_NS}/constrainttemplatepodstatuses"
  start_background _probe_loop
  start_background _reaper
  phase_mark baseline
  log "observers started: pids $(tr '\n' ' ' <"${STATE_DIR}/pids")"
}

cmd_stop_observers() {
  touch "${STATE_DIR}/stop"
  if [ -f "${STATE_DIR}/pids" ]; then
    while read -r pid; do
      [ -n "${pid}" ] || continue
      # After a reboot the PID can belong to another process: kill only an observer of this script.
      case "$(ps -p "${pid}" -o command= 2>/dev/null || true)" in
        *cpu-starvation.sh*) kill_tree "${pid}" ;;
      esac
    done <"${STATE_DIR}/pids"
    rm -f "${STATE_DIR}/pids"
  fi
  # The copy holds credentials; do not leave it behind.
  remove_kubeconfig
  log "observers stopped"
}

cmd_baseline() {
  load_config
  log "collecting the baseline for ${BASELINE_WINDOW}s"
  sleep "${BASELINE_WINDOW}"
  phase_mark baseline-end
  arm_deadline $((HOLD + RECOVERY + $(after_span)))
  check_probe_infrastructure "$(phase_time baseline)" "$(phase_time baseline-end)"
}

# check_probe_infrastructure <from> <to> fails when the probes in the window could not reach
# admission (a broken probe or an unreachable API server), or when less than half of them
# succeeded: the observers are broken, and the pressure would measure nothing. It runs before
# the pressure starts and again in analyse.
check_probe_infrastructure() {
  local from="$1" to="$2" stats total good infra unreachable foreign class
  stats="$(awk -v from="${from}" -v to="${to}" '
    $1 >= from && $1 < to {
      total++
      if ($3 == "ok" || $3 == "denied") good++
      if ($3 == "infra") infra++
      if ($3 == "unreachable") unreachable++
      if ($3 == "foreign-webhook") foreign++
    }
    END { printf "%d %d %d %d %d\n", total, good, infra, unreachable, foreign }' "${STATE_DIR}/probes.log" 2>/dev/null || echo "0 0 0 0 0")"
  read -r total good infra unreachable foreign <<EOF_STATS
${stats}
EOF_STATS
  log "baseline probes: ${good}/${total} answered as expected, ${infra} probe infrastructure errors, ${unreachable} with the API server unreachable, ${foreign} failed in webhooks of other modules"
  if [ "${total}" -gt 0 ] && [ "${infra}" -eq 0 ] && [ "${unreachable}" -eq 0 ] && [ $((good * 2)) -ge "${total}" ]; then
    return 0
  fi
  log "FAIL: probe infrastructure broken: the admission probes do not work without any pressure"
  class=""
  [ "${unreachable}" -eq 0 ] || class="unreachable"
  [ "${infra}" -eq 0 ] || class="infra"
  if [ -n "${class}" ] && [ -f "${STATE_DIR}/probe-${class}-error.txt" ]; then
    log "first probe ${class} error, in full:"
    cat "${STATE_DIR}/probe-${class}-error.txt"
  else
    log "first failed probe of the baseline, in full:"
    awk -v from="${from}" -v to="${to}" '$1 >= from && $1 < to && $3 != "ok" && $3 != "denied" { print; exit }' \
      "${STATE_DIR}/probes.log" 2>/dev/null || true
  fi
  [ "${total}" -gt 0 ] || log "the probe log has no entries in the baseline; see ${STATE_DIR}/background.out"
  exit 1
}

# cmd_freeze_vpa switches the VerticalPodAutoscaler of the target Deployment to updateMode Off:
# its updater must not evict or resize the target while the test measures restarts and
# replacements. The original mode is kept in STATE_DIR and in an annotation on the VPA, so
# `cleanup` can restore it without the state directory.
cmd_freeze_vpa() {
  load_config
  local deployment line name target mode
  deployment="$(target_deployment)"
  : >"${STATE_DIR}/vpa-frozen"
  if ! line="$(kubectl -n "${MODULE_NS}" get verticalpodautoscalers.autoscaling.k8s.io \
    -o jsonpath='{range .items[*]}{.metadata.name}{" "}{.spec.targetRef.name}{" "}{.spec.updatePolicy.updateMode}{"\n"}{end}' 2>/dev/null)"; then
    log "VPA API is not available: nothing to freeze"
    return 0
  fi
  printf '%s\n' "${line}" | while read -r name target mode; do
    [ "${target:-}" = "${deployment}" ] || continue
    [ "${mode:-Auto}" = "Off" ] && continue
    log "VPA ${name}: updateMode ${mode:-<default>} -> Off for the test"
    printf '%s %s\n' "${name}" "${mode:-}" >>"${STATE_DIR}/vpa-frozen"
    kubectl -n "${MODULE_NS}" annotate verticalpodautoscalers.autoscaling.k8s.io "${name}" --overwrite \
      "${VPA_MODE_ANNOTATION}=${mode:--}"
    kubectl -n "${MODULE_NS}" patch verticalpodautoscalers.autoscaling.k8s.io "${name}" --type=merge \
      -p '{"spec":{"updatePolicy":{"updateMode":"Off"}}}'
  done
  [ -s "${STATE_DIR}/vpa-frozen" ] || log "no VPA of ${deployment} to freeze"
}

# restore_vpa_mode <name> <mode or empty> puts the update mode back and drops the annotation.
restore_vpa_mode() {
  local name="$1" mode="$2"
  if [ -n "${mode}" ]; then
    kubectl -n "${MODULE_NS}" patch verticalpodautoscalers.autoscaling.k8s.io "${name}" --type=merge \
      -p "{\"spec\":{\"updatePolicy\":{\"updateMode\":\"${mode}\"}}}" || return 1
  else
    kubectl -n "${MODULE_NS}" patch verticalpodautoscalers.autoscaling.k8s.io "${name}" --type=json \
      -p '[{"op":"remove","path":"/spec/updatePolicy/updateMode"}]' || return 1
  fi
  kubectl -n "${MODULE_NS}" annotate verticalpodautoscalers.autoscaling.k8s.io "${name}" "${VPA_MODE_ANNOTATION}-" ||
    log "WARNING: VPA ${name}: cannot remove the annotation ${VPA_MODE_ANNOTATION}"
  log "VPA ${name}: updateMode restored to ${mode:-<default>}"
}

cmd_restore_vpa() {
  local name mode failed=false
  [ -f "${STATE_DIR}/vpa-frozen" ] || return 0
  while read -r name mode; do
    [ -n "${name}" ] || continue
    if ! restore_vpa_mode "${name}" "${mode}"; then
      log "WARNING: VPA ${name}: cannot restore updateMode ${mode:-<default>}; run task cleanup"
      failed=true
    fi
  done <"${STATE_DIR}/vpa-frozen"
  # Keep the record when a restore failed, so `cleanup` can retry it.
  [ "${failed}" = "true" ] || rm -f "${STATE_DIR}/vpa-frozen"
}

# cmd_resize raises the CPU request of the target container to its current usage in place and,
# with CPU_LIMIT=true, sets the CPU limit equal to the request. The pod gets an annotation, so
# `cleanup` can find a resized pod without the state directory.
cmd_resize() {
  load_config
  local pod node req lim usage new new_lim patch restarts_before restarts_after applied i
  mark_ramp_up
  : >"${STATE_DIR}/resized"
  while read -r pod node; do
    [ -n "${pod}" ] || continue
    req="$(kubectl -n "${MODULE_NS}" get pod "${pod}" \
      -o jsonpath="{.spec.containers[?(@.name==\"${TARGET_CONTAINER}\")].resources.requests.cpu}")"
    lim="$(kubectl -n "${MODULE_NS}" get pod "${pod}" \
      -o jsonpath="{.spec.containers[?(@.name==\"${TARGET_CONTAINER}\")].resources.limits.cpu}")"
    usage="$(kubectl top pod "${pod}" -n "${MODULE_NS}" --containers --no-headers 2>/dev/null |
      awk -v c="${TARGET_CONTAINER}" '$2 == c { print $3 }' || true)"
    [ -n "${usage}" ] || log "WARNING: kubectl top returned nothing for ${pod}; the request is not raised above its current value"
    new="$(cpu_to_milli "${req}")"
    [ "$(cpu_to_milli "${usage}")" -gt "${new}" ] && new="$(cpu_to_milli "${usage}")"
    [ "${LA_MIN_REQUEST_MILLI}" -gt "${new}" ] && new="${LA_MIN_REQUEST_MILLI}"
    log "${pod}/${TARGET_CONTAINER}: request ${req:-<none>}, limit ${lim:-<none>}, usage ${usage:-<unknown>} -> request ${new}m"

    new_lim=""
    if [ "${CPU_LIMIT}" = "true" ]; then
      new_lim="${new}"
    elif [ -n "${lim}" ] && [ "$(cpu_to_milli "${lim}")" -lt "${new}" ]; then
      new_lim="${new}"
    fi
    if [ -n "${new_lim}" ]; then
      patch="{\"spec\":{\"containers\":[{\"name\":\"${TARGET_CONTAINER}\",\"resources\":{\"requests\":{\"cpu\":\"${new}m\"},\"limits\":{\"cpu\":\"${new_lim}m\"}}}]}}"
    else
      patch="{\"spec\":{\"containers\":[{\"name\":\"${TARGET_CONTAINER}\",\"resources\":{\"requests\":{\"cpu\":\"${new}m\"}}}]}}"
    fi
    printf '%s %s %s %s %s\n' "${pod}" "${req:--}" "${lim:--}" "${new}" "${new_lim:--}" >>"${STATE_DIR}/resized"

    restarts_before="$(kubectl -n "${MODULE_NS}" get pod "${pod}" \
      -o jsonpath="{.status.containerStatuses[?(@.name==\"${TARGET_CONTAINER}\")].restartCount}")"
    kubectl -n "${MODULE_NS}" annotate pod "${pod}" --overwrite "${RESIZED_ANNOTATION}=${req:--}"
    kubectl -n "${MODULE_NS}" patch pod "${pod}" --subresource resize -p "${patch}"

    applied=""
    for i in $(seq 1 45); do
      applied="$(kubectl -n "${MODULE_NS}" get pod "${pod}" \
        -o jsonpath="{.status.containerStatuses[?(@.name==\"${TARGET_CONTAINER}\")].resources.requests.cpu}")"
      [ "$(cpu_to_milli "${applied}")" -eq "${new}" ] && break
      sleep 2
    done
    if [ "$(cpu_to_milli "${applied}")" -ne "${new}" ]; then
      kubectl -n "${MODULE_NS}" get pod "${pod}" -o jsonpath='{.status.conditions}{"\n"}'
      fail "the kubelet did not apply the resize of ${pod} within 90s (status request ${applied:-<none>})"
    fi
    restarts_after="$(kubectl -n "${MODULE_NS}" get pod "${pod}" \
      -o jsonpath="{.status.containerStatuses[?(@.name==\"${TARGET_CONTAINER}\")].restartCount}")"
    [ "${restarts_before}" = "${restarts_after}" ] ||
      log "WARNING: ${pod}/${TARGET_CONTAINER} restarted during the resize (resizePolicy RestartContainer?)"
    log "${pod}/${TARGET_CONTAINER}: resize applied in place"
  done <"${STATE_DIR}/targets"
  # A container restart during the resize must not count as a restart under pressure.
  module_pods_snapshot >"${STATE_DIR}/pods-before"
}

# deployment_of_pod prints the Deployment of a module pod from its control-plane label.
deployment_of_pod() {
  case "$(kubectl -n "${MODULE_NS}" get pod "$1" -o jsonpath='{.metadata.labels.control-plane}' 2>/dev/null)" in
    controller-manager) echo "gatekeeper-controller-manager" ;;
    audit-controller) echo "gatekeeper-audit" ;;
  esac
}

# restart_deployment <name> replaces the pods of a module Deployment with pods built from its template.
restart_deployment() {
  log "restarting deployment/$1 to drop the resources set by the test"
  kubectl -n "${MODULE_NS}" rollout restart "deployment/$1"
  kubectl -n "${MODULE_NS}" rollout status "deployment/$1" --timeout=600s
}

# cmd_restore_resize puts the original CPU request back in place. A CPU limit cannot be removed
# in place, so a pod that got a new limit is replaced through a rolling restart of its Deployment.
cmd_restore_resize() {
  load_config
  local pod req lim new new_lim restart=false
  [ -f "${STATE_DIR}/resized" ] || return 0
  while read -r pod req lim new new_lim; do
    [ -n "${pod}" ] || continue
    if ! kubectl -n "${MODULE_NS}" get pod "${pod}" -o name >/dev/null 2>&1; then
      continue
    fi
    if [ "${new_lim}" != "-" ] || [ "${req}" = "-" ]; then
      restart=true
      continue
    fi
    if kubectl -n "${MODULE_NS}" patch pod "${pod}" --subresource resize \
      -p "{\"spec\":{\"containers\":[{\"name\":\"${TARGET_CONTAINER}\",\"resources\":{\"requests\":{\"cpu\":\"${req}\"}}}]}}"; then
      log "${pod}/${TARGET_CONTAINER}: request restored to ${req}"
      kubectl -n "${MODULE_NS}" annotate pod "${pod}" "${RESIZED_ANNOTATION}-" ||
        log "WARNING: ${pod}: cannot remove the annotation ${RESIZED_ANNOTATION}"
    else
      log "WARNING: ${pod}/${TARGET_CONTAINER}: restoring the request ${req} in place failed"
      restart=true
    fi
  done <"${STATE_DIR}/resized"
  if [ "${restart}" = "true" ]; then
    restart_deployment "$(target_deployment)"
  fi
  rm -f "${STATE_DIR}/resized"
}

# stress_layout <free milli> <cores> <max pods> prints "<replicas> <threads per pod> <request milli per pod>".
#
# Noisy neighbour: one pod requests all free CPU and runs THREADS_FACTOR burner threads per CPU.
#
# High load average: CFS shares the CPU between cgroups by weight, and the threads inside one
# cgroup neither take a larger share from another cgroup nor delay it much. Every pod is a cgroup
# of its own, so the load is spread over many small Burstable pods: their requests add up to the
# free CPU, and each runs 2 to 4 threads. The number of pods is the smallest of MAX_STRESS_PODS,
# the free pod slots of the node and the pods that fit STRESS_POD_MIN_MILLI each; the threads
# per pod then get as close to THREADS_FACTOR per CPU as 4 threads per pod allow.
stress_layout() {
  awk -v free="$1" -v cores="$2" -v maxpods="$3" -v mode="${MODE}" -v factor="${THREADS_FACTOR}" \
    -v minmilli="${STRESS_POD_MIN_MILLI}" 'BEGIN {
    if (mode != "high-load-average") { printf "1 %d %d\n", cores * factor, free; exit }
    target = cores * factor
    n = maxpods
    if (int(free / minmilli) < n) n = int(free / minmilli)
    # No more pods than 2 threads each need.
    if (int((target + 1) / 2) < n) n = int((target + 1) / 2)
    if (n < 1) { print "0 0 0"; exit }
    t = int((target + n - 1) / n)
    if (t < 2) t = 2
    if (t > 4) t = 4
    printf "%d %d %d\n", n, t, int(free / n)
  }'
}

cmd_stress_start() {
  load_config
  local node idx=0 hostname alloc requested free capacity cores max_pods node_pods slots layout replicas threads request name
  mark_ramp_up
  arm_deadline $((HOLD + RECOVERY + $(after_span)))
  : >"${STATE_DIR}/stress"
  awk '{ print $2 }' "${STATE_DIR}/targets" | sort -u >"${STATE_DIR}/target-nodes"
  while read -r node; do
    idx=$((idx + 1))
    name="ape-cpu-stress-${idx}"
    hostname="$(kubectl get node "${node}" -o jsonpath='{.metadata.labels.kubernetes\.io/hostname}')"
    alloc="$(cpu_to_milli "$(kubectl get node "${node}" -o jsonpath='{.status.allocatable.cpu}')")"
    capacity="$(cpu_to_milli "$(kubectl get node "${node}" -o jsonpath='{.status.capacity.cpu}')")"
    requested="$(node_requested_milli "${node}")"
    free=$((alloc - requested - STRESS_MARGIN_MILLI))
    [ "${free}" -ge 100 ] ||
      fail "node ${node}: allocatable ${alloc}m, requested ${requested}m, nothing left to request for the stress pods"
    cores=$(((capacity + 999) / 1000))
    max_pods="$(kubectl get node "${node}" -o jsonpath='{.status.allocatable.pods}')"
    node_pods="$(kubectl get pods -A --field-selector="spec.nodeName=${node},status.phase!=Succeeded,status.phase!=Failed" \
      -o name | grep -c . || true)"
    slots=$((${max_pods:-110} - node_pods - STRESS_POD_SLOT_RESERVE))
    [ "${slots}" -lt "${MAX_STRESS_PODS}" ] || slots="${MAX_STRESS_PODS}"
    layout="$(stress_layout "${free}" "${cores}" "${slots}")"
    read -r replicas threads request <<EOF_LAYOUT
${layout}
EOF_LAYOUT
    [ "${replicas}" -ge 1 ] ||
      fail "node ${node}: no room for stress pods (maxPods ${max_pods}, ${node_pods} pods, reserve ${STRESS_POD_SLOT_RESERVE})"
    log "node ${node}: allocatable ${alloc}m, requested ${requested}m, ${node_pods}/${max_pods} pods, ${cores} CPU -> ${name}: ${replicas} pod(s) x ${threads} burner threads = $((replicas * threads)) threads (target $((cores * THREADS_FACTOR))), ${request}m requested per pod"
    sed -e "s|__NAME__|${name}|g" \
      -e "s|__HOSTNAME__|${hostname}|g" \
      -e "s|__IMAGE__|${STRESS_IMAGE}|g" \
      -e "s|__REPLICAS__|${replicas}|g" \
      -e "s|__THREADS__|${threads}|g" \
      -e "s|__CPU_REQUEST__|${request}m|g" \
      "${MANIFESTS_DIR}/cpu-stress-deployment.yaml" >"${STATE_DIR}/${name}.yaml"
    kubectl -n "${NAMESPACE}" apply -f "${STATE_DIR}/${name}.yaml"
    printf '%s %s %s\n' "${name}" "${node}" "${replicas}" >>"${STATE_DIR}/stress"
  done <"${STATE_DIR}/target-nodes"
  while read -r name node _; do
    if ! kubectl -n "${NAMESPACE}" rollout status "deployment/${name}" --timeout=600s; then
      kubectl -n "${NAMESPACE}" describe pods -l "ape-cpu-stress/name=${name}" | tail -n 30
      fail "the stress pods of ${name} on ${node} did not start"
    fi
  done <"${STATE_DIR}/stress"
  # The pressure phase starts once every stress pod runs; the time before it is the ramp-up.
  record_cpu_sample pressure
  phase_mark pressure
  arm_deadline $((HOLD + RECOVERY + $(after_span)))
}

# running_stress_pods prints the number of running stress pods, or fails when the API server
# does not answer: an API error is not the same as no pods.
running_stress_pods() {
  local out
  out="$(kubectl -n "${NAMESPACE}" get pods -l app=ape-cpu-stress --field-selector=status.phase=Running \
    -o name --request-timeout=20s 2>"${STATE_DIR}/hold-kubectl.err")" || return 1
  printf '%s\n' "${out}" | grep -c '^pod/' || true
}

cmd_hold() {
  load_config
  local start elapsed problems expected running attempt short=0
  start="$(now)"
  expected="$(awk '{ n += $3 } END { printf "%d\n", n }' "${STATE_DIR}/stress")"
  log "holding the pressure for ${HOLD}s"
  while :; do
    elapsed=$(($(now) - start))
    [ "${elapsed}" -ge "${HOLD}" ] && break
    sleep 30
    elapsed=$(($(now) - start))
    running=""
    for attempt in 1 2 3; do
      if running="$(running_stress_pods)"; then
        break
      fi
      running=""
      log "listing the stress pods failed (attempt ${attempt}/3): $(head -n 1 "${STATE_DIR}/hold-kubectl.err")"
      sleep 5
    done
    if [ -z "${running}" ]; then
      log "WARNING: the API server did not list the stress pods 3 times in a row; the probes record its availability"
    elif [ "${running}" -lt "${expected}" ]; then
      short=$((short + 1))
      # One short count can be a pod that is being replaced; two in a row end the scenario.
      [ "${short}" -lt 2 ] || fail "only ${running} of ${expected} stress pods are running: the pressure ended early"
      log "WARNING: only ${running} of ${expected} stress pods are running; checking again"
    else
      short=0
    fi
    problems="$(module_health | tr '\n' ';' || true)"
    log "pressure ${elapsed}s/${HOLD}s; stress pods ${running:-unknown}/${expected}; probes so far: $(awk -v t="$(phase_time pressure)" '$1 >= t { n[$2 " " $3]++ } END { for (k in n) printf "%s=%d ", k, n[k] }' "${STATE_DIR}/probes.log"); module: ${problems:-healthy}"
  done
  record_cpu_sample release
  audit_last_run >"${STATE_DIR}/audit-at-release"
}

delete_stress() {
  local name node i
  [ -f "${STATE_DIR}/stress" ] || return 0
  while read -r name node _; do
    [ -n "${name}" ] || continue
    # Deletion passes through the starved webhook (failurePolicy Fail), so retry.
    for i in $(seq 1 10); do
      kubectl -n "${NAMESPACE}" delete deployment "${name}" --ignore-not-found --wait=false --request-timeout=30s && break
      log "deleting ${name} on ${node} failed (attempt ${i}), retrying"
      sleep 3
    done
  done <"${STATE_DIR}/stress"
  kubectl -n "${NAMESPACE}" delete pods -l app=ape-cpu-stress --ignore-not-found --grace-period=1 --request-timeout=60s || true
  kubectl -n "${NAMESPACE}" wait pods -l app=ape-cpu-stress --for=delete --timeout=180s || true
}

cmd_stress_delete() {
  delete_stress
}

cmd_release() {
  load_config
  local release deadline problems r_allowed="" r_denied=""
  # Deleting many stress pods through a starved API server can take a few minutes.
  arm_deadline $((RECOVERY + $(after_span) + 600))
  delete_stress
  phase_mark release
  release="$(phase_time release)"
  arm_deadline $((RECOVERY + $(after_span)))
  deadline=$((release + RECOVERY))
  while :; do
    problems="$(module_health || true)"
    r_allowed="$(probe_allowed)"
    r_denied="$(probe_denied)"
    if [ -z "${problems}" ] && [ "${r_allowed}" = "ok" ] && [ "${r_denied}" = "denied" ]; then
      phase_mark recovered
      log "recovered $(($(phase_time recovered) - release))s after the release (budget ${RECOVERY}s)"
      return 0
    fi
    [ "$(now)" -lt "${deadline}" ] || break
    sleep 3
  done
  log "module pods: ${problems:-healthy}; allowed probe: ${r_allowed}; deny probe: ${r_denied}"
  fail "the module did not recover within ${RECOVERY}s after the pressure ended"
}

cmd_after() {
  load_config
  local release end audit_before audit_now audit_deadline
  release="$(phase_time release)"
  end=$((release + AFTER_WINDOW))
  audit_before="$(cat "${STATE_DIR}/audit-at-release" 2>/dev/null || echo 0)"
  audit_deadline=$((release + AUDIT_RECOVERY))
  [ "${audit_deadline}" -gt "${end}" ] && end="${audit_deadline}"
  arm_deadline $((end - $(now)))
  log "observing ${AFTER_WINDOW}s after the release; waiting for an audit cycle that ends after $(fmt_time "${release}")"
  rm -f "${STATE_DIR}/audit-after-release"
  while [ "$(now)" -lt "${end}" ]; do
    if [ ! -f "${STATE_DIR}/audit-after-release" ]; then
      audit_now="$(audit_last_run || echo 0)"
      if [ "${audit_now:-0}" -gt "${release}" ]; then
        printf '%s %s\n' "${audit_now}" "$((audit_now - release))" >"${STATE_DIR}/audit-after-release"
        log "audit cycle completed $((audit_now - release))s after the release (last before the release: ${audit_before})"
        # The audit check is done; keep only the write-storm window running.
        end=$((release + AFTER_WINDOW))
      fi
    fi
    sleep 15
  done
  phase_mark end
}

# rate_check <name> <count> <seconds> <base per minute> prints the rate and returns 1 on a storm.
rate_check() {
  awk -v name="$1" -v c="$2" -v s="$3" -v base="$4" 'BEGIN {
    r = (s > 0) ? c * 60 / s : 0
    storm = (r > 2 * base && r > base + 10)
    printf "%-38s %6d events in %5ds = %7.2f/min (baseline %.2f/min)%s\n", name, c, s, r, base, storm ? "  <-- STORM" : ""
    exit storm
  }'
}

count_between() {
  awk -v from="$2" -v to="$3" -v type="${4:-}" \
    '$1 >= from && $1 < to && (type == "" || $2 == type) { n++ } END { printf "%d\n", n }' "$1"
}

# foreign_webhooks <from> <to> prints "<webhook>: <count> probe(s), first at <epoch>" for every
# webhook of another module that failed the probes in the window.
foreign_webhooks() {
  awk -v from="$1" -v to="$2" '
    $1 >= from && $1 < to && $3 == "foreign-webhook" {
      w = ($5 ~ /^webhook=/) ? substr($5, 9) : "<unnamed>"
      if (!(w in n)) { order[++k] = w; first[w] = $1 }
      n[w]++
    }
    END { for (i = 1; i <= k; i++) printf "%s: %d probe(s), first at %s\n", order[i], n[order[i]], first[order[i]] }' \
    "${STATE_DIR}/probes.log" 2>/dev/null || true
}

cmd_analyse() {
  load_config
  cmd_stop_observers
  local rc b be ru p r rec e f name log base_n base_rate n_u n_p n_a vwc_uid vwc_hooks pods_after leases_after foreign
  rm -f "${STATE_DIR}/failures"
  b="$(phase_time baseline)"
  be="$(phase_time baseline-end)"
  ru="$(phase_time ramp-up)"
  p="$(phase_time pressure)"
  r="$(phase_time release)"
  rec="$(phase_time recovered)"
  e="$(phase_time end)"
  [ -n "${b}" ] && [ -n "${be}" ] && [ -n "${ru}" ] && [ -n "${p}" ] && [ -n "${r}" ] && [ -n "${rec}" ] && [ -n "${e}" ] ||
    fail "phase marks are incomplete: $(tr '\n' ' ' <"${STATE_DIR}/phases")"
  # The baseline ends at baseline-end: the resize and the start of the stress pods belong to the ramp-up.
  check_probe_infrastructure "${b}" "${be}"
  # The summary below runs in a subshell, so the list is collected here for the final note too.
  foreign="$(foreign_webhooks "${b}" "${e}")"

  {
    echo "== CPU starvation summary: target ${TARGET}, mode ${MODE}, CPU limit ${CPU_LIMIT}"
    echo "phases: baseline $((be - b))s, ramp-up $((p - ru))s, pressure $((r - p))s, recovery $((rec - r))s (budget ${RECOVERY}s), after $((e - r))s"

    echo
    echo "-- Admission probes (failure budget ${MAX_FAILURE_PERCENT}%, at least ${MIN_PROBES} allowed probes under pressure and after)"
    echo "   ramp-up: from the resize or the start of the stress pods until all of them run; same budget, no minimum count"
    for f in "baseline ${b} ${be}" "ramp-up ${ru} ${p}" "pressure ${p} ${r}" "after ${rec} ${e}"; do
      # shellcheck disable=SC2086
      set -- ${f}
      awk -v ph="$1" -v from="$2" -v to="$3" -v max="${MAX_FAILURE_PERCENT}" -v min="${MIN_PROBES}" '
        $1 >= from && $1 < to {
          if ($3 == "infra") { inf++; next }
          if ($3 == "unreachable") { unr++; next }
          # A failure in a webhook of another module says nothing about this module.
          if ($3 == "foreign-webhook") { fw++; next }
          if ($2 == "allowed") { at++; if ($3 != "ok") af++; if ($4 != "NA") lat[++nl] = $4 }
          if ($2 == "denied") { dt++; if ($3 == "allowed") da++; else if ($3 != "denied") de++ }
        }
        END {
          asort_lat(lat, nl)
          p50 = (nl > 0) ? lat[int((nl + 1) * 0.5)] "ms" : "n/a"
          p95 = (nl > 0) ? lat[int((nl - 1) * 0.95) + 1] "ms" : "n/a"
          bad = af + de; total = at + dt
          pct = (total > 0) ? bad * 100 / total : 0
          # Probes that never reached the API server count against the same budget, but apart.
          upct = (total + unr > 0) ? unr * 100 / (total + unr) : 0
          verdict = "ok"
          if (inf > 0) verdict = "FAIL: probe infrastructure broken (" inf " probes failed on the kubeconfig, authentication or RBAC)"
          else if (da > 0) verdict = "FAIL: the privileged Pod was admitted"
          else if (pct > max) verdict = "FAIL: admission failures above the budget"
          else if (ph != "baseline" && upct > max) verdict = "FAIL: API server unreachable under pressure (" unr " probes, " sprintf("%.2f", upct) "%)"
          else if (ph != "baseline" && ph != "ramp-up" && at < min) verdict = "FAIL: too few probes in the phase"
          if (verdict == "ok" && fw > 0) verdict = "ok, " fw " foreign webhook failures not counted"
          printf "%-8s allowed probe %4d/%-4d ok, deny probe %4d/%-4d denied (%d admitted, %d errors), failures %.2f%%, API server unreachable %d, foreign webhooks %d, latency p50 %s p95 %s: %s\n",
            ph, at - af, at, dt - da - de, dt, da, de, pct, unr, fw, p50, p95, verdict
        }
        function asort_lat(a, n,   i, j, t) {
          for (i = 2; i <= n; i++) { t = a[i]; for (j = i - 1; j >= 1 && a[j] + 0 > t + 0; j--) a[j + 1] = a[j]; a[j + 1] = t }
        }' "${STATE_DIR}/probes.log"
    done
    echo "first admission errors from the ramp-up on:"
    awk -v from="${ru}" -v to="${e}" '$1 >= from && $1 < to && $3 == "error" && n++ < 5 { print "  " $0 }' "${STATE_DIR}/probes.log"
    if [ -n "${foreign}" ]; then
      echo "failures in webhooks of other modules (not counted against the module, check those webhooks):"
      printf '%s\n' "${foreign}" | sed 's/^/  /'
    fi
    for f in infra unreachable foreign-webhook; do
      if [ -f "${STATE_DIR}/probe-${f}-error.txt" ]; then
        echo "first probe ${f} error, in full:"
        sed 's/^/  /' "${STATE_DIR}/probe-${f}-error.txt"
      fi
    done

    echo
    echo "-- Write storm (MODIFIED events; storm = above 2x baseline and above baseline + 10/min)"
    for name in leases constraintpodstatuses constrainttemplatepodstatuses; do
      log="${STATE_DIR}/watch-${name}.log"
      touch "${log}"
      base_n="$(count_between "${log}" "${b}" "${be}" MODIFIED)"
      base_rate="$(awk -v c="${base_n}" -v s="$((be - b))" 'BEGIN { printf "%.4f", (s > 0) ? c * 60 / s : 0 }')"
      rate_check "${name} baseline" "${base_n}" "$((be - b))" "${base_rate}" || true
      n_u="$(count_between "${log}" "${ru}" "${p}" MODIFIED)"
      rate_check "${name} ramp-up" "${n_u}" "$((p - ru))" "${base_rate}" || echo "STORM ${name} ramp-up" >>"${STATE_DIR}/failures"
      n_p="$(count_between "${log}" "${p}" "${r}" MODIFIED)"
      rate_check "${name} pressure" "${n_p}" "$((r - p))" "${base_rate}" || echo "STORM ${name} pressure" >>"${STATE_DIR}/failures"
      n_a="$(count_between "${log}" "${r}" "${e}" MODIFIED)"
      rate_check "${name} after" "${n_a}" "$((e - r))" "${base_rate}" || echo "STORM ${name} after" >>"${STATE_DIR}/failures"
      # A broken watch reports zero events and would hide a storm.
      if [ "$(awk '$2 != 0 { n++ } END { printf "%d\n", n }' "${STATE_DIR}/watch-${name}.exits" 2>/dev/null || echo 0)" -gt 10 ]; then
        echo "  ${name}: the watch failed repeatedly; first errors:"
        awk 'NF && n++ < 3 { print "    " $0 }' "${STATE_DIR}/watch-${name}.err" 2>/dev/null || true
        echo "observer broken: the ${name} watch failed repeatedly" >>"${STATE_DIR}/failures"
      fi
    done

    echo
    echo "-- ValidatingWebhookConfiguration ${VWC_NAME}"
    rc=0
    read -r vwc_uid vwc_hooks <"${STATE_DIR}/vwc-before"
    awk -v uid="${vwc_uid}" -v hooks="${vwc_hooks}" -v max="${MAX_FAILURE_PERCENT}" '
      { polls++ }
      $2 == "present" { present++; if ($3 != uid) replaced++; if ($4 + 0 < hooks + 0) fewer++ }
      $2 == "missing" { missing++ }
      $2 == "error" || $2 == "foreign-webhook" { errors++; if (first_error == "") first_error = $0 }
      $2 == "unreachable" { unreachable++; if (first_error == "") first_error = $0 }
      $2 == "infra" { infra++; if (first == "") first = $0 }
      END {
        printf "polls: %d present, %d missing, %d recreated (other uid), %d with fewer webhooks, %d API errors, %d with the API server unreachable, %d poll infrastructure errors\n",
          present, missing, replaced, fewer, errors, unreachable, infra
        if (infra > 0 || present == 0) {
          print "  the poll is broken, so the check proves nothing; first error: " first
          exit 2
        }
        if (missing + replaced + fewer > 0) exit 1
        # A poll that got no answer proves nothing about the configuration at that moment.
        if ((errors + unreachable) * 100 / polls > max) {
          printf "  %.2f%% of the polls got no answer, above the %s%% budget; first such poll: %s\n",
            (errors + unreachable) * 100 / polls, max, first_error
          exit 3
        }
      }' "${STATE_DIR}/vwc.log" || rc=$?
    case "${rc}" in
      0) ;;
      2) echo "observer broken: the ValidatingWebhookConfiguration poll could not reach the API server" >>"${STATE_DIR}/failures" ;;
      3) echo "the ValidatingWebhookConfiguration poll got API errors or no answer above the budget" >>"${STATE_DIR}/failures" ;;
      *) echo "VWC disappeared or changed" >>"${STATE_DIR}/failures" ;;
    esac

    echo
    echo "-- Restarts and pod replacements in ${MODULE_NS}"
    pods_after="$(module_pods_snapshot)"
    printf '%s\n' "${pods_after}" >"${STATE_DIR}/pods-after"
    awk 'NR == FNR { after[$2] = $3; next }
      {
        if (!($2 in after)) { print "  " $1 ": replaced or gone"; bad++ }
        else if (after[$2] != $3) { print "  " $1 ": restarts " $3 " -> " after[$2]; bad++ }
      }
      END { if (!bad) print "  no restarts, no replaced pods"; exit (bad > 0) }' \
      "${STATE_DIR}/pods-after" "${STATE_DIR}/pods-before" || echo "module pods restarted or were replaced" >>"${STATE_DIR}/failures"
    leases_after="$(leases_snapshot)"
    if [ "${leases_after}" = "$(cat "${STATE_DIR}/leases-before")" ]; then
      echo "  leases: unchanged ($(grep -c . "${STATE_DIR}/leases-before" || true) in the namespace)"
    else
      echo "  leases changed:"
      printf '%s\n' "${leases_after}" | sed 's/^/    /'
      echo "lease holder or transitions changed" >>"${STATE_DIR}/failures"
    fi

    echo
    echo "-- Audit"
    if [ -f "${STATE_DIR}/audit-after-release" ]; then
      echo "  a cycle completed $(awk '{ print $2 }' "${STATE_DIR}/audit-after-release")s after the release (budget ${AUDIT_RECOVERY}s)"
    else
      echo "  no audit cycle completed within ${AUDIT_RECOVERY}s after the release"
      echo "audit did not run after the release" >>"${STATE_DIR}/failures"
    fi
    echo "  last cycle before the pressure ended at epoch $(cat "${STATE_DIR}/audit-at-release" 2>/dev/null || echo unknown); pressure started at ${p}"

    echo
    echo "-- CPU received under pressure (informational, kubelet counters)"
    awk '$1 == "pressure" { t[$3] = $2; n[$3] = $5; c[$3] = $6; w[$3] = $7; node[$3] = $4 }
      $1 == "release" && ($3 in t) {
        d = $2 - t[$3]; if (d <= 0) next
        printf "  node %s: %s cores busy; %s/%s: %s cores used, waited for CPU %s of the time\n", node[$3],
          ($5 == "NA" || n[$3] == "NA") ? "n/a" : sprintf("%.2f", ($5 - n[$3]) / d), $3, "manager",
          ($6 == "NA" || c[$3] == "NA") ? "n/a" : sprintf("%.3f", ($6 - c[$3]) / d),
          ($7 == "NA" || w[$3] == "NA") ? "n/a" : sprintf("%.0f%%", ($7 - w[$3]) * 100 / d)
      }' "${STATE_DIR}/cpu-samples" 2>/dev/null || true
  } | tee "${STATE_DIR}/summary.txt"

  # The probe verdicts are printed by awk; collect them here.
  if grep -q ': FAIL' "${STATE_DIR}/summary.txt"; then
    grep ': FAIL' "${STATE_DIR}/summary.txt" | sed 's/^/probes: /' >>"${STATE_DIR}/failures"
  fi
  if [ -n "${foreign}" ]; then
    echo
    echo "== NOTE: probes failed in webhooks of other modules; these failures do not fail the test:"
    printf '%s\n' "${foreign}" | sed 's/^/  /'
  fi
  if [ -s "${STATE_DIR}/failures" ]; then
    echo
    echo "== FAILED:"
    sed 's/^/  /' "${STATE_DIR}/failures"
    exit 1
  fi
  echo
  echo "== PASSED"
}

# ---------------------------------------------------------------------------------------------
# Cleanup after an interrupted run.
# ---------------------------------------------------------------------------------------------

# cleanup_state <state dir> undoes what one run left: it stops its observers, deletes the
# kubeconfig copy and the stress Deployments, restores the VPA update mode and the resized
# container, and deletes the Chainsaw namespace.
cleanup_state() {
  STATE_DIR="$1"
  NAMESPACE="${1##*/state-}"
  CONFIG_FILE="${STATE_DIR}/config.env"
  log "cleaning up the run in namespace ${NAMESPACE} (${STATE_DIR})"
  cmd_stop_observers
  if kubectl get namespace "${NAMESPACE}" -o name >/dev/null 2>&1; then
    delete_stress
  fi
  cmd_restore_vpa
  [ ! -f "${CONFIG_FILE}" ] || cmd_restore_resize
  case "${NAMESPACE}" in
    chainsaw-*) kubectl delete namespace "${NAMESPACE}" --ignore-not-found --wait=false ;;
  esac
}

# cleanup_sweep finds what the test changed in the cluster by labels and annotations, for runs
# whose state directory is gone.
cleanup_sweep() {
  local out ns name mode pod deleting deployment restarted=" "
  out="$(kubectl get deployments -A -l app=ape-cpu-stress,e2e-test=ape-cpu-starvation \
    -o jsonpath='{range .items[*]}{.metadata.namespace}{" "}{.metadata.name}{"\n"}{end}')"
  while read -r ns name; do
    [ -n "${ns}" ] || continue
    log "deleting the stress Deployment ${ns}/${name}"
    kubectl -n "${ns}" delete deployment "${name}" --ignore-not-found --wait=false
  done <<EOF_STRESS
${out}
EOF_STRESS

  out="$(kubectl -n "${MODULE_NS}" get verticalpodautoscalers.autoscaling.k8s.io \
    -o jsonpath='{range .items[*]}{.metadata.name}{" "}{.metadata.annotations.ape-cpu-starvation\.e2e\.deckhouse\.io/update-mode}{"\n"}{end}' \
    2>/dev/null)" || out=""
  while read -r name mode; do
    [ -n "${mode:-}" ] || continue
    [ "${mode}" != "-" ] || mode=""
    restore_vpa_mode "${name}" "${mode}" || log "WARNING: VPA ${name}: cannot restore updateMode ${mode:-<default>}"
  done <<EOF_VPA
${out}
EOF_VPA

  out="$(kubectl -n "${MODULE_NS}" get pods \
    -o jsonpath='{range .items[*]}{.metadata.name}{" deleting="}{.metadata.deletionTimestamp}{" resized="}{.metadata.annotations.ape-cpu-starvation\.e2e\.deckhouse\.io/resized}{"\n"}{end}')"
  while read -r pod deleting mode; do
    # A pod that is already being replaced needs no restart.
    [ "${deleting:-}" = "deleting=" ] && [ -n "${mode#resized=}" ] || continue
    deployment="$(deployment_of_pod "${pod}")"
    [ -n "${deployment}" ] || continue
    case "${restarted}" in *" ${deployment} "*) continue ;; esac
    log "pod ${pod} still has the resources set by the test"
    restart_deployment "${deployment}"
    restarted="${restarted}${deployment} "
  done <<EOF_PODS
${out}
EOF_PODS
}

# cmd_cleanup <scenario dir>... cleans up every run recorded in <scenario dir>/reports/state-*
# and then everything the labels and annotations still point to.
cmd_cleanup() {
  local scenario state dir
  [ "$#" -gt 0 ] || set -- .
  for scenario in "$@"; do
    for state in "${scenario%/}"/reports/state-*; do
      [ -d "${state}" ] && [ ! -f "${state}/cleaned" ] || continue
      if (cleanup_state "${state}"); then
        touch "${state}/cleaned"
      else
        log "WARNING: the cleanup of ${state} did not complete; run it again"
      fi
    done
  done
  cleanup_sweep
  # Kubeconfig copies whose state directory is gone.
  for dir in "${TMPDIR:-/tmp}"/ape-cpu-starvation.*; do
    [ -d "${dir}" ] || continue
    [ "$(find "${dir}" -mindepth 1 ! -name kubeconfig | head -n 1)" = "" ] || continue
    log "deleting the kubeconfig copy ${dir}"
    rm -rf "${dir}"
  done
  log "cleanup done; check that nothing is left:"
  kubectl get pods -A -l app=ape-cpu-stress
}

if [ "${1:-}" != "cleanup" ]; then
  init_state
fi
case "${1:-}" in
  _probe_loop | _watch_raw | _vwc_poll | _reaper)
    OBSERVER_ROLE="${1#_}"
    # bash runs the EXIT trap also when stop-observers kills the observer with SIGTERM.
    trap on_observer_exit EXIT
    ;;
esac

case "${1:-}" in
  preflight) cmd_preflight ;;
  start-observers) cmd_start_observers ;;
  stop-observers) cmd_stop_observers ;;
  baseline) cmd_baseline ;;
  freeze-vpa) cmd_freeze_vpa ;;
  restore-vpa) cmd_restore_vpa ;;
  resize) cmd_resize ;;
  restore-resize) cmd_restore_resize ;;
  stress-start) cmd_stress_start ;;
  stress-delete) cmd_stress_delete ;;
  hold) cmd_hold ;;
  release) cmd_release ;;
  after) cmd_after ;;
  analyse) cmd_analyse ;;
  cleanup)
    shift
    cmd_cleanup "$@"
    ;;
  _probe_loop) _probe_loop ;;
  _watch_raw) _watch_raw "$2" "$3" ;;
  _vwc_poll) _vwc_poll ;;
  _reaper) _reaper ;;
  *)
    echo "usage: $0 {preflight|freeze-vpa|start-observers|baseline|resize|stress-start|hold|release|after|analyse|stop-observers|stress-delete|restore-resize|restore-vpa|cleanup [<scenario dir>...]}" >&2
    exit 2
    ;;
esac
