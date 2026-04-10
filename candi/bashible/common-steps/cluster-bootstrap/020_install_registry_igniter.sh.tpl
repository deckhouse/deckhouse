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

{{- if has (.registry).mode (list "Proxy" "Local") }}

pod_kill_and_wait() {
  local pod_prefix="${1}"
  local sleep_interval=1
  local max_attempts=10

  echo "Removing pod: ${pod_prefix}"

  if [ -z "${pod_prefix}" ]; then
    echo "Empty prefix, skip"
    return 0
  fi

  if ! command -v crictl > /dev/null 2>&1; then
    echo "crictl not found, skip"
    return 0
  fi

  if ! crictl info > /dev/null 2>&1; then
    echo "containerd not ready, skip"
    return 0
  fi

  local pods=$(crictl pods -o json 2>/dev/null | jq -r --arg PREFIX "${pod_prefix}" '
    .items[]? |
    select(.metadata.name | startswith($PREFIX)) |
    .id
  ' 2>/dev/null)

  if [ -z "${pods}" ]; then
    echo "${pod_prefix}: no pods, skip"
    return 0
  fi

  for pod in ${pods}; do
    crictl stopp "${pod}" > /dev/null 2>&1 || true
    crictl rmp "${pod}" > /dev/null 2>&1 || true
  done

  echo "${pod_prefix}: waiting for removal..."
  for ((i=1; i<=max_attempts; i++)); do
    echo "Attempt: ${i}/${max_attempts}"
    sleep ${sleep_interval}

    local remaining_pods=$(crictl pods -o json 2>/dev/null | jq -r --arg PREFIX "${pod_prefix}" '
      .items[]? |
      select(.metadata.name | startswith($PREFIX)) |
      .id
    ' 2>/dev/null)

    if [ -z "${remaining_pods}" ]; then
      echo "${pod_prefix}: successfully removed"
      return 0
    fi
  done

  echo "${pod_prefix}: failed to remove (timeout ${sleep_interval}s * ${max_attempts})"
  exit 1
}

bb-package-install "module-registry-auth:{{ .images.registry.dockerAuth }}" "module-registry-distribution:{{ .images.registry.dockerDistribution }}" "cfssl:{{ .images.registrypackages.cfssl165 }}"

# Prepare proxy envs
bb-set-proxy

# Prepare vars
discovered_node_ip="$(bb-d8-node-ip)"

base_path="${REGISTRY_MODULE_IGNITER_DIR}"
pki_path="${base_path}/pki"
auth_path="${base_path}/auth"
distribution_path="${base_path}/distribution"
log_path="${base_path}/logs"
data_path="/opt/deckhouse/registry/local_data"

igniter_stop_sh="${base_path}/stop_registry_igniter.sh"
igniter_start_sh="${base_path}/start_registry_igniter.sh"

static_pod_file="/etc/kubernetes/manifests/registry-nodeservices.yaml"
static_pod_name="registry-nodeservices"

# Create the directories
mkdir -p "${base_path}" \
         "${pki_path}" \
         "${auth_path}" \
         "${distribution_path}" \
         "${log_path}" \
         "${data_path}"

# Generate certs
bb-sync-file "${pki_path}/ca.crt" - << EOF
{{ .registry.bootstrap.init.ca.cert }}
EOF

bb-sync-file "${pki_path}/ca.key" - << EOF
{{ .registry.bootstrap.init.ca.key }}
EOF

{{- with ((.registry.bootstrap).proxy).ca }}
bb-sync-file "${pki_path}/upstream-registry-ca.crt" - << EOF
{{ . }}
EOF
{{- end }}

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
  "hosts": ["127.0.0.1", "localhost", "registry.d8-system.svc", "${discovered_node_ip}"],
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

# Auth certs
echo "${client_server_csr_json}" | /opt/deckhouse/bin/cfssl gencert \
  -cn="registry-auth" \
  -ca="${pki_path}/ca.crt" \
  -ca-key="${pki_path}/ca.key" \
  -config="${pki_path}/profiles.json" \
  -profile="client-server" - | /opt/deckhouse/bin/cfssljson -bare "${pki_path}/auth"
mv "${pki_path}/auth.pem" "${pki_path}/auth.crt"
mv "${pki_path}/auth-key.pem" "${pki_path}/auth.key"

# Distribution certs
echo "${client_server_csr_json}" | /opt/deckhouse/bin/cfssl gencert \
  -cn="registry-distribution" \
  -ca="${pki_path}/ca.crt" \
  -ca-key="${pki_path}/ca.key" \
  -config="${pki_path}/profiles.json" \
  -profile="client-server" - | /opt/deckhouse/bin/cfssljson -bare "${pki_path}/distribution"
mv "${pki_path}/distribution.pem" "${pki_path}/distribution.crt"
mv "${pki_path}/distribution-key.pem" "${pki_path}/distribution.key"

# Auth token certs
echo "${auth_token_csr_json}" | /opt/deckhouse/bin/cfssl gencert \
  -cn="registry-auth-token" \
  -ca="${pki_path}/ca.crt" \
  -ca-key="${pki_path}/ca.key" \
  -config="${pki_path}/profiles.json" \
  -profile="auth-token" - | /opt/deckhouse/bin/cfssljson -bare "${pki_path}/token"
mv "${pki_path}/token.pem" "${pki_path}/token.crt"
mv "${pki_path}/token-key.pem" "${pki_path}/token.key"

# Cleanup
rm -f "${pki_path}/auth.csr" \
      "${pki_path}/distribution.csr" \
      "${pki_path}/token.csr" \
      "${pki_path}/profiles.json" \
      "${pki_path}/ca.key"

# Prepare auth manifest
bb-sync-file "${auth_path}/config.yaml" - << EOF
server:
  addr: "127.0.0.1:5051"
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

  {{- if eq .registry.mode "Local" }}
  {{ .registry.bootstrap.init.rw_user.name | quote }}:
    password: {{ .registry.bootstrap.init.rw_user.password_hash | quote | replace "$" "\\$" }}
  {{- end }}

acl:
  - match: { account: {{ .registry.bootstrap.init.ro_user.name | quote }} }
    actions: ["pull"]
    comment: "has readonly access"
  {{- if eq .registry.mode "Local" }}
  - match: { account: {{ .registry.bootstrap.init.rw_user.name | quote }} }
    actions: [ "*" ]
    comment: "has full access"
  {{- end }}
EOF

# Prepare distribution manifest
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
  addr: "${discovered_node_ip}:5001"
  prefix: /
  secret: asecretforbootstrap
  debug:
    addr: "127.0.0.1:5002"
    prometheus:
      enabled: true
      path: /metrics
  tls:
    certificate: "${pki_path}/distribution.crt"
    key: "${pki_path}/distribution.key"

{{- with .registry.bootstrap.proxy }}
proxy:
  remoteurl: "{{ .scheme }}://{{ .host }}"
  {{- if .username }}
  username: {{ .username | quote }}
  password: {{ .password | quote }}
  {{- end }}
  remotepathonly: {{ .path | quote }}
  localpathalias: "/system/deckhouse"
  {{- with .ca }}
  ca: "${pki_path}/upstream-registry-ca.crt"
  {{- end }}
  {{- with .ttl }}
  ttl: {{ . | quote }}
  {{- end }}
{{- end }}

auth:
  token:
    realm: https://${discovered_node_ip}:5051/auth
    service: Deckhouse registry
    issuer: Registry server
    rootcertbundle: "${pki_path}/token.crt"
    autoredirect: true
    proxy:
      url: https://127.0.0.1:5051/auth
      ca: "${pki_path}/ca.crt"
EOF

# Prepare start script
bb-sync-file "${igniter_start_sh}" - << EOF
#!/bin/bash

# Unset all registry env
for var in \$(compgen -e REGISTRY); do
    unset "\${var}"
done

start_and_wait() {
    local log_path=\${1}
    local bin_path=\${2}
    shift 2
    local args=("\$@")

    local sleep_interval=1
    local max_attempts=10

    echo "Starting and waiting background process: \${bin_path}"

    if pgrep -f "\${bin_path}" > /dev/null 2>&1; then
        echo "\${bin_path}: already running"
        return 0
    fi

    "\${bin_path}" "\${args[@]}" > "\${log_path}" 2>&1 &

    for ((i=1; i<=max_attempts; i++)); do
        echo "Attempt: \${i}/\${max_attempts}"
        sleep \${sleep_interval}

        if pgrep -f "\${bin_path}" > /dev/null 2>&1; then
            echo "\${bin_path}: started"
            return 0
        fi
    done

    echo "\${bin_path}: failed to start (timeout \${sleep_interval}s * \${max_attempts})"
    return 1
}

liveness_probe() {
    local address=\${1}
    local ca_path=\${2}

    local sleep_interval=1
    local max_attempts=20

    echo "Waiting liveness probe: \${address}"

    for ((i=1; i<=max_attempts; i++)); do
        if [[ \${i} -ne 1 ]]; then
            echo "Attempt: \${i}/\${max_attempts}"
            sleep \${sleep_interval}
        fi

        local response=\$(d8-curl --cacert "\${ca_path}" -s -o /dev/null -w "%{http_code}" "\${address}" 2>/dev/null)
        if [[ "\${response}" == "200" ]]; then
            echo "\${address} is reachable"
            return 0
        fi
    done

    echo "\${address}: not reachable (timeout \${sleep_interval}s * \${max_attempts})"
    return 1
}

echo "Starting registry auth..."
if ! start_and_wait "${log_path}/auth.log" /opt/deckhouse/bin/ign-auth -logtostderr "${auth_path}/config.yaml"; then
    echo "ERROR: registry auth failed to start, see ${log_path}/auth.log"
    exit 1
fi
if ! liveness_probe "https://127.0.0.1:5051" "${pki_path}/ca.crt"; then
    echo "ERROR: registry auth liveness probe failed, see ${log_path}/auth.log"
    exit 1
fi

echo "Starting registry distribution..."
if ! start_and_wait "${log_path}/distribution.log" /opt/deckhouse/bin/ign-registry serve "${distribution_path}/config.yaml"; then
    echo "ERROR: registry distribution failed to start, see ${log_path}/distribution.log"
    exit 1
fi
if ! liveness_probe "https://${discovered_node_ip}:5001" "${pki_path}/ca.crt"; then
    echo "ERROR: registry distribution liveness probe failed, see ${log_path}/distribution.log"
    exit 1
fi

echo "All services started successfully, logs: ${log_path}"
EOF

# Prepare stop script
bb-sync-file "${igniter_stop_sh}" - << EOF
#!/bin/bash

kill_and_wait() {
    local bin_path=\${1}

    echo "Stopping and waiting background process: \${bin_path}"

    pkill -f "\${bin_path}" 2>/dev/null || true

    local sleep_interval=1
    local max_attempts=10
    for ((i=1; i<=max_attempts; i++)); do
        if [[ \${i} -ne 1 ]]; then
            echo "Attempt: \${i}/\${max_attempts}"
            sleep \${sleep_interval}
        fi

        if ! pgrep -f "\${bin_path}" > /dev/null 2>&1; then
            echo "\${bin_path}: stopped"
            return 0
        fi
    done

    echo "\${bin_path}: timeout, sending SIGKILL and wait"
    pkill -9 -f "\${bin_path}" 2>/dev/null || true

    local sleep_interval=1
    local max_attempts=5
    for ((i=1; i<=max_attempts; i++)); do
        if [[ \${i} -ne 1 ]]; then
            echo "Attempt: \${i}/\${max_attempts}"
            sleep \${sleep_interval}
        fi

        if ! pgrep -f "\${bin_path}" > /dev/null 2>&1; then
            echo "\${bin_path}: stopped (forced)"
            return 0
        fi
    done

    echo "\${bin_path}: still running after SIGKILL."
    return 1
}

echo "Stopping registry distribution..."
if ! kill_and_wait "/opt/deckhouse/bin/ign-registry"; then
    echo "ERROR: Failed to stop registry distribution, see ${log_path}/distribution.log"
    exit 1
fi

echo "Stopping registry auth..."
if ! kill_and_wait "/opt/deckhouse/bin/ign-auth"; then
    echo "ERROR: Failed to stop registry auth, see ${log_path}/auth.log"
    exit 1
fi

echo "All services stopped"
EOF

chmod a+x "${igniter_stop_sh}"
chmod a+x "${igniter_start_sh}"

# Switching static pod to igniter

# Stop static pod
rm -f "${static_pod_file}"
pod_kill_and_wait "${static_pod_name}"

# Start igniter
bash "${igniter_stop_sh}"
bash "${igniter_start_sh}"

# Unset proxy envs
bb-unset-proxy

{{- end }}
