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
  machine-controller-manager finds it again by that name; a CloudPermanent node's
  infrastructure state is kept in a Secret named after it. A rename there would
  orphan the machine rather than rename the node.
*/}}
bb-sync-file /var/lib/bashible/rename_node.sh - << "RENAME_NODE_SCRIPT_EOF"
#!/bin/bash

# Renames this node in the cluster.
#
# The machine stays where it is - its disks, its container runtime and its
# hostname are untouched. What changes is the name the node is known by in the
# cluster: the Node object of the old name is removed, and the machine registers
# again under the new one.
#
# The order matters. The old Node object has to be gone before the machine comes
# back, because for the time both exist they are one address wearing two names,
# and a CNI that keys its peers by address tears down the entry for one when the
# other goes away - leaving the renamed node reachable by nobody. Removing a Node
# needs credentials this machine does not have, so it asks node-manager instead,
# by annotating itself, and waits for the object to go.
#
# The machine is rebooted at the end. A rename invalidates more on-node state than
# is worth chasing one item at a time: the CNI bridge still carries the subnet of
# the old lease, and anything that read the node name at start-up still has the
# old one. A reboot settles all of it at once.
#
# The node's workload does not survive this. Drain it first.

set -Eeuo pipefail

BOOTSTRAP_DIR="/var/lib/bashible"
NAME_FILE="${BOOTSTRAP_DIR}/discovered-node-name"
REQUEST_FILE="${BOOTSTRAP_DIR}/node-name"
TOKEN_FILE="${BOOTSTRAP_DIR}/bootstrap-token"
CA_FILE="${BOOTSTRAP_DIR}/ca.crt"
CLUSTER_CA="/etc/kubernetes/pki/ca.crt"
KUBE_ENDPOINTS="{{ .clusterMasterKubeAPIEndpoints | join " " }}"

export PATH="/opt/deckhouse/bin:${PATH}"

new_name=""
token=""
skip_drain="no"
assume_yes="no"
wait_limit="${RENAME_WAIT_SECONDS:-900}"

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

The machine is rebooted once the rename is prepared.
USAGE
}

# Ask the API server as kubelet does. bb-curl-kube lives in bashible.sh and is not
# available to a script run on its own, so the server, the client certificate and
# the CA all come straight out of the kubeconfig kubelet uses.
kubelet_patch_node() {
  local name="$1" body="$2"
  local kubeconfig="/etc/kubernetes/kubelet.conf"
  [[ -s "$kubeconfig" ]] || { >&2 echo "${kubeconfig} is missing"; return 1; }

  local server cert key ca_b64 ca_tmp rc
  server="$(awk '/server:/{print $2; exit}' "$kubeconfig")"
  cert="$(awk '/client-certificate:/{print $2; exit}' "$kubeconfig")"
  key="$(awk '/client-key:/{print $2; exit}' "$kubeconfig")"
  ca_b64="$(awk '/certificate-authority-data:/{print $2; exit}' "$kubeconfig")"
  [[ -n "$server" && -s "$cert" && -s "$key" && -n "$ca_b64" ]] || {
    >&2 echo "${kubeconfig} does not carry a server, a client certificate and a CA"
    return 1
  }

  ca_tmp="$(mktemp)"
  printf '%s' "$ca_b64" | base64 -d > "$ca_tmp"

  d8-curl -sS -f -x "" --connect-timeout 10 --max-time 30 \
    --cacert "$ca_tmp" --cert "$cert" --key "$key" \
    -X PATCH -H "Content-Type: application/strategic-merge-patch+json" \
    --data "$body" \
    "${server%/}/api/v1/nodes/${name}" >/dev/null
  rc=$?

  unlink "$ca_tmp"
  return $rc
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

# The bootstrap copy of the cluster CA does not outlive the bootstrap - 098_cleanup
# removes it at the end of every bashible run - but the cluster CA itself is on
# every node that joined. Put it back: this script needs it to reach the API
# server, and so does the step that regenerates bootstrap-kubelet.conf once
# kubelet's own kubeconfig has been taken away.
if [[ ! -s "$CA_FILE" ]]; then
  [[ -s "$CLUSTER_CA" ]] ||
    fail "neither ${CA_FILE} nor ${CLUSTER_CA} is present: this node has not been bootstrapped by Deckhouse"
  install -m 0644 "$CLUSTER_CA" "$CA_FILE"
fi

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
  # The old Node object goes away as part of this, and its pods with it.
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

The Node object '${old_name}' has to be removed, and everything still scheduled on
it goes with it. This machine will be rebooted once the rename is prepared. The
hostname ($(hostname)) is not changed.

CONFIRM
  read -r -p "Type the new name to continue: " answer
  [[ "$answer" == "$new_name" ]] || fail "aborted"
fi

# kubelet has to stop before the Node object is removed: a running kubelet
# registers itself again within seconds, and the operator would be deleting an
# object that keeps coming back under the old name.
log "stopping bashible and kubelet"
systemctl stop bashible.timer >/dev/null 2>&1 || true
systemctl stop bashible.service >/dev/null 2>&1 || true
systemctl stop kubelet.service

# Asked for with kubelet's own credentials, which is what makes the request
# trustworthy: NodeRestriction lets a kubelet write its own Node object and no
# other, so this can only ever be a node asking about itself. kubelet is already
# stopped, so nothing will recreate the object once node-manager removes it.
log "asking node-manager to remove the Node object '${old_name}'"
if ! kubelet_patch_node "${old_name}" \
    "$(jq -nc --arg n "${new_name}" '{"metadata":{"annotations":{"node.deckhouse.io/rename-to":$n}}}')"; then
  fail "could not ask for the rename: patching node '${old_name}' failed. kubelet is stopped, so nothing has changed yet; start it again with 'systemctl start kubelet' to put the node back."
fi

log "waiting for the Node object '${old_name}' to be removed"
waited=0
while :; do
  status="$(kube_status "/api/v1/nodes/${old_name}" || echo "")"
  [[ "$status" == "404" ]] && break
  (( waited += 10 ))
  if (( waited >= wait_limit )); then
    fail "node '${old_name}' is still in the cluster after ${wait_limit}s. Look for a NodeRenameRejected event on it for why node-manager would not remove it. kubelet is stopped and nothing else has changed yet: start it again with 'systemctl start kubelet' to put the node back."
  fi
  sleep 10
done
log "the Node object '${old_name}' is gone"

log "pinning the new node name"
printf '%s\n' "$token" > "$TOKEN_FILE"
chmod 0600 "$TOKEN_FILE"
printf '%s\n' "$new_name" > "$REQUEST_FILE"
printf '%s\n' "$new_name" > "$NAME_FILE"

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

# A rename is the node's identity bootstrap run again, and bashible has to be
# told so. Without this marker its update-approval steps run, and they reach the
# API through kubelet's own kubeconfig - the very credentials this script has
# just taken away. They would retry forever and bashible would never get as far
# as the steps that hand kubelet a new identity. The marker is removed by
# 098_cleanup at the end of a successful run, exactly as after a first bootstrap.
touch "${BOOTSTRAP_DIR}/first_run"

log "rebooting; the node will come back as '${new_name}'"
log "bashible runs on boot, registers the node under the new name, and brings the CNI up on the subnet the new Node is given."
systemctl reboot
RENAME_NODE_SCRIPT_EOF

chmod 0700 /var/lib/bashible/rename_node.sh
{{- end }}
