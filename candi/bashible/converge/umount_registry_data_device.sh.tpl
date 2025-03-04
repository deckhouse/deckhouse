# Copyright 2024 Flant JSC
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

{{- printf "{{- $nodeTypeList := list \"CloudPermanent\" }}" }}
{{- printf "  {{- if has .nodeGroup.nodeType $nodeTypeList }}" }}
{{- printf "    {{- if eq .nodeGroup.name \"master\" }}" }}
NODE_NAME="{{ .nodeName }}"
UNMOUNT_ALLOWED_ANNOTATION="{{ .unmountAllowedAnnotation }}"
UNMOUNT_DONE_ANNOTATION="{{ .unmountDoneAnnotation }}"

function is_annotation_exist(){
    local annotation="$1"
    local node="$D8_NODE_HOSTNAME"
    local node_annotations=$(bb-kubectl --request-timeout=60s --kubeconfig=/etc/kubernetes/kubelet.conf get node $node -o json | jq '.metadata.annotations')

    if echo "$node_annotations" | jq 'has("'$annotation'")' | grep -q 'true'; then
        return 0
    fi
    return 1
}

function create_annotation(){
    local annotation="$1=\"\""
    local node="$D8_NODE_HOSTNAME"
    bb-kubectl --request-timeout=60s --kubeconfig=/etc/kubernetes/kubelet.conf annotate node $node --overwrite $annotation
}

function find_path_by_data_device_mountpoint() {
  local data_device_mountpoint="$1"
  lsblk -o path,type,mountpoint,fstype --tree --json | jq -r "
    [
      .blockdevices[] 
      | select(.mountpoint == \"$data_device_mountpoint\")  # Match the specific device mountpoint
      | .path
    ] | first"
}

function is_data_device_mounted() {
  local data_device_mountpoint="$1"
  local data_device
  data_device=$(find_path_by_data_device_mountpoint "$data_device_mountpoint")
  if [ "$data_device" != "null" ] && [ -n "$data_device" ]; then
    return 0
  else
    return 1
  fi
}

function teardown_registry_data_device() {
    local mount_point="/mnt/system-registry-data"
    local fstab_file="/etc/fstab"
    local link_target="/opt/deckhouse/system-registry"
    local label="registry-data"

    # Umount data device
    if is_data_device_mounted "$mount_point"; then
        umount $mount_point
    fi
    
    # Remove the entry from /etc/fstab
    if grep -q "$label" "$fstab_file"; then
        sed -i "/^LABEL=${label}.*/d" "$fstab_file"
    fi

    # Remove the mount point if it exists
    if [[ -e "$mount_point" ]]; then
        rm -rf "$mount_point"
    fi
  
    # Remove the symbolic link if it exists
    if [[ -L "$link_target" ]]; then
        rm -f "$link_target"
    fi
}

if [[ "$D8_NODE_HOSTNAME" != "$NODE_NAME" ]]; then
    exit 0
fi

if is_annotation_exist "$UNMOUNT_ALLOWED_ANNOTATION"; then
    teardown_registry_data_device
    create_annotation "$UNMOUNT_DONE_ANNOTATION"
fi

{{- printf "  {{- end }}" }}
{{- printf "{{- end }}" }}
