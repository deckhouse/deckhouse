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

{{- /*
  New-arch on-node bootstrap seed (air-gap Local install only).
  Stands up a raw-process docker-distribution + docker-auth serving an
  authoritative store on dedicated LOOPBACK ports (distribution 127.0.0.1:5010,
  auth 127.0.0.1:5061) so it never collides with the agent's :5001 or the cache's
  auth :5051. Fills the store ONCE over the SSH reverse tunnel
  (registry-syncer 127.0.0.1:5511 -> 127.0.0.1:5010), then creates the
  registry-bootstrap secret. Ephemeral: no static-pod promotion; dhctl tears it
  down at finalize.

  Gate: new-model installs needing an on-node seed — registry module enabled AND
  bootstrap.seed (air-gap: cache enabled, no upstream). Connected installs pull
  from upstream during bring-up and skip the seed.
*/ -}}
{{- if and (.registry).registryModuleEnable (.registry.bootstrap).seed }}

bb-package-install "module-registry-auth:{{ .images.registry.dockerAuth }}" \
                   "module-registry-distribution:{{ .images.registry.dockerDistribution }}" \
                   "module-registry-syncer:{{ .images.registry.syncer }}" \
                   "cfssl:{{ .images.registrypackages.cfssl165 }}"

bb-set-proxy

base_path="/opt/deckhouse/registry/bootstrap-seed"
pki_path="${base_path}/pki"
auth_path="${base_path}/auth"
distribution_path="${base_path}/distribution"
log_path="${base_path}/logs"
data_path="/opt/deckhouse/registry/bootstrap-data"

seed_stop_sh="${base_path}/stop_registry_seed.sh"
seed_start_sh="${base_path}/start_registry_seed.sh"

mkdir -p "${base_path}" "${pki_path}" "${auth_path}" "${distribution_path}" "${log_path}" "${data_path}"

# --- PKI from the new-arch registry-init CA -------------------------------------
bb-sync-file "${pki_path}/ca.crt" - << EOF
{{ .registry.bootstrap.init.ca.cert }}
EOF

bb-sync-file "${pki_path}/ca.key" - << EOF
{{ .registry.bootstrap.init.ca.key }}
EOF

bb-sync-file "${pki_path}/profiles.json" - << EOF
{
    "signing": {
        "default": {
            "expiry": "87600h"
        },
        "profiles": {
            "client-server": {
                "expiry": "87600h",
                "usages": [
                    "signing",
                    "digital signature",
                    "key encipherment",
                    "client auth",
                    "server auth"
                ]
            },
            "auth-token": {
                "expiry": "87600h",
                "usages": [
                    "signing",
                    "digital signature",
                    "key encipherment",
                    "client auth",
                    "server auth"
                ]
            }
        }
    }
}
EOF

client_server_csr_json=$(cat << EOF
{
  "hosts": ["127.0.0.1", "localhost", "registry.d8-system.svc"],
  "key": {"algo": "rsa", "size": 2048}
}
EOF
)

auth_token_csr_json=$(cat << EOF
{
  "key": {"algo": "rsa", "size": 2048}
}
EOF
)

echo "${client_server_csr_json}" | /opt/deckhouse/bin/cfssl gencert \
  -cn="registry-auth" \
  -ca="${pki_path}/ca.crt" \
  -ca-key="${pki_path}/ca.key" \
  -config="${pki_path}/profiles.json" \
  -profile="client-server" - | /opt/deckhouse/bin/cfssljson -bare "${pki_path}/auth"
mv "${pki_path}/auth.pem" "${pki_path}/auth.crt"
mv "${pki_path}/auth-key.pem" "${pki_path}/auth.key"

echo "${client_server_csr_json}" | /opt/deckhouse/bin/cfssl gencert \
  -cn="registry-distribution" \
  -ca="${pki_path}/ca.crt" \
  -ca-key="${pki_path}/ca.key" \
  -config="${pki_path}/profiles.json" \
  -profile="client-server" - | /opt/deckhouse/bin/cfssljson -bare "${pki_path}/distribution"
mv "${pki_path}/distribution.pem" "${pki_path}/distribution.crt"
mv "${pki_path}/distribution-key.pem" "${pki_path}/distribution.key"

echo "${auth_token_csr_json}" | /opt/deckhouse/bin/cfssl gencert \
  -cn="registry-auth-token" \
  -ca="${pki_path}/ca.crt" \
  -ca-key="${pki_path}/ca.key" \
  -config="${pki_path}/profiles.json" \
  -profile="auth-token" - | /opt/deckhouse/bin/cfssljson -bare "${pki_path}/token"
mv "${pki_path}/token.pem" "${pki_path}/token.crt"
mv "${pki_path}/token-key.pem" "${pki_path}/token.key"

rm -f "${pki_path}/auth.csr" \
      "${pki_path}/distribution.csr" \
      "${pki_path}/token.csr" \
      "${pki_path}/profiles.json" \
      "${pki_path}/ca.key"

# --- docker-auth config (loopback :5061) ----------------------------------------
bb-sync-file "${auth_path}/config.yaml" - << EOF
server:
  addr: "127.0.0.1:5061"
  real_ip_header: "X-Forwarded-For"
  certificate: "${pki_path}/auth.crt"
  key: "${pki_path}/auth.key"
token:
  issuer: "Registry server"
  expiration: 900
  certificate: "${pki_path}/token.crt"
  key: "${pki_path}/token.key"

users:
  {{ .registry.bootstrap.init.ro_user.name | quote }}:
    password: {{ .registry.bootstrap.init.ro_user.password_hash | quote | replace "$" "\\$" }}
  {{ .registry.bootstrap.init.rw_user.name | quote }}:
    password: {{ .registry.bootstrap.init.rw_user.password_hash | quote | replace "$" "\\$" }}

acl:
  - match: { account: {{ .registry.bootstrap.init.ro_user.name | quote }} }
    actions: ["pull"]
    comment: "has readonly access"
  - match: { account: {{ .registry.bootstrap.init.rw_user.name | quote }} }
    actions: [ "*" ]
    comment: "has full access"
EOF

# --- docker-distribution config (loopback :5010, authoritative store, no proxy) --
bb-sync-file "${distribution_path}/config.yaml" - << EOF
version: 0.1
log:
  level: info

storage:
  filesystem:
    rootdirectory: "${data_path}"
  delete:
    enabled: true
  redirect:
    disable: true

http:
  addr: "127.0.0.1:5010"
  prefix: /
  secret: asecretforbootstrap
  debug:
    addr: "127.0.0.1:5012"
    prometheus:
      enabled: true
      path: /metrics
  tls:
    certificate: "${pki_path}/distribution.crt"
    key: "${pki_path}/distribution.key"

auth:
  token:
    realm: https://127.0.0.1:5061/auth
    service: Deckhouse registry
    issuer: Registry server
    rootcertbundle: "${pki_path}/token.crt"
    autoredirect: true
    proxy:
      url: https://127.0.0.1:5061/auth
      ca: "${pki_path}/ca.crt"
EOF

# --- start / stop helpers -------------------------------------------------------
bb-sync-file "${seed_start_sh}" - << EOF
#!/usr/bin/env bash
set -Eeuo pipefail

start_and_wait() {
    local log_path=\${1}
    local bin_path=\${2}
    shift 2
    local args=("\$@")

    local sleep_interval=1
    local max_attempts=10

    echo "Starting background process: \${bin_path}"

    if pgrep -f "\${bin_path}" > /dev/null 2>&1; then
        echo "\${bin_path}: already running"
        return 0
    fi

    "\${bin_path}" "\${args[@]}" > "\${log_path}" 2>&1 &

    for ((i=1; i<=max_attempts; i++)); do
        echo "\${bin_path}: waiting for process to come up, attempt \${i} of \${max_attempts}"
        sleep \${sleep_interval}

        if pgrep -f "\${bin_path}" > /dev/null 2>&1; then
            echo "\${bin_path}: started"
            return 0
        fi
    done

    echo "\${bin_path}: failed to start within \${sleep_interval}s x \${max_attempts}"
    return 1
}

liveness_probe() {
    local address=\${1}
    local ca_path=\${2}

    local sleep_interval=1
    local max_attempts=30

    echo "Probing liveness of \${address}"

    for ((i=1; i<=max_attempts; i++)); do
        if [[ \${i} -ne 1 ]]; then
            echo "\${address}: probe attempt \${i} of \${max_attempts}"
            sleep \${sleep_interval}
        fi

        local response=\$(d8-curl --cacert "\${ca_path}" -s -o /dev/null -w "%{http_code}" "\${address}" 2>/dev/null)
        if [[ "\${response}" == "200" ]]; then
            echo "\${address} is reachable"
            return 0
        fi
    done

    echo "\${address}: not reachable within \${sleep_interval}s x \${max_attempts}"
    return 1
}

echo "Starting bootstrap-seed auth"
if ! start_and_wait "${log_path}/auth.log" /opt/deckhouse/bin/ign-auth -logtostderr "${auth_path}/config.yaml"; then
    echo "ERROR: bootstrap-seed auth failed to start, see ${log_path}/auth.log"
    exit 1
fi
if ! liveness_probe "https://127.0.0.1:5061" "${pki_path}/ca.crt"; then
    echo "ERROR: bootstrap-seed auth liveness probe failed, see ${log_path}/auth.log"
    exit 1
fi

echo "Starting bootstrap-seed distribution"
if ! start_and_wait "${log_path}/distribution.log" /opt/deckhouse/bin/ign-registry serve "${distribution_path}/config.yaml"; then
    echo "ERROR: bootstrap-seed distribution failed to start, see ${log_path}/distribution.log"
    exit 1
fi
if ! liveness_probe "https://127.0.0.1:5010" "${pki_path}/ca.crt"; then
    echo "ERROR: bootstrap-seed distribution liveness probe failed, see ${log_path}/distribution.log"
    exit 1
fi

echo "bootstrap-seed started"
EOF
chmod +x "${seed_start_sh}"

bb-sync-file "${seed_stop_sh}" - << "EOF"
#!/usr/bin/env bash
pkill -f '/opt/deckhouse/bin/ign-registry' || true
pkill -f '/opt/deckhouse/bin/ign-auth' || true
sleep 2
pkill -9 -f '/opt/deckhouse/bin/ign-registry' || true
pkill -9 -f '/opt/deckhouse/bin/ign-auth' || true
EOF
chmod +x "${seed_stop_sh}"

bash "${seed_stop_sh}"
bash "${seed_start_sh}"

# --- fill the seed ONCE over the SSH reverse tunnel (retriable; only tunnel use) -
syncer_config_path="$(bb-tmp-file)"
bb-sync-file $syncer_config_path - << EOF
source:
  address: 127.0.0.1:5511
destination:
  address: "127.0.0.1:5010"
  ca: |
    {{ .registry.bootstrap.init.ca.cert | nindent 4 }}
  user:
    name: {{ .registry.bootstrap.init.rw_user.name | quote }}
    password: {{ .registry.bootstrap.init.rw_user.password | quote }}
EOF

registry-syncer $syncer_config_path | bb-log-stream-dhctl

# --- create the registry-bootstrap secret (agent fallback signal) ---------------
seed_secret_path="$(bb-tmp-file)"
bb-sync-file $seed_secret_path - << EOF
host: 127.0.0.1:5010
scheme: https
ca: |
{{ .registry.bootstrap.init.ca.cert | nindent 2 }}
EOF

export BB_KUBE_AUTH_TYPE="admin-cert"
export BB_KUBE_APISERVER_URL=""
bb-curl-helper-extract-admin-certs

bb-curl-kube "/api/v1/namespaces/d8-system" >/dev/null 2>&1 || \
  bb-curl-kube "/api/v1/namespaces" \
    -X POST \
    -H "Content-Type: application/json" \
    --data '{"apiVersion":"v1","kind":"Namespace","metadata":{"name":"d8-system"}}' >/dev/null

bb-curl-kube "/api/v1/namespaces/d8-system/secrets/registry-bootstrap" -X DELETE >/dev/null 2>&1 || true

bb-curl-kube "/api/v1/namespaces/d8-system/secrets" \
  -X POST \
  -H "Content-Type: application/json" \
  --data "$(jq -nc \
    --arg seed "$(base64 -w0 < "$seed_secret_path")" \
    '{"apiVersion":"v1","kind":"Secret","metadata":{"name":"registry-bootstrap","namespace":"d8-system","labels":{"app":"registry"},"annotations":{"helm.sh/resource-policy":"keep"}},"type":"Opaque","data":{"bootstrap-seed.yaml":$seed}}')" >/dev/null

rm -f "$syncer_config_path" "$seed_secret_path"

bb-unset-proxy

{{- end }}
