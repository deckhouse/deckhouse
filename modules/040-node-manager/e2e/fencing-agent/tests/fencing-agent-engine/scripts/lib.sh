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

# Shared helpers of the fencing-agent-engine scenario, sourced by every step.
#
# Chainsaw passes NODE_GROUP and sets NAMESPACE (the test namespace the probe
# DaemonSet lives in). Node-side facts are read through that probe: kernel.panic
# and /proc/cmdline are global, and sysfs shows the host's watchdog class.

set -eu

: "${NODE_GROUP:?NODE_GROUP is not set}"
: "${NAMESPACE:?NAMESPACE is not set}"

# shellcheck disable=SC2034 # used by the steps that source this file
AGENT_NAMESPACE=d8-cloud-instance-manager
STATE_DIR="${TMPDIR:-/tmp}/fencing-agent-engine-${NAMESPACE}"
WATCHDOG_SYSFS=/sys/class/watchdog/watchdog0
mkdir -p "${STATE_DIR}"

# group_nodes lists the Ready nodes of the group under test, one per line.
group_nodes() {
  kubectl get nodes -l "node.deckhouse.io/group=${NODE_GROUP}" \
    -o jsonpath='{range .items[*]}{.metadata.name}{" "}{.status.conditions[?(@.type=="Ready")].status}{"\n"}{end}' |
    awk '$2 == "True" { print $1 }'
}

# retry <seconds> <what> <command...> runs the command until it succeeds.
retry() {
  deadline=$(($(date +%s) + $1))
  what=$2
  shift 2
  until "$@"; do
    if [ "$(date +%s)" -ge "${deadline}" ]; then
      echo "timed out waiting for: ${what}" >&2
      return 1
    fi
    sleep 5
  done
}

# for_each_node <command...> runs the command with every node name appended and
# fails on the first node it fails for.
for_each_node() {
  nodes=$(group_nodes)
  if [ -z "${nodes}" ]; then
    echo "node group ${NODE_GROUP} has no Ready nodes" >&2
    return 1
  fi
  for node in ${nodes}; do
    "$@" "${node}" || return 1
  done
}

probe_pod() {
  kubectl -n "${NAMESPACE}" get pods -l app=fencing-e2e-probe \
    --field-selector "spec.nodeName=$1,status.phase=Running" \
    -o jsonpath='{.items[0].metadata.name}'
}

# on_node <node> <command...> runs the command in the probe pod of that node.
on_node() {
  node=$1
  shift
  pod=$(probe_pod "${node}")
  if [ -z "${pod}" ]; then
    echo "no running probe pod on ${node}" >&2
    return 1
  fi
  kubectl -n "${NAMESPACE}" exec "${pod}" -- "$@"
}

node_read() {
  on_node "$1" cat "$2" | tr -d '[:space:]'
}

# expect_eq <what> <got> <want>
expect_eq() {
  if [ "$2" != "$3" ]; then
    echo "$1: got '$2', want '$3'" >&2
    return 1
  fi
}

nodeconfig_panic() {
  kubectl get nodeconfigs.internal.deckhouse.io "$1" -o jsonpath='{.spec.kernel.sysctl.kernel\.panic}'
}

# duration_seconds turns a Go duration with whole seconds ("3s", "1m30s",
# "2000ms") into seconds.
duration_seconds() {
  case "$1" in
    *ms) echo $((${1%ms} / 1000)) ;;
    *) echo $(($(echo "$1" | sed -e 's/h/*3600+/' -e 's/m/*60+/' -e 's/s//' -e 's/+$//'))) ;;
  esac
}

# profile_timeout_seconds is the watchdog timeout of the FencingSLAProfile the
# fencing mode of the group selects; the agent applies it to the device with
# WDIOC_SETTIMEOUT.
profile_timeout_seconds() {
  profile=$(kubectl get nodegroup "${NODE_GROUP}" -o jsonpath='{.spec.fencing.mode}')
  profile=$(echo "${profile}" | tr '[:upper:]' '[:lower:]')
  timeout=$(kubectl get fencingslaprofiles.node-manager.deckhouse.io "${profile}" -o jsonpath='{.spec.watchdog.timeout}')
  duration_seconds "${timeout}"
}

watchdog_state() {
  node_read "$1" "${WATCHDOG_SYSFS}/state"
}

# node_event_count <node> <reason> counts the agent's Events of that reason.
node_event_count() {
  kubectl get events -A \
    --field-selector "involvedObject.kind=Node,involvedObject.name=$1,reason=$2" \
    -o name | wc -l | tr -d '[:space:]'
}

# node_condition <node> <field> prints a field of the condition Fencing.
node_condition() {
  kubectl get node "$1" \
    -o jsonpath="{.status.conditions[?(@.type==\"Fencing\")].$2}"
}

# node_annotation <node> <name> prints a node-manager.deckhouse.io annotation.
node_annotation() {
  kubectl get node "$1" \
    -o jsonpath="{.metadata.annotations.node-manager\\.deckhouse\\.io/$2}"
}

# others_fenced is the number of other node groups with a mode other than Disabled.
# While it is not 0, the agents keep running in every group.
others_fenced() {
  cat "${STATE_DIR}/others-fenced"
}

boot_id() {
  kubectl get node "$1" -o jsonpath='{.status.nodeInfo.bootID}'
}
