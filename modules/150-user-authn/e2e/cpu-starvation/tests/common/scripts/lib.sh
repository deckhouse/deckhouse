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

# Shared functions of the CPU starvation scenarios. Source it from a Chainsaw script:
#   . ../common/scripts/lib.sh
# Chainsaw sets NAMESPACE to the test namespace. The state of a run that later steps need is kept
# in the ConfigMap cpu-test-state of the test namespace, because Chainsaw bindings do not cross steps.
# A function that reads the API server fails when the read fails, so an API error is never taken for
# an empty result. Assign the output of such a function to a variable before using it: a command
# substitution inside the arguments of another command loses its exit status.

set -eu

UA_NS=d8-user-authn
STATE=cpu-test-state
LOAD_POD=dex-load
KUBECTL="kubectl --request-timeout=30s"
VPA=verticalpodautoscalers.autoscaling.k8s.io
# Every object a run creates or changes outside the test namespace carries this label, and so does
# the test namespace itself (chainsaw-config.yaml); `task cleanup` removes what an interrupted run left.
E2E_LABEL_KEY=e2e-test
E2E_LABEL_VALUE=user-authn-cpu-starvation
E2E_LABEL="$E2E_LABEL_KEY=$E2E_LABEL_VALUE"
# The update mode the VPA dex had before a run switched it to Off.
VPA_MODE_ANNOTATION=e2e-test.deckhouse.io/original-update-mode
# The update mode of the VPA dex in the module templates.
VPA_DEFAULT_MODE=InPlaceOrRecreate

log() { echo "$(date -u +%H:%M:%S) $*"; }

# Prints "=<VALUE>" when the map at the go-template path $1 of the object has the key $2, and
# nothing when it has not. The remaining arguments select the object for kubectl get.
map_lookup() {
  _path=$1 _key=$2
  shift 2
  $KUBECTL get "$@" -o go-template="{{range \$k, \$v := $_path}}{{if eq \$k \"$_key\"}}={{\$v}}{{end}}{{end}}"
}

# Writes a key of the run state. Quotes, backslashes and line breaks of the value are replaced,
# so the value always fits into the JSON of the patch.
state_set() {
  # shellcheck disable=SC1003 # the last character of the set is a backslash
  _value=$(printf '%s' "$2" | tr '\n\t"\\' "  '/")
  if ! _err=$($KUBECTL -n "$NAMESPACE" patch configmap "$STATE" --type merge \
    -p "{\"data\":{\"$1\":\"$_value\"}}" 2>&1 >/dev/null); then
    case "$_err" in
      *NotFound* | *"not found"*)
        $KUBECTL -n "$NAMESPACE" create configmap "$STATE" --from-literal="$1=$_value" >/dev/null ;;
      *)
        log "cannot write $1 to the ConfigMap $STATE: $_err" >&2
        return 1 ;;
    esac
  fi
}

# Prints a key of the run state; fails when the key is missing or the ConfigMap cannot be read.
state_get() {
  _found=$(map_lookup .data "$1" -n "$NAMESPACE" configmap "$STATE") ||
    { log "cannot read $1 from the ConfigMap $STATE" >&2; return 1; }
  case "$_found" in
    =*) printf '%s\n' "${_found#=}" ;;
    *) log "the ConfigMap $STATE has no key $1" >&2; return 1 ;;
  esac
}

# Prints a key of the run state, or nothing when the key is missing; fails when the ConfigMap cannot be read.
state_get_optional() {
  _found=$(map_lookup .data "$1" -n "$NAMESPACE" configmap "$STATE") ||
    { log "cannot read $1 from the ConfigMap $STATE" >&2; return 1; }
  case "$_found" in
    =*) printf '%s\n' "${_found#=}" ;;
  esac
}

# The Dex pod under test: the first running replica by name. Its node gets the pressure.
dex_target_pod() {
  _pods=$($KUBECTL -n "$UA_NS" get pods -l app=dex --field-selector=status.phase=Running \
    -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}') || return 1
  printf '%s\n' "$_pods" | sort | sed -n '/./{p;q;}'
}

# Converts a Kubernetes CPU quantity (2, 1.5, 250m) to millicores.
to_millicores() {
  awk -v q="$1" 'BEGIN { if (q ~ /m$/) { sub(/m$/, "", q); printf "%d", q } else printf "%d", q * 1000 }'
}

# Free CPU of a node in millicores: allocatable minus the effective CPU requests of the pods that hold
# resources. The effective request of a pod is the one the scheduler uses: the larger of the
# containers plus the native sidecars (init containers with restartPolicy Always) and of the largest
# regular init container plus the sidecars started before it, plus the pod overhead.
node_free_millicores() {
  _alloc=$($KUBECTL get node "$1" -o jsonpath='{.status.allocatable.cpu}') || return 1
  [ -n "$_alloc" ] || { log "node $1 reports no allocatable CPU" >&2; return 1; }
  _pods=$($KUBECTL get pods -A --field-selector="spec.nodeName=$1,status.phase!=Succeeded,status.phase!=Failed" \
    -o jsonpath='{range .items[*]}P|{.spec.overhead.cpu}{"\n"}{range .spec.initContainers[*]}I|{.restartPolicy}|{.resources.requests.cpu}{"\n"}{end}{range .spec.containers[*]}C|{.resources.requests.cpu}{"\n"}{end}{end}') ||
    return 1
  _used=$(printf '%s\n' "$_pods" | pod_cpu_requests)
  echo $(($(to_millicores "$_alloc") - _used))
}

# Sums the effective CPU requests in millicores of the "P|<overhead>", "I|<restartPolicy>|<cpu>" and
# "C|<cpu>" lines that node_free_millicores reads, one P line per pod followed by its containers.
pod_cpu_requests() {
  awk -F'|' '
    function mc(q) { if (q == "") return 0; if (q ~ /m$/) { sub(/m$/, "", q); return q + 0 } return q * 1000 }
    function flush() { if (pod) { p = run + side; if (init > p) p = init; total += p + over } }
    $1 == "P" { flush(); pod = 1; over = mc($2); run = 0; side = 0; init = 0; next }
    $1 == "I" { if ($2 == "Always") side += mc($3); else if (mc($3) + side > init) init = mc($3) + side; next }
    $1 == "C" { run += mc($2) }
    END { flush(); printf "%d", total }'
}

node_cpus() {
  _cap=$($KUBECTL get node "$1" -o jsonpath='{.status.capacity.cpu}') || return 1
  [ -n "$_cap" ] || { log "node $1 reports no CPU capacity" >&2; return 1; }
  to_millicores "$_cap" | awk '{ printf "%d", $1 / 1000 }'
}

# "<container>=<restartCount> ..." of a pod, sorted by container name.
restart_counts() {
  _counts=$($KUBECTL -n "$UA_NS" get pod "$1" \
    -o jsonpath='{range .status.containerStatuses[*]}{.name}={.restartCount}{"\n"}{end}') || return 1
  printf '%s\n' "$_counts" | sort | tr '\n' ' '
}

# "<lease>:<holderIdentity>:<leaseTransitions> ..." of the module leases.
lease_state() {
  _leases=$($KUBECTL -n "$UA_NS" get leases \
    -o jsonpath='{range .items[*]}{.metadata.name}:{.spec.holderIdentity}:{.spec.leaseTransitions}{"\n"}{end}') ||
    return 1
  printf '%s\n' "$_leases" | sort | tr '\n' ' '
}

authrequest_count() {
  _names=$($KUBECTL -n "$UA_NS" get authrequests.dex.coreos.com -o name) || return 1
  printf '%s\n' "$_names" | awk 'NF { n++ } END { print n + 0 }'
}

# Runs flowsum.py inside the load pod, so marks share the clock of the flows and the writes.
flowsum() {
  $KUBECTL -n "$NAMESPACE" exec "$LOAD_POD" -c load -- python3 -I /scripts/flowsum.py "$@"
}

# In-place resize of one container of a pod: resize_container <POD> <CONTAINER> <requests|limits> <CPU>.
resize_container() {
  $KUBECTL -n "$UA_NS" patch pod "$1" --subresource resize \
    -p "{\"spec\":{\"containers\":[{\"name\":\"$2\",\"resources\":{\"$3\":{\"cpu\":\"$4\"}}}]}}" >/dev/null
  log "resize $UA_NS/$1/$2 $3.cpu=$4"
}

# Waits until the kubelet reports the CPU of a container as applied: wait_resized <POD> <CONTAINER> <requests|limits> <CPU>.
wait_resized() {
  want=$(to_millicores "$4")
  i=0
  while [ $i -lt 30 ]; do
    got=$($KUBECTL -n "$UA_NS" get pod "$1" \
      -o jsonpath="{.status.containerStatuses[?(@.name==\"$2\")].resources.$3.cpu}")
    if [ -n "$got" ] && [ "$(to_millicores "$got")" -eq "$want" ]; then
      log "applied $1/$2 $3.cpu=$got"
      return 0
    fi
    i=$((i + 1))
    sleep 2
  done
  log "resize of $1/$2 to $3.cpu=$4 is not applied; pod conditions:"
  $KUBECTL -n "$UA_NS" get pod "$1" -o jsonpath='{range .status.conditions[*]}{.type}={.status} {.reason} {.message}{"\n"}{end}'
  return 1
}

# Records a failed check: fail_check <NAME> <MESSAGE>. Every check is its own key failed.<NAME>, so a
# check never overwrites another one. The verdict step fails the test when any check failed, so every
# check still runs and reports on a broken Dex. Fails when the check cannot be recorded.
fail_check() {
  _check=$1
  shift
  log "FAIL: $_check: $*"
  state_set "failed.$_check" "$*" || { log "cannot record the failed check $_check" >&2; return 1; }
}

# Switches the VPA dex to updateMode Off and keeps its mode in an annotation of the VPA. The
# annotation of an interrupted run is kept, because it holds the mode before that run. The key
# holdVpaOff of the run state tells step_hold to check that the mode stays Off.
vpa_off() {
  _vpa=$($KUBECTL -n "$UA_NS" get "$VPA" dex --ignore-not-found -o name) || return 1
  [ -n "$_vpa" ] || { log "no VPA dex in $UA_NS"; return 0; }
  _saved=$(map_lookup .metadata.annotations "$VPA_MODE_ANNOTATION" -n "$UA_NS" "$VPA" dex) || return 1
  _mode=$($KUBECTL -n "$UA_NS" get "$VPA" dex -o jsonpath='{.spec.updatePolicy.updateMode}') || return 1
  case "$_saved" in
    =*) _keep=${_saved#=}; log "VPA dex: keeping the mode $_keep saved by an earlier run" ;;
    *) _keep=$_mode ;;
  esac
  # The VPA would resize the pod back to its recommendation during the run.
  $KUBECTL -n "$UA_NS" patch "$VPA" dex --type merge \
    -p "{\"metadata\":{\"annotations\":{\"$VPA_MODE_ANNOTATION\":\"$_keep\"}},\"spec\":{\"updatePolicy\":{\"updateMode\":\"Off\"}}}" >/dev/null
  log "VPA dex: updateMode ${_mode:-<unset>} -> Off for the run"
  state_set holdVpaOff true
}

# Returns the VPA dex to the mode saved by vpa_off and removes the annotation. A VPA left in Off without
# the annotation gets the mode of the module templates.
vpa_restore() {
  _vpa=$($KUBECTL -n "$UA_NS" get "$VPA" dex --ignore-not-found -o name) || return 1
  [ -n "$_vpa" ] || return 0
  _saved=$(map_lookup .metadata.annotations "$VPA_MODE_ANNOTATION" -n "$UA_NS" "$VPA" dex) || return 1
  _mode=$($KUBECTL -n "$UA_NS" get "$VPA" dex -o jsonpath='{.spec.updatePolicy.updateMode}') || return 1
  case "$_saved" in
    =*) _restore=${_saved#=} ;;
    *)
      if [ "$_mode" != Off ]; then
        log "VPA dex: updateMode $_mode, not changed by a run"
        return 0
      fi
      _restore= ;;
  esac
  _restore=${_restore:-$VPA_DEFAULT_MODE}
  $KUBECTL -n "$UA_NS" patch "$VPA" dex --type merge \
    -p "{\"metadata\":{\"annotations\":{\"$VPA_MODE_ANNOTATION\":null}},\"spec\":{\"updatePolicy\":{\"updateMode\":\"$_restore\"}}}" >/dev/null
  log "VPA dex: updateMode $_mode -> $_restore"
}

# Deletes the Dex pods that a run has changed (they carry the e2e label) and waits for the Deployment.
delete_changed_dex_pods() {
  $KUBECTL -n "$UA_NS" delete pod -l "$E2E_LABEL" --ignore-not-found
  kubectl -n "$UA_NS" wait pod -l "$E2E_LABEL" --for=delete --timeout=120s >/dev/null 2>&1 || true
  kubectl -n "$UA_NS" rollout status deployment/dex --timeout=300s
}

# Removes what interrupted runs left behind: `task cleanup`. Every step runs even when an earlier one
# fails, and the function fails when any step failed.
cleanup_leftovers() {
  _rc=0
  log "VPA dex"
  vpa_restore || _rc=1
  log "test namespaces (burner, load pod, DexClient, run state)"
  _ns=$($KUBECTL get namespaces -l "$E2E_LABEL" -o name) || _rc=1
  if [ -n "${_ns:-}" ]; then
    # shellcheck disable=SC2086 # one argument per namespace
    $KUBECTL delete $_ns --wait=false || _rc=1
  fi
  log "users, RBAC"
  $KUBECTL delete users.deckhouse.io -l "$E2E_LABEL" || _rc=1
  $KUBECTL -n "$UA_NS" delete roles.rbac.authorization.k8s.io,rolebindings.rbac.authorization.k8s.io -l "$E2E_LABEL" || _rc=1
  $KUBECTL delete clusterroles.rbac.authorization.k8s.io,clusterrolebindings.rbac.authorization.k8s.io -l "$E2E_LABEL" || _rc=1
  log "changed Dex pods"
  delete_changed_dex_pods || _rc=1
  if [ -n "${_ns:-}" ]; then
    # shellcheck disable=SC2086 # one argument per namespace
    kubectl wait $_ns --for=delete --timeout=600s || _rc=1
  fi
  if [ "$_rc" -ne 0 ]; then
    log "cleanup incomplete, see the errors above" >&2
    return 1
  fi
  log "cleanup done"
}

# Checks the Kubernetes version of the API server: check_kubernetes_minor <MINOR>.
check_kubernetes_minor() {
  _version=$($KUBECTL get --raw /version) || { log "cannot read the API server version" >&2; return 1; }
  _minor=$(printf '%s\n' "$_version" | sed -n 's/.*"minor": *"\([0-9]*\).*/\1/p' | sed -n 1p)
  [ -n "$_minor" ] || { log "no minor version in the /version response: $_version" >&2; return 1; }
  log "Kubernetes server version 1.$_minor"
  [ "$_minor" -ge "$1" ] || { log "the scenario needs Kubernetes 1.$1 or newer" >&2; return 1; }
}

# At least two Ready nodes, so the load runs outside the Dex node.
check_nodes() {
  _nodes=$($KUBECTL get nodes --no-headers) || return 1
  _ready=$(printf '%s\n' "$_nodes" | awk '$2 == "Ready" { n++ } END { print n + 0 }')
  log "Ready nodes: $_ready"
  [ "$_ready" -ge 2 ] || { log "the scenarios need at least two Ready nodes" >&2; return 1; }
}

# ---------------------------------------------------------------------------------------------------
# Steps. Each function is one Chainsaw script; the bindings arrive as environment variables.

# Publishes the scripts of the load pod and prints "<NODE> <URL>": the node of the Dex pod under test
# and the HTTPS endpoint of that pod. The load goes to the pod and not to the Service, so in a cluster
# with several Dex replicas all of it reaches the replica on the starved node.
step_prepare_load() {
  $KUBECTL -n "$NAMESPACE" create configmap dex-load-scripts \
    --from-file=../common/scripts/dexload.py \
    --from-file=../common/scripts/writewatch.py \
    --from-file=../common/scripts/flowsum.py >/dev/null
  pod=$(dex_target_pod)
  [ -n "$pod" ] || { log "no running Dex pod in $UA_NS" >&2; return 1; }
  target=$($KUBECTL -n "$UA_NS" get pod "$pod" \
    -o jsonpath='{.spec.nodeName} {.status.podIP} {.spec.containers[?(@.name=="dex")].ports[?(@.name=="https")].containerPort}')
  # shellcheck disable=SC2086 # three words
  set -- $target
  [ $# -eq 3 ] || { log "cannot read the node, the IP and the HTTPS port of the Dex pod $pod: [$target]" >&2; return 1; }
  node=$1 ip=$2 port=$3
  case "$ip" in *:*) ip="[$ip]" ;; esac
  state_set dexPod "$pod"
  state_set dexNode "$node"
  log "Dex pod $pod on $node, load target https://$ip:$port" >&2
  echo "$node https://$ip:$port"
}

# Prints the bcrypt hash that the init container of the load pod computed.
step_password_hash() {
  $KUBECTL -n "$NAMESPACE" wait pod/"$LOAD_POD" --timeout=180s \
    --for=jsonpath='{.status.initContainerStatuses[0].state.terminated.reason}'=Completed >/dev/null
  hash=$($KUBECTL -n "$NAMESPACE" logs "$LOAD_POD" -c hash | sed -n 's/^HASH //p')
  # shellcheck disable=SC2016 # the bcrypt prefix, not a shell expression
  case "$hash" in
    '$2'*) echo "$hash" ;;
    *) log "the init container printed no bcrypt hash" >&2; return 1 ;;
  esac
}

# Waits until the load has completed a login and a refresh: Deckhouse has registered the client
# and the users in Dex.
step_warmup() {
  $KUBECTL -n "$NAMESPACE" wait pod/"$LOAD_POD" --for=condition=Ready --timeout=180s >/dev/null
  i=0
  while [ $i -lt 60 ]; do
    if flowsum warmup; then return 0; fi
    i=$((i + 1))
    sleep 5
  done
  log "no successful login and refresh in 5 minutes; last flows:"
  $KUBECTL -n "$NAMESPACE" logs "$LOAD_POD" -c load --tail=20 || true
  return 1
}

# LA scenarios: gives Dex the CPU it uses under the load, in place, before the pressure.
# The pod gets the e2e label first, so the cleanup and `task cleanup` find and recreate it.
# The key holdDexRequest of the run state tells step_hold to check that the request stays.
# Environment: DEX_REQUEST, PROXY_REQUEST, SET_LIMITS (true: limits equal to the requests).
step_raise_dex_request() {
  pod=$(state_get dexPod)
  vpa_off
  $KUBECTL -n "$UA_NS" label pod "$pod" "$E2E_LABEL" --overwrite >/dev/null
  resize_container "$pod" dex requests "$DEX_REQUEST"
  resize_container "$pod" kube-rbac-proxy requests "$PROXY_REQUEST"
  if [ "$SET_LIMITS" = true ]; then
    resize_container "$pod" dex limits "$DEX_REQUEST"
    resize_container "$pod" kube-rbac-proxy limits "$PROXY_REQUEST"
  fi
  wait_resized "$pod" dex requests "$DEX_REQUEST"
  wait_resized "$pod" kube-rbac-proxy requests "$PROXY_REQUEST"
  if [ "$SET_LIMITS" = true ]; then
    wait_resized "$pod" dex limits "$DEX_REQUEST"
    wait_resized "$pod" kube-rbac-proxy limits "$PROXY_REQUEST"
  fi
  state_set holdDexRequest "$DEX_REQUEST"
}

# Cleanup of the LA scenarios. Limits cannot be removed in place, so the resized pod is deleted and
# the Deployment recreates it from its template. Both parts run even when the other one fails.
step_restore_dex() {
  rc=0
  vpa_restore || rc=1
  delete_changed_dex_pods || rc=1
  return "$rc"
}

# Records the state before the pressure and measures the baseline for BASELINE seconds.
step_record_baseline() {
  pod=$(state_get dexPod)
  node=$(state_get dexNode)
  uid=$($KUBECTL -n "$UA_NS" get pod "$pod" -o jsonpath='{.metadata.uid}')
  [ -n "$uid" ] || { log "the Dex pod $pod has no UID" >&2; return 1; }
  restarts=$(restart_counts "$pod")
  leases=$(lease_state)
  authrequests=$(authrequest_count)
  state_set dexUID "$uid"
  state_set restarts "$restarts"
  state_set leases "$leases"
  state_set authRequestsBefore "$authrequests"
  log "Dex pod $pod on $node: restarts $restarts"
  resources=$($KUBECTL -n "$UA_NS" get pod "$pod" \
    -o jsonpath='{range .spec.containers[*]}{.name}={.resources}{" "}{end}')
  log "Dex resources: $resources"
  log "leases: $leases"
  log "AuthRequests: $authrequests"
  mark=$(flowsum mark baseline)
  log "baseline mark at $mark; measuring for ${BASELINE}s"
  sleep "$BASELINE"
}

# Pod slots of a node left for new pods: allocatable pods minus the pods that are not finished.
node_free_pod_slots() {
  _max=$($KUBECTL get node "$1" -o jsonpath='{.status.allocatable.pods}') || return 1
  [ -n "$_max" ] || { log "node $1 reports no allocatable pods" >&2; return 1; }
  _names=$($KUBECTL get pods -A --field-selector="spec.nodeName=$1,status.phase!=Succeeded,status.phase!=Failed" -o name) ||
    return 1
  _count=$(printf '%s\n' "$_names" | awk 'NF { n++ } END { print n + 0 }')
  echo $((_max - _count))
}

# Prints the size of the burner as "<CPU>m <THREADS> <REPLICAS>": the CPU request and the busy threads of
# one burner pod and the number of pods. The pods together request the free CPU of the node.
# CFS shares the CPU between cgroups by their weight, so the threads inside one pod compete only with
# each other: one pod with many threads raises the load average but takes no more CPU from Dex than one
# thread per CPU. With THREADS_PER_POD 0 the burner is one pod with THREADS_FACTOR threads per CPU
# (noisy neighbour). With THREADS_PER_POD N the burner is THREADS_FACTOR threads per CPU spread over
# pods of N threads each, every pod its own cgroup with its own weight, so Dex competes with all of
# them for the CPU (high load average). The number of pods is limited by MAX_PODS and by the free pod
# slots of the node minus POD_SLOT_MARGIN.
# Environment: THREADS_FACTOR, CPU_MARGIN (millicores left free on the node), THREADS_PER_POD, MAX_PODS.
POD_SLOT_MARGIN=5
step_stress_size() {
  node=$(state_get dexNode)
  unrequested=$(node_free_millicores "$node")
  cpus=$(node_cpus "$node")
  free=$((unrequested - CPU_MARGIN))
  total=$((cpus * THREADS_FACTOR))
  if [ "${THREADS_PER_POD:-0}" -le 0 ]; then
    replicas=1
    threads=$total
  else
    slots=$(node_free_pod_slots "$node")
    threads=$THREADS_PER_POD
    replicas=$(((total + threads - 1) / threads))
    limit=$((slots - POD_SLOT_MARGIN))
    [ "$limit" -le "$MAX_PODS" ] || limit=$MAX_PODS
    if [ "$replicas" -gt "$limit" ]; then
      log "node $node: $total threads need $replicas pods, $slots pod slots free; the burner is limited to $limit pods" >&2
      replicas=$limit
    fi
  fi
  log "node $node: $cpus CPUs, ${unrequested}m not requested; burner: $replicas pods requesting ${free}m in total, $threads threads each" >&2
  if [ "$replicas" -lt 1 ]; then
    log "the node has no free pod slots for the burner" >&2
    return 1
  fi
  if [ "$free" -lt 100 ]; then
    log "the node has no free CPU to request" >&2
    return 1
  fi
  if [ "$threads" -lt 1 ]; then
    log "the burner would run no threads" >&2
    return 1
  fi
  echo "$((free / replicas))m $threads $replicas"
}

step_mark_pressure() {
  $KUBECTL -n "$NAMESPACE" get pods -l app=cpu-stress -o wide | awk 'NR <= 4 { print } END { print NR - 1 " burner pods" }'
  mark=$(flowsum mark pressure)
  log "pressure mark at $mark"
}

# Prints why the VPA dex no longer has updateMode Off, or nothing when it has; fails when the VPA cannot
# be read.
hold_vpa_drift() {
  _vpa=$($KUBECTL -n "$UA_NS" get "$VPA" dex --ignore-not-found \
    -o jsonpath='{.metadata.name}|{.spec.updatePolicy.updateMode}') || return 1
  case "$_vpa" in
    'dex|Off') ;;
    '') echo "VPA dex was deleted during the run" ;;
    'dex|') echo "VPA updateMode was removed during the run, the VPA default applies" ;;
    *) echo "VPA updateMode changed to ${_vpa#*|} during the run" ;;
  esac
}

# Prints why the CPU request of the container dex that the kubelet reports differs from the expected
# one, or nothing when it does not: hold_dex_request_drift <POD> <CPU>. Fails when the pod cannot be
# read or reports no request of dex.
hold_dex_request_drift() {
  _status=$($KUBECTL -n "$UA_NS" get pod "$1" --ignore-not-found \
    -o jsonpath="{.metadata.name}|{.status.containerStatuses[?(@.name==\"dex\")].resources.requests.cpu}") ||
    return 1
  _got=${_status#*|}
  if [ -z "$_status" ]; then
    echo "the Dex pod $1 was deleted during the run, its CPU request of $2 is lost"
  elif [ -z "$_got" ]; then
    log "the Dex pod $1 reports no CPU request of the container dex" >&2
    return 1
  elif [ "$(to_millicores "$_got")" -ne "$(to_millicores "$2")" ]; then
    echo "dex CPU request changed from $2 to $_got"
  fi
}

# Holds the pressure for HOLD seconds and logs the Dex pod every 30 seconds. A failed read of the
# AuthRequests is logged and skipped; the step fails when the burner is not running.
# The LA scenarios also check every 30 seconds that the VPA dex keeps updateMode Off (when the run
# switched it) and that the kubelet reports the raised CPU request of dex. The step fails when either
# drifts; a failed read of the API server is logged and retried at the next check.
step_hold() {
  pod=$(state_get dexPod)
  want_vpa_off=$(state_get_optional holdVpaOff)
  want_request=$(state_get_optional holdDexRequest)
  [ "$want_vpa_off" != true ] || log "checking that the VPA dex keeps updateMode Off"
  [ -z "$want_request" ] || log "checking that the Dex pod $pod keeps the CPU request $want_request of dex"
  max=0
  start=$(date +%s)
  while [ $(($(date +%s) - start)) -lt "$HOLD" ]; do
    if [ "$want_vpa_off" = true ]; then
      if drift=$(hold_vpa_drift); then
        [ -z "$drift" ] || { log "$drift" >&2; return 1; }
      else
        log "cannot read the VPA dex, the next check is in 30s" >&2
      fi
    fi
    if [ -n "$want_request" ]; then
      if drift=$(hold_dex_request_drift "$pod" "$want_request"); then
        [ -z "$drift" ] || { log "$drift" >&2; return 1; }
      else
        log "cannot read the CPU request of dex, the next check is in 30s" >&2
      fi
    fi
    if n=$(authrequest_count); then
      [ "$n" -gt "$max" ] && max=$n
    else
      n="unknown (API error)"
    fi
    status=$($KUBECTL -n "$UA_NS" get pod "$pod" --no-headers 2>&1 | awk '{print $2, $3, $4}') || true
    log "dex: $status AuthRequests=$n"
    if burner=$($KUBECTL -n "$NAMESPACE" get pods -l app=cpu-stress --field-selector=status.phase=Running -o name); then
      if [ -z "$burner" ]; then
        log "the burner is not running" >&2
        return 1
      fi
    else
      log "cannot list the burner pods" >&2
    fi
    sleep 30
  done
  state_set authRequestsMax "$max"
}

step_mark_release() {
  $KUBECTL -n "$NAMESPACE" wait pod -l app=cpu-stress --for=delete --timeout=120s >/dev/null 2>&1 || true
  mark=$(flowsum mark release)
  log "release mark at $mark"
}

# Every pod of the module is Ready within RECOVERY seconds after the release, none in CrashLoopBackOff.
step_assert_pods_recover() {
  if ! $KUBECTL -n "$UA_NS" wait pod --all --for=condition=Ready --timeout="${RECOVERY}s"; then
    fail_check pods-ready "not every pod of $UA_NS is Ready ${RECOVERY}s after the release"
  fi
  if states=$($KUBECTL -n "$UA_NS" get pods \
    -o jsonpath='{range .items[*]}{.metadata.name}{" "}{.status.containerStatuses[*].state.waiting.reason}{"\n"}{end}'); then
    loop=$(printf '%s\n' "$states" | grep CrashLoopBackOff || true)
    [ -z "$loop" ] || fail_check crashloop "CrashLoopBackOff after the release: $loop"
  else
    fail_check crashloop "cannot read the pods of $UA_NS"
  fi
  $KUBECTL -n "$UA_NS" get pods -o wide
}

# Keeps the load running until AFTER seconds after the release, then marks the end.
step_observe_after() {
  release=$($KUBECTL -n "$NAMESPACE" exec "$LOAD_POD" -c load -- sed -n 's/^release //p' /data/marks)
  [ -n "$release" ] || { log "no release mark in the load pod" >&2; return 1; }
  now=$($KUBECTL -n "$NAMESPACE" exec "$LOAD_POD" -c load -- date +%s)
  # shellcheck disable=SC2153 # AFTER comes from the environment
  left=$((release + AFTER - now))
  log "observing the recovery for ${left}s more"
  [ "$left" -le 0 ] || sleep "$left"
  mark=$(flowsum mark end)
  log "end mark at $mark"
}

# Success rates per phase, recovery of the logins and the write storm check (flowsum.py), then the
# restarts and the leases against the baseline. A failed read of the API server fails the check.
# Environment: RECOVERY, MIN_PRESSURE, MIN_RECOVERED, WRITE_FACTOR, WRITE_SLACK, ASSERT_RESTARTS, ASSERT_LEASES.
step_evaluate() {
  if ! flowsum report --recovery "$RECOVERY" --min-pressure "$MIN_PRESSURE" --min-recovered "$MIN_RECOVERED" \
    --write-factor "$WRITE_FACTOR" --write-slack "$WRITE_SLACK"; then
    fail_check function "see the report above"
  fi

  pod=$(state_get dexPod)
  uid_before=$(state_get dexUID)
  before=$(state_get restarts)
  [ -n "$uid_before" ] || { log "no UID of the Dex pod was recorded before the run" >&2; return 1; }
  msg=
  if ! uid=$($KUBECTL -n "$UA_NS" get pod "$pod" --ignore-not-found -o jsonpath='{.metadata.uid}'); then
    fail_check restarts "cannot read the Dex pod $pod"
  elif [ "$uid" != "$uid_before" ]; then
    msg="the Dex pod $pod was replaced during the run"
  elif ! after=$(restart_counts "$pod"); then
    fail_check restarts "cannot read the restart counts of the Dex pod $pod"
  else
    log "Dex restarts: before [$before] after [$after]"
    [ "$before" = "$after" ] || msg="Dex containers restarted: before [$before] after [$after]"
  fi
  if [ -n "$msg" ]; then
    if [ "$ASSERT_RESTARTS" = true ]; then fail_check restarts "$msg"; else log "INFO: $msg"; fi
  fi
  log "restarts of the other pods of $UA_NS:"
  $KUBECTL -n "$UA_NS" get pods \
    -o 'custom-columns=POD:.metadata.name,CONTAINERS:.status.containerStatuses[*].name,RESTARTS:.status.containerStatuses[*].restartCount' ||
    true

  before=$(state_get leases)
  if ! after=$(lease_state); then
    fail_check leases "cannot read the leases of $UA_NS"
    return 0
  fi
  log "leases: before [$before] after [$after]"
  if [ "$before" != "$after" ]; then
    if [ "$ASSERT_LEASES" = true ]; then
      fail_check leases "holderIdentity or leaseTransitions of a lease changed"
    else
      log "INFO: holderIdentity or leaseTransitions of a lease changed"
    fi
  fi
}

# Stops the load and waits up to AUTHREQ_DRAIN seconds until the AuthRequests return to the count
# before the run plus AUTHREQ_SLACK. Dex removes the AuthRequests of finished logins at once and
# collects the abandoned ones after their 10-minute expiry. A failed read is retried.
step_drain_authrequests() {
  $KUBECTL -n "$NAMESPACE" delete pod "$LOAD_POD" --wait=true >/dev/null
  before=$(state_get authRequestsBefore)
  max=$(state_get authRequestsMax)
  log "AuthRequests: before the run $before, at most $max under the pressure (quota 10000)"
  limit=$((before + AUTHREQ_SLACK))
  start=$(date +%s)
  while :; do
    if n=$(authrequest_count); then
      if [ "$n" -le "$limit" ]; then
        log "AuthRequests: $n after $(($(date +%s) - start))s (limit $limit)"
        return 0
      fi
    else
      n="unknown (API error)"
    fi
    if [ $(($(date +%s) - start)) -ge "$AUTHREQ_DRAIN" ]; then
      fail_check authrequests "$n AuthRequests ${AUTHREQ_DRAIN}s after the load stopped (limit $limit)"
      return 0
    fi
    log "AuthRequests: $n (limit $limit)"
    sleep 30
  done
}

# Fails when a check recorded a failed.<NAME> key or when the run state cannot be read.
step_verdict() {
  # shellcheck disable=SC2016 # a go-template, not a shell expression
  state=$($KUBECTL -n "$NAMESPACE" get configmap "$STATE" \
    -o go-template='{{range $k, $v := .data}}{{$k}} {{$v}}{{"\n"}}{{end}}') ||
    { log "cannot read the checks from the ConfigMap $STATE" >&2; return 1; }
  failed=$(printf '%s\n' "$state" | awk 'index($0, "failed.") == 1 { print "  " substr($0, 8) }')
  if [ -n "$failed" ]; then
    log "failed checks (details in the steps above):"
    printf '%s\n' "$failed"
    return 1
  fi
  log "all checks passed"
}
