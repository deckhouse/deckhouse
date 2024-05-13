# Copyright 2023 Flant JSC
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

{{- if eq .registry.registryMode "Proxy" }}
UPSTREAM_REGISTRY_AUTH="$(base64 -d <<< "{{ .upstreamRegistry.auth | default "" }}")"
if [[ "$UPSTREAM_REGISTRY_AUTH" == *":"* ]]; then
    export UPSTREAM_REGISTRY_LOGIN="$(echo "$UPSTREAM_REGISTRY_AUTH" | cut -d':' -f1)"
    export UPSTREAM_REGISTRY_PASSWORD="$(echo "$UPSTREAM_REGISTRY_AUTH" | cut -d':' -f2)"
else
    export UPSTREAM_REGISTRY_LOGIN=""
    export UPSTREAM_REGISTRY_PASSWORD=""
fi
{{- end -}}

{{- if and .registry.registryMode (ne .registry.registryMode "Direct") }}
# Create a directories for the system registry configuration
mkdir -p /etc/kubernetes/system-registry/{auth_config,seaweedfs_config,distribution_config}

# Create a directories for the system registry data if it does not exist
mkdir -p /opt/deckhouse/system-registry/seaweedfs_data/

# Read previously discovered IP address of the node
discovered_node_ip="$(</var/lib/bashible/discovered-node-ip)"

bb-sync-file /etc/kubernetes/system-registry/auth_config/auth_config.yaml - << EOF
server:
  addr: "${discovered_node_ip}:5051"
  #addr: "0.0.0.0:5051"

token:
  issuer: "Registry server"
  expiration: 900
  certificate: "/config/token.crt"
  key: "/config/token.key"

users:
  # Password is specified as a BCrypt hash. Use htpasswd -nB USERNAME to generate.
  "pusher":
    password: "\$2y\$05\$d9Ko2sN9YKSgeu9oxfPiAeopkPTaD65RWQiZtaZ2.hnNnLyFObRne"  # pusher
  "puller":
    password: "\$2y\$05\$wVbhDuuhL/TAVj4xMt3lbeCAYWxP1JJNZJdDS/Elk7Ohf7yhT5wNq"  # puller

acl:
  - match: { account: "pusher" }
    actions: [ "*" ]
    comment: "Pusher has full access to everything."
  - match: {account: "/.+/"}  # Match all accounts.
    actions: ["pull"]
    comment: "readonly access to all accounts"
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
tls_ca_file="/pki/etcd/ca.crt"
tls_client_crt_file="/pki/apiserver-etcd-client.crt"
tls_client_key_file="/pki/apiserver-etcd-client.key"
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
    regionendpoint: http://localhost:8333
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
  addr: ${discovered_node_ip}:5000
  #addr: 0.0.0.0:5000
  prefix: /
  secret: asecretforlocaldevelopment
  debug:
    addr: localhost:5001
    prometheus:
      enabled: true
      path: /metrics
{{- if eq .registry.registryMode "Proxy" -}}
{{- $scheme := .upstreamRegistry.scheme | trimSuffix "/" | trimPrefix "/" -}}
{{- $address := .upstreamRegistry.address | trimSuffix "/" | trimPrefix "/" }}
proxy:
  remoteurl: "{{ $scheme }}://{{ $address }}"
  username: "$UPSTREAM_REGISTRY_LOGIN"
  password: "$UPSTREAM_REGISTRY_PASSWORD"
  ttl: 72h
  {{- end }}

auth:
  token:
    realm: http://${discovered_node_ip}:5051/auth
    service: Docker registry
    issuer: Registry server
    rootcertbundle: /config/token.crt
    autoredirect: false
EOF

# TEMPORARY: generate self-signed certificates
if [ ! -f "/etc/kubernetes/system-registry/auth_config/token.key" ]; then
openssl genrsa -out /etc/kubernetes/system-registry/auth_config/token.key 4096
fi

if [ ! -f "/etc/kubernetes/system-registry/auth_config/token.crt" ]; then
openssl req -new -x509 -days 365 -key /etc/kubernetes/system-registry/auth_config/token.key -subj "/CN=localhost" -out /etc/kubernetes/system-registry/auth_config/token.crt
fi

bb-sync-file /etc/kubernetes/manifests/system-registry.yaml - << EOF
apiVersion: v1
kind: Pod
metadata:
  labels:
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
    volumeMounts:
      - mountPath: /config/
        name: distribution-config-volume
      - mountPath: /config/token.crt
        name: distribution-auth-token-crt-file
  - name: auth
    image: {{ printf "%s%s@%s" $.registry.address $.registry.path (index $.images.systemRegistry "dockerAuth") }}
    imagePullPolicy: IfNotPresent
    args:
      - -logtostderr
      - /config/auth_config.yaml
    volumeMounts:
      - mountPath: /config/
        name: auth-config-volume
  - name: seaweedfs
    image: {{ printf "%s%s@%s" $.registry.address $.registry.path (index $.images.systemRegistry "seaweedfs") }}
    imagePullPolicy: IfNotPresent
    args:
      - -config_dir="/etc/seaweedfs"
      - -logtostderr=true
      - -v=0
      - server
      - -filer
      - -s3
      - -dir=/seaweedfs_data
      - -volume.max=0
      - -master.volumeSizeLimitMB=1024
      - -metricsPort=9324
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
      - mountPath: /seaweedfs_data
        name: seaweedfs-data-volume
      - mountPath: /etc/seaweedfs
        name: seaweedfs-config-volume
      - mountPath: /pki
        name: kubernetes-pki-volume

  priorityClassName: system-node-critical

  volumes:
  - name: kubernetes-pki-volume
    hostPath:
      path: /etc/kubernetes/pki/
      type: Directory
  - name: auth-config-volume
    hostPath:
      path: /etc/kubernetes/system-registry/auth_config/
      type: DirectoryOrCreate
  - name: distribution-auth-token-crt-file
    hostPath:
      path: /etc/kubernetes/system-registry/auth_config/token.crt
      type: File
  - name: seaweedfs-config-volume
    hostPath:
      path: /etc/kubernetes/system-registry/seaweedfs_config/
      type: DirectoryOrCreate
  - name: distribution-config-volume
    hostPath:
      path: /etc/kubernetes/system-registry/distribution_config/
      type: DirectoryOrCreate
  - name: seaweedfs-data-volume
    hostPath:
      path: /opt/deckhouse/system-registry/seaweedfs_data/
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
{{- end -}}
