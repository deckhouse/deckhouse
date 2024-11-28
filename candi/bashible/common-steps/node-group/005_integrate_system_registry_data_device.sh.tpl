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

{{- $nodeTypeList := list "CloudPermanent" }}
{{- if has .nodeGroup.nodeType $nodeTypeList }}
  {{- if eq .nodeGroup.name "master" }}

function exec_kubectl() {
  kubectl --request-timeout=60s --kubeconfig=/etc/kubernetes/kubelet.conf ${@}
}

function setup_registry_data_device() {
    local data_device="$1"
    local mount_point="/mnt/system-registry-data"
    local fstab_file="/etc/fstab"
    local symlink_target="/opt/deckhouse/system-registry"
    local label="registry-data"

    if ! [ -b "$data_device" ]; then
      >&2 echo "Failed to find $data_device disk."
      exit 1
    fi

    # Ensure the mount directory exists
    mkdir -p "$mount_point"

    # Format the data device if it is not already ext4
    if ! file -s "$data_device" | grep -q ext4; then
        mkfs.ext4 -F -L "$label" "$data_device"
    fi

    # Add an entry to /etc/fstab if it does not already exist
    if ! grep -q "$label" "$fstab_file"; then
        echo "LABEL=$label $mount_point ext4 defaults,discard,x-systemd.automount 0 0" >> "$fstab_file"
    fi

    # Mount the device if it is not already mounted
    if ! mount | grep -q "$mount_point"; then
        mount -L "$label"
    fi

    # Create a symlink if the target directory is empty
    if [[ "$(find "$symlink_target" -type f 2>/dev/null | wc -l)" == "0" ]]; then
        rm -rf "$symlink_target"
        ln -s "$mount_point" "$symlink_target"
    fi
}

function teardown_registry_data_device() {
    local mount_point="/mnt/system-registry-data"
    local fstab_file="/etc/fstab"
    local link_target="/opt/deckhouse/system-registry"
    local label="registry-data"
  
    # Remove the symbolic link if it exists and points to the correct location
    if [[ -L "$link_target" && "$(readlink "$link_target")" == "$mount_point" ]]; then
        rm -f "$link_target"
    fi
    
    # Remove the entry from /etc/fstab
    if grep -q "$label" "$fstab_file"; then
        sed -i "/^LABEL=${label}.*/d" "$fstab_file"
    fi
    
    # Unmount the device
    if mount | grep -q "$mount_point"; then
        umount "$mount_point"
    fi
}

{{- /*
# This function attempts to retrieve a Kubernetes secret related to the device registry
# from the specified namespace. It retries multiple times if the secret is not found.
#
# Behavior:
#   - Tries to retrieve the secret with multiple retries.
#   - If the secret is successfully retrieved, it will be printed to stdout.
#   - If the secret is not found after the maximum number of
#     attempts, an error message will be printed and the function
#     will exit with a non-zero status.
#
# Return Values:
#   - On success: Outputs the secret's data (typically in JSON format).
#   - On failure after all retries: Outputs an error message indicating the failure.
#
# Example output (on success):
# ------------------
# kubectl:
# {
#     "apiVersion": "v1",
#     "data": {
#         "<prefix>-master-1": "",
#         "<prefix>-master-2": ""
#     },
#     "kind": "Secret",
#     "metadata": {
#         "name": "d8-masters-system-registry-data-device-path",
#         "namespace": "d8-system"
#     },
#     "type": "Opaque"
# }
# ------------------
# curl:
# {
#     "kind": "Secret",
#     "apiVersion": "v1",
#     "metadata": {
#         "name": "d8-masters-system-registry-data-device-path",
#         "namespace": "d8-system",
#         "managedFields": [
#             {
#                 ...
#             }
#         ]
#     },
#     "data": {
#         "<prefix>-master-1": "",
#         "<prefix>-master-2": ""
#     },
#     "type": "Opaque"
# }
# ------------------
# "" - empty
# ------------------
*/}}
function fetch_registry_data_device_secret() {
  local secret_name="d8-masters-system-registry-data-device-path"
  local namespace="d8-system"
  local max_attempts=5
  local sleep_interval=10

  for ((i=1; i<=max_attempts; i++)); do
    if [ "$FIRST_BASHIBLE_RUN" == "yes" ]; then
      # Ensure bootstrap-token exists before proceeding
      if [ -f "$BOOTSTRAP_DIR/bootstrap-token" ]; then
        # Iterate through each API server endpoint
        for server in {{ .normal.apiserverEndpoints | join " " }}; do
          local http_status
          # Check HTTP status without outputting error details
          http_status=$(d8-curl -s -w "%{http_code}" -o /dev/null \
            -X GET "https://$server/api/v1/namespaces/$namespace/secrets/$secret_name" \
            --header "Authorization: Bearer $(<"$BOOTSTRAP_DIR/bootstrap-token")" \
            --cacert "$BOOTSTRAP_DIR/ca.crt" 2>/dev/null)

          if [ "$http_status" -eq 404 ]; then
            # Secret does not exist (HTTP 404), return successfully
            return 0
          fi

          # Try to retrieve the secret; if successful, output the result
          if output=$(d8-curl -s -f \
                -X GET "https://$server/api/v1/namespaces/$namespace/secrets/$secret_name" \
                --header "Authorization: Bearer $(<"$BOOTSTRAP_DIR/bootstrap-token")" \
                --cacert "$BOOTSTRAP_DIR/ca.crt" 2>/dev/null); then
            echo "$output"
            return 0
          fi

          # Output error message if the attempt fails
          >&2 echo "Attempt $i: Failed to get secret $secret_name from server $server"
        done
      else
        # Error: bootstrap-token is missing
        >&2 echo "Failed to get secret $secret_name: can't find bootstrap-token."
        exit 1
      fi
    else
      # Use kubectl to retrieve the secret
      if output=$(exec_kubectl get secret "$secret_name" -n "$namespace" --ignore-not-found=true -o json 2>/dev/null); then
        echo "$output"
        return 0
      fi
      # Output error message if kubectl fails
      >&2 echo "Attempt $i: Failed to get secret $secret_name using kubectl."
    fi

    # Wait before the next retry attempt
    if [ $i -lt $max_attempts ]; then
      sleep $sleep_interval
    else
      # Output error message if maximum retries are exceeded
      >&2 echo "Exceeded maximum retry attempts to get secret $secret_name."
      exit 1
    fi
  done
}

{{- /*
# This function extracts the device name (e.g., /dev/sdc) from a Kubernetes secret,
# based on the provided hostname. It calls the function fetch_registry_data_device_secret 
# to retrieve the secret, then uses jq to extract the relevant device name for the current hostname,
# and finally decodes the base64-encoded value to retrieve the actual device name.
#
# Behavior:
#   - Calls fetch_registry_data_device_secret to get the secret data.
#   - Uses jq to filter out the entry corresponding to the hostname specified by the $HOSTNAME environment variable.
#   - Decodes the base64-encoded value of the extracted device name.
#
# Assumptions:
#   - The secret is structured with a key-value pair where the key is a hostname 
#     (e.g., "d8-master-1" or "d8-master-2") and the value is base64-encoded device name 
#     (e.g., "/dev/sdc").
#
# Example:
#   If the secret has the following data:
#   {
#     "data": {
#       "d8-master-1": "L2Rldi9zZGMA",  # base64-encoded "/dev/sdc"
#       "d8-master-2": "L2Rldi9zZGMA"   # base64-encoded "/dev/sdc"
#     }
#   }
#
#   And $HOSTNAME is "d8-master-1", the function will:
#   - Extract the value for "d8-master-1" (which is "L2Rldi9zZGMA").
#   - Decode it using base64, resulting in the device name "/dev/sdc".
#
# Return Values:
#   - On success: The decoded device name for the specified hostname (e.g., "/dev/sdc").
#   - On failure: If the key corresponding to $HOSTNAME is not found, it will output an empty string.
*/}}
function extract_registry_data_device_from_secret() {
  fetch_registry_data_device_secret | jq -re --arg hostname "$HOSTNAME" '.data[$hostname] // empty' | base64 -d
}

function get_registry_data_device_from_terraform() {
  local data_device=""
  if [ -f "$BOOTSTRAP_DIR/system_registry_data_device_path" ]; then
    # for first master node (after bootstrap)
    data_device=$(<"$BOOTSTRAP_DIR/system_registry_data_device_path")
  else
    # for other master nodes (and first, but only after converge)
    data_device=$(extract_registry_data_device_from_secret)
  fi
  echo "$data_device"
}

function find_first_unmounted_data_device() {
  lsblk -o path,type,mountpoint,fstype --tree --json | jq -r \
    '[ .blockdevices[] | select (.path | contains("zram") | not ) | select ( .type == "disk" and .mountpoint == null and .children == null) | .path ] | sort | first'
}

function find_mounted_registry_data_device() {
  lsblk -o path,type,mountpoint,fstype --tree --json | jq -r \
    '[.blockdevices[] | select(.mountpoint == "/mnt/system-registry-data" ) | .path] | first'
}

{{- /*
# Example (lsblk -o path,type,mountpoint,fstype --tree --json):
#         {
#          "path": "/dev/vda",
#          "type": "disk",
#          "mountpoint": null,
#          "fstype": null,
#          "children": [
#             {
#                "path": "/dev/vda1",
#                "type": "part",
#                "mountpoint": "/",
#                "fstype": "ext4"
#             },{
#                "path": "/dev/vda15",
#                "type": "part",
#                "mountpoint": "/boot/efi",
#                "fstype": "vfat"
#             }
#          ]
#       },{
#          "path": "/dev/vdb",
#          "type": "disk",
#          "mountpoint": null,
#          "fstype": null
#       }
*/}}

function is_unmounted_data_device_exists() {
  local data_device
  data_device=$(find_first_unmounted_data_device)
  if [ "$data_device" != "null" ] && [ -n "$data_device" ]; then
    return 0
  else
    return 1
  fi
}

function is_registry_data_device_mounted() {
  local data_device
  data_device=$(find_mounted_registry_data_device)
  if [ "$data_device" != "null" ] && [ -n "$data_device" ]; then
    return 0
  else
    return 1
  fi
}

function create_registry_data_device_installed_file() {
  local installed_file="$BOOTSTRAP_DIR/system-registry-data-device-installed"
  touch "$installed_file"
}

function remove_registry_data_device_installed_file() {
  local installed_file="$BOOTSTRAP_DIR/system-registry-data-device-installed"
  if [ -f "$installed_file" ]; then
    rm -f "$installed_file"
  fi
}

if is_registry_data_device_mounted; then
  create_registry_data_device_installed_file
else
  if is_unmounted_data_device_exists; then
    data_device=$(get_registry_data_device_from_terraform)
    if ! [ -b "$data_device" ]; then
      >&2 echo "Failed to find $data_device disk. Detecting the correct one..."
      data_device=$(find_first_unmounted_data_device)
    fi
    setup_registry_data_device "$data_device"
    create_registry_data_device_installed_file
  else
    teardown_registry_data_device
    remove_registry_data_device_installed_file
  fi
fi

  {{- end  }}
{{- end  }}
