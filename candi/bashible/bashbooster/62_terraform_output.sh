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

bb-get-kubernetes-data-device-from-file-or-secret() {
  # for bootstrap first master node or for providers overrides
  file="/var/lib/bashible/kubernetes_data_device_path"
  if [ -f "$file" ]; then
    cat $file
  else
    # for other cases
    __bb-fetch-data-from-secret "d8-system" "d8-masters-kubernetes-data-device-path" | jq -re --arg hostname "$(bb-d8-node-name)" '.data[$hostname] // empty' | base64 -d
  fi
}

bb-get-registry-data-device-from-terraform-output() {
  if [ "$RUN_TYPE" = "Normal" ]; then
    # other nodes
    __bb-fetch-data-from-secret "d8-system" "d8-masters-system-registry-data-device-path" | jq -re --arg hostname "$(bb-d8-node-name)" '.data[$hostname] // empty' | base64 -d
    else
    # for bootstrap first master node
    file="/var/lib/bashible/system_registry_data_device_path"
    if [ -f "$file" ]; then
      cat $file
    fi
  fi
}

# Description:
#   This function fetches Kubernetes secret data from a specified namespace and secret name.
#   Depending on the context (e.g., the first run of Bashible or subsequent runs), it uses
#   either direct API calls to the Kubernetes API server or `kubectl` commands to retrieve the secret data.
#
# Return Values:
#   - 0:
#     - The secret was successfully retrieved and output as JSON.
#     - The secret does not exist (HTTP 404), which is treated as a valid outcome.
#   - 1:
#     - Failure in accessing the Kubernetes API server or using `kubectl`.
#     - Missing `bootstrap-token` file or critical error in secret retrieval.
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
__bb-fetch-data-from-secret() {
  local namespace="$1"
  local secret_name="$2"

  if [ "$FIRST_BASHIBLE_RUN" == "yes" ]; then
    # Ensure bootstrap-token exists before proceeding
    if [ -f "$BOOTSTRAP_DIR/bootstrap-token" ]; then
      # Iterate through each API server endpoint
      IFS=',' read -ra SERVERS <<< "$API_SERVER_ENDPOINTS"
      for server in "${SERVERS[@]}"; do
        local http_status
        # Check HTTP status without outputting error details
        http_status=$(bb-rp-curl -s -w "%{http_code}" -o /dev/null \
          -X GET "https://$server/api/v1/namespaces/$namespace/secrets/$secret_name" \
          --connect-timeout 10 \
          --header "Authorization: Bearer $(<"$BOOTSTRAP_DIR/bootstrap-token")" \
          --cacert "$BOOTSTRAP_DIR/ca.crt" 2>/dev/null)

        if [ "$http_status" -eq 404 ]; then
          # Secret does not exist (HTTP 404), return successfully
          return 0
        fi

        # Try to retrieve the secret; if successful, output the result
        if output=$(bb-rp-curl -s -f \
              -X GET "https://$server/api/v1/namespaces/$namespace/secrets/$secret_name" \
              --connect-timeout 10 \
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
    if output=$(bb-kubectl --request-timeout=10s --kubeconfig=/etc/kubernetes/kubelet.conf get secret "$secret_name" -n "$namespace" --ignore-not-found=true -o json 2>/dev/null); then
      echo "$output"
      return 0
    fi
    # Output error message if kubectl fails
    >&2 echo "Failed to get secret $secret_name using kubectl."
    exit 1
  fi
}
