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

{{- $nodeTypeList := list "CloudPermanent" }}
{{- if has .nodeGroup.nodeType $nodeTypeList }}
  {{- if eq .nodeGroup.name "master" }}


function exec_kubectl() {
  kubectl --request-timeout=60s --kubeconfig=/etc/kubernetes/kubelet.conf ${@}
}

{{- /*
# This function attempts to retrieve a Kubernetes secret related to the device kubernetes data
# from the specified namespace.
#
# Behavior:
#   - If the secret is successfully found, it will be printed to stdout.
#   - If the secret is not found, an error message will be printed and the function
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
#         "name": "d8-masters-kubernetes-data-device-path",
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
#         "name": "d8-masters-kubernetes-data-device-path",
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
function fetch_kubernetes_data_device_secret() {
  local secret_name="d8-masters-kubernetes-data-device-path"
  local namespace="d8-system"

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
        >&2 echo "Failed to get secret $secret_name from server $server"
        exit 1
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
    >&2 echo "Failed to get secret $secret_name using kubectl."
    exit 1
  fi
}

{{- /*
# This function extracts the device name (e.g., /dev/sdc) from a Kubernetes secret,
# based on the provided hostname. It calls the function fetch_kubernetes_data_device_secret 
# to retrieve the secret, then uses jq to extract the relevant device name for the current hostname,
# and finally decodes the base64-encoded value to retrieve the actual device name.
#
# Behavior:
#   - Calls fetch_kubernetes_data_device_secret to get the secret data.
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
function extract_kubernetes_data_device_from_secret() {
  fetch_kubernetes_data_device_secret | jq -re --arg hostname "$HOSTNAME" '.data[$hostname] // empty' | base64 -d
}

{{- /*
# This function retrieves the kubernetes data data device path from a file or a Kubernetes secret.
# 
# Behavior:
#   - Checks if the file kubernetes_data_device_path exists in the $BOOTSTRAP_DIR directory.
#   - If the file exists:
#     - Reads its content and assigns the value to the `data_device` variable.
#     - This is used for the first master node during the bootstrap process.
#       to ensure that subsequent operations use up-to-date data from the Kubernetes secret.
#   - If the file does not exist:
#     - Calls the `extract_kubernetes_data_device_from_secret` function to retrieve the device 
#       information from the Kubernetes secret.
#
# Purpose:
#   This function provides flexibility in determining the kubernetes data data device path by supporting 
#   both:
#   - A file-based approach during the bootstrap process for the first master node.
#   - A secret-based approach for updated and consistent information after the converges.
#
# Return Values:
#   - The resolved device path as a string, either from the file or the Kubernetes secret.
#   - If no data is available, returns an empty string.
*/}}
function get_kubernetes_data_device_from_file_or_from_secret() {
  local data_device=""
  if [ -f "$BOOTSTRAP_DIR/kubernetes_data_device_path" ]; then
    # For the first master node (after bootstrap)
    data_device=$(<"$BOOTSTRAP_DIR/kubernetes_data_device_path")
  else
    data_device=$(extract_kubernetes_data_device_from_secret)
  fi
  echo "$data_device"
}


{{- /*
# Function to find all unmounted data devices
*/}}
function find_all_unmounted_data_devices() {
  lsblk -o path,type,mountpoint,fstype --tree --json | jq -r \
    '[ .blockdevices[] | select (.path | contains("zram") | not ) | select ( .type == "disk" and .mountpoint == null and .children == null) | .path ] | sort'
}

{{- /*
# Function to find the first unmounted data device
*/}}
function find_first_unmounted_data_device() {
  local all_unmounted_data_devices="$(find_all_unmounted_data_devices)"
  echo "$all_unmounted_data_devices" | jq '. | first'
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

{{- /*
# Get the system registry data device path from a file
# The file always exists (created in step 000_create_system_registry_data_device_path.sh.tpl)
# and is removed after completion (removed in step 005_integrate_system_registry_data_device.sh.tpl)
*/}}
function get_system_registry_data_device() {
  local system_registry_data_device=""
  if [ -f "$BOOTSTRAP_DIR/system_registry_data_device_path" ]; then
    system_registry_data_device=$(<"$BOOTSTRAP_DIR/system_registry_data_device_path")
  fi
  echo "$system_registry_data_device"
}

{{- /*
# Check the expected disk count
*/}}
function check_expected_disk_count() {
  local expected_disks_count=1  # For Kubernetes data

  # If the system registry data device exists (not empty result)
  if [ -n "$(get_system_registry_data_device)" ]; then
    expected_disks_count=$((expected_disks_count + 1))
  fi

  # Find all unmounted data devices and count them
  local all_unmounted_data_devices=$(find_all_unmounted_data_devices)
  local all_unmounted_data_devices_count=$(echo "$all_unmounted_data_devices" | jq '. | length')

  # Compare the count of found devices with the expected count
  if [ "$all_unmounted_data_devices_count" -ne "$expected_disks_count" ]; then
    >&2 echo "Received disks: $all_unmounted_data_devices, expected count: $expected_disks_count"
    exit 1
  fi
}


# Skip for
if [[ "$FIRST_BASHIBLE_RUN" != "yes" ]]; then
  exit 0
fi

# Skip for
if [ -f /var/lib/bashible/kubernetes-data-device-installed ]; then
  exit 0
fi

# Wait all disks
check_expected_disk_count

# Get Kubernetes data device
DATA_DEVICE=$(get_kubernetes_data_device_from_file_or_from_secret)
if [ -z "$DATA_DEVICE" ]; then
  >&2 echo "failed to get kubernetes data device path"
  exit 1
fi

if ! [ -b "$DATA_DEVICE" ]; then
  >&2 echo "Failed to find $DATA_DEVICE disk. Trying to detect the correct one..."

  {{- /*
    # Sometimes the device path (`device_path`) returned by Terraform points to a non-existent device.
    # In such a situation, we want to find an unpartitioned unused device
    # without a file system, assuming that it is the correct one.
    # To form the mounting order of devices in Terraform, we specify mounting with the `depends` condition.
    # Additionally, we define the array of disks in Terraform when creating the instance machine.
  */}}
  DATA_DEVICE=$(find_first_unmounted_data_device)
  if ! [ -b "$DATA_DEVICE" ]; then
    >&2 echo "Failed to find a valid disk by lsblk."
    exit 1
  fi
fi

# Mount kubernetes data device steps:
mkdir -p /mnt/kubernetes-data

# always format the device to ensure it's clean, because etcd will not join the cluster if the device
# contains a filesystem with etcd database from a previous installation
mkfs.ext4 -F -L kubernetes-data $DATA_DEVICE

if grep -qv kubernetes-data /etc/fstab; then
  cat >> /etc/fstab << EOF
LABEL=kubernetes-data           /mnt/kubernetes-data     ext4   defaults,discard,x-systemd.automount        0 0
EOF
fi

if ! mount | grep -q $DATA_DEVICE; then
  mount -L kubernetes-data
fi

mkdir -p /mnt/kubernetes-data/var-lib-etcd
chmod 700 /mnt/kubernetes-data/var-lib-etcd
mkdir -p /mnt/kubernetes-data/etc-kubernetes

# if there is kubernetes dir with regular files then we can't delete it
# if there aren't files then we can delete dir to prevent symlink creation problems
if [[ "$(find /etc/kubernetes/ -type f 2>/dev/null | wc -l)" == "0" ]]; then
  rm -rf /etc/kubernetes
  ln -s /mnt/kubernetes-data/etc-kubernetes /etc/kubernetes
fi

if [[ "$(find /var/lib/etcd/ -type f 2>/dev/null | wc -l)" == "0" ]]; then
  rm -rf /var/lib/etcd
  ln -s /mnt/kubernetes-data/var-lib-etcd /var/lib/etcd
fi

touch /var/lib/bashible/kubernetes-data-device-installed

  {{- end  }}
{{- end  }}
