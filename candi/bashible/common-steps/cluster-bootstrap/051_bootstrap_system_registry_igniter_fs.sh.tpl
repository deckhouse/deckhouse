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

{{- if and .registry.registryMode (ne .registry.registryMode "Direct") }}
{{- if eq .registry.registryStorageMode "Fs" }}

# Prepare UPSTREAM_REGISTRY vars for registryMode == Proxy
{{- if eq .registry.registryMode "Proxy" }}
UPSTREAM_REGISTRY_AUTH="$(base64 -d <<< "{{ .registry.upstreamRegistry.auth | default "" }}")"
if [[ "$UPSTREAM_REGISTRY_AUTH" == *":"* ]]; then
    export UPSTREAM_REGISTRY_LOGIN="$(echo "$UPSTREAM_REGISTRY_AUTH" | cut -d':' -f1)"
    export UPSTREAM_REGISTRY_PASSWORD="$(echo "$UPSTREAM_REGISTRY_AUTH" | cut -d':' -f2)"
else
    export UPSTREAM_REGISTRY_LOGIN=""
    export UPSTREAM_REGISTRY_PASSWORD=""
fi
{{- end }}

# Prepare vars
discovered_node_ip="$(</var/lib/bashible/discovered-node-ip)"
internal_registry_domain="{{ .registry.address }}"
if [[ "$internal_registry_domain" == *":"* ]]; then
    internal_registry_domain="$(echo "$internal_registry_domain" | cut -d':' -f1)"
fi

# Install igniter packages
bb-package-install "dockerAuth:{{ .images.systemRegistry.dockerAuth }}" "dockerDistribution:{{ .images.systemRegistry.dockerDistribution }}" "cfssl:{{ .images.registrypackages.cfssl165 }}"

# Create a directories for the system registry configuration
mkdir -p $IGNITER_DIR

# Create a directories for the system registry data if it does not exist
mkdir -p /opt/deckhouse/system-registry/local_data/

# Prepare certs
bb-sync-file "$IGNITER_DIR/ca.crt" - << EOF
{{ .registry.internalRegistryAccess.ca.cert }}
EOF

bb-sync-file "$IGNITER_DIR/ca.key" - << EOF
{{ .registry.internalRegistryAccess.ca.key }}
EOF

{{- if eq .registry.registryMode "Proxy" }}
bb-sync-file "$IGNITER_DIR/upstream-registry-ca.crt" - << EOF
{{ .registry.upstreamRegistry.ca }}
EOF
{{- end }}

# Auth certs
openssl genrsa -out "$IGNITER_DIR/auth.key" 2048

openssl req -new -key "$IGNITER_DIR/auth.key" \
-subj "/CN=embedded-registry-auth" \
-addext "subjectAltName=IP:127.0.0.1,DNS:localhost,IP:${discovered_node_ip},DNS:${internal_registry_domain}" \
-out "$IGNITER_DIR/auth.csr"

openssl x509 -req -in "$IGNITER_DIR/auth.csr" -CA "$IGNITER_DIR/ca.crt" -CAkey "$IGNITER_DIR/ca.key" -CAcreateserial \
-out "$IGNITER_DIR/auth.crt" -days 365 -sha256 \
-extfile <(printf "subjectAltName=IP:127.0.0.1,DNS:localhost,IP:${discovered_node_ip},DNS:${internal_registry_domain}")


# Distribution certs
openssl genrsa -out "$IGNITER_DIR/distribution.key" 2048

openssl req -new -key "$IGNITER_DIR/distribution.key" \
-subj "/CN=embedded-registry-distribution" \
-addext "subjectAltName=IP:127.0.0.1,DNS:localhost,IP:${discovered_node_ip},DNS:${internal_registry_domain}" \
-out "$IGNITER_DIR/distribution.csr"

openssl x509 -req -in "$IGNITER_DIR/distribution.csr" -CA "$IGNITER_DIR/ca.crt" -CAkey "$IGNITER_DIR/ca.key" -CAcreateserial \
-out "$IGNITER_DIR/distribution.crt" -days 365 -sha256 \
-extfile <(printf "subjectAltName=IP:127.0.0.1,DNS:localhost,IP:${discovered_node_ip},DNS:${internal_registry_domain}")

bb-sync-file "$IGNITER_DIR/auth_config.yaml" - << EOF
server:
  addr: "127.0.0.1:5051"
  certificate: "$IGNITER_DIR/auth.crt"
  key: "$IGNITER_DIR/auth.key"
token:
  issuer: "Registry server"
  expiration: 900
  certificate: "$IGNITER_DIR/auth.crt"
  key: "$IGNITER_DIR/auth.key"

users:
  # Password is specified as a BCrypt hash. Use htpasswd -nB USERNAME to generate.
  {{ .registry.internalRegistryAccess.userRw.name | quote }}:
    password: "{{ .registry.internalRegistryAccess.userRw.passwordHash | replace "$" "\\$" }}"
  {{ .registry.internalRegistryAccess.userRo.name | quote }}:
    password: "{{ .registry.internalRegistryAccess.userRo.passwordHash | replace "$" "\\$" }}"

acl:
  - match: { account: {{ .registry.internalRegistryAccess.userRw.name | quote }} }
    actions: [ "*" ]
    comment: "has full access"
  - match: { account: {{ .registry.internalRegistryAccess.userRo.name | quote }} }
    actions: ["pull"]
    comment: "has readonly access"
  # Access is denied by default.
EOF

bb-sync-file "$IGNITER_DIR/distribution_config.yaml" - << EOF
version: 0.1
log:
  level: info

storage:
  filesystem:
    rootdirectory: /opt/deckhouse/system-registry/local_data
  delete:
    enabled: true
  redirect:
    disable: true
  cache:
    blobdescriptor: inmemory

http:
  addr: 0.0.0.0:5001
  prefix: /
  secret: asecretforlocaldevelopment
  debug:
    addr: 127.0.0.1:5002
    prometheus:
      enabled: true
      path: /metrics
  tls:
    certificate: $IGNITER_DIR/distribution.crt
    key: $IGNITER_DIR/distribution.key
#    clientcas:
#      - $IGNITER_DIR/ca.crt

{{- if eq .registry.registryMode "Proxy" -}}
{{- $scheme := .registry.upstreamRegistry.scheme | trimSuffix "/" | trimPrefix "/" -}}
{{- $address := .registry.upstreamRegistry.address | trimSuffix "/" | trimPrefix "/" }}
proxy:
  remoteurl: "{{ $scheme }}://{{ $address }}"
  username: "$UPSTREAM_REGISTRY_LOGIN"
  password: "$UPSTREAM_REGISTRY_PASSWORD"
  remotepathonly: "{{ .registry.upstreamRegistry.path }}"
  localpathalias: "{{ .registry.path }}"
  ttl: "{{ .registry.ttl }}"
{{- end }}

auth:
  token:
    realm: https://127.0.0.1:5051/auth
    service: Docker registry
    issuer: Registry server
    rootcertbundle: "$IGNITER_DIR/auth.crt"
    autoredirect: false
EOF

bb-sync-file "$IGNITER_DIR/start_system_registry_igniter.sh" - << EOF
#!/bin/bash

for var in \$(compgen -e REGISTRY); do
    unset \$var
done

mkdir -p $IGNITER_DIR/logs

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

check_and_run "auth_server" "/opt/deckhouse/bin/auth_server -logtostderr $IGNITER_DIR/auth_config.yaml" "$IGNITER_DIR/logs/auth.log"
check_and_run "registry" "/opt/deckhouse/bin/registry serve $IGNITER_DIR/distribution_config.yaml" "$IGNITER_DIR/logs/distribution.log"

for (( attempt=1; attempt <= \$max_attempts; attempt++ )); do
    response=\$(d8-curl --cacert "$IGNITER_DIR/ca.crt" -s -o /dev/null -w "%{http_code}" https://127.0.0.1:5001)
    if [[ "\$response" == "200" ]]; then
        docker_registry_started=true
        break
    fi
    sleep 1
done
if [ "\$docker_registry_started" = false ]; then
    echo "Failed to confirm the startup of Docker registry after \$max_attempts attempts. Please check the logs at ${IGNITER_DIR}/logs/distribution.log"
    exit 1
fi

echo "All services are starting in the background and logs are being written to $IGNITER_DIR/logs"

EOF

bb-sync-file "$IGNITER_DIR/stop_system_registry_igniter.sh" - << EOF
#!/bin/bash

stop_service() {
    service_name=\$1
    pkill -x \$service_name
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

stop_service "registry"
stop_service "auth_server"

echo "All services have been stopped."
EOF

chmod a+x "$IGNITER_DIR/start_system_registry_igniter.sh"
chmod a+x "$IGNITER_DIR/stop_system_registry_igniter.sh"

bash "$IGNITER_DIR/stop_system_registry_igniter.sh"
bash "$IGNITER_DIR/start_system_registry_igniter.sh"

{{- end }}
{{- end }}
