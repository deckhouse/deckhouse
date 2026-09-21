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
# bashible: parallel-group=light-prep

{{- if and (eq .runType "Normal") (or (eq .nodeGroup.nodeType "Static") (eq .nodeGroup.nodeType "CloudStatic")) }}
{{- /*
  Renaming is offered only where the node's name is the node's own business. A
  CloudEphemeral node is named after the machine object that owns it, and
  machine-controller-manager finds it again by that name, so a rename there would
  orphan the machine rather than rename the node.
*/}}
bb-sync-file /var/lib/bashible/rename_node.sh - << "RENAME_NODE_SCRIPT_EOF"
#!/bin/bash

# Renames this node in the cluster.
#
# The machine stays exactly where it is - its disks, its container runtime and its
# hostname are untouched. What changes is the name the node is known by in the
# cluster: kubelet drops the identity it registered with, bootstraps a new one and
# registers a fresh Node object under the new name. The Node object of the old name
# is removed afterwards by node-manager, which waits until the renamed node is back
# and only removes a Node that is this very machine.
#
# The node's workload does not survive this: the old Node object goes away and
# everything scheduled on it goes with it. Drain the node first.

set -Eeuo pipefail

BOOTSTRAP_DIR="/var/lib/bashible"
NAME_FILE="${BOOTSTRAP_DIR}/discovered-node-name"
REQUEST_FILE="${BOOTSTRAP_DIR}/node-name"
RENAMED_FROM_FILE="${BOOTSTRAP_DIR}/node-renamed-from"
TOKEN_FILE="${BOOTSTRAP_DIR}/bootstrap-token"
CA_FILE="${BOOTSTRAP_DIR}/ca.crt"
KUBE_ENDPOINTS="{{ .clusterMasterKubeAPIEndpoints | join " " }}"

export PATH="/opt/deckhouse/bin:${PATH}"

new_name=""
token=""
skip_drain="no"
assume_yes="no"

usage() {
  cat >&2 <<USAGE
Usage: $0 --new-name <name> --bootstrap-token <token> [--skip-drain] [--yes]

  --new-name <name>         The name this node should be known by in the cluster.
                            An RFC 1123 DNS subdomain, unique across the cluster.
  --bootstrap-token <token> A bootstrap token of this node's NodeGroup. kubelet
                            authenticates with it to register under the new name.
                            Get it where you have cluster access:

                              d8 k -n kube-system get secret \\
                                -l node-manager.deckhouse.io/node-group=<node-group> \\
                                -o jsonpath='{.items[0].data.token-id}{"."}{.items[0].data.token-secret}' \\
                                | base64 -d

  --skip-drain              Proceed even though the node has not been cordoned.
                            The workload on it is then cut off instead of moved.
  --yes                     Do not ask for confirmation.
USAGE
}

log()  { echo "[rename-node] $*"; }
fail() { echo "[rename-node] ERROR: $*" >&2; exit 1; }

while [[ $# -gt 0 ]]; do
  case "$1" in
    --new-name)          new_name="${2:-}"; shift 2 ;;
    --new-name=*)        new_name="${1#*=}"; shift ;;
    --bootstrap-token)   token="${2:-}"; shift 2 ;;
    --bootstrap-token=*) token="${1#*=}"; shift ;;
    --skip-drain)        skip_drain="yes"; shift ;;
    --yes|-y)            assume_yes="yes"; shift ;;
    -h|--help)           usage; exit 0 ;;
    *)                   usage; fail "unknown argument '$1'" ;;
  esac
done

[[ "$(id -u)" == "0" ]] || fail "this script has to be run as root"
[[ -n "$new_name" ]] || { usage; fail "--new-name is required"; }
[[ -n "$token" ]] || { usage; fail "--bootstrap-token is required"; }
[[ -s "$NAME_FILE" ]] || fail "${NAME_FILE} is missing: this node has not been bootstrapped by Deckhouse"
[[ -s "$CA_FILE" ]] || fail "${CA_FILE} is missing: this node has not been bootstrapped by Deckhouse"

old_name="$(<"$NAME_FILE")"

if (( ${#new_name} > 253 )) ||
   [[ ! "$new_name" =~ ^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$ ]]; then
  fail "'${new_name}' is not a valid RFC 1123 DNS subdomain, so no Node can be named that"
fi

if [[ "$new_name" == "$old_name" ]]; then
  log "this node is already named '${new_name}', nothing to do"
  exit 0
fi

# Every API read below authenticates with the bootstrap token rather than the
# node's own kubelet credentials: those are the first thing the rename throws
# away, so a resumed run would have nothing left to read the cluster with.
kube_status() {
  local path="$1" server code
  for server in ${KUBE_ENDPOINTS}; do
    code="$(d8-curl -sS -o /dev/null -w '%{http_code}' -x "" --connect-timeout 10 --max-time 30 \
      -H "Authorization: Bearer ${token}" --cacert "$CA_FILE" \
      "https://${server}${path}" 2>/dev/null || true)"
    case "$code" in
      200|404) echo "$code"; return 0 ;;
      401|403) fail "the API server at ${server} rejected the bootstrap token (HTTP ${code}). Pass a token that is still valid." ;;
    esac
  done
  return 1
}

kube_body() {
  local path="$1" server body
  for server in ${KUBE_ENDPOINTS}; do
    if body="$(d8-curl -sS -f -x "" --connect-timeout 10 --max-time 30 \
        -H "Authorization: Bearer ${token}" --cacert "$CA_FILE" \
        "https://${server}${path}" 2>/dev/null)"; then
      printf '%s' "$body"
      return 0
    fi
  done
  return 1
}

log "checking the new name against the cluster"
found=""
for _ in $(seq 1 6); do
  if found="$(kube_status "/api/v1/nodes/${new_name}")"; then
    break
  fi
  log "no API server answered, retrying in 10s"
  sleep 10
done
[[ -n "$found" ]] || fail "could not reach any of the API servers: ${KUBE_ENDPOINTS}"
[[ "$found" == "404" ]] || fail "a node named '${new_name}' already exists in the cluster; a node name has to be unique"

if [[ "$skip_drain" != "yes" ]]; then
  # The old Node object goes away when the rename lands, and its pods with it.
  # Requiring a cordon is how this script refuses to be the one that notices.
  unschedulable="false"
  if body="$(kube_body "/api/v1/nodes/${old_name}")"; then
    unschedulable="$(jq -r '.spec.unschedulable // false' <<<"$body")"
  fi
  if [[ "$unschedulable" != "true" ]]; then
    fail "node '${old_name}' is still schedulable. Drain it first:

    d8 k drain ${old_name} --ignore-daemonsets --delete-emptydir-data

  or pass --skip-drain to rename it without moving its workload."
  fi
fi

if [[ "$assume_yes" != "yes" ]]; then
  cat <<CONFIRM
This node is about to be renamed:

  from: ${old_name}
  to:   ${new_name}

kubelet will re-register the node under the new name. The Node object '${old_name}'
and everything still scheduled on it will be removed. The hostname of this machine
($(hostname)) is not changed.

CONFIRM
  read -r -p "Type the new name to continue: " answer
  [[ "$answer" == "$new_name" ]] || fail "aborted"
fi

log "stopping bashible and kubelet"
systemctl stop bashible.timer >/dev/null 2>&1 || true
systemctl stop bashible.service >/dev/null 2>&1 || true
systemctl stop kubelet.service

log "pinning the new node name"
printf '%s\n' "$token" > "$TOKEN_FILE"
chmod 0600 "$TOKEN_FILE"
printf '%s\n' "$new_name" > "$REQUEST_FILE"
printf '%s\n' "$new_name" > "$NAME_FILE"
printf '%s\n' "$old_name" > "$RENAMED_FROM_FILE"

log "dropping the kubelet identity of '${old_name}'"
# kubelet re-runs its TLS bootstrap when it has no kubeconfig of its own, which is
# the only way to get a client certificate for the new name: the certificate is
# what carries the node's identity, and it says system:node:${old_name}.
UNLINK=(
  /etc/kubernetes/kubelet.conf
  /etc/kubernetes/bootstrap-kubelet.conf
)
while IFS= read -r -d '' pki; do
  UNLINK+=("$pki")
done < <(find /var/lib/kubelet/pki -maxdepth 1 \( -name 'kubelet-client-*' -o -name 'kubelet-server-*' \) -print0 2>/dev/null)

# bashible skips a run whose configuration checksum has not moved. Nothing about
# the NodeGroup changed here, only this node, so the record of the last run has to
# go as well or the run that rewrites the kubelet unit never happens.
UNLINK+=("${BOOTSTRAP_DIR}/configuration_checksum" "${BOOTSTRAP_DIR}/uptime")

for path in "${UNLINK[@]}"; do
  # -L as well as -e: kubelet-client-current.pem is a symlink, and one whose
  # target this loop may already have removed still has to go.
  if [[ -e "$path" || -L "$path" ]]; then
    unlink "$path"
  fi
done

log "running bashible to re-register the node as '${new_name}'"
if ! "${BOOTSTRAP_DIR}/bashible.sh"; then
  fail "bashible failed. The node name is already set to '${new_name}'; fix the cause and run it again:

    ${BOOTSTRAP_DIR}/bashible.sh"
fi

log "waiting for node '${new_name}' to register"
registered="no"
for _ in $(seq 1 60); do
  if [[ "$(kube_status "/api/v1/nodes/${new_name}" || echo "")" == "200" ]]; then
    registered="yes"
    break
  fi
  sleep 10
done
[[ "$registered" == "yes" ]] || fail "node '${new_name}' has not registered within 10 minutes; check 'journalctl -u kubelet'"

systemctl start bashible.timer >/dev/null 2>&1 || true

log "node renamed: ${old_name} -> ${new_name}"
log "node-manager removes the Node object '${old_name}' once it has seen the renamed node settle."
log "If it is still there after a few minutes, remove it by hand: d8 k delete node ${old_name}"
RENAME_NODE_SCRIPT_EOF

chmod 0700 /var/lib/bashible/rename_node.sh
{{- end }}
