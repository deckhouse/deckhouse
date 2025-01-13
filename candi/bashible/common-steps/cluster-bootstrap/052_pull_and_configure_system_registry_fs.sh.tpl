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

{{- if and .registry.embeddedRegistryModuleMode (ne .registry.embeddedRegistryModuleMode "Direct") }}
{{- if eq .registry.registryStorageMode "Fs" }}

# Prepare UPSTREAM_REGISTRY vars for embeddedRegistryModuleMode == Proxy
{{- if eq .registry.embeddedRegistryModuleMode "Proxy" }}
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
registry_pki_path="/etc/kubernetes/system-registry/pki"
internal_registry_domain="{{ .registry.address }}"
if [[ "$internal_registry_domain" == *":"* ]]; then
    internal_registry_domain="$(echo "$internal_registry_domain" | cut -d':' -f1)"
fi


# Create a directories for the system registry configuration
mkdir -p /etc/kubernetes/system-registry/{auth_config,distribution_config,pki}

# Create a directories for the system registry data if it does not exist
mkdir -p /opt/deckhouse/system-registry/local_data/

# Prepare certs
bb-sync-file "$registry_pki_path/ca.crt" - << EOF
{{ .registry.internalRegistryAccess.ca.cert }}
EOF

bb-sync-file "$registry_pki_path/ca.key" - << EOF
{{ .registry.internalRegistryAccess.ca.key }}
EOF

{{- if eq .registry.embeddedRegistryModuleMode "Proxy" }}
bb-sync-file "$registry_pki_path/upstream-registry-ca.crt" - << EOF
{{ .registry.upstreamRegistry.ca }}
EOF
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
bb-sync-file "$registry_pki_path/client-server.json" - << EOF
{
  "hosts": ["127.0.0.1", "localhost", "${discovered_node_ip}", "${internal_registry_domain}"],
  "key": {"algo": "rsa", "size": 2048}
}
EOF
bb-sync-file "$registry_pki_path/auth-token.json" - << EOF
{
  "key": {"algo": "rsa", "size": 2048}
}
EOF

# Auth certs
/opt/deckhouse/bin/cfssl gencert \
  -cn="embedded-registry-auth" \
  -ca="$registry_pki_path/ca.crt" \
  -ca-key="$registry_pki_path/ca.key" \
  -config="$registry_pki_path/profiles.json" \
  -profile="client-server" "$registry_pki_path/client-server.json" | /opt/deckhouse/bin/cfssljson -bare "${registry_pki_path}/auth"
mv "${registry_pki_path}/auth.pem" "${registry_pki_path}/auth.crt"
mv "${registry_pki_path}/auth-key.pem" "${registry_pki_path}/auth.key"

# Distribution certs
/opt/deckhouse/bin/cfssl gencert \
  -cn="embedded-registry-distribution" \
  -ca="$registry_pki_path/ca.crt" \
  -ca-key="$registry_pki_path/ca.key" \
  -config="$registry_pki_path/profiles.json" \
  -profile="client-server" "$registry_pki_path/client-server.json" | /opt/deckhouse/bin/cfssljson -bare "${registry_pki_path}/distribution"
mv "${registry_pki_path}/distribution.pem" "${registry_pki_path}/distribution.crt"
mv "${registry_pki_path}/distribution-key.pem" "${registry_pki_path}/distribution.key"

# Auth token certs
/opt/deckhouse/bin/cfssl gencert \
  -cn="embedded-registry-auth-token" \
  -ca="$registry_pki_path/ca.crt" \
  -ca-key="$registry_pki_path/ca.key" \
  -config="$registry_pki_path/profiles.json" \
  -profile="auth-token" "$registry_pki_path/auth-token.json" | /opt/deckhouse/bin/cfssljson -bare "${registry_pki_path}/token"
mv "${registry_pki_path}/token.pem" "${registry_pki_path}/token.crt"
mv "${registry_pki_path}/token-key.pem" "${registry_pki_path}/token.key"

bb-sync-file /etc/kubernetes/system-registry/auth_config/config.yaml - << EOF
server:
  addr: "${discovered_node_ip}:5051"
  certificate: "/system_registry_pki/auth.crt"
  key: "/system_registry_pki/auth.key"
token:
  issuer: "Registry server"
  expiration: 900
  certificate: "/system_registry_pki/token.crt"
  key: "/system_registry_pki/token.key"

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
  addr: ${discovered_node_ip}:5001
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
{{- if eq .registry.embeddedRegistryModuleMode "Proxy" -}}
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
    realm: "https://${discovered_node_ip}:5051/auth"
    service: Deckhouse registry
    issuer: Registry server
    rootcertbundle: /system_registry_pki/token.crt
    autoredirect: false
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
    image: {{ printf "%s%s@%s" $.registry.address $.registry.path (index $.images.systemRegistry "dockerDistribution") }}
    imagePullPolicy: IfNotPresent
    args:
      - serve
      - /config/config.yaml
    {{- if and (.proxy) (eq .registry.embeddedRegistryModuleMode "Proxy") }}
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
    image: {{ printf "%s%s@%s" $.registry.address $.registry.path (index $.images.systemRegistry "dockerAuth") }}
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

/opt/deckhouse/bin/crictl pull {{ printf "%s%s@%s" $.registry.address $.registry.path (index $.images.systemRegistry "dockerDistribution") }}
/opt/deckhouse/bin/crictl pull {{ printf "%s%s@%s" $.registry.address $.registry.path (index $.images.systemRegistry "dockerAuth") }}
/opt/deckhouse/bin/crictl pull {{ printf "%s%s@%s" $.registry.address $.registry.path (index $.images.common "pause") }}

bash "$IGNITER_DIR/stop_system_registry_igniter.sh"


{{- end }}
{{- end }}
