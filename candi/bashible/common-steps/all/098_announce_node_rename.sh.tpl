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

{{- if eq .runType "Normal" }}
# rename_node.sh leaves this file behind after it has pinned a new node name and
# let kubelet register under it. Announcing the old name on the new Node is what
# lets node-manager collect the Node object the rename left behind: it will only
# remove one that is not running and is the same machine as this one.
#
# The annotation is the handshake, so it is node-manager that clears it once the
# old Node is gone. All this step has to be sure of is that the annotation landed
# before the local record of the rename goes away.
renamed_from_file="/var/lib/bashible/node-renamed-from"

if [ -s "$renamed_from_file" ]; then
  renamed_from="$(<"$renamed_from_file")"

  if [ "$renamed_from" == "$(bb-d8-node-name)" ]; then
    # Nothing was renamed after all - the run that would have changed the name
    # never reached kubelet. Drop the record rather than annotate a node with
    # its own name.
    unlink "$renamed_from_file"
  elif ! test -f /etc/kubernetes/kubelet.conf; then
    bb-log-info "Node has not finished registering yet, the rename of ${renamed_from} will be announced on a later run"
  elif bb-curl-helper-patch-node-metadata "$(bb-d8-node-name)" "annotations" "node.deckhouse.io/renamed-from=${renamed_from}"; then
    bb-log-info "Announced the rename ${renamed_from} -> $(bb-d8-node-name); node-manager will collect the Node object ${renamed_from}"
    unlink "$renamed_from_file"
  else
    bb-log-warning "Failed to announce the rename of ${renamed_from}, will retry on the next run"
  fi
fi
{{- end }}
