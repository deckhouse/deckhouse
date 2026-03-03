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

bb-package-install "module-registry-auth:{{ .images.registry.dockerAuth }}" "module-registry-distribution:{{ .images.registry.dockerDistribution }}" "cfssl:{{ .images.registrypackages.cfssl165 }}"

# Prepare proxy envs
bb-set-proxy

# Prepare vars
discovered_node_ip="$(bb-d8-node-ip)"
pki_path="${REGISTRY_MODULE_IGNITER_DIR}/pki"

# Create the directories
mkdir -p ${REGISTRY_MODULE_IGNITER_DIR}/{pki,logs} \
        /opt/deckhouse/registry/local_data

# Prepare certs
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
echo "$client_server_csr_json" | /opt/deckhouse/bin/cfssl gencert \
  -cn="registry-auth" \
  -ca="${pki_path}/ca.crt" \
  -ca-key="${pki_path}/ca.key" \
  -config="${pki_path}/profiles.json" \
  -profile="client-server" - | /opt/deckhouse/bin/cfssljson -bare "${pki_path}/auth"
mv "${pki_path}/auth.pem" "${pki_path}/auth.crt"
mv "${pki_path}/auth-key.pem" "${pki_path}/auth.key"

# Distribution certs
echo "$client_server_csr_json" | /opt/deckhouse/bin/cfssl gencert \
  -cn="registry-distribution" \
  -ca="${pki_path}/ca.crt" \
  -ca-key="${pki_path}/ca.key" \
  -config="${pki_path}/profiles.json" \
  -profile="client-server" - | /opt/deckhouse/bin/cfssljson -bare "${pki_path}/distribution"
mv "${pki_path}/distribution.pem" "${pki_path}/distribution.crt"
mv "${pki_path}/distribution-key.pem" "${pki_path}/distribution.key"

# Auth token certs
echo "$auth_token_csr_json" | /opt/deckhouse/bin/cfssl gencert \
  -cn="registry-auth-token" \
  -ca="${pki_path}/ca.crt" \
  -ca-key="${pki_path}/ca.key" \
  -config="${pki_path}/profiles.json" \
  -profile="auth-token" - | /opt/deckhouse/bin/cfssljson -bare "${pki_path}/token"
mv "${pki_path}/token.pem" "${pki_path}/token.crt"
mv "${pki_path}/token-key.pem" "${pki_path}/token.key"

# Cleanup
rm "${pki_path}/auth.csr"\
    "${pki_path}/distribution.csr" \
    "${pki_path}/token.csr" \
    "${pki_path}/profiles.json"

# Prepare auth manifest
bb-sync-file "${REGISTRY_MODULE_IGNITER_DIR}/auth_config.yaml" - << EOF
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
bb-sync-file "${REGISTRY_MODULE_IGNITER_DIR}/distribution_config.yaml" - << EOF
version: 0.1
log:
  level: info

storage:
  filesystem:
    rootdirectory: /opt/deckhouse/registry/local_data
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
bb-sync-file "${REGISTRY_MODULE_IGNITER_DIR}/start_registry_igniter.sh" - << EOF
#!/bin/bash

for var in \$(compgen -e REGISTRY); do
    unset \$var
done

check_and_run() {
    service_name=\$1
    command=\$2
    log_path=\$3

    if pgrep -x "\$service_name" > /dev/null; then
        echo "\$service_name is already running."
    else
        \$command > \$log_path 2>&1 &
        echo "\$service_name started."
    fi
}

echo "Awaiting the startup of the registry storage and Docker registry..."
max_attempts=30
docker_registry_started=false

check_and_run "ign-auth" "/opt/deckhouse/bin/ign-auth -logtostderr ${REGISTRY_MODULE_IGNITER_DIR}/auth_config.yaml" "${REGISTRY_MODULE_IGNITER_DIR}/logs/auth.log"
check_and_run "ign-registry" "/opt/deckhouse/bin/ign-registry serve ${REGISTRY_MODULE_IGNITER_DIR}/distribution_config.yaml" "${REGISTRY_MODULE_IGNITER_DIR}/logs/distribution.log"

for (( attempt=1; attempt <= \$max_attempts; attempt++ )); do
    response=\$(d8-curl --cacert "${pki_path}/ca.crt" -s -o /dev/null -w "%{http_code}" https://${discovered_node_ip}:5001)
    if [[ "\$response" == "200" ]]; then
        docker_registry_started=true
        break
    fi
    sleep 1
done
if [ "\$docker_registry_started" = false ]; then
    echo "Failed to confirm the startup of Docker registry after \$max_attempts attempts. Please check the logs at ${REGISTRY_MODULE_IGNITER_DIR}/logs/distribution.log"
    exit 1
fi

echo "All services are starting in the background and logs are being written to ${REGISTRY_MODULE_IGNITER_DIR}/logs"
EOF

# Prepare stop script
bb-sync-file "${REGISTRY_MODULE_IGNITER_DIR}/stop_registry_igniter.sh" - << EOF
#!/bin/bash

stop_service() {
    service_name=\$1
    pkill -x \$service_name || true
    wait_time=0
    while ps -C \$service_name > /dev/null; do
        sleep 1
        ((wait_time++))
        if [ \$wait_time -gt 20 ]; then
            echo "Process \$service_name has not completed in 20 seconds, SIGKILL is being sent..."
            pkill -9 -x \$service_name
            break
        fi
    done
    echo "\$service_name stopped"
}

stop_service "ign-registry"
stop_service "ign-auth"

echo "All services have been stopped."
EOF

chmod a+x "${REGISTRY_MODULE_IGNITER_DIR}/start_registry_igniter.sh"
chmod a+x "${REGISTRY_MODULE_IGNITER_DIR}/stop_registry_igniter.sh"


# Switching registry from static pod to igniter
rm -f /etc/kubernetes/manifests/registry-nodeservices.yaml
bash "${REGISTRY_MODULE_IGNITER_DIR}/stop_registry_igniter.sh"
bash "${REGISTRY_MODULE_IGNITER_DIR}/start_registry_igniter.sh"

# Unset proxy envs
bb-unset-proxy

{{- end }}
