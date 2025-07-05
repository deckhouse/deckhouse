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

{{- $registryMode := .registry.mode }}
{{- if has $registryMode (list "Proxy" "Detached") }}

# registry proxy mode data

{{- $upstreamRegistryHost := "" }}
{{- $upstreamRegistryPath := "" }}
{{- $upstreamRegistryScheme := "" }}
{{- $upstreamRegistryCACert := "" }}
{{- $upstreamRegistryUserName := "" }}
{{- $upstreamRegistryUserPassword := "" }}
{{- $internalRegistryTTL := "" }}
{{- if eq $registryMode "Proxy" }}
  {{- $upstreamRegistryHost = .registry.bootstrap.upstreamRegistryData.address }}
  {{- $upstreamRegistryPath = .registry.bootstrap.upstreamRegistryData.path }}
  {{- $upstreamRegistryScheme = .registry.bootstrap.upstreamRegistryData.scheme }}
  {{- with .registry.bootstrap.upstreamRegistryData.ca }}
  {{- $upstreamRegistryCACert = . }}
  {{- end }}
  {{- $upstreamRegistryUserName = .registry.bootstrap.upstreamRegistryData.username }}
  {{- $upstreamRegistryUserPassword = .registry.bootstrap.upstreamRegistryData.password }}
  {{- $internalRegistryTTL = .registry.bootstrap.internalRegistryTTL }}
{{- end }}

# registry common data
{{- $internalRegistryUserRO := .registry.bootstrap.internalRegistryPKI.userRO }}
{{- $internalRegistryUserRW := .registry.bootstrap.internalRegistryPKI.userRW }}
{{- $internalRegistryCACert := .registry.bootstrap.internalRegistryPKI.ca.cert }}
{{- $internalRegistryCAKey := .registry.bootstrap.internalRegistryPKI.ca.key }}
{{- $internalRegistryHost := .registry.bootstrap.internalRegistryData.address }}
{{- $internalRegistryHostWithouPort :=  (splitList ":" $internalRegistryHost) | first }}
{{- $internalRegistryPath := .registry.bootstrap.internalRegistryData.path }}
{{- $internalRegistryScheme := .registry.bootstrap.upstreamRegistryData.scheme }}


# Prepare vars
discovered_node_ip="$(bb-d8-node-ip)"

# Install igniter packages
bb-package-install "dockerAuth:{{ .images.systemRegistry.dockerAuth }}" "dockerDistribution:{{ .images.systemRegistry.dockerDistribution }}" "cfssl:{{ .images.registrypackages.cfssl165 }}"

# Create a directories for the system registry configuration
mkdir -p $IGNITER_DIR

# Create a directories for the system registry data if it does not exist
mkdir -p /opt/deckhouse/system-registry/local_data/

# Prepare certs
bb-sync-file "$IGNITER_DIR/ca.crt" - << EOF
{{ $internalRegistryCACert }}
EOF

bb-sync-file "$IGNITER_DIR/ca.key" - << EOF
{{ $internalRegistryCAKey }}
EOF

{{- if eq $registryMode "Proxy" }}
  {{- if $upstreamRegistryCACert }}
bb-sync-file "$IGNITER_DIR/upstream-registry-ca.crt" - << EOF
{{ $upstreamRegistryCACert }}
EOF
  {{- end }}
{{- end }}

bb-sync-file "$IGNITER_DIR/profiles.json" - << EOF
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
  "hosts": ["127.0.0.1", "localhost", "${discovered_node_ip}", {{ $internalRegistryHostWithouPort | quote }}],
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
echo $client_server_csr_json | /opt/deckhouse/bin/cfssl gencert \
  -cn="embedded-registry-auth" \
  -ca="$IGNITER_DIR/ca.crt" \
  -ca-key="$IGNITER_DIR/ca.key" \
  -config="$IGNITER_DIR/profiles.json" \
  -profile="client-server" - | /opt/deckhouse/bin/cfssljson -bare "${IGNITER_DIR}/auth"
mv "${IGNITER_DIR}/auth.pem" "${IGNITER_DIR}/auth.crt"
mv "${IGNITER_DIR}/auth-key.pem" "${IGNITER_DIR}/auth.key"

# Distribution certs
echo $client_server_csr_json | /opt/deckhouse/bin/cfssl gencert \
  -cn="embedded-registry-distribution" \
  -ca="$IGNITER_DIR/ca.crt" \
  -ca-key="$IGNITER_DIR/ca.key" \
  -config="$IGNITER_DIR/profiles.json" \
  -profile="client-server" - | /opt/deckhouse/bin/cfssljson -bare "${IGNITER_DIR}/distribution"
mv "${IGNITER_DIR}/distribution.pem" "${IGNITER_DIR}/distribution.crt"
mv "${IGNITER_DIR}/distribution-key.pem" "${IGNITER_DIR}/distribution.key"

# Auth token certs
echo $auth_token_csr_json | /opt/deckhouse/bin/cfssl gencert \
  -cn="embedded-registry-auth-token" \
  -ca="$IGNITER_DIR/ca.crt" \
  -ca-key="$IGNITER_DIR/ca.key" \
  -config="$IGNITER_DIR/profiles.json" \
  -profile="auth-token" - | /opt/deckhouse/bin/cfssljson -bare "${IGNITER_DIR}/token"
mv "${IGNITER_DIR}/token.pem" "${IGNITER_DIR}/token.crt"
mv "${IGNITER_DIR}/token-key.pem" "${IGNITER_DIR}/token.key"

# Cleanup
rm "${IGNITER_DIR}/auth.csr"\
    "${IGNITER_DIR}/distribution.csr" \
    "${IGNITER_DIR}/token.csr" \
    "${IGNITER_DIR}/profiles.json"

bb-sync-file "$IGNITER_DIR/auth_config.yaml" - << EOF
server:
  addr: "127.0.0.1:5051"
  real_ip_header: "X-Forwarded-For"
  certificate: "$IGNITER_DIR/auth.crt"
  key: "$IGNITER_DIR/auth.key"
token:
  issuer: "Registry server"
  expiration: 900
  certificate: "$IGNITER_DIR/token.crt"
  key: "$IGNITER_DIR/token.key"

users:
  # Password is specified as a BCrypt hash. Use htpasswd -nB USERNAME to generate.
  {{ $internalRegistryUserRW.name | quote }}:
    password: "{{ $internalRegistryUserRW.passwordHash | replace "$" "\\$" }}"
  {{ $internalRegistryUserRO.name | quote }}:
    password: "{{ $internalRegistryUserRO.passwordHash | replace "$" "\\$" }}"

acl:
  # Access is denied by default.
  - match: { account: {{ $internalRegistryUserRW.name | quote }} }
    actions: [ "*" ]
    comment: "has full access"
  - match: { account: {{ $internalRegistryUserRO.name | quote }} }
    actions: ["pull"]
    comment: "has readonly access"
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
  addr: "0.0.0.0:5001"
  prefix: /
  secret: asecretforlocaldevelopment
  debug:
    addr: "127.0.0.1:5002"
    prometheus:
      enabled: true
      path: /metrics
  tls:
    certificate: $IGNITER_DIR/distribution.crt
    key: $IGNITER_DIR/distribution.key

{{- if eq $registryMode "Proxy" }}
proxy:
  remoteurl: "{{ $upstreamRegistryScheme }}://{{ $upstreamRegistryHost }}"
  username: {{ $upstreamRegistryUserName | quote }}
  password: {{ $upstreamRegistryUserPassword | quote }}
  remotepathonly: {{ $upstreamRegistryPath | quote }}
  localpathalias: {{ $internalRegistryPath | quote }}
    {{- if $upstreamRegistryCACert }}
  ca: $IGNITER_DIR/upstream-registry-ca.crt
    {{- end }}
  ttl: {{ $internalRegistryTTL | quote }}
{{- end }}

auth:
  token:
    realm: https://127.0.0.1:5051/auth
    service: Deckhouse registry
    issuer: Registry server
    rootcertbundle: "$IGNITER_DIR/token.crt"
    autoredirect: true
    proxy:
      url: https://127.0.0.1:5051/auth
      ca: $IGNITER_DIR/ca.crt
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
