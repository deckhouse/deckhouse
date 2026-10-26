#!/bin/sh

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

# Helper commands of the cert-manager CPU starvation scenarios.
#
# Chainsaw runs each command from the scenario directory with NAMESPACE set to the test namespace.
# Chainsaw outputs do not survive a step, so the commands keep the state of a run (target pod,
# phase marks, watch logs, snapshots) in files under STATE_DIR, which is reports/state-<NAMESPACE>
# in the scenario directory (reports/ is in .gitignore). The cleanup of the test removes STATE_DIR
# after it prints the summary; set CPU_STARVATION_KEEP_STATE=true to keep it.
#
# Usage: e2e.sh <COMMAND> [ARGUMENTS...]; see the case statement at the end for the commands.

set -eu

CM_NS=d8-cert-manager
ISSUER=e2e-cpu-starvation-selfsigned
STRESS=cpu-stress
# The label of the suite on the stress Deployment and the test namespace; `task cleanup` deletes only
# objects with it.
SUITE_LABEL_KEY=e2e-test
SUITE_LABEL_VALUE=cert-manager-cpu-starvation
STRESS_SELECTOR="app=$STRESS,$SUITE_LABEL_KEY=$SUITE_LABEL_VALUE"
SCRIPTS_DIR=$(cd "$(dirname "$0")" && pwd)
COMMON_DIR=$(dirname "$SCRIPTS_DIR")

: "${NAMESPACE:?NAMESPACE must be set; run the command from a Chainsaw test}"
STATE_DIR=$(pwd)/reports/state-$NAMESPACE
(
  umask 077
  mkdir -p "$STATE_DIR"
)
chmod 700 "$STATE_DIR"
# Chainsaw gives every script operation a temporary kubeconfig in KUBECONFIG and deletes it when the
# operation ends. The background processes outlive the operation, so they use a copy of it. The copy
# holds credentials, so it lives in a private temporary directory outside reports/; the file
# kubeconfig-path in STATE_DIR keeps its path. The prefix is also known to cleanup-leftovers.sh.
KUBECONFIG_DIR_PREFIX=${TMPDIR:-/tmp}
KUBECONFIG_DIR_PREFIX=${KUBECONFIG_DIR_PREFIX%/}/cert-manager-cpu-starvation.
# The background processes stop when their pid file disappears or at this deadline (epoch seconds),
# so an interrupted run that never reached the cleanup does not leave them running forever.
BG_DEADLINE=$STATE_DIR/bg-deadline

# Every API request has a deadline: without one, a request to an API server or a webhook that is short
# of CPU can hang until Chainsaw kills the shell, and then the step fails without any message.
REQUEST_TIMEOUT=${CPU_STARVATION_REQUEST_TIMEOUT:-60s}
RETRIES=${CPU_STARVATION_RETRIES:-5}

# Report every non-zero exit, so a command that fails under `set -e` never ends a step silently.
COMMAND_LINE="$*"
on_exit() {
  RC=$?
  if [ "$RC" -ne 0 ]; then
    printf '%s %s\n' "$(date -u +%H:%M:%S)" "e2e.sh $COMMAND_LINE exited with status $RC" >&2
  fi
}
trap on_exit EXIT

# Converts a Kubernetes CPU quantity (2, 0.5, 250m, 1500u, 3n) to millicores.
AWK_MC='
function mc(q) {
  if (q == "") return 0
  if (q ~ /m$/) { sub(/m$/, "", q); return q + 0 }
  if (q ~ /u$/) { sub(/u$/, "", q); return q / 1000 }
  if (q ~ /n$/) { sub(/n$/, "", q); return q / 1000000 }
  return q * 1000
}'

log() { printf '%s %s\n' "$(date -u +%H:%M:%S)" "$*" >&2; }
fail() {
  log "FAIL: $*"
  exit 1
}
now() { date +%s; }
state_get() { cat "$STATE_DIR/$1" 2>/dev/null || true; }
state_set() { printf '%s\n' "$2" >"$STATE_DIR/$1"; }
# kc runs kubectl with the request deadline.
kc() { kubectl --request-timeout="$REQUEST_TIMEOUT" "$@"; }

# INFRA_ERRORS matches kubectl errors that show the request never reached the API server or was
# rejected before admission: a missing kubeconfig (kubectl falls back to localhost:8080), no
# connection, failed authentication or authorization. Such an error is a failure of the test
# infrastructure, never a failure of the module.
INFRA_ERRORS='localhost:8080|no configuration has been provided|connection refused|Unable to connect to the server|no such host|couldn.t get current server API group list|provide credentials|Unauthorized|is forbidden|certificate signed by unknown authority'

# is_infra_error TEXT succeeds when TEXT is an infrastructure error. An answer that mentions a webhook
# came through admission, whatever went wrong there, so it is not an infrastructure error.
is_infra_error() {
  if printf '%s\n' "$1" | grep -q 'webhook'; then
    return 1
  fi
  printf '%s\n' "$1" | grep -qE "$INFRA_ERRORS"
}

# first_infra_error FILE prints the first infrastructure error in FILE, if any.
first_infra_error() {
  [ -f "$1" ] || return 0
  grep -v 'webhook' "$1" | grep -E "$INFRA_ERRORS" | head -n 1 || true
}

# retry DESCRIPTION COMMAND...: runs COMMAND up to RETRIES times and prints its stdout once it succeeds.
# The stderr of every failed attempt goes to the step log; after the last attempt the script fails
# with the last error. Use it only for idempotent commands (get, apply, delete --ignore-not-found).
retry() {
  R_WHAT=$1
  shift
  R_ERR=$STATE_DIR/retry.$$.err
  R_ATTEMPT=1
  while :; do
    R_OUT=$("$@" 2>"$R_ERR") && R_RC=0 || R_RC=$?
    if [ "$R_RC" -eq 0 ]; then
      if [ -s "$R_ERR" ]; then cat "$R_ERR" >&2; fi
      rm -f "$R_ERR"
      if [ -n "$R_OUT" ]; then printf '%s\n' "$R_OUT"; fi
      return 0
    fi
    R_LAST=$(tr '\n' ' ' <"$R_ERR")
    log "$R_WHAT: attempt $R_ATTEMPT of $RETRIES failed with status $R_RC: ${R_LAST:-no error output}"
    if [ "$R_ATTEMPT" -ge "$RETRIES" ]; then
      rm -f "$R_ERR"
      fail "$R_WHAT failed $RETRIES times; last error: ${R_LAST:-status $R_RC}"
    fi
    sleep $((R_ATTEMPT * 3))
    R_ATTEMPT=$((R_ATTEMPT + 1))
  done
}

millicores() { awk -v q="$1" "$AWK_MC"' BEGIN { printf "%d\n", mc(q) }'; }

# pod_requests reads the pod lines of stress-params on stdin and prints the sum of their effective
# CPU requests in millicores.
pod_requests() {
  awk "$AWK_MC"'
    {
      c = 0; sc = 0; i = 0; o = 0
      for (f = 2; f <= NF; f++) {
        t = substr($f, 1, 1); v = substr($f, 2)
        if (t == "c") c += mc(v)
        else if (t == "i") {
          p = v; sub(/:.*/, "", p); sub(/^[^:]*:/, "", v); v = mc(v)
          if (p == "Always") { sc += v; if (sc > i) i = sc }
          else if (sc + v > i) i = sc + v
        }
        else if (t == "o") o += mc(v)
      }
      s += (c + sc > i ? c + sc : i) + o
    }
    END { printf "%d\n", s }'
}
line_count() { wc -l <"$1" | tr -d ' '; }
require_state() {
  VALUE=$(state_get "$1")
  [ -n "$VALUE" ] || fail "state $1 is missing in $STATE_DIR; an earlier step did not run"
  printf '%s\n' "$VALUE"
}

# mark_field PHASE FIELD prints a field of a phase mark: 2 = epoch, 3 = lease events, 4 = certificate events.
mark_field() {
  VALUE=$(awk -v p="$1" -v f="$2" '$1 == p { print $f; exit }' "$STATE_DIR/marks" 2>/dev/null || true)
  [ -n "$VALUE" ] || fail "phase mark $1 is missing"
  printf '%s\n' "$VALUE"
}

# --- target selection and snapshots ----------------------------------------------------------

# label-namespace: marks the test namespace with the label of the suite, so `task cleanup` can find
# it after an interrupted run without touching the namespaces of other suites.
cmd_label_namespace() {
  retry "label namespace $NAMESPACE" kc label namespace "$NAMESPACE" --overwrite \
    "$SUITE_LABEL_KEY=$SUITE_LABEL_VALUE" >&2
}

# target controller|webhook: stores the pod, node, Deployment and container to put under pressure.
# For the controller it is the pod that holds the leader lease, because only the leader works.
cmd_target() {
  case $1 in
    controller)
      DEPLOY=cert-manager
      CONTAINER=cert-manager
      HOLDER=$(retry "read the controller lease" kc -n "$CM_NS" get lease cert-manager-controller \
        -o jsonpath='{.spec.holderIdentity}')
      ;;
    webhook)
      DEPLOY=webhook
      CONTAINER=webhook
      HOLDER=
      ;;
    *) fail "unknown target $1" ;;
  esac
  PODS=$(retry "list the $DEPLOY pods" kc -n "$CM_NS" get pods -l "app=$DEPLOY" \
    --field-selector=status.phase=Running -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}')
  [ -n "$PODS" ] || fail "no running pod with app=$DEPLOY in $CM_NS"
  POD=
  if [ "$1" = controller ]; then
    [ -n "$HOLDER" ] || fail "lease $CM_NS/cert-manager-controller has no holder; the controller has no leader"
    for P in $PODS; do
      case $HOLDER in "$P"-*) POD=$P ;; esac
    done
    # Another pod would be the wrong target: only the leader works, so the pressure would test nothing.
    [ -n "$POD" ] || fail "no running pod with app=$DEPLOY in $CM_NS holds the lease cert-manager-controller" \
      "(holder: $HOLDER, running pods: $(printf '%s' "$PODS" | tr '\n' ' '))"
  else
    POD=$(printf '%s\n' "$PODS" | head -n 1)
  fi
  NODE=$(retry "read the node of $POD" kc -n "$CM_NS" get pod "$POD" -o jsonpath='{.spec.nodeName}')
  [ -n "$NODE" ] || fail "pod $CM_NS/$POD has no node"
  state_set target-deploy "$DEPLOY"
  state_set target-container "$CONTAINER"
  state_set target-pod "$POD"
  state_set target-node "$NODE"
  log "target $1: pod $CM_NS/$POD on node $NODE (lease holder: ${HOLDER:-none})"
  ROLES=$(kc get node "$NODE" -o jsonpath='{.metadata.labels}' 2>/dev/null | tr ',' '\n' | grep node-role || true)
  log "roles of node $NODE: $(printf '%s' "$ROLES" | tr '\n' ' ')"
}

# snapshot NAME: records restart counts of all running pods of the module and the state of its leases.
# Each line is "pod <POD> <CONTAINER> restarts=<N>" or "lease <LEASE> holder=<H> transitions=<N>".
# The API is read first and the output parsed afterwards, so a failed read fails the command instead of
# leaving a section of the snapshot empty.
cmd_snapshot() {
  PODS=$(retry "list the pods of $CM_NS" kc -n "$CM_NS" get pods --field-selector=status.phase=Running \
    -o jsonpath='{range .items[*]}{.metadata.name}{range .status.containerStatuses[*]}{" "}{.name}{"="}{.restartCount}{end}{"\n"}{end}')
  LEASES=$(retry "list the leases of $CM_NS" kc -n "$CM_NS" get leases \
    -o jsonpath='{range .items[*]}{"lease "}{.metadata.name}{" holder="}{.spec.holderIdentity}{" transitions="}{.spec.leaseTransitions}{"\n"}{end}')
  [ -n "$PODS" ] || fail "no running pods in $CM_NS"
  {
    printf '%s\n' "$PODS" | awk 'NF { for (i = 2; i <= NF; i++) { split($i, c, "="); print "pod " $1 " " c[1] " restarts=" c[2] } }'
    printf '%s\n' "$LEASES" | awk 'NF'
  } | sort >"$STATE_DIR/snapshot-$1"
  log "snapshot $1:"
  cat "$STATE_DIR/snapshot-$1" >&2
}

# compare BEFORE AFTER: fails when a container restarted, a pod was replaced or a lease changed hands.
# Every change is printed on its own line, naming the container that restarted.
cmd_compare() {
  CHANGES=$(awk '
    FNR == NR { key = $1 " " $2 " " $3; if ($1 == "lease") key = $1 " " $2; before[key] = $0; next }
    {
      key = $1 " " $2 " " $3; if ($1 == "lease") key = $1 " " $2
      seen[key] = 1
      if (!(key in before)) {
        if ($1 == "pod") print "new pod container (a pod was replaced or added): pod " $2 " container " $3 " " $4
        else print "new lease: " $0
      } else if (before[key] != $0) {
        if ($1 == "pod") {
          split(before[key], b, " ")
          print "container " $3 " of pod " $2 " restarted: " b[4] " -> " $4
        } else print "lease " $2 " changed: " substr(before[key], length($1 " " $2) + 2) " -> " substr($0, length($1 " " $2) + 2)
      }
    }
    END {
      for (key in before) if (!(key in seen)) {
        split(before[key], b, " ")
        if (b[1] == "pod") print "pod " b[2] " (container " b[3] ") is gone: it was deleted, replaced or stopped running"
        else print "lease " b[2] " is gone"
      }
    }' "$STATE_DIR/snapshot-$1" "$STATE_DIR/snapshot-$2")
  if [ -n "$CHANGES" ]; then
    printf '%s\n' "$CHANGES" | while IFS= read -r LINE; do log "CHANGED: $LINE"; done
    fail "restart counts, pods or leases of $CM_NS changed between $1 and $2"
  fi
  log "no restarts, no replaced pods, no lease transitions between $1 and $2"
}

# --- background processes ---------------------------------------------------------------------

# kubeconfig_path prints the path of the kubeconfig copy of the background processes, if any.
kubeconfig_path() { state_get kubeconfig-path; }

# save_kubeconfig copies the kubeconfig of the current operation for the background processes and
# checks that the copy works. The copy holds credentials: it goes to a private directory made by
# mktemp outside reports/, is readable only by the user, and is removed when the background processes
# stop. It is written atomically and a second call of the run reuses the directory, so a running
# background process never reads a half-written or a deleted file.
save_kubeconfig() {
  BG_KUBECONFIG=$(kubeconfig_path)
  if [ -z "$BG_KUBECONFIG" ] || [ ! -d "$(dirname "$BG_KUBECONFIG")" ]; then
    BG_DIR=$(mktemp -d "${KUBECONFIG_DIR_PREFIX}XXXXXX") ||
      fail "cannot create a temporary directory for the kubeconfig copy"
    chmod 700 "$BG_DIR"
    BG_KUBECONFIG=$BG_DIR/kubeconfig
    state_set kubeconfig-path "$BG_KUBECONFIG"
  fi
  (
    umask 077
    kubectl config view --raw --minify --flatten >"$BG_KUBECONFIG.tmp"
  ) || fail "cannot copy the kubeconfig of the step for the background processes"
  mv -f "$BG_KUBECONFIG.tmp" "$BG_KUBECONFIG"
  if ! ERR=$(KUBECONFIG=$BG_KUBECONFIG kubectl --request-timeout=20s get --raw /version 2>&1 >/dev/null); then
    fail "the copy of the kubeconfig for the background processes does not work: $(printf '%s' "$ERR" | tr '\n' ' ')"
  fi
}

# remove_kubeconfig deletes the kubeconfig copy together with its directory.
remove_kubeconfig() {
  BG_KUBECONFIG=$(kubeconfig_path)
  [ -n "$BG_KUBECONFIG" ] || return 0
  BG_DIR=$(dirname "$BG_KUBECONFIG")
  # Delete only a directory that save_kubeconfig created.
  case $BG_DIR in
    "$KUBECONFIG_DIR_PREFIX"*) rm -rf "$BG_DIR" ;;
    *) log "not removing $BG_DIR: it was not created by save_kubeconfig" ;;
  esac
  rm -f "$STATE_DIR/kubeconfig-path"
}

# set_bg_deadline HOLD BUDGET RECOVERY AFTER writes the deadline of the background processes, unless
# an earlier command of the run wrote it. It covers the longest run of the phases plus one hour.
set_bg_deadline() {
  [ -s "$BG_DEADLINE" ] && return 0
  BG_SECONDS=$((2 * $1 + 2 * $2 + 2 * $3 + $4 + 3600))
  echo $(($(now) + BG_SECONDS)) >"$BG_DEADLINE"
  log "background processes stop in ${BG_SECONDS}s at the latest"
}

# bg_running PID_FILE succeeds while a background process must keep running.
bg_running() {
  [ -f "$1" ] && [ "$(now)" -lt "$(cat "$BG_DEADLINE" 2>/dev/null || echo 0)" ]
}

# kill_tree PID stops a process together with its children (kubectl started by a loop).
kill_tree() {
  for CHILD_PID in $(pgrep -P "$1" 2>/dev/null || true); do
    kill_tree "$CHILD_PID"
  done
  kill "$1" 2>/dev/null || true
}

# bg-start NAME COMMAND...: runs COMMAND detached from the step with the copy of the kubeconfig.
# The pid file exists before COMMAND starts, so COMMAND may poll it with bg_running from the start.
cmd_bg_start() {
  NAME=$1
  shift
  BG_KUBECONFIG=$(kubeconfig_path)
  [ -n "$BG_KUBECONFIG" ] && [ -s "$BG_KUBECONFIG" ] ||
    fail "no kubeconfig for the background processes; save_kubeconfig did not run"
  [ -s "$BG_DEADLINE" ] || fail "no deadline for the background processes; set_bg_deadline did not run"
  PID_FILE=$STATE_DIR/bg-$NAME.pid
  : >"$PID_FILE"
  KUBECONFIG=$BG_KUBECONFIG BG_PID_FILE=$PID_FILE BG_DEADLINE=$BG_DEADLINE \
    "$@" >"$STATE_DIR/bg-$NAME.out" 2>&1 </dev/null &
  echo "$!" >"$PID_FILE"
  log "started $NAME (pid $!)"
}

# bg-stop NAME: removes the pid file, which ends the loop of the process, and kills the process tree.
cmd_bg_stop() {
  PID_FILE=$STATE_DIR/bg-$1.pid
  [ -f "$PID_FILE" ] || return 0
  PID=$(cat "$PID_FILE")
  rm -f "$PID_FILE"
  [ -n "$PID" ] && kill_tree "$PID"
  log "stopped $1"
}

cmd_bg_stop_all() {
  for PID_FILE in "$STATE_DIR"/bg-*.pid; do
    [ -f "$PID_FILE" ] || continue
    NAME=${PID_FILE##*/bg-}
    cmd_bg_stop "${NAME%.pid}"
  done
  remove_kubeconfig
}

# --- write-storm check --------------------------------------------------------------------------

# watch-start HOLD BUDGET RECOVERY AFTER: watches the leases of the module and the Certificates of the
# test namespace. The arguments are the phase durations of the test; they set the deadline of the watches.
cmd_watch_start() {
  [ "$#" -eq 4 ] || fail "usage: watch-start HOLD BUDGET RECOVERY AFTER"
  set_bg_deadline "$@"
  save_kubeconfig
  : >"$STATE_DIR/events-leases.log"
  : >"$STATE_DIR/events-certificates.log"
  : >"$STATE_DIR/events-leases.err"
  : >"$STATE_DIR/events-certificates.err"
  cmd_bg_start watch-leases sh "$SCRIPTS_DIR/watch-writes.sh" "$STATE_DIR/events-leases.log" \
    leases -n "$CM_NS"
  cmd_bg_start watch-certificates sh "$SCRIPTS_DIR/watch-writes.sh" "$STATE_DIR/events-certificates.log" \
    certificates.cert-manager.io -n "$NAMESPACE"
  # Give both watches time to connect, so the baseline window does not start with a gap.
  sleep 5
  check_watch_errors
}

# check_watch_errors fails when a watch could not reach the API server: then the event logs are
# empty because of the test infrastructure, not because the module wrote nothing.
check_watch_errors() {
  for RES in leases certificates; do
    ERR=$(first_infra_error "$STATE_DIR/events-$RES.err")
    [ -z "$ERR" ] || fail "infrastructure failure: the watch of $RES cannot reach the API server: $ERR"
  done
}

# mark PHASE: records the moment a phase starts and how many watch events were seen until then.
# The first mark of a phase wins: the high-load-average scenarios mark the pressure before the
# resize of the controller, and the pressure step template marks it again, which is then ignored.
cmd_mark() {
  if awk -v p="$1" '$1 == p { found = 1 } END { exit !found }' "$STATE_DIR/marks" 2>/dev/null; then
    log "phase $1 started earlier, keeping its first mark"
    return 0
  fi
  [ -f "$STATE_DIR/events-leases.log" ] || : >"$STATE_DIR/events-leases.log"
  [ -f "$STATE_DIR/events-certificates.log" ] || : >"$STATE_DIR/events-certificates.log"
  echo "$1 $(now) $(line_count "$STATE_DIR/events-leases.log") $(line_count "$STATE_DIR/events-certificates.log")" \
    >>"$STATE_DIR/marks"
  log "phase $1 starts"
}

# wait-window PHASE SECONDS: sleeps until SECONDS have passed since the mark of PHASE.
cmd_wait_window() {
  END=$(($(mark_field "$1" 2) + $2))
  LEFT=$((END - $(now)))
  if [ "$LEFT" -gt 0 ]; then
    log "waiting ${LEFT}s until the $1 window of $2s ends"
    sleep "$LEFT"
  fi
}

# writes-check COUNT: compares MODIFIED events per minute under pressure and after it with the baseline.
# The windows are [baseline, baseline-end), [pressure, release) and [release, end): the pressure
# window starts before the stress pods are created and the controller is resized, so their start-up
# never counts as the baseline.
# A window fails when its rate is more than twice the baseline and more than the baseline + 10/min.
# The baseline must show that the watches work: the leader renews the controller lease every 5 seconds,
# so the leases need at least one MODIFIED event, and the COUNT Certificates of the baseline burst
# need at least COUNT MODIFIED events (each one gets its status set at least once).
cmd_writes_check() {
  [ "$#" -eq 1 ] || fail "usage: writes-check COUNT"
  check_watch_errors
  RESULT=0
  INCOMPLETE=0
  printf '%-14s %-9s %8s %8s %10s\n' resource window events seconds per-minute >"$STATE_DIR/writes-summary"
  for RES in leases certificates; do
    FIELD=3
    [ "$RES" = certificates ] && FIELD=4
    BASE_RATE=
    for WINDOW in baseline:baseline-end pressure:release after:end; do
      FROM=${WINDOW%%:*}
      TO=${WINDOW##*:}
      [ "$FROM" = after ] && FROM=release
      T0=$(mark_field "$FROM" 2)
      T1=$(mark_field "$TO" 2)
      L0=$(mark_field "$FROM" "$FIELD")
      L1=$(mark_field "$TO" "$FIELD")
      COUNT=$(awk -v a="$L0" -v b="$L1" 'NR > a && NR <= b && $1 == "MODIFIED" { c++ } END { print c + 0 }' \
        "$STATE_DIR/events-$RES.log")
      SECONDS_=$((T1 - T0))
      [ "$SECONDS_" -gt 0 ] || SECONDS_=1
      RATE=$(awk -v c="$COUNT" -v s="$SECONDS_" 'BEGIN { printf "%.1f", c * 60 / s }')
      printf '%-14s %-9s %8s %8s %10s\n' "$RES" "${WINDOW%%:*}" "$COUNT" "$SECONDS_" "$RATE" \
        >>"$STATE_DIR/writes-summary"
      if [ -z "$BASE_RATE" ]; then
        BASE_RATE=$RATE
        MIN=1
        [ "$RES" = certificates ] && MIN=$1
        if [ "$COUNT" -lt "$MIN" ]; then
          echo "the baseline has $COUNT MODIFIED events of $RES, expected at least $MIN: the watch did not work" \
            >>"$STATE_DIR/writes-summary"
          INCOMPLETE=1
        fi
        continue
      fi
      if awk -v r="$RATE" -v b="$BASE_RATE" 'BEGIN { exit !(r > 2 * b && r > b + 10) }'; then
        echo "write storm: $RES ${WINDOW%%:*} $RATE/min against baseline $BASE_RATE/min" \
          >>"$STATE_DIR/writes-summary"
        RESULT=1
      fi
    done
  done
  cat "$STATE_DIR/writes-summary" >&2
  if [ "$INCOMPLETE" -ne 0 ]; then
    for RES in leases certificates; do
      if [ -s "$STATE_DIR/events-$RES.err" ]; then
        log "errors of the $RES watch:"
        tail -n 5 "$STATE_DIR/events-$RES.err" >&2
      fi
    done
    fail "the watch logs are incomplete, the write rates cannot be compared"
  fi
  [ "$RESULT" -eq 0 ] || fail "MODIFIED events grew under CPU pressure"
  log "no write storm"
}

# --- Certificates -------------------------------------------------------------------------------

# burst BATCH COUNT: creates COUNT Certificates labelled e2e-batch=BATCH in the test namespace.
cmd_burst() {
  BATCH=$1
  COUNT=$2
  MANIFEST=$STATE_DIR/burst-$BATCH.yaml
  I=1
  while [ "$I" -le "$COUNT" ]; do
    cat <<EOF
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: $BATCH-$I
  labels:
    e2e-batch: $BATCH
spec:
  secretName: $BATCH-$I-tls
  commonName: $BATCH-$I.example.test
  dnsNames:
    - $BATCH-$I.example.test
  issuerRef:
    kind: ClusterIssuer
    name: $ISSUER
EOF
    I=$((I + 1))
  done >"$MANIFEST"
  # The clock of the burst starts before the first apply: a slow API server is part of the result.
  state_set "burst-$BATCH-start" "$(now)"
  T0=$(now)
  retry "apply the $COUNT Certificates of batch $BATCH" kc -n "$NAMESPACE" apply -f "$MANIFEST" >/dev/null
  log "created $COUNT Certificates of batch $BATCH in $(($(now) - T0))s"
}

# ready_count BATCH prints the number of Ready Certificates of BATCH, or "?" when the API did not answer;
# the reason goes to the step log. The callers poll, so one failed read only skips a round.
ready_count() {
  if R_LIST=$(kc -n "$NAMESPACE" get certificates.cert-manager.io -l "e2e-batch=$1" \
    -o jsonpath='{range .items[*]}{.status.conditions[?(@.type=="Ready")].status}{"\n"}{end}' 2>"$STATE_DIR/ready.err"); then
    printf '%s\n' "$R_LIST" | grep -c True || true
  else
    log "cannot list the Certificates of batch $1: $(tr '\n' ' ' <"$STATE_DIR/ready.err")"
    echo "?"
  fi
}

# Records the peak CPU usage of the target container reported by the metrics API (informational,
# and the request of the high-load-average scenarios).
sample_usage() {
  POD=$(state_get target-pod)
  CONTAINER=$(state_get target-container)
  [ -n "$POD" ] || return 0
  USAGE=$(kc top pod -n "$CM_NS" "$POD" --containers --no-headers 2>/dev/null |
    awk -v c="$CONTAINER" '$2 == c { print $3 }')
  [ -n "$USAGE" ] || return 0
  USAGE=$(millicores "$USAGE")
  PEAK=$(state_get usage-peak)
  if [ -z "$PEAK" ] || [ "$USAGE" -gt "$PEAK" ]; then
    state_set usage-peak "$USAGE"
  fi
}

# burst-wait BATCH COUNT TIMEOUT [PHASE]: waits until COUNT Certificates of BATCH are Ready.
# The TIMEOUT counts from the burst, or from the mark of PHASE when it is given.
cmd_burst_wait() {
  BATCH=$1
  COUNT=$2
  TIMEOUT=$3
  START=$(require_state "burst-$BATCH-start")
  if [ -n "${4:-}" ]; then
    DEADLINE=$(($(mark_field "$4" 2) + TIMEOUT))
    SINCE="since the $4 mark"
  else
    DEADLINE=$((START + TIMEOUT))
    SINCE="since the burst"
  fi
  while :; do
    READY=$(ready_count "$BATCH")
    ELAPSED=$(($(now) - START))
    sample_usage
    log "batch $BATCH: $READY/$COUNT Ready after ${ELAPSED}s"
    if [ "$READY" != "?" ] && [ "$READY" -ge "$COUNT" ]; then
      state_set "burst-$BATCH-seconds" "$ELAPSED"
      break
    fi
    if [ "$(now)" -ge "$DEADLINE" ]; then
      kc -n "$NAMESPACE" get certificates.cert-manager.io -l "e2e-batch=$BATCH" >&2 || true
      fail "only $READY of $COUNT Certificates of batch $BATCH are Ready after ${ELAPSED}s (budget ${TIMEOUT}s $SINCE)"
    fi
    sleep 5
  done
  log "batch $BATCH: all $COUNT Certificates Ready in ${ELAPSED}s; peak usage of the target: $(state_get usage-peak)m"
}

# --- CPU pressure -------------------------------------------------------------------------------

# stress-params THREADS_FACTOR THREADS_PER_POD MAX_PODS: prints the parameters of the stress
# Deployment as JSON: node, CPU request and busy loops per pod, number of replicas.
# The stress pods together request the free allocatable CPU of the target node minus 100m, so they
# take the whole node by weight and the module keeps only its own request.
#   THREADS_PER_POD=0 (noisy neighbour): one pod runs THREADS_FACTOR busy loops per node CPU.
#   THREADS_PER_POD>0 (high load average): the THREADS_FACTOR busy loops per node CPU are spread over
#   separate pods with THREADS_PER_POD loops each. CFS shares the CPU between cgroups by weight, so
#   threads inside one pod only compete with each other; each pod is a cgroup of its own, and many
#   runnable cgroups make the run queue of the module long. The number of pods is limited by MAX_PODS
#   and by the free pod slots of the node (allocatable pods minus running pods minus 5).
cmd_stress_params() {
  [ "$#" -eq 3 ] || fail "usage: stress-params THREADS_FACTOR THREADS_PER_POD MAX_PODS"
  FACTOR=$1
  PER_POD=$2
  MAX_PODS=$3
  NODE=$(require_state target-node)
  NODE_CPU=$(retry "read the CPU of node $NODE" kc get node "$NODE" \
    -o jsonpath='{.status.allocatable.cpu} {.status.capacity.cpu} {.status.allocatable.pods}')
  # shellcheck disable=SC2086 # split the three values into positional parameters
  set -- $NODE_CPU
  [ "$#" -eq 3 ] || fail "cannot read the CPU and the pod capacity of node $NODE: '$NODE_CPU'"
  ALLOCATABLE=$(millicores "$1")
  CAPACITY=$(millicores "$2")
  POD_SLOTS=$3
  [ "$ALLOCATABLE" -gt 0 ] && [ "$CAPACITY" -gt 0 ] || fail "cannot read the CPU of node $NODE: '$NODE_CPU'"
  CPUS=$(((CAPACITY + 999) / 1000))
  # The effective request of a pod follows the scheduler: native sidecars (init containers with
  # restartPolicy Always) run next to the containers, so they add to the containers sum; a regular init
  # container runs next to the sidecars declared before it. The request is
  # max(containers + sidecars, largest of init container + earlier sidecars) + pod overhead.
  # Each field is c<CPU> (container), i<RESTART POLICY>:<CPU> (init container) or o<CPU> (overhead).
  NODE_PODS=$(retry "list the pods of node $NODE" kc get pods -A \
    --field-selector="spec.nodeName=$NODE,status.phase!=Succeeded,status.phase!=Failed" \
    -o jsonpath='{range .items[*]}{"pod"}{range .spec.containers[*]}{" c"}{.resources.requests.cpu}{end}{range .spec.initContainers[*]}{" i"}{.restartPolicy}{":"}{.resources.requests.cpu}{end}{" o"}{.spec.overhead.cpu}{"\n"}{end}')
  [ -n "$NODE_PODS" ] || fail "no pods found on node $NODE"
  REQUESTED=$(printf '%s\n' "$NODE_PODS" | pod_requests)
  FREE=$((ALLOCATABLE - REQUESTED - 100))
  [ "$FREE" -ge 100 ] || fail "node $NODE has only $((ALLOCATABLE - REQUESTED))m of unrequested CPU"
  THREADS=$((FACTOR * CPUS))
  if [ "$PER_POD" -eq 0 ]; then
    REPLICAS=1
    PER_POD=$THREADS
  else
    REPLICAS=$(((THREADS + PER_POD - 1) / PER_POD))
    FREE_SLOTS=$((POD_SLOTS - $(printf '%s\n' "$NODE_PODS" | grep -c .) - 5))
    LIMIT=$MAX_PODS
    [ "$FREE_SLOTS" -lt "$LIMIT" ] && LIMIT=$FREE_SLOTS
    [ "$LIMIT" -ge 1 ] || fail "node $NODE has no free pod slots for the stress pods (allocatable pods: $POD_SLOTS)"
    if [ "$REPLICAS" -gt "$LIMIT" ]; then
      log "limiting the stress pods to $LIMIT (wanted $REPLICAS; MAX_PODS $MAX_PODS, free pod slots $FREE_SLOTS)"
      REPLICAS=$LIMIT
    fi
  fi
  POD_CPU=$((FREE / REPLICAS))
  [ "$POD_CPU" -ge 1 ] || fail "${FREE}m of free CPU is too little for $REPLICAS stress pods"
  log "node $NODE: allocatable ${ALLOCATABLE}m, requested ${REQUESTED}m, $CPUS CPUs ->" \
    "$REPLICAS stress pods with ${POD_CPU}m and $PER_POD busy loops each ($((REPLICAS * PER_POD)) loops in total)"
  state_set node-cpus "$CPUS"
  state_set stress-replicas "$REPLICAS"
  printf '{"node":"%s","cpu":"%sm","threads":"%s","replicas":%s}\n' "$NODE" "$POD_CPU" "$PER_POD" "$REPLICAS"
}

# stress_running returns 1 when the API answered and fewer stress pods run than stress-params planned,
# 2 when the API did not answer.
stress_running() {
  WANT=$(require_state stress-replicas)
  PHASES=$(retry "read the stress pods" kc -n "$NAMESPACE" get pods -l "$STRESS_SELECTOR" \
    -o jsonpath='{range .items[*]}{.status.phase}{"\n"}{end}') || return 2
  RUNNING=$(printf '%s\n' "$PHASES" | grep -c Running || true)
  [ "$RUNNING" -ge "$WANT" ] || {
    log "$RUNNING of $WANT stress pods are running"
    return 1
  }
}

# hold SECONDS BATCH COUNT BUDGET: keeps the pressure for SECONDS since all stress pods run (mark
# stress-ready) and, beyond that, until all COUNT Certificates of BATCH are Ready. Fails when the budget
# since the burst runs out or a stress pod stops.
cmd_hold() {
  HOLD=$1
  BATCH=$2
  COUNT=$3
  BUDGET=$4
  PRESSURE_START=$(mark_field stress-ready 2)
  BURST_START=$(require_state "burst-$BATCH-start")
  while :; do
    T=$(now)
    stress_running && SR=0 || SR=$?
    case $SR in
      0) ;;
      1) fail "not all stress pods are running; the pressure is not applied as planned" ;;
      *) fail "cannot read the state of the stress pods" ;;
    esac
    READY=$(ready_count "$BATCH")
    log "pressure: ${HOLD}s hold, $((T - PRESSURE_START))s passed since the stress pods run, batch $BATCH $READY/$COUNT Ready after $((T - BURST_START))s"
    DONE=false
    if [ "$READY" != "?" ] && [ "$READY" -ge "$COUNT" ]; then DONE=true; fi
    if [ "$DONE" = true ] && [ -z "$(state_get "burst-$BATCH-seconds")" ]; then
      state_set "burst-$BATCH-seconds" "$((T - BURST_START))"
    fi
    if [ "$DONE" = true ] && [ $((T - PRESSURE_START)) -ge "$HOLD" ]; then
      break
    fi
    if [ "$DONE" = false ] && [ $((T - BURST_START)) -ge "$BUDGET" ]; then
      kc -n "$NAMESPACE" get certificates.cert-manager.io -l "e2e-batch=$BATCH" >&2 || true
      fail "only $READY of $COUNT Certificates of batch $BATCH are Ready under pressure after ${BUDGET}s"
    fi
    sleep 10
  done
  log "batch $BATCH: all $COUNT Certificates Ready under pressure in $(state_get "burst-$BATCH-seconds")s (baseline: $(state_get burst-baseline-seconds)s)"
}

# release: deletes the stress Deployment and waits until its pods are gone.
cmd_release() {
  retry "delete the stress Deployment" kc -n "$NAMESPACE" delete deployment "$STRESS" \
    --ignore-not-found --wait=true --timeout=120s >&2
  I=0
  while [ -n "$(kc -n "$NAMESPACE" get pods -l "$STRESS_SELECTOR" -o name 2>/dev/null || echo unknown)" ]; do
    I=$((I + 1))
    [ "$I" -lt 60 ] || fail "stress pods are still there 120s after the Deployment was deleted"
    sleep 2
  done
  cmd_mark release
}

# --- in-place resize of the target container ----------------------------------------------------

kube_minor() {
  retry "read the Kubernetes version" kc get --raw /version | tr -d ' \n' | sed -n 's/.*"minor":"\([0-9]*\).*/\1/p'
}

# require-resize: in-place resize needs Kubernetes 1.33 or newer.
cmd_require_resize() {
  MINOR=$(kube_minor)
  [ -n "$MINOR" ] || fail "cannot read the Kubernetes version"
  [ "$MINOR" -ge 33 ] || fail "in-place pod resize needs Kubernetes 1.33 or newer, the cluster runs 1.$MINOR"
  log "Kubernetes 1.$MINOR supports in-place pod resize"
}

# la-request MIN_CPU: prints max(peak usage of the target seen in the baseline, MIN_CPU) in millicores.
cmd_la_request() {
  MIN=$(millicores "$1")
  PEAK=$(state_get usage-peak)
  CPU=$MIN
  if [ -n "$PEAK" ] && [ "$PEAK" -gt "$MIN" ]; then
    CPU=$PEAK
  fi
  log "peak usage of the target in the baseline: ${PEAK:-unknown}m; request for the high-load-average run: ${CPU}m"
  printf '%sm\n' "$CPU"
}

# resize requests|limits CPU: changes the CPU request or limit of the target container in place.
cmd_resize() {
  KIND=$1
  CPU=$2
  POD=$(require_state target-pod)
  CONTAINER=$(require_state target-container)
  state_set resized true
  retry "resize $KIND.cpu of $CM_NS/$POD/$CONTAINER" kc -n "$CM_NS" patch pod "$POD" --subresource resize \
    -p "{\"spec\":{\"containers\":[{\"name\":\"$CONTAINER\",\"resources\":{\"$KIND\":{\"cpu\":\"$CPU\"}}}]}}" >&2
  WANT=$(millicores "$CPU")
  I=0
  while [ "$I" -lt 30 ]; do
    GOT=$(kc -n "$CM_NS" get pod "$POD" \
      -o jsonpath="{.status.containerStatuses[?(@.name==\"$CONTAINER\")].resources.$KIND.cpu}" || true)
    if [ -n "$GOT" ] && [ "$(millicores "$GOT")" -eq "$WANT" ]; then
      log "$CM_NS/$POD/$CONTAINER: $KIND.cpu is $GOT"
      return 0
    fi
    I=$((I + 1))
    sleep 2
  done
  kc -n "$CM_NS" get pod "$POD" -o jsonpath='{.status.conditions}{"\n"}' >&2 || true
  fail "kubelet did not apply $KIND.cpu=$CPU to $CM_NS/$POD/$CONTAINER in 60s"
}

# lift-limit: raises the CPU limit of the target container to the CPU count of its node.
# A limit cannot be removed in place; the cleanup recreates the pod from the Deployment template.
cmd_lift_limit() {
  CPUS=$(state_get node-cpus)
  [ -n "$CPUS" ] || CPUS=$(retry "read the CPU count of the target node" kc get node "$(require_state target-node)" \
    -o jsonpath='{.status.capacity.cpu}')
  cmd_resize limits "$CPUS"
}

# reset-target: deletes a resized target pod, so the Deployment recreates it from its template.
cmd_reset_target() {
  [ "$(state_get resized)" = true ] || return 0
  POD=$(state_get target-pod)
  DEPLOY=$(state_get target-deploy)
  retry "delete the resized pod $CM_NS/$POD" kc -n "$CM_NS" delete pod "$POD" --ignore-not-found --wait=true --timeout=120s >&2
  kubectl -n "$CM_NS" rollout status "deployment/$DEPLOY" --timeout=300s >&2
  state_set resized false
  log "recreated the target pod from the template of deployment/$DEPLOY"
}

# --- health -------------------------------------------------------------------------------------

# healthy TIMEOUT [PHASE]: waits until every Deployment of the module (cert-manager, webhook and, when
# they are deployed, cainjector and yandex-dns-webhook) is fully Ready and no container of the module
# is in CrashLoopBackOff. The TIMEOUT counts from now, or from the mark of PHASE when it is given.
cmd_healthy() {
  TIMEOUT=$1
  if [ -n "${2:-}" ]; then
    DEADLINE=$(($(mark_field "$2" 2) + TIMEOUT))
  else
    DEADLINE=$(($(now) + TIMEOUT))
  fi
  while :; do
    OK=true
    if DEPLOYS=$(kc -n "$CM_NS" get deployments \
      -o jsonpath='{range .items[*]}{.metadata.name}{" "}{.spec.replicas}{"/"}{.status.readyReplicas}{"/"}{.status.updatedReplicas}{"\n"}{end}' 2>&1); then
      NOT_READY=$(printf '%s\n' "$DEPLOYS" | deployments_not_ready)
      if [ -n "$NOT_READY" ]; then
        OK=false
        log "deployments of $CM_NS not ready (replicas/ready/updated): $NOT_READY"
      fi
    else
      OK=false
      log "cannot list the deployments of $CM_NS: $(printf '%s' "$DEPLOYS" | tr '\n' ' ')"
    fi
    if WAITING=$(kc -n "$CM_NS" get pods \
      -o jsonpath='{range .items[*]}{range .status.containerStatuses[*]}{.state.waiting.reason}{" "}{.name}{"\n"}{end}{end}' 2>&1); then
      CRASHING=$(printf '%s\n' "$WAITING" | awk '$1 == "CrashLoopBackOff" { print $2 }' | tr '\n' ' ')
      if [ -n "$CRASHING" ]; then
        OK=false
        log "containers of $CM_NS in CrashLoopBackOff: $CRASHING"
      fi
    else
      OK=false
      log "cannot list the pods of $CM_NS: $(printf '%s' "$WAITING" | tr '\n' ' ')"
    fi
    if [ "$OK" = true ]; then
      log "all Deployments of $CM_NS are Ready, no container is in CrashLoopBackOff"
      return 0
    fi
    [ "$(now)" -lt "$DEADLINE" ] || {
      kc -n "$CM_NS" get pods -o wide >&2 || true
      fail "the module did not become healthy in ${TIMEOUT}s${2:+ after the $2 mark}"
    }
    sleep 5
  done
}

# deployments_not_ready reads "<NAME> <REPLICAS>/<READY>/<UPDATED>" lines on stdin and prints the
# Deployments that are not fully Ready, and the cert-manager and webhook Deployments when they are missing.
deployments_not_ready() {
  awk '
    NF {
      seen[$1] = 1
      split($2, r, "/")
      want = r[1] + 0; ready = r[2] + 0; updated = r[3] + 0
      if (ready != want || updated != want) printf "%s=%s ", $1, $2
    }
    END {
      if (!("cert-manager" in seen)) printf "cert-manager=missing "
      if (!("webhook" in seen)) printf "webhook=missing "
    }'
}

# --- admission through the webhook --------------------------------------------------------------

# admission-loop: creates a Certificate and a CertificateRequest with --dry-run=server in turn,
# about twice a second, and logs "<epoch> <kind> ok|fail|infra <seconds> [error]" for each request.
# The mutating webhook handles CertificateRequest CREATE, the validating webhook handles both.
# "infra" marks a request that never reached admission (see is_infra_error); it is not counted as
# an admission failure, but fails the test. Run by admission-start through bg-start; the loop ends
# when its pid file disappears or at the deadline of the background processes.
# Client-side validation is off: it downloads the OpenAPI schema, which is not part of admission.
cmd_admission_loop() {
  : "${BG_PID_FILE:?admission-loop runs only through bg-start}"
  I=0
  while bg_running "$BG_PID_FILE"; do
    if [ $((I % 2)) -eq 0 ]; then KIND=certificate; else KIND=certificaterequest; fi
    T0=$(now)
    if ERR=$(kubectl -n "$NAMESPACE" create --dry-run=server --validate=false --request-timeout=40s -o name \
      -f "$COMMON_DIR/manifests/admission-$KIND.yaml" 2>&1); then
      echo "$T0 $KIND ok $(($(now) - T0))"
    else
      RESULT=fail
      is_infra_error "$ERR" && RESULT=infra
      echo "$T0 $KIND $RESULT $(($(now) - T0)) $(printf '%s' "$ERR" | tr '\n' ' ' | cut -c1-300)"
    fi
    I=$((I + 1))
    sleep 0.5
  done >>"$STATE_DIR/admission.log"
}

# admission-start HOLD BUDGET RECOVERY AFTER: starts the admission loop and waits for its first
# successful request. The arguments set the deadline of the loop, as for watch-start.
cmd_admission_start() {
  [ "$#" -eq 4 ] || fail "usage: admission-start HOLD BUDGET RECOVERY AFTER"
  set_bg_deadline "$@"
  save_kubeconfig
  : >"$STATE_DIR/admission.log"
  cmd_bg_start admission sh "$SCRIPTS_DIR/e2e.sh" admission-loop
  I=0
  while ! grep -q ' ok ' "$STATE_DIR/admission.log"; do
    INFRA=$(grep -m 1 ' infra ' "$STATE_DIR/admission.log" || true)
    [ -z "$INFRA" ] || fail "infrastructure failure: the admission loop cannot reach the API server: $INFRA"
    I=$((I + 1))
    if [ "$I" -ge 30 ]; then
      cat "$STATE_DIR/admission.log" >&2
      fail "no admission request passed the cert-manager webhooks in 30s without pressure"
    fi
    sleep 1
  done
  log "the admission loop runs"
}

# admission-check MAX_PERCENT: fails when more than MAX_PERCENT of the admission requests failed
# in the baseline, under pressure or after it, or when a window has no requests at all. The windows are
# the same as in writes-check.
cmd_admission_check() {
  RESULT=0
  INFRA=0
  printf '%-9s %8s %8s %9s %12s\n' window requests failed failed-% max-seconds >"$STATE_DIR/admission-summary"
  for WINDOW in baseline:baseline-end pressure:release release:end; do
    FROM=${WINDOW%%:*}
    TO=${WINDOW##*:}
    T0=$(mark_field "$FROM" 2)
    T1=$(mark_field "$TO" 2)
    LINE=$(awk -v a="$T0" -v b="$T1" '
      $1 >= a && $1 < b { if ($3 == "infra") { i++; next } n++; if ($3 != "ok") f++; if ($4 > m) m = $4 }
      END { printf "%d %d %.2f %d %d\n", n, f, (n ? f * 100 / n : 0), m, i }' "$STATE_DIR/admission.log")
    # shellcheck disable=SC2086 # split the five numbers into positional parameters
    set -- $LINE
    NAME=$FROM
    [ "$NAME" = release ] && NAME=after
    printf '%-9s %8s %8s %9s %12s\n' "$NAME" "$1" "$2" "$3" "$4" >>"$STATE_DIR/admission-summary"
    if [ "$5" -gt 0 ]; then
      echo "$5 requests in the $NAME window did not reach the API server (infrastructure failure)" \
        >>"$STATE_DIR/admission-summary"
      INFRA=1
    fi
    if [ "$1" -eq 0 ]; then
      echo "no admission requests in the $NAME window" >>"$STATE_DIR/admission-summary"
      RESULT=1
    elif awk -v p="$3" -v m="$MAX_PERCENT_ARG" 'BEGIN { exit !(p > m) }'; then
      echo "admission failures in the $NAME window exceed ${MAX_PERCENT_ARG}%" >>"$STATE_DIR/admission-summary"
      RESULT=1
    fi
  done
  cat "$STATE_DIR/admission-summary" >&2
  grep ' fail ' "$STATE_DIR/admission.log" | head -n 5 >&2 || true
  if [ "$INFRA" -ne 0 ]; then
    log "first infrastructure error: $(grep -m 1 ' infra ' "$STATE_DIR/admission.log")"
    fail "infrastructure failure: the admission loop could not reach the API server, the result is not valid"
  fi
  [ "$RESULT" -eq 0 ] || fail "admission through the cert-manager webhook degraded"
  log "admission through the cert-manager webhook kept working"
}

# admission-probe TIMEOUT: waits until a dry-run Certificate passes admission after the release.
cmd_admission_probe() {
  RELEASE=$(mark_field release 2)
  while :; do
    if ERR=$(kubectl -n "$NAMESPACE" create --dry-run=server --validate=false --request-timeout=10s -o name \
      -f "$COMMON_DIR/manifests/admission-certificate.yaml" 2>&1 >/dev/null); then
      log "admission works $(($(now) - RELEASE))s after the release"
      return 0
    fi
    if is_infra_error "$ERR"; then
      fail "infrastructure failure: the admission probe cannot reach the API server: $(printf '%s' "$ERR" | tr '\n' ' ')"
    fi
    [ $(($(now) - RELEASE)) -lt "$1" ] ||
      fail "admission does not work $1s after the release; last error: $(printf '%s' "$ERR" | tr '\n' ' ')"
    sleep 2
  done
}

# --- reporting and cleanup ----------------------------------------------------------------------

cmd_report() {
  log "state of the run: $STATE_DIR"
  for F in marks snapshot-before snapshot-after writes-summary admission-summary; do
    if [ -f "$STATE_DIR/$F" ]; then
      echo "--- $F" >&2
      cat "$STATE_DIR/$F" >&2
    fi
  done
  for F in burst-baseline-seconds burst-pressure-seconds burst-recovery-seconds usage-peak; do
    [ -f "$STATE_DIR/$F" ] && echo "$F: $(cat "$STATE_DIR/$F")" >&2
  done
  return 0
}

# cleanup: stops the background processes, removes the copy of the kubeconfig, prints the summary and
# removes STATE_DIR unless CPU_STARVATION_KEEP_STATE is true.
cmd_cleanup() {
  cmd_bg_stop_all
  cmd_report
  if [ "${CPU_STARVATION_KEEP_STATE:-false}" = true ]; then
    log "keeping the state of the run in $STATE_DIR"
    return 0
  fi
  rm -rf "$STATE_DIR"
  log "removed the state of the run"
}

COMMAND=${1:-}
[ "$#" -gt 0 ] && shift
case $COMMAND in
  label-namespace) cmd_label_namespace ;;
  target) cmd_target "$@" ;;
  snapshot) cmd_snapshot "$@" ;;
  compare) cmd_compare "$@" ;;
  bg-stop-all) cmd_bg_stop_all ;;
  watch-start) cmd_watch_start "$@" ;;
  watch-stop)
    cmd_bg_stop watch-leases
    cmd_bg_stop watch-certificates
    ;;
  mark) cmd_mark "$@" ;;
  wait-window) cmd_wait_window "$@" ;;
  writes-check) cmd_writes_check "$@" ;;
  burst) cmd_burst "$@" ;;
  burst-wait) cmd_burst_wait "$@" ;;
  stress-params) cmd_stress_params "$@" ;;
  hold) cmd_hold "$@" ;;
  release) cmd_release ;;
  require-resize) cmd_require_resize ;;
  la-request) cmd_la_request "$@" ;;
  resize) cmd_resize "$@" ;;
  lift-limit) cmd_lift_limit ;;
  reset-target) cmd_reset_target ;;
  healthy) cmd_healthy "$@" ;;
  admission-loop) cmd_admission_loop ;;
  admission-start) cmd_admission_start "$@" ;;
  admission-stop) cmd_bg_stop admission ;;
  admission-check)
    MAX_PERCENT_ARG=$1
    cmd_admission_check
    ;;
  admission-probe) cmd_admission_probe "$@" ;;
  report) cmd_report ;;
  cleanup) cmd_cleanup ;;
  *) fail "unknown command '$COMMAND'" ;;
esac
