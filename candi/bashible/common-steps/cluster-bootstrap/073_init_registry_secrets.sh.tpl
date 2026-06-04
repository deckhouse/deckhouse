# Copyright 2025 Flant JSC
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

{{- with ((.registry).bootstrap).init }}

# Create init registry config file
INIT_CONFIG_PATH="$(bb-tmp-file)"
bb-sync-file $INIT_CONFIG_PATH - << "EOF"
{{ . | toYaml }}
EOF

# Force admin-cert auth for operations requiring elevated privileges
export BB_KUBE_AUTH_TYPE="admin-cert"
export BB_KUBE_APISERVER_URL=""
bb-curl-helper-extract-admin-certs

# Create d8-system namespace if it doesn't exist
bb-curl-kube "/api/v1/namespaces/d8-system" >/dev/null 2>&1 || \
  bb-curl-kube "/api/v1/namespaces" \
    -X POST \
    -H "Content-Type: application/json" \
    --data '{"apiVersion":"v1","kind":"Namespace","metadata":{"name":"d8-system"}}'

# Upload init registry secret
bb-curl-kube "/api/v1/namespaces/d8-system/secrets/registry-init" -X DELETE || true

bb-curl-kube "/api/v1/namespaces/d8-system/secrets" \
  -X POST \
  -H "Content-Type: application/json" \
  --data "$(jq -nc --arg data "$(base64 -w0 < "$INIT_CONFIG_PATH")" \
    '{"apiVersion":"v1","kind":"Secret","metadata":{"name":"registry-init","namespace":"d8-system"},"type":"Opaque","data":{"config":$data}}')"

{{- end }}
