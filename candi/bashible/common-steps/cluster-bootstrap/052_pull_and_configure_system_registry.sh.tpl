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

# other data
{{ $imgDockerDistribution := printf "%s@%s" .registry.imagesBase (index $.images.systemRegistry "dockerDistribution") }}
{{ $imgDockerAuth := printf "%s@%s" .registry.imagesBase (index $.images.systemRegistry "dockerAuth") }}
{{ $imgPause := printf "%s@%s" .registry.imagesBase (index $.images.common "pause") }}

# Prepare vars
discovered_node_ip="$(bb-d8-node-ip)"
registry_pki_path="/etc/kubernetes/system-registry/pki"


# Create a directories for the system registry configuration
mkdir -p /etc/kubernetes/system-registry/{auth_config,distribution_config,pki}

# Create a directories for the system registry data if it does not exist
mkdir -p /opt/deckhouse/system-registry/local_data/

# Prepare certs
bb-sync-file "$registry_pki_path/ca.crt" - << EOF
{{ $internalRegistryCACert }}
EOF

bb-sync-file "$registry_pki_path/ca.key" - << EOF
{{ $internalRegistryCAKey }}
EOF

{{- if eq $registryMode "Proxy" }}
  {{- if $upstreamRegistryCACert }}
bb-sync-file "$registry_pki_path/upstream-registry-ca.crt" - << EOF
{{ $upstreamRegistryCACert }}
EOF
  {{- end }}
{{- end }}

bb-sync-file "$registry_pki_path/profiles.json" - << EOF
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
  -ca="$registry_pki_path/ca.crt" \
  -ca-key="$registry_pki_path/ca.key" \
  -config="$registry_pki_path/profiles.json" \
  -profile="client-server" - | /opt/deckhouse/bin/cfssljson -bare "${registry_pki_path}/auth"
mv "${registry_pki_path}/auth.pem" "${registry_pki_path}/auth.crt"
mv "${registry_pki_path}/auth-key.pem" "${registry_pki_path}/auth.key"

# Distribution certs
echo $client_server_csr_json | /opt/deckhouse/bin/cfssl gencert \
  -cn="embedded-registry-distribution" \
  -ca="$registry_pki_path/ca.crt" \
  -ca-key="$registry_pki_path/ca.key" \
  -config="$registry_pki_path/profiles.json" \
  -profile="client-server" - | /opt/deckhouse/bin/cfssljson -bare "${registry_pki_path}/distribution"
mv "${registry_pki_path}/distribution.pem" "${registry_pki_path}/distribution.crt"
mv "${registry_pki_path}/distribution-key.pem" "${registry_pki_path}/distribution.key"

# Auth token certs
echo $auth_token_csr_json | /opt/deckhouse/bin/cfssl gencert \
  -cn="embedded-registry-auth-token" \
  -ca="$registry_pki_path/ca.crt" \
  -ca-key="$registry_pki_path/ca.key" \
  -config="$registry_pki_path/profiles.json" \
  -profile="auth-token" - | /opt/deckhouse/bin/cfssljson -bare "${registry_pki_path}/token"
mv "${registry_pki_path}/token.pem" "${registry_pki_path}/token.crt"
mv "${registry_pki_path}/token-key.pem" "${registry_pki_path}/token.key"

# Cleanup
rm "${registry_pki_path}/auth.csr"\
    "${registry_pki_path}/distribution.csr" \
    "${registry_pki_path}/token.csr" \
    "${registry_pki_path}/profiles.json"

bb-sync-file /etc/kubernetes/system-registry/auth_config/config.yaml - << EOF
server:
  addr: "127.0.0.1:5051"
  real_ip_header: "X-Forwarded-For"
  certificate: "/system_registry_pki/auth.crt"
  key: "/system_registry_pki/auth.key"
token:
  issuer: "Registry server"
  expiration: 900
  certificate: "/system_registry_pki/token.crt"
  key: "/system_registry_pki/token.key"

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

bb-sync-file /etc/kubernetes/system-registry/distribution_config/config.yaml - << EOF
version: 0.1
log:
  level: info

storage:
  filesystem:
    rootdirectory: /data
  delete:
    enabled: true
  redirect:
    disable: true
  cache:
    blobdescriptor: inmemory

http:
  addr: "${discovered_node_ip}:5001"
  prefix: /
  secret: asecretforlocaldevelopment
  debug:
    addr: "127.0.0.1:5002"
    prometheus:
      enabled: true
      path: /metrics
  tls:
    certificate: /system_registry_pki/distribution.crt
    key: /system_registry_pki/distribution.key

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
    rootcertbundle: /system_registry_pki/token.crt
    autoredirect: true
    proxy:
      url: https://127.0.0.1:5051/auth
      ca: /system_registry_pki/ca.crt
EOF

bb-sync-file /etc/kubernetes/manifests/system-registry.yaml - << EOF
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: system-registry
    component: system-registry
    tier: control-plane
  name: system-registry
  namespace: d8-system
spec:
  securityContext:
    runAsGroup: 0
    runAsNonRoot: false
    runAsUser: 0
    seccompProfile:
      type: RuntimeDefault
  dnsPolicy: ClusterFirst
  hostNetwork: true
  containers:
  - name: distribution
    image: {{ $imgDockerDistribution }}
    imagePullPolicy: IfNotPresent
    args:
      - serve
      - /config/config.yaml
    {{- if and .proxy (eq $registryMode "Proxy") }}
    env:
    - name: HTTP_PROXY
      value: "${HTTP_PROXY}"
    - name: http_proxy
      value: "${HTTP_PROXY}"
    - name: HTTPS_PROXY
      value: "${HTTPS_PROXY}"
    - name: https_proxy
      value: "${HTTPS_PROXY}"
    - name: NO_PROXY
      value: "${NO_PROXY}"
    - name: no_proxy
      value: "${NO_PROXY}"
    {{- end }}
    ports:
    - name: emb-reg-dist
      containerPort: 5001
      hostPort: 5001
    volumeMounts:
      - mountPath: /data
        name: distribution-data-volume
      - mountPath: /config
        name: distribution-config-volume
      - mountPath: /system_registry_pki
        name: system-registry-pki-volume
  - name: auth
    image: {{ $imgDockerAuth }}
    imagePullPolicy: IfNotPresent
    args:
      - -logtostderr
      - /config/config.yaml
    volumeMounts:
      - mountPath: /config
        name: auth-config-volume
      - mountPath: /system_registry_pki
        name: system-registry-pki-volume
  priorityClassName: system-node-critical
  volumes:
  - name: kubernetes-pki-volume
    hostPath:
      path: /etc/kubernetes/pki
      type: Directory
  - name: system-registry-pki-volume
    hostPath:
      path: /etc/kubernetes/system-registry/pki
      type: Directory
  - name: auth-config-volume
    hostPath:
      path: /etc/kubernetes/system-registry/auth_config
      type: DirectoryOrCreate
  - name: distribution-config-volume
    hostPath:
      path: /etc/kubernetes/system-registry/distribution_config
      type: DirectoryOrCreate
  - name: distribution-data-volume
    hostPath:
      path: /opt/deckhouse/system-registry/local_data
      type: DirectoryOrCreate
  - name: tmp
    emptyDir: {}
EOF

/opt/deckhouse/bin/crictl pull {{ $imgDockerDistribution }}
/opt/deckhouse/bin/crictl pull {{ $imgDockerAuth }}
/opt/deckhouse/bin/crictl pull {{ $imgPause }}

bash "$IGNITER_DIR/stop_system_registry_igniter.sh"

{{- end }}
