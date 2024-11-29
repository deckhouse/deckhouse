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

{{- /*
# This function retrieves the system registry data device path from a file or a Kubernetes secret.
# 
# Behavior:
#   - Checks if the file system_registry_data_device_path exists in the $BOOTSTRAP_DIR directory.
#   - If the file exists:
#     - Reads its content and assigns the value to the `data_device` variable.
#     - This is used for the first master node during the bootstrap process.
#     - The file is always removed in the `005_integrate_system_registry_data_device.sh.tpl` script 
#       to ensure that subsequent operations use up-to-date data from the Kubernetes secret.
#   - If the file does not exist:
#     - Calls the `extract_registry_data_device_from_secret` function to retrieve the device 
#       information from the Kubernetes secret.
#
# Purpose:
#   This function provides flexibility in determining the system registry data device path by supporting 
#   both:
#   - A file-based approach during the bootstrap process for the first master node.
#   - A secret-based approach for updated and consistent information after the system converges.
#
# Return Values:
#   - The resolved device path as a string, either from the file or the Kubernetes secret.
#   - If no data is available, returns an empty string.
*/}}
function get_registry_data_device_from_secret_or_from_file() {
  local data_device=""
  if [ -f "$BOOTSTRAP_DIR/system_registry_data_device_path" ]; then
    # For the first master node (after bootstrap)
    # Always removed in 005_integrate_system_registry_data_device.sh.tpl
    data_device=$(<"$BOOTSTRAP_DIR/system_registry_data_device_path")
  else
    data_device=$(extract_registry_data_device_from_secret)
  fi
  echo "$data_device"
}


# Retrieve the data device
dataDevice=$(get_registry_data_device_from_secret_or_from_file)

# Write the new device path to the file
echo "$dataDevice" > "$BOOTSTRAP_DIR/system_registry_data_device_path"
