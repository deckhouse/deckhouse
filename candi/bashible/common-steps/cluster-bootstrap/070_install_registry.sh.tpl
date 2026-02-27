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

{{ $imgDockerDistribution := printf "%s@%s" .registry.imagesBase (index $.images.registry "dockerDistribution") }}
{{ $imgDockerAuth := printf "%s@%s" .registry.imagesBase (index $.images.registry "dockerAuth") }}

# Prepare proxy envs
bb-set-proxy

# Prepare vars
discovered_node_ip="$(bb-d8-node-ip)"
static_pod_path=$(bb-tmp-file)
pki_path="/etc/kubernetes/registry/pki"

# Create the directories
mkdir -p /etc/kubernetes/registry/{auth,distribution,pki} \
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
bb-sync-file "/etc/kubernetes/registry/auth/config.yaml" - << EOF
server:
  addr: "127.0.0.1:5051"
  real_ip_header: "X-Forwarded-For"
  certificate: "/pki/auth.crt"
  key: "/pki/auth.key"
token:
  issuer: "Registry server"
  expiration: 900
  certificate: "/pki/token.crt"
  key: "/pki/token.key"

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
bb-sync-file "/etc/kubernetes/registry/distribution/config.yaml" - << EOF
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
    certificate: "/pki/distribution.crt"
    key: "/pki/distribution.key"

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
  ca: "/pki/upstream-registry-ca.crt"
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
    rootcertbundle: "/pki/token.crt"
    autoredirect: true
    proxy:
      url: https://127.0.0.1:5051/auth
      ca: "/pki/ca.crt"
EOF

# Prepare static pod manifest
bb-sync-file "${static_pod_path}" - << EOF
apiVersion: v1
kind: Pod
metadata:
  labels:
    app.kubernetes.io/managed-by: registry-nodeservices
    heritage: deckhouse
    module: registry
    app: registry
    component: registry-service
    tier: control-plane
    type: node-services
  annotations:
    registry.deckhouse.io/config-hash: {{ .registry.bootstrap | toYaml | sha256sum }}
  name: registry-nodeservices
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
{{- with .registry.bootstrap.proxy }}
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
      - name: distribution
        containerPort: 5001
        hostPort: 5001
      - name: debug
        containerPort: 5002
    livenessProbe:
      httpGet:
        path: /
        port: distribution
        scheme: HTTPS
        {{- /*
          # use default host == PodIP && HostIP, because hostNetwork
        */}}
    readinessProbe:
      httpGet:
        path: /
        port: distribution
        scheme: HTTPS
        {{- /*
          # use default host == PodIP && HostIP, because hostNetwork
        */}}
    volumeMounts:
      - mountPath: /data
        name: data
      - mountPath: /config
        name: distribution-config
      - mountPath: /pki
        name: pki
  - name: auth
    image: {{ $imgDockerAuth }}
    imagePullPolicy: IfNotPresent
    ports:
      - name: auth
        containerPort: 5051
    livenessProbe:
      httpGet:
        path: /
        port: auth
        scheme: HTTPS
        host: 127.0.0.1
        {{- /*
          # can use host: 127.0.0.1, because hostNetwork
          # https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/#http-probes
        */}}
    readinessProbe:
      httpGet:
        path: /
        port: auth
        scheme: HTTPS
        host: 127.0.0.1
        {{- /*
          # can use host: 127.0.0.1, because hostNetwork
          # https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/#http-probes
        */}}
    args:
      - -logtostderr
      - /config/config.yaml
    volumeMounts:
      - mountPath: /config
        name: auth-config
      - mountPath: /pki
        name: pki
  priorityClassName: system-node-critical
  volumes:
  # PKI
  - name: pki
    hostPath:
      path: /etc/kubernetes/registry/pki
      type: Directory
  # Configuration
  - name: auth-config
    hostPath:
      path: /etc/kubernetes/registry/auth
      type: DirectoryOrCreate
  - name: distribution-config
    hostPath:
      path: /etc/kubernetes/registry/distribution
      type: DirectoryOrCreate
  # Data
  - name: data
    hostPath:
      path: /opt/deckhouse/registry/local_data
      type: DirectoryOrCreate
EOF

# Prepull static pod images
/opt/deckhouse/bin/crictl pull {{ $imgDockerDistribution }}
/opt/deckhouse/bin/crictl pull {{ $imgDockerAuth }}


# Switching registry from igniter to static pod
bash "${REGISTRY_MODULE_IGNITER_DIR}/stop_registry_igniter.sh"
mv "${static_pod_path}" /etc/kubernetes/manifests/registry-nodeservices.yaml

# Unset proxy envs
bb-unset-proxy

{{- end }}
