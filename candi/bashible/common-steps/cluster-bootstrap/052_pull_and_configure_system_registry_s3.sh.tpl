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
{{- if eq .registry.registryStorageMode "S3" }}

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
registry_pki_path="/etc/kubernetes/system-registry/pki"
internal_registry_domain="{{ .registry.address }}"
if [[ "$internal_registry_domain" == *":"* ]]; then
    internal_registry_domain="$(echo "$internal_registry_domain" | cut -d':' -f1)"
fi


# Create a directories for the system registry configuration
mkdir -p /etc/kubernetes/system-registry/{auth_config,seaweedfs_config,distribution_config,pki}

# Create a directories for the system registry data if it does not exist
mkdir -p /opt/deckhouse/system-registry/seaweedfs_data/

# Prepare certs
bb-sync-file "$registry_pki_path/ca.crt" - << EOF
{{ .registry.internalRegistryAccess.ca.cert }}
EOF

bb-sync-file "$registry_pki_path/ca.key" - << EOF
{{ .registry.internalRegistryAccess.ca.key }}
EOF

{{- if eq .registry.registryMode "Proxy" }}
bb-sync-file "$registry_pki_path/upstream-registry-ca.crt" - << EOF
{{ .registry.upstreamRegistry.ca }}
EOF
{{- end }}

# Auth certs
if [ ! -f "$registry_pki_path/auth.key" ]; then
    /opt/deckhouse/bin/openssl genrsa -out "$registry_pki_path/auth.key" 2048
fi
if [ ! -f "$registry_pki_path/auth.csr" ]; then
    /opt/deckhouse/bin/openssl req -new -key "$registry_pki_path/auth.key" \
    -subj "/ST=./L=./O=./OU=./CN=embedded-registry-auth" \
    -addext "subjectAltName=IP:127.0.0.1,DNS:localhost,IP:${discovered_node_ip},DNS:${internal_registry_domain}" \
    -out "$registry_pki_path/auth.csr"
fi
if [ ! -f "$registry_pki_path/auth.crt" ]; then
    /opt/deckhouse/bin/openssl x509 -req -in "$registry_pki_path/auth.csr" -CA "$registry_pki_path/ca.crt" -CAkey "$registry_pki_path/ca.key" -CAcreateserial \
    -out "$registry_pki_path/auth.crt" -days 3650 -sha256 \
    -extfile <(printf "subjectAltName=IP:127.0.0.1,DNS:localhost,IP:${discovered_node_ip},DNS:${internal_registry_domain}")
fi

# Distribution certs
if [ ! -f "$registry_pki_path/distribution.key" ]; then
    /opt/deckhouse/bin/openssl genrsa -out "$registry_pki_path/distribution.key" 2048
fi
if [ ! -f "$registry_pki_path/distribution.csr" ]; then
    /opt/deckhouse/bin/openssl req -new -key "$registry_pki_path/distribution.key" \
    -subj "/CN=embedded-registry-distribution" \
    -addext "subjectAltName=IP:127.0.0.1,DNS:localhost,IP:${discovered_node_ip},DNS:${internal_registry_domain}" \
    -out "$registry_pki_path/distribution.csr"
fi
if [ ! -f "$registry_pki_path/distribution.crt" ]; then
    /opt/deckhouse/bin/openssl x509 -req -in "$registry_pki_path/distribution.csr" -CA "$registry_pki_path/ca.crt" -CAkey "$registry_pki_path/ca.key" -CAcreateserial \
    -out "$registry_pki_path/distribution.crt" -days 3650 -sha256 \
    -extfile <(printf "subjectAltName=IP:127.0.0.1,DNS:localhost,IP:${discovered_node_ip},DNS:${internal_registry_domain}")
fi

# Auth token certs
if [ ! -f "$registry_pki_path/token.key" ]; then
    /opt/deckhouse/bin/openssl genrsa -out "$registry_pki_path/token.key" 2048
fi
if [ ! -f "$registry_pki_path/token.csr" ]; then
    /opt/deckhouse/bin/openssl req -new -key "$registry_pki_path/token.key" \
    -subj "/CN=embedded-registry-auth-token" \
    -out "$registry_pki_path/token.csr"
fi
if [ ! -f "$registry_pki_path/token.crt" ]; then
    /opt/deckhouse/bin/openssl x509 -req -in "$registry_pki_path/token.csr" -CA "$registry_pki_path/ca.crt" -CAkey "$registry_pki_path/ca.key" -CAcreateserial \
    -out "$registry_pki_path/token.crt" -days 3650 -sha256
fi

# Seaweedfs certs
if [ ! -f "$registry_pki_path/seaweedfs.key" ]; then
    /opt/deckhouse/bin/openssl genrsa -out "$registry_pki_path/seaweedfs.key" 2048
fi
if [ ! -f "$registry_pki_path/seaweedfs.csr" ]; then
    /opt/deckhouse/bin/openssl req -new -key "$registry_pki_path/seaweedfs.key" \
    -subj "/CN=embedded-registry-seaweedfs" \
    -addext "subjectAltName=IP:127.0.0.1,DNS:localhost,IP:${discovered_node_ip}" \
    -out "$registry_pki_path/seaweedfs.csr"
fi
if [ ! -f "$registry_pki_path/seaweedfs.crt" ]; then
    /opt/deckhouse/bin/openssl x509 -req -in "$registry_pki_path/seaweedfs.csr" -CA "$registry_pki_path/ca.crt" -CAkey "$registry_pki_path/ca.key" -CAcreateserial \
    -out "$registry_pki_path/seaweedfs.crt" -days 3650 -sha256 \
    -extfile <(printf "subjectAltName=IP:127.0.0.1,DNS:localhost,IP:${discovered_node_ip}")
fi

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

bb-sync-file /etc/kubernetes/system-registry/seaweedfs_config/filer.toml - << EOF
[filer.options]
recursive_delete = false # do we really need for registry?

[etcd]
enabled = true
{{- if eq .runType "Normal" }}
servers = "{{- range $key, $value := .normal.apiserverEndpoints }}{{ $parts := splitList ":" $value }}{{ $ip := index $parts 0 }}{{ $ip }}:2379;{{- end }}"
{{- else if eq .runType "ClusterBootstrap" }}
servers = "${discovered_node_ip}:2379"
{{- end }}

key_prefix = "seaweedfs_meta."
tls_ca_file= "/kubernetes_pki/etcd/ca.crt"
tls_client_crt_file="/kubernetes_pki/apiserver-etcd-client.crt"
tls_client_key_file="/kubernetes_pki/apiserver-etcd-client.key"
EOF

bb-sync-file /etc/kubernetes/system-registry/seaweedfs_config/master.toml - << EOF
[master.volume_growth]
copy_1 = 1
copy_2 = 2
copy_3 = 3
copy_other = 1
EOF

bb-sync-file /etc/kubernetes/system-registry/distribution_config/config.yaml - << EOF
version: 0.1
log:
  level: info

storage:
  s3:
    accesskey: awsaccesskey
    secretkey: awssecretkey
    region: us-west-1
    regionendpoint: http://127.0.0.1:8333
    bucket: registry
    encrypt: false
    secure: false
    v4auth: true
    chunksize: 5242880
    rootdirectory: /
    multipartcopy:
      maxconcurrency: 100
      chunksize: 33554432
      thresholdsize: 33554432
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
    realm: "https://${discovered_node_ip}:5051/auth"
    service: Docker registry
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
  dnsPolicy: ClusterFirst
  hostNetwork: true
  containers:
  - name: distribution
    image: {{ printf "%s%s@%s" $.registry.address $.registry.path (index $.images.systemRegistry "dockerDistribution") }}
    imagePullPolicy: IfNotPresent
    args:
      - serve
      - /config/config.yaml
    {{- if and (.proxy) (eq .registry.registryMode "Proxy") }}
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
  - name: seaweedfs
    image: {{ printf "%s%s@%s" $.registry.address $.registry.path (index $.images.systemRegistry "seaweedfs") }}
    imagePullPolicy: IfNotPresent
    args:
      - -config_dir=/config
      - -logtostderr=true
      - -v=0
      - server
      - -filer
      - -s3
      - -dir=/data
      - -volume.port=8081
      - -volume.max=0
      - -master.volumeSizeLimitMB=1024
      - -master.raftHashicorp
      - -metricsPort=9324
      - -metricsIp=127.0.0.1
      - -volume.readMode=redirect
      - -s3.allowDeleteBucketNotEmpty=true
      - -master.defaultReplication=000
      - -volume.pprof
      - -filer.maxMB=16
      - -ip=${discovered_node_ip}
      - -master.peers=${discovered_node_ip}:9333
    env:
      - name: GOGC
        value: "20"
      - name: GOMEMLIMIT
        value: "500MiB"
    volumeMounts:
      - mountPath: /data
        name: seaweedfs-data-volume
      - mountPath: /config
        name: seaweedfs-config-volume
      - mountPath: /kubernetes_pki
        name: kubernetes-pki-volume
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
  - name: seaweedfs-config-volume
    hostPath:
      path: /etc/kubernetes/system-registry/seaweedfs_config
      type: DirectoryOrCreate
  - name: distribution-config-volume
    hostPath:
      path: /etc/kubernetes/system-registry/distribution_config
      type: DirectoryOrCreate
  - name: seaweedfs-data-volume
    hostPath:
      path: /opt/deckhouse/system-registry/seaweedfs_data
      type: DirectoryOrCreate
  - name: tmp
    emptyDir: {}
EOF

/opt/deckhouse/bin/crictl pull {{ printf "%s%s@%s" $.registry.address $.registry.path (index $.images.systemRegistry "dockerDistribution") }}
/opt/deckhouse/bin/crictl pull {{ printf "%s%s@%s" $.registry.address $.registry.path (index $.images.systemRegistry "dockerAuth") }}
/opt/deckhouse/bin/crictl pull {{ printf "%s%s@%s" $.registry.address $.registry.path (index $.images.systemRegistry "seaweedfs") }}
/opt/deckhouse/bin/crictl pull {{ printf "%s%s@%s" $.registry.address $.registry.path (index $.images.controlPlaneManager "etcd") }}
/opt/deckhouse/bin/crictl pull {{ printf "%s%s@%s" $.registry.address $.registry.path (index $.images.common "pause") }}

bash "$IGNITER_DIR/stop_system_registry_igniter.sh"

{{- end }}
{{- end }}
