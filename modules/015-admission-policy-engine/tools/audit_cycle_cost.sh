#!/usr/bin/env bash
#
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

#
# audit_cycle_cost.sh - attribute CPU and memory cost to ONE audit cycle on a
# live cluster, using only metrics that are already exposed - no
# --enable-pprof, no Deployment patch, no restart.
#
# How: the audit log already marks each cycle's exact start/end
# ("audit_started"/"audit_finished", with an audit_id shared by both lines).
# This script watches the log for the next cycle boundary and takes a
# Prometheus /metrics snapshot right at the start and right at the end, then
# diffs the Go-runtime counters between them:
#
#   process_cpu_seconds_total        -> CPU-seconds actually burned by the
#                                        process DURING that one cycle
#                                        (delta / wall-clock = avg cores used)
#   go_memstats_mallocs_total        -> allocation count during the cycle
#                                        (memory churn / GC pressure, not
#                                        net growth - see heap_alloc below)
#   go_memstats_heap_alloc_bytes     -> net heap growth during the cycle
#                                        (can be negative if GC ran)
#   go_gc_duration_seconds_count     -> number of GC cycles triggered
#   go_goroutines                    -> goroutine count delta (leak signal
#                                        if it trends up cycle over cycle)
#
# process_cpu_seconds_total and go_memstats_mallocs_total are cumulative
# counters, so a delta across an exact time window is a real, unambiguous
# measurement of what that window cost - no sampling/averaging error, and no
# code changes to the running binary.
#
# Usage:
#   ./audit_cycle_cost.sh [-n namespace] [-t timeout_seconds]
#
# Requires: kubectl, curl, awk. Uses python3 if present for a cleaner report;
# falls back to awk-only output otherwise.

set -uo pipefail

NAMESPACE="d8-admission-policy-engine"
TIMEOUT=180

while getopts "n:t:h" opt; do
  case "$opt" in
    n) NAMESPACE="$OPTARG" ;;
    t) TIMEOUT="$OPTARG" ;;
    h) sed -n '2,30p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) exit 2 ;;
  esac
done

AUDIT_POD="$(kubectl -n "$NAMESPACE" get pods -l control-plane=audit-controller -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)"
if [ -z "$AUDIT_POD" ]; then
  echo "ERROR: no pod with label control-plane=audit-controller in namespace $NAMESPACE" >&2
  exit 1
fi
echo "audit pod: $AUDIT_POD"

PORT=18900
kubectl -n "$NAMESPACE" port-forward "pod/$AUDIT_POD" "${PORT}:8888" >/dev/null 2>&1 &
PF_PID=$!
cleanup() { kill "$PF_PID" >/dev/null 2>&1; wait "$PF_PID" 2>/dev/null; }
trap cleanup EXIT

for _ in $(seq 1 20); do
  curl -s -m 1 "http://127.0.0.1:${PORT}/metrics" -o /dev/null 2>/dev/null && break
  sleep 0.3
done

# `date -r <epoch>` means two different things on BSD/macOS (format an epoch
# seconds value) and GNU/Linux (show a FILE's mtime, errors on a bare
# number) - try the GNU form first, then the BSD form, then give up and
# just print the current time (cosmetic only - the CPU/memory math below
# uses the raw epoch floats directly and is unaffected either way).
fmt_time() {
  date -u -d "@${1%.*}" +%T 2>/dev/null || date -u -r "${1%.*}" +%T 2>/dev/null || date -u +%T
}

get_audit_interval() {
  kubectl -n "$NAMESPACE" get deploy gatekeeper-audit \
    -o jsonpath='{.spec.template.spec.containers[0].args}' 2>/dev/null \
    | grep -oE 'audit-interval=[0-9]+' | grep -oE '[0-9]+' || echo "?"
}
interval="$(get_audit_interval)"
echo "configured --audit-interval: ${interval}s (this script's -t timeout should be a bit more than that)"
echo ""

echo "waiting for the start of a NEW audit cycle (up to ${TIMEOUT}s)..."
deadline=$(( $(date +%s) + TIMEOUT ))
last_id=""
start_id=""
while :; do
  line="$(kubectl -n "$NAMESPACE" logs "$AUDIT_POD" -c manager --tail=50 2>/dev/null | grep 'audit_started' | tail -1)"
  id="$(grep -oE '"audit_id":"[^"]+"' <<<"$line" | head -1)"
  if [ -n "$id" ] && [ "$id" != "$last_id" ]; then
    start_id="$id"
    break
  fi
  [ -n "$id" ] && last_id="$id"
  if [ "$(date +%s)" -gt "$deadline" ]; then
    echo "ERROR: timed out waiting for a new audit cycle to start" >&2
    exit 1
  fi
  sleep 1
done

before_wall=$(date +%s.%N)
before="$(curl -s -m 2 "http://127.0.0.1:${PORT}/metrics")"
echo "cycle started ($start_id) @ $(fmt_time "$before_wall") - snapshot taken"

restart_before="$(kubectl -n "$NAMESPACE" get pod "$AUDIT_POD" -o jsonpath='{.status.containerStatuses[?(@.name=="manager")].restartCount}' 2>/dev/null)"

deadline=$(( $(date +%s) + TIMEOUT * 3 ))
while :; do
  # --tail is generous on purpose: right after "audit_finished" Gatekeeper
  # immediately logs one burst line per constraint kind (status update), so a
  # small tail window can scroll the finished marker itself out of view
  # before the next poll - this bit us during testing on adyakonov-cloud.
  fline="$(kubectl -n "$NAMESPACE" logs "$AUDIT_POD" -c manager --tail=500 2>/dev/null | grep 'audit_finished' | grep -F "$start_id")"
  if [ -n "$fline" ]; then break; fi
  restart_now="$(kubectl -n "$NAMESPACE" get pod "$AUDIT_POD" -o jsonpath='{.status.containerStatuses[?(@.name=="manager")].restartCount}' 2>/dev/null)"
  if [ -n "$restart_now" ] && [ "$restart_now" != "$restart_before" ]; then
    echo "ERROR: the manager container restarted mid-cycle (restartCount $restart_before -> $restart_now)." >&2
    echo "       This measurement is invalid - the log buffer for this cycle was lost." >&2
    echo "       This usually means something else (node/cluster pressure, a failed" >&2
    echo "       liveness probe) interrupted the pod, not that the cycle itself hung -" >&2
    echo "       check 'kubectl get events' around this time, then re-run." >&2
    exit 1
  fi
  if [ "$(date +%s)" -gt "$deadline" ]; then
    echo "ERROR: timed out waiting for the cycle to finish" >&2
    exit 1
  fi
  sleep 1
done
after_wall=$(date +%s.%N)
after="$(curl -s -m 2 "http://127.0.0.1:${PORT}/metrics")"
logged_duration="$(grep -oE '"duration":"[^"]+"' <<<"$fline" | head -1)"
echo "cycle finished @ $(fmt_time "$after_wall") - snapshot taken (logged $logged_duration)"
echo ""

val() { # $1=metric $2=snapshot-text
  awk -v m="^$1 " '$0 ~ m {print $2; exit}' <<<"$2"
}

cpu_before=$(val process_cpu_seconds_total "$before")
cpu_after=$(val process_cpu_seconds_total "$after")
rss_before=$(val process_resident_memory_bytes "$before")
rss_after=$(val process_resident_memory_bytes "$after")
heap_before=$(val go_memstats_heap_alloc_bytes "$before")
heap_after=$(val go_memstats_heap_alloc_bytes "$after")
mallocs_before=$(val go_memstats_mallocs_total "$before")
mallocs_after=$(val go_memstats_mallocs_total "$after")
frees_before=$(val go_memstats_frees_total "$before")
frees_after=$(val go_memstats_frees_total "$after")
gc_before=$(val go_gc_duration_seconds_count "$before")
gc_after=$(val go_gc_duration_seconds_count "$after")
goroutines_before=$(val go_goroutines "$before")
goroutines_after=$(val go_goroutines "$after")

wall=$(awk -v a="$before_wall" -v b="$after_wall" 'BEGIN{printf "%.2f", b-a}')

echo "=== Cost of this one audit cycle (measured, not estimated) ==="
awk -v wall="$wall" \
    -v cb="$cpu_before" -v ca="$cpu_after" \
    -v rb="$rss_before" -v ra="$rss_after" \
    -v hb="$heap_before" -v ha="$heap_after" \
    -v mb="$mallocs_before" -v ma="$mallocs_after" \
    -v fb="$frees_before" -v fa="$frees_after" \
    -v gb="$gc_before" -v ga="$gc_after" \
    -v grb="$goroutines_before" -v gra="$goroutines_after" \
'BEGIN {
  cpu_delta = ca - cb
  printf "wall time                 %.2fs\n", wall
  printf "CPU-seconds consumed      %.3fs  (avg %.2f cores during the cycle)\n", cpu_delta, cpu_delta/wall
  printf "heap alloc, net change    %+d bytes (%+.1f MiB)\n", (ha-hb), (ha-hb)/1048576
  printf "RSS, net change           %+d bytes (%+.1f MiB)\n", (ra-rb), (ra-rb)/1048576
  printf "allocations (mallocs)     %d objects\n", (ma-mb)
  printf "frees                     %d objects\n", (fa-fb)
  printf "net live objects          %+d\n", (ma-mb)-(fa-fb)
  printf "GC cycles triggered       %d\n", (ga-gb)
  printf "goroutines, net change    %+d (%s -> %s)\n", (gra-grb), grb, gra
}'
echo ""
echo "Reading this:"
echo "  - 'avg cores during the cycle' vs. idle CPU (section 3 of"
echo "    live_resource_check.sh, between cycles) is the real, measured"
echo "    audit-specific CPU cost - not an estimate from a synthetic bench."
echo "  - a high allocation count with heap alloc net change near zero means"
echo "    the cycle churns a lot of short-lived memory that GC reclaims"
echo "    right away (normal for a per-object eval loop); heap alloc net"
echo "    change trending UP cycle over cycle (run this script a few times)"
echo "    would point at an actual leak, which a single cycle cannot show."
echo "  - cross-reference with gatekeeper_sync/gatekeeper_constraints counts"
echo "    from live_resource_check.sh section 4, and with the per-rule"
echo "    ns/op and B/op numbers from ../charts/constraint-templates/tests/"
echo "    tools/rulebench.sh for the SAME active ConstraintTemplate kinds -"
echo "    (objects synced) x (per-object rule cost) is a metrics-only,"
echo "    zero-mutation way to attribute this cycle's cost to specific rules."
