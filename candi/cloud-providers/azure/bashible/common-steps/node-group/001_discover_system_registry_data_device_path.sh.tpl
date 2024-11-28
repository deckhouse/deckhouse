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
#   - If the system_registry_data_device_path file exists, reads its content and assigns the value to `data_device`.
#     This is used:
#     - For the first master node during the bootstrap process.
#     - For specific cloud providers that may require an override device path.
#   - If the file does not exist, it falls back to calling the extract_registry_data_device_from_secret 
#     function to fetch the device information from the Kubernetes secret.
#
# Purpose:
#   The function provides flexibility in determining the device path by supporting both a file-based 
#   approach (for bootstrap and overrides) and a secret-based approach (for up-to-date information 
#   after convergence).
*/}}
function get_registry_data_device_from_secret_or_from_file() {
  local data_device=""
  if [ -f "$BOOTSTRAP_DIR/system_registry_data_device_path" ]; then
    # For the first master node (after bootstrap) or for overrides for specific clouds
    data_device=$(<"$BOOTSTRAP_DIR/system_registry_data_device_path")
  else
    data_device=$(extract_registry_data_device_from_secret)
  fi
  echo "$data_device"
}

# Retrieve the data device
dataDevice=$(get_registry_data_device_from_secret_or_from_file)

# Path to the file containing the device path
system_registry_file="$BOOTSTRAP_DIR/system_registry_data_device_path"

# If dataDevice is non-empty and begins with /dev, write it to the file
if [ -n "$dataDevice" ] && [[ "$dataDevice" == /dev/* ]]; then
  echo "system_registry_data_device: $dataDevice"
  echo "$dataDevice" > "$system_registry_file"
else
  # Check for devices using ls
  get_disks_by_lun_id="$(ls /dev/disk/azure/*/lun11 -l 2>/dev/null)"
  
  # If the result is empty, clear the file
  if [ -z "$get_disks_by_lun_id" ]; then
    : > "$system_registry_file"
  else
    # If not empty, continue processing
    if [ "$(wc -l <<< "$get_disks_by_lun_id")" -ne 1 ]; then
      >&2 echo "Failed to discover system-registry-data device"
      exit 1
    fi
    
    # Extract the device path
    new_device_path="$(awk '{gsub("../../..", "/dev"); print $11}' <<< "$get_disks_by_lun_id")"
    
    # Write the new device path to the file
    echo "system_registry_data_device: $dataDevice"
    echo "$new_device_path" > "$system_registry_file"
  fi
fi

blkid

  {{- end  }}
{{- end  }}
