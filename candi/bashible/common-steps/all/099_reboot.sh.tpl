# Copyright 2021 Flant JSC
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

# If reboot flag is not set, nothing to do
if ! bb-flag? reboot; then
  exit 0
fi

{{- if eq .runType "ImageBuilding" }}
# Nothing to do on image building
exit 0
{{- end }}

# Reboot on cluster bootstrap
{{- if eq .runType "ClusterBootstrap" }}
bb-flag-unset disruption
# to prevent extra reboot during first "Normal" run.
bb-flag-unset reboot
  {{- if eq .cri "Containerd" }}
systemctl stop kubelet
# to speed up reboot process, we manually stop containers and kill appropriate containerd-shim processes with SIGKILL
# https://github.com/containerd/containerd/issues/386
crictl stop $(crictl ps -q)
kill -KILL $(ps ax | grep containerd-shim | grep -v grep |awk '{print $1}')
  {{- end }}
exit 0
{{- end }}

bb-deckhouse-get-disruptive-update-approval
bb-log-info "Rebooting machine after bootstrap process completed"
bb-flag-unset reboot

# If it is first run bashible on bootstrap simple reboot node
if [ "$FIRST_BASHIBLE_RUN" == "yes" ]; then
  bb-flag-unset disruption
  shutdown -r now
  exit 0
fi

# When we bootstrap the node we do not start the kubelet, if the node need to reboot.
# If the kubelet does not start in the first time, /etc/kubernetes/kubelet.conf file will not created.
# This is normally, after reboot the kubelet will start and the file will be created.
# If the kubelet is not started (on bootstrap), the node will not join into the cluster and
# we dont need to set the status of the node to NotReady.
# Why don't we start the kubelet when we bootstrap the node (in some cases)?
# We want bootstrap the node fully, reboot it and after reboot join the node into the cluster.
if [ ! -f "/etc/kubernetes/kubelet.conf" ]; then
  bb-flag-unset disruption
  shutdown -r now
  exit 0
fi

# Set the node status and reboot
systemctl stop kubelet
# Wait till kubelet stopped
attempt=0
until ! pidof kubelet > /dev/null; do
  attempt=$(( attempt + 1 ))
  if [ "$attempt" -gt "20" ]; then
    bb-log-error "Can't stop kubelet. Will try to set NotReady status while kubelet is running."
    break
  fi
  bb-log-info "Waiting till kubelet stopped (20sec)..."
  sleep 1
done

# Our task is to force setting Node status to NotReady to prevent unwanted schedulings during reboot.
attempt=0
while true; do
  attempt=$(( attempt + 1 ))
  if [[ ${attempt} -gt 3 ]]; then
    bb-log-warning "Can't update Node status condition to NotReady. Will reboot as is."
    break
  fi

  bb-log-info "Setting node status to NotReady..."

  url="https://127.0.0.1:6445/api/v1/nodes/${D8_NODE_HOSTNAME}"
  ready_condition_key=""
  if ! ready_condition_key="$(d8-curl -s -f -X GET "$url" --cacert /etc/kubernetes/pki/ca.crt \
       --cert /var/lib/kubelet/pki/kubelet-client-current.pem |
       jq -r '.status.conditions | to_entries[] | select(.value.type == "Ready") | .key')"; then
    bb-log-warning "failed to get ready condition from node"
    sleep 2
    continue
  fi

  patch="$(jq -ns --arg ready_condition_key "${ready_condition_key}" --arg current_time "`date -u +'%Y-%m-%dT%H:%M:%SZ'`" '
  [
    {
      "op": "replace",
      "path": ("/status/conditions/" + $ready_condition_key),
      "value": {
        "type": "Ready",
        "status": "False",
        "lastHeartbeatTime": $current_time,
        "lastTransitionTime": $current_time,
        "reason": "KubeletReady",
        "message": "Status NotReady was set by bashible during reboot step (candi/bashible/common-steps/all/099_reboot.sh)"
      }
    }
  ]')"

  if d8-curl -s -f -X PATCH "$url/status" --cacert /etc/kubernetes/pki/ca.crt \
     --cert /var/lib/kubelet/pki/kubelet-client-current.pem --data "${patch}" \
     --header "Content-Type: application/json-patch+json" >/dev/null; then
    break
  fi

  bb-log-warning "failed to patch node ready condition"
  sleep 2
done

bb-flag-unset disruption
shutdown -r now
