#!/usr/bin/env bash
#
# live_resource_check.sh - diagnose WHY admission-policy-engine (Gatekeeper)
# is consuming CPU/memory in a live cluster, and separate three kinds of
# cause: (a) fixable by a flag/CR we own, (b) architectural (inherent to
# Gatekeeper/OPA, e.g. full-cluster Pod sync, discovery), (c) environmental
# (node/cluster pressure unrelated to this module).
#
# This is the live-cluster counterpart to the static per-rule benchmarks in
# ../charts/constraint-templates/tests/tools/ (bench_rules.py, rulebench) -
# those measure a single Rego rule in isolation; this script measures the
# actual running Deployment: real resource usage, real audit-cycle timing,
# real synced-object counts, and whether anomalies correlate with the wider
# cluster rather than with this module specifically.
#
# Usage:
#   ./live_resource_check.sh [-n namespace] [-s samples] [-i interval] [-o outdir]
#
#   -n  namespace to inspect (default: d8-admission-policy-engine)
#   -s  number of `kubectl top` samples to take (default: 3)
#   -i  seconds between samples (default: 15)
#   -o  directory to write the full report + raw metrics into
#       (default: ./ape-diag-<timestamp>)
#
# Requires: kubectl (with access to the target cluster/context already
# selected - this script never switches context), awk, curl. python3 is used
# opportunistically for a couple of JSON comparisons and skipped gracefully
# if absent.
#
# See ./README.md for what each section means and how to read the output,
# a worked example from a real cluster, and thresholds/rules of thumb -
# this script mirrors that document's structure exactly.

set -uo pipefail

NAMESPACE="d8-admission-policy-engine"
SAMPLES=3
INTERVAL=15
OUTDIR=""

while getopts "n:s:i:o:h" opt; do
  case "$opt" in
    n) NAMESPACE="$OPTARG" ;;
    s) SAMPLES="$OPTARG" ;;
    i) INTERVAL="$OPTARG" ;;
    o) OUTDIR="$OPTARG" ;;
    h)
      sed -n '2,30p' "$0" | sed 's/^# \{0,1\}//'
      exit 0
      ;;
    *) exit 2 ;;
  esac
done

[ -n "$OUTDIR" ] || OUTDIR="./ape-diag-$(date +%Y%m%d-%H%M%S)"
mkdir -p "$OUTDIR"
REPORT="$OUTDIR/report.txt"
: > "$REPORT"

log()   { printf '%s\n' "$*" | tee -a "$REPORT"; }
hr()    { log "----------------------------------------------------------------------"; }
head1() { hr; log "## $*"; hr; }

CTX="$(kubectl config current-context 2>/dev/null || echo unknown)"
log "admission-policy-engine live resource check"
log "context:   $CTX"
log "namespace: $NAMESPACE"
log "started:   $(date -u +%FT%TZ)"
log "output:    $OUTDIR"
log ""

if ! kubectl get ns "$NAMESPACE" >/dev/null 2>&1; then
  log "ERROR: namespace $NAMESPACE not found in context $CTX"
  exit 1
fi

# ---------------------------------------------------------------------------
head1 "1. Pods, restarts, last-restart reason"
# ---------------------------------------------------------------------------
kubectl -n "$NAMESPACE" get pods -o wide | tee -a "$REPORT"
log ""
kubectl -n "$NAMESPACE" get pods -o json > "$OUTDIR/pods.json"
if command -v python3 >/dev/null 2>&1; then
  python3 - "$OUTDIR/pods.json" <<'PY' | tee -a "$REPORT"
import json, sys
d = json.load(open(sys.argv[1]))
for pod in d.get("items", []):
    name = pod["metadata"]["name"]
    for cs in pod.get("status", {}).get("containerStatuses", []):
        rc = cs.get("restartCount", 0)
        if rc == 0:
            continue
        last = cs.get("lastState", {}).get("terminated", {})
        reason = last.get("reason", "?")
        exit_code = last.get("exitCode", "?")
        finished = last.get("finishedAt", "?")
        print(f"  {name}/{cs['name']}: restartCount={rc} lastReason={reason} exitCode={exit_code} finishedAt={finished}")
PY
else
  log "  (python3 not found - skipping per-container restart-reason breakdown; see $OUTDIR/pods.json)"
fi
log ""
log "Interpretation: exitCode=0 + reason=Completed on a restart almost always"
log "means the process was killed by a failed liveness probe (it did not"
log "crash on its own) - look at section 3 for whether CPU/mem was spiking"
log "at that time, and section 5 for whether the WHOLE CLUSTER had a"
log "problem at that timestamp (disk/CPU pressure), not just this pod."

# ---------------------------------------------------------------------------
head1 "2. Requests/limits vs container resources.requests"
# ---------------------------------------------------------------------------
kubectl -n "$NAMESPACE" get pods -o custom-columns=\
'POD:.metadata.name,CONTAINER:.spec.containers[*].name,CPU_REQ:.spec.containers[*].resources.requests.cpu,MEM_REQ:.spec.containers[*].resources.requests.memory,CPU_LIM:.spec.containers[*].resources.limits.cpu,MEM_LIM:.spec.containers[*].resources.limits.memory' \
  | tee -a "$REPORT"
log ""
log "No limits shown above (blank column) means the container can grow"
log "unbounded relative to its request when the vertical-pod-autoscaler"
log "module is disabled - check whether that's expected for this version"
log "of the module before treating high usage as a leak."

# ---------------------------------------------------------------------------
head1 "3. Resource usage over time ($SAMPLES samples, ${INTERVAL}s apart)"
# ---------------------------------------------------------------------------
log "A single 'kubectl top' snapshot cannot tell a periodic spike (e.g. an"
log "audit cycle) from a sustained baseline. Taking several samples can."
log ""
TOP_LOG="$OUTDIR/top_samples.txt"
: > "$TOP_LOG"
for i in $(seq 1 "$SAMPLES"); do
  {
    echo "--- sample $i @ $(date -u +%T) ---"
    kubectl -n "$NAMESPACE" top pods --containers --no-headers 2>&1
  } | tee -a "$TOP_LOG" | tee -a "$REPORT"
  if [ "$i" -lt "$SAMPLES" ]; then sleep "$INTERVAL"; fi
done
log ""
log "Per container/metric min-max across samples:"
awk '
  /^--- sample/ { next }
  NF >= 4 {
    key = $1 "/" $2
    cpu = $3; mem = $4
    gsub(/[a-zA-Z]/, "", cpu); gsub(/[a-zA-Z]/, "", mem)
    if (!(key in cpu_min) || cpu+0 < cpu_min[key]) cpu_min[key]=cpu+0
    if (!(key in cpu_max) || cpu+0 > cpu_max[key]) cpu_max[key]=cpu+0
    if (!(key in mem_min) || mem+0 < mem_min[key]) mem_min[key]=mem+0
    if (!(key in mem_max) || mem+0 > mem_max[key]) mem_max[key]=mem+0
    seen[key]=1
  }
  END {
    for (k in seen) {
      printf "  %-55s cpu %4d-%4dm   mem %5d-%5dMi\n", k, cpu_min[k], cpu_max[k], mem_min[k], mem_max[k]
    }
  }
' "$TOP_LOG" | sort | tee -a "$REPORT"
log ""
log "A wide cpu min-max range with a narrow mem range = periodic CPU spike"
log "(e.g. an audit cycle) riding on a flat memory baseline - look at"
log "section 6 for the audit-cycle timing to confirm. A memory value"
log "consistently above the request (section 2) even at the LOW end of the"
log "range = steady-state base cost, not a spike - see section 4/7 for what"
log "makes up that base (synced objects, discovery tree, compiled templates)."

# ---------------------------------------------------------------------------
head1 "4. Gatekeeper's own metrics (per pod, via port-forward to :8888)"
# ---------------------------------------------------------------------------
log "Bypasses kube-rbac-proxy's auth by port-forwarding straight to the"
log "manager container's raw metrics port - fine for read-only diagnosis."
log ""

fetch_metrics() {
  local pod="$1" localport="$2"
  local out="$OUTDIR/metrics_${pod}.prom"
  # </dev/null matters: this function is called from inside a `while read`
  # loop fed by a pipe (see below), and a backgrounded child that inherits
  # that same stdin can steal bytes meant for `read`, silently dropping
  # pods from the loop. Cost nothing to be explicit here.
  kubectl -n "$NAMESPACE" port-forward "pod/$pod" "${localport}:8888" </dev/null >/dev/null 2>&1 &
  local pf_pid=$!
  # give it a moment to establish
  for _ in $(seq 1 20); do
    curl -s -m 1 "http://127.0.0.1:${localport}/metrics" -o "$out" 2>/dev/null && [ -s "$out" ] && break
    sleep 0.3
  done
  kill "$pf_pid" >/dev/null 2>&1
  wait "$pf_pid" 2>/dev/null
  [ -s "$out" ] && echo "$out"
}

port=18800
kubectl -n "$NAMESPACE" get pods -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' | while read -r pod; do
  [ -n "$pod" ] || continue
  port=$((port + 1))
  metrics_file="$(fetch_metrics "$pod" "$port")"
  if [ -z "$metrics_file" ]; then
    log "  $pod: could not fetch /metrics (pod may not expose :8888, e.g. ratify) - skipped"
    continue
  fi
  log "--- $pod ---"
  awk -v pod="$pod" '
    /^process_resident_memory_bytes /   { printf "  rss                         %.1f MiB\n", $2/1048576 }
    /^process_cpu_seconds_total /       { printf "  cpu seconds (since start)   %.1f s\n", $2 }
    /^go_goroutines /                   { printf "  goroutines                  %d\n", $2 }
    /^go_memstats_heap_alloc_bytes /    { printf "  heap in use                 %.1f MiB\n", $2/1048576 }
    /^gatekeeper_watch_manager_watched_gvk / { printf "  watched GVKs                %d\n", $2 }
  ' "$metrics_file" | tee -a "$REPORT"

  # audit duration: histogram -> avg = sum/count
  audit_sum=$(awk '/^gatekeeper_audit_duration_seconds_sum/{print $2}' "$metrics_file")
  audit_count=$(awk '/^gatekeeper_audit_duration_seconds_count/{print $2}' "$metrics_file")
  if [ -n "${audit_sum:-}" ] && [ -n "${audit_count:-}" ] && [ "${audit_count:-0}" != "0" ]; then
    avg=$(awk -v s="$audit_sum" -v c="$audit_count" 'BEGIN{printf "%.2f", s/c}')
    log "  audit runs so far / avg dur  ${audit_count} runs, avg ${avg}s"
  fi

  log "  synced object counts by kind (gatekeeper_sync, memory driver):"
  grep '^gatekeeper_sync{' "$metrics_file" | sed -E 's/^gatekeeper_sync\{([^}]*)\} (.*)$/    \1 -> \2/' | tee -a "$REPORT"

  log "  active constraints by kind (gatekeeper_constraints):"
  grep '^gatekeeper_constraints{' "$metrics_file" | sed -E 's/^gatekeeper_constraints\{([^}]*)\} (.*)$/    \1 -> \2/' | tee -a "$REPORT"
  log ""
done

log "Large gatekeeper_sync counts for a kind (esp. Pod) scale with TOTAL"
log "cluster size, not with how many SecurityPolicy/OperationPolicy CRs are"
log "configured - a big number here for a kind you didn't expect usually"
log "means a built-in constraint needs it unconditionally (see the"
log "'architectural, not fixable' finding for D8DenyExecHeritage in the"
log "main resource-audit report) rather than a misconfiguration."
log ""
log "IMPORTANT: don't try to predict audit CPU from gatekeeper_constraints"
log "count alone. Measured on adyakonov-cloud (audit_cycle_cost.sh): pure"
log "Rego-eval time for one cycle's worth of (object, constraint) pairs is"
log "under 2% of the CPU that cycle actually burned - the rest is API"
log "discovery (scales with CRD count) + LIST calls + JSON unmarshalling +"
log "constraint-status PATCH writes. Use audit_cycle_cost.sh (this"
log "directory) for the real, measured number, not a rule-count estimate."

# ---------------------------------------------------------------------------
head1 "5. Audit-cycle timing trend (from logs)"
# ---------------------------------------------------------------------------
AUDIT_POD="$(kubectl -n "$NAMESPACE" get pods -l control-plane=audit-controller -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)"
if [ -n "$AUDIT_POD" ]; then
  LOG_FILE="$OUTDIR/audit_log_tail.txt"
  kubectl -n "$NAMESPACE" logs "$AUDIT_POD" -c manager --tail=3000 2>/dev/null \
    | grep -E 'audit_started|audit_finished' > "$LOG_FILE" || true
  n_cycles=$(grep -c audit_finished "$LOG_FILE" 2>/dev/null || echo 0)
  log "audit pod: $AUDIT_POD, cycles found in last 3000 log lines: $n_cycles"
  if [ "${n_cycles:-0}" -gt 0 ]; then
    grep -oE '"duration":"[0-9.]+s"' "$LOG_FILE" | grep -oE '[0-9.]+' | awk '
      { sum+=$1; n++; if ($1>max) max=$1; vals[n]=$1 }
      END {
        if (n==0) { print "  no duration fields parsed"; exit }
        avg=sum/n
        printf "  cycles=%d  avg=%.2fs  max=%.2fs  max/avg=%.1fx\n", n, avg, max, max/avg
        if (max > 3*avg) {
          print "  -> at least one cycle ran 3x+ longer than average - check section 1"
          print "     restarts and section 7 cluster-wide events around that time."
        }
      }
    ' | tee -a "$REPORT"
  fi
else
  log "no pod with label control-plane=audit-controller found in $NAMESPACE - skipping"
fi
log ""
log "Compare the average cycle time here with --audit-interval in the"
log "audit Deployment's args (kubectl -n $NAMESPACE get deploy gatekeeper-audit"
log "-o jsonpath='{.spec.template.spec.containers[0].args}'). If the average"
log "cycle time is a large fraction of the interval, the audit loop is"
log "running almost back-to-back - lowering --audit-interval's frequency is"
log "the single highest-leverage lever here (roughly linear: half the"
log "frequency, half the average CPU). Reducing the number/scope of"
log "constraints helps too, but expect a smaller effect than intuition"
log "suggests - most of a cycle's CPU is API interaction (discovery/LIST/"
log "unmarshal/status-write), not Rego eval; run audit_cycle_cost.sh next"
log "to get the real measured split for this cluster."

# ---------------------------------------------------------------------------
head1 "6. Cluster-wide scale (what this module's cost scales with)"
# ---------------------------------------------------------------------------
n_pods=$(kubectl get pods -A --no-headers 2>/dev/null | wc -l | tr -d ' ')
n_ns=$(kubectl get ns --no-headers 2>/dev/null | wc -l | tr -d ' ')
n_nodes=$(kubectl get nodes --no-headers 2>/dev/null | wc -l | tr -d ' ')
n_crds=$(kubectl get crds --no-headers 2>/dev/null | wc -l | tr -d ' ')
log "pods in cluster:      $n_pods"
log "namespaces:            $n_ns"
log "nodes:                 $n_nodes"
log "CRDs in cluster:       $n_crds  (discovery cost during audit scales with this)"
log ""
n_sp=$(kubectl get securitypolicies.deckhouse.io --no-headers 2>/dev/null | wc -l | tr -d ' ')
n_op=$(kubectl get operationpolicies.deckhouse.io --no-headers 2>/dev/null | wc -l | tr -d ' ')
log "SecurityPolicy CRs:    $n_sp"
log "OperationPolicy CRs:   $n_op"
log ""
kubectl get nodes -o custom-columns=NAME:.metadata.name,'MEMPRESSURE:.status.conditions[?(@.type=="MemoryPressure")].status','DISKPRESSURE:.status.conditions[?(@.type=="DiskPressure")].status','PIDPRESSURE:.status.conditions[?(@.type=="PIDPressure")].status' 2>/dev/null | tee -a "$REPORT"

# ---------------------------------------------------------------------------
head1 "7. Cluster-wide warning events (is it just us, or everyone?)"
# ---------------------------------------------------------------------------
EVENTS_FILE="$OUTDIR/warning_events.txt"
kubectl get events -A --field-selector type=Warning --sort-by='.lastTimestamp' 2>/dev/null | tail -80 > "$EVENTS_FILE"
tail -20 "$EVENTS_FILE" | tee -a "$REPORT"
distinct_ns=$(awk 'NR>1{print $1}' "$EVENTS_FILE" | sort -u | wc -l | tr -d ' ')
log ""
log "Distinct namespaces with recent Warning events: $distinct_ns (full list in $EVENTS_FILE)"
if [ "${distinct_ns:-0}" -gt 5 ]; then
  log "-> Warnings are spread across many unrelated namespaces at the same"
  log "   time as this module's restarts/spikes - that points at a NODE or"
  log "   API-server-wide problem (disk pressure, CPU steal, network), not"
  log "   a defect in admission-policy-engine specifically. Check section 6"
  log "   node conditions above, and node disk headroom:"
  log "     kubectl get --raw /api/v1/nodes/<node>/proxy/stats/summary | jq '.node.fs'"
else
  log "-> Warnings are concentrated in $NAMESPACE (or a few namespaces) -"
  log "   more likely specific to this module than a cluster-wide issue."
fi

# ---------------------------------------------------------------------------
head1 "Summary"
# ---------------------------------------------------------------------------
log "Full report and raw data saved under: $OUTDIR"
log "  report.txt              - this output"
log "  pods.json               - full pod specs/status"
log "  top_samples.txt         - raw kubectl top samples"
log "  metrics_<pod>.prom      - raw Prometheus scrape per pod"
log "  audit_log_tail.txt      - audit-cycle log lines"
log "  warning_events.txt      - cluster-wide Warning events"
log ""
log "Next steps:"
log "  - For the real, measured CPU/memory cost of one audit cycle (not an"
log "    estimate): ./audit_cycle_cost.sh"
log "  - If a specific ConstraintTemplate looks implicated (from section 4's"
log "    active-constraints list), benchmark it in isolation with"
log "    ../charts/constraint-templates/tests/tools/rulebench.sh and"
log "    bench_rules.py (time + memory per rule, see that directory's README)"
log "    - keeping in mind that for the AUDIT path specifically, per-rule"
log "    cost is a small slice of the total (see section 4/5 above); for the"
log "    WEBHOOK path (controller-manager), rulebench numbers ARE directly"
log "    representative, since one admission review = one object eval with"
log "    no discovery/LIST overhead."
