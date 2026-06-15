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

{{ $img_docker_distribution := printf "%s@%s" .registry.imagesBase (index $.images.registry "dockerDistribution") }}
{{ $img_docker_auth := printf "%s@%s" .registry.imagesBase (index $.images.registry "dockerAuth") }}

check_container_running() {
  local container_name=${1}
  local max_attempts=20
  local sleep_interval=10

  echo "Checking container: ${container_name}"

  for ((i=1; i<=max_attempts; i++)); do
    if [[ ${i} -ne 1 ]]; then
      echo "Attempt: ${i}/${max_attempts}"
      sleep ${sleep_interval}
    fi

    if crictl ps -o json | jq -e --arg name "${container_name}" '.containers[] | select(.metadata.name == $name and .state == "CONTAINER_RUNNING")' > /dev/null 2>&1; then
      echo "${container_name}: running"
      return 0
    fi
  done

  echo "${container_name}: not running (timeout ${sleep_interval}s * ${max_attempts})"
  exit 1
}

# Prepare proxy envs
bb-set-proxy

# Prepare vars
discovered_node_ip="$(bb-d8-node-ip)"

static_pod_tmp_file=$(bb-tmp-file)
static_pod_dest_path="/etc/kubernetes/manifests"
static_pod_dest_file="${static_pod_dest_path}/registry-nodeservices.yaml"

base_path="/etc/kubernetes/registry"
pki_path="${base_path}/pki"
auth_path="${base_path}/auth"
distribution_path="${base_path}/distribution"
data_path="/opt/deckhouse/registry/local_data"

# igniter
igniter_pki_path="${REGISTRY_MODULE_IGNITER_DIR}/pki"
igniter_stop_sh="${REGISTRY_MODULE_IGNITER_DIR}/stop_registry_igniter.sh"

# Create the directories
mkdir -p "${base_path}" \
         "${pki_path}" \
         "${auth_path}" \
         "${distribution_path}" \
         "${data_path}" \
         "${static_pod_dest_path}"

# Prepare certs
cp "${igniter_pki_path}/ca.crt" "${pki_path}/ca.crt"
cp "${igniter_pki_path}/auth.key" "${pki_path}/auth.key"
cp "${igniter_pki_path}/auth.crt" "${pki_path}/auth.crt"
cp "${igniter_pki_path}/distribution.key" "${pki_path}/distribution.key"
cp "${igniter_pki_path}/distribution.crt" "${pki_path}/distribution.crt"
cp "${igniter_pki_path}/token.key" "${pki_path}/token.key"
cp "${igniter_pki_path}/token.crt" "${pki_path}/token.crt"
{{- with ((.registry.bootstrap).proxy).ca }}
cp "${igniter_pki_path}/upstream-registry-ca.crt" "${pki_path}/upstream-registry-ca.crt"
{{- end }}

# Prepare auth manifest
bb-sync-file "${auth_path}/config.yaml" - << EOF
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
bb-sync-file "${distribution_path}/config.yaml" - << EOF
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
bb-sync-file "${static_pod_tmp_file}" - << EOF
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
    image: {{ $img_docker_distribution }}
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
    image: {{ $img_docker_auth }}
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
      path: "${pki_path}"
      type: Directory
  # Configuration
  - name: auth-config
    hostPath:
      path: "${auth_path}"
      type: DirectoryOrCreate
  - name: distribution-config
    hostPath:
      path: "${distribution_path}"
      type: DirectoryOrCreate
  # Data
  - name: data
    hostPath:
      path: "${data_path}"
      type: DirectoryOrCreate
EOF


# Check registry-proxy
check_container_running registry-proxy
check_container_running registry-proxy-reloader

# Prepull static pod images
crictl pull {{ $img_docker_distribution }}
crictl pull {{ $img_docker_auth }}

# Switching igniter to static pod
bash "${igniter_stop_sh}"
mv "${static_pod_tmp_file}" "${static_pod_dest_file}"

# Check static pod
check_container_running auth
check_container_running distribution

# Unset proxy envs
bb-unset-proxy

{{- end }}
