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

# The node agent: what the container runtime asks about every registry.
#
# A static pod rather than a DaemonSet, because it is on the path of every pull on the
# node — including the pulls that would bring up the controllers a DaemonSet needs. Its
# image is imported locally by step 034, so starting it needs no registry, which is the
# whole point of it.
#
# Three things are set up here, in this order:
#
#   1. its certificate material, generated on the node and never leaving it;
#   2. the layout it routes by until the API server answers for the first time;
#   3. the static pod manifest.
#
# What is deliberately NOT set up here is the runtime's registry configuration. The
# agent owns the runtime's registry drop-in directory and step 030 stands down in its
# favour: two writers in one directory is the confusion this implementation exists to
# remove. Which directory that is comes from the context, so the agent's own idea of it
# and this step's cannot drift apart.

{{- if .registry.agent }}

{{- $agent_image := printf "%s@%s" .registry.imagesBase ( index .images.registry "registryAgent" ) }}
{{- if or ( eq .cri "Containerd") ( eq .cri "ContainerdV2") }}
  {{- $agent_image = "deckhouse.local/images:registry-agent" }}
{{- end }}
{{- /*
  Where the kubelet probes the agent: the address it serves the runtime on, taken apart rather than
  restated, so that a different endpoint moves the probes with the listener instead of leaving them
  knocking on a port nothing answers — which liveness would answer by restarting a healthy agent
  forever.
*/}}
{{- $agent_probe_host := .registry.agent.endpoint | splitList ":" | first }}
{{- $agent_probe_port := .registry.agent.endpoint | splitList ":" | last }}

agent_path="/etc/kubernetes/registry-agent"
pki_path="${agent_path}/pki"
bootstrap_layout="${agent_path}/bootstrap-layout.json"
agent_kubeconfig="${agent_path}/kubeconfig"
cache_path="/var/lib/deckhouse/registry-agent"

# Where the in-cluster cache keeps its blobs on this node. Always this path here: only a node of
# an Immutable group keeps them elsewhere, and such a node runs no bashible steps.
store_path="/opt/deckhouse/registry"

# The node exporter's textfile directory, which it mounts from this host path and reads every
# *.prom file of. The contract is monitoring-kubernetes': its node-exporter DaemonSet mounts
# /var/run/node-exporter-textfile (DirectoryOrCreate) and passes
# --collector.textfile.directory=/host/textfile* (last in this repository at
# modules/340-monitoring-kubernetes/templates/node-exporter/daemonset.yaml before 0a4d0cfea0
# moved the module out).
metrics_textfile_dir="/var/run/node-exporter-textfile"

# The certificate authorities this cluster accepts, staged one file per registry by step 003 for
# whichever implementation is in charge. The agent needs them for the reason the runtime used to:
# every registry now reaches the runtime through the agent's single drop-in, so a ModuleSource with
# its own authority has nobody but the agent left to verify it.
trust_path="/opt/deckhouse/share/ca-certificates"

drop_in_root="$(dirname "$(dirname "{{ .registry.agent.dropInFile }}")")"

mkdir -p "${pki_path}" "${cache_path}" /etc/kubernetes/manifests "${drop_in_root}"
chmod 0700 "${cache_path}"

rpp-get install "cfssl:{{ .images.registrypackages.cfssl171 }}"

# The material the container runtime verifies the agent against.
#
# Generated here, on the node, and self-signed. The only party that ever has to trust it
# is the runtime on this very node, so there is nothing about it to agree with the
# cluster — and that is exactly what lets a node come up with a working agent before it
# can reach the cluster at all. It is also why this is a separate authority from the one
# the registry storage uses: whether a node can pull must not wait on cluster-wide
# certificate material arriving first.
#
# Kept once generated. Reissuing on every bashible run would rewrite the authority the
# runtime is configured to trust, and in each rewrite there is a window where the two
# disagree and nothing on the node can pull.
agent_pki_is_current() {
  local file
  for file in ca.crt agent.crt agent.key; do
    if [[ ! -s "${pki_path}/${file}" ]]; then
      return 1
    fi
  done

  # Renewed ahead of expiry rather than at it. An expired serving certificate is a node
  # that cannot pull anything, and nothing in the failure points at a certificate.
  local not_after expires_at now
  not_after="$(/opt/deckhouse/bin/cfssl certinfo -cert "${pki_path}/agent.crt" 2>/dev/null \
    | /opt/deckhouse/bin/jq -r '.not_after // empty')"
  if [[ -z "${not_after}" ]]; then
    return 1
  fi

  expires_at="$(date -d "${not_after}" +%s 2>/dev/null || echo 0)"
  now="$(date +%s)"
  # Thirty days.
  if (( expires_at - now < 2592000 )); then
    return 1
  fi

  return 0
}

generate_agent_pki() {
  local staging="${agent_path}/pki.new"

  rm -rf "${staging}"
  mkdir -p "${staging}"
  chmod 0700 "${staging}"

  cat > "${staging}/profiles.json" << "PROFILES"
{
    "signing": {
        "default": {
            "expiry": "87600h"
        },
        "profiles": {
            "server": {
                "expiry": "87600h",
                "usages": [
                    "signing",
                    "digital signature",
                    "key encipherment",
                    "server auth"
                ]
            }
        }
    }
}
PROFILES

  # Loopback names only. The agent serves the runtime on this node and nothing else, so
  # naming the node's own address would add a reason to reissue — the address changing —
  # without adding anything that uses it.
  cat > "${staging}/server-csr.json" << "SERVER_CSR"
{
  "CN": "registry-agent",
  "hosts": ["127.0.0.1", "::1", "localhost"],
  "key": {"algo": "rsa", "size": 2048}
}
SERVER_CSR

  cat > "${staging}/ca-csr.json" << "CA_CSR"
{
  "CN": "registry-agent-ca",
  "key": {"algo": "rsa", "size": 2048},
  "ca": {"expiry": "175200h"}
}
CA_CSR

  /opt/deckhouse/bin/cfssl gencert -initca "${staging}/ca-csr.json" \
    | /opt/deckhouse/bin/cfssljson -bare "${staging}/ca"

  /opt/deckhouse/bin/cfssl gencert \
    -ca="${staging}/ca.pem" \
    -ca-key="${staging}/ca-key.pem" \
    -config="${staging}/profiles.json" \
    -profile="server" \
    "${staging}/server-csr.json" | /opt/deckhouse/bin/cfssljson -bare "${staging}/agent"

  # Moved into place only once all three exist. Installed one at a time from the start,
  # a failure half-way would leave the agent holding a key that does not match its
  # certificate — it would refuse to start rather than serve a handshake nobody can
  # complete, but it would refuse for a reason that is hard to see.
  install -m 0644 "${staging}/ca.pem" "${pki_path}/ca.crt"
  install -m 0644 "${staging}/agent.pem" "${pki_path}/agent.crt"
  install -m 0600 "${staging}/agent-key.pem" "${pki_path}/agent.key"

  # The signing key is not kept. Nothing renews from it — a renewal generates a fresh
  # authority, and the agent rewrites the runtime's copy of it on its next pass — so
  # keeping it would be a private key with no reader.
  rm -rf "${staging}"
}

if agent_pki_is_current; then
  bb-log-info "The registry agent certificate material is current"
else
  bb-log-info "Generating the registry agent certificate material"
  generate_agent_pki
fi

# The authority is readable, and that is a requirement rather than a relaxation.
#
# Two things on the node verify the agent, and only one of them is root: the container runtime, which
# is, and registry-packages-proxy, which is not — it runs as an unprivileged user in a pod that mounts
# this directory from the host and reads this file to dial the agent at all.
#
# Without this the directory is 0700 and the file 0600, so that proxy cannot read it, and the failure
# is silent by construction: an unreadable authority reads to it as "there is no agent here", and it
# falls through to fetching from somewhere else entirely.
#
# Outside the branch above, so that it also repairs a node whose material was generated before this
# existed. The private key keeps 0600 — nothing but the agent has any business with it — and so does
# the bootstrap layout below, which carries registry credentials.
chmod 0755 "${agent_path}" "${pki_path}"
chmod 0644 "${pki_path}/ca.crt"

{{- with .registry.agent.layout }}

# The layout the agent routes by until the API server answers for the first time.
#
# A node joining a new cluster has to pull the control plane before there is a control
# plane to ask how, so the answer has to be on the node already. Superseded the moment
# the API server does answer, and not consulted again after that: the agent stores what
# the cluster says and prefers it from then on.
#
# Holds registry credentials, hence 0600 — the same credentials this node's bashible
# configuration already carries.
bb-sync-file "${bootstrap_layout}" - << "BOOTSTRAP_LAYOUT"
{{ . }}
BOOTSTRAP_LAYOUT
{{- else }}

# No bootstrap layout configured. The file is emptied rather than removed: the pod mounts
# it, and the agent reads an empty file as "there is none".
if [[ -s "${bootstrap_layout}" ]]; then
  : > "${bootstrap_layout}"
fi
{{- end }}

if [[ ! -e "${bootstrap_layout}" ]]; then
  install -m 0600 /dev/null "${bootstrap_layout}"
fi
chmod 0600 "${bootstrap_layout}"

# The agent's own kubeconfig, over the certificate the kubelet keeps for itself.
#
# The agent reads this node's layout with the kubelet's identity, so nothing has to be
# distributed to the node for it. Using `/etc/kubernetes/kubelet.conf` directly would mean
# mounting that whole directory — on a master it holds `pki/ca.key` and `admin.conf` — and
# every narrower mount of that one file fails: `FileOrCreate` has the kubelet create an empty
# file, after which step 061 skips the bootstrap kubeconfig; `File` leaves the pod pending
# through the window in which nothing else can pull images; and mounting it once it exists
# answers the question at the one moment the answer is "not yet".
#
# So it is written here, in the directory the agent already has, on every pass. The identity
# is still the kubelet's, by reference to the certificate the kubelet rotates for itself, so
# nothing is copied and nothing goes stale; before the node's TLS bootstrap that certificate
# does not exist and the agent pulls from the bootstrap layout instead.
#
# The authority is embedded rather than referenced, because neither copy is there the whole
# time: step 098 clears `/var/lib/bashible/ca.crt` after bootstrap, and
# `/etc/kubernetes/pki/ca.crt` does not exist yet on the first pass. Either file is the same
# certificate, and the kubeconfig is written only when one of them is readable.
agent_ca=""
if [[ -s /var/lib/bashible/ca.crt ]]; then
  agent_ca="/var/lib/bashible/ca.crt"
elif [[ -s /etc/kubernetes/pki/ca.crt ]]; then
  agent_ca="/etc/kubernetes/pki/ca.crt"
fi

if [[ -z "${agent_ca}" ]]; then
  bb-log-warning "No cluster CA on the node yet; the registry agent runs on the layout on disk"
else
  bb-sync-file "${agent_kubeconfig}" - << EOF
apiVersion: v1
kind: Config
current-context: registry-agent@default
clusters:
- cluster:
    certificate-authority-data: $(base64 -w0 < "${agent_ca}")
    server: https://127.0.0.1:6445
  name: default
contexts:
- context:
    cluster: default
    user: registry-agent
  name: registry-agent@default
users:
- name: registry-agent
  user:
    client-certificate: /var/lib/kubelet/pki/kubelet-client-current.pem
    client-key: /var/lib/kubelet/pki/kubelet-client-current.pem
EOF
  chmod 0600 "${agent_kubeconfig}"
fi

# What the agent is given of the node besides its own directory: the certificate the kubelet
# rotates for itself, and nothing else under /etc.
kubeconfig_mount="$(cat << "MOUNT"
    - mountPath: /var/lib/kubelet/pki
      name: kubelet-pki
      readOnly: true
MOUNT
)"
kubeconfig_volume="$(cat << "VOLUME"
  - hostPath:
      path: /var/lib/kubelet/pki
      type: DirectoryOrCreate
    name: kubelet-pki
VOLUME
)"

# The proxy ClusterConfiguration names, in the agent's own environment.
#
# The agent is the one process on the node that fetches from the upstream registry: the runtime
# asks the agent on the loopback address, and the agent asks the registry. So the runtime's proxy
# — step 003's drop-in for containerd — never reaches the registry at all, and without these the
# images of a cluster behind a proxy are fetched past it. Its HTTP client takes the proxy from the
# environment like any Go client does.
#
# Taken from this step's own context, as step 003 takes the runtime's, and for 003's reason: the
# context is what the configuration checksum is computed from, so a node sent through bashible by a
# proxy change gets the new proxy here — bb-set-proxy's bashbooster is re-rendered later, by helm, and
# its change sends no node through bashible again. NO_PROXY is the context's list: the loopback
# address the agent serves and reads the API server on, the in-cluster registry, the cluster's networks
# and whatever the cluster adds. A changed proxy changes the manifest, and kubelet restarts the agent
# with it.
#
# The values are written into double-quoted YAML scalars: the ClusterConfiguration schema allows
# neither a double quote nor a backslash in any of them.
proxy_env=""
# Nothing inherited: without a proxy in the context the agent gets none, whatever bashible's own
# environment carries.
unset HTTP_PROXY http_proxy HTTPS_PROXY https_proxy NO_PROXY no_proxy
{{- if .proxy }}
  {{- if .proxy.httpProxy }}
export HTTP_PROXY={{ .proxy.httpProxy | replace "'" "'\"'\"'" | squote }}
export http_proxy=${HTTP_PROXY}
  {{- end }}
  {{- if .proxy.httpsProxy }}
export HTTPS_PROXY={{ .proxy.httpsProxy | replace "'" "'\"'\"'" | squote }}
export https_proxy=${HTTPS_PROXY}
  {{- end }}
export NO_PROXY={{ concat (list "127.0.0.1" "169.254.169.254" "registry.d8-system.svc") (.proxy.noProxy | default list) | compact | uniq | join "," | replace "'" "'\"'\"'" | squote }}
export no_proxy=${NO_PROXY}
{{- end }}
for proxy_var in HTTP_PROXY http_proxy HTTPS_PROXY https_proxy NO_PROXY no_proxy; do
  [[ -n "${!proxy_var:-}" ]] || continue
  proxy_env+=${proxy_env:+$'\n'}
  proxy_env+="    - name: ${proxy_var}"$'\n'"      value: \"${!proxy_var}\""
done
bb-unset-proxy

# The manifest carries the agent's image digest as an annotation, so that a new agent actually
# starts.
#
# For containerd the image reference is a fixed local tag — the agent cannot be pulled through the
# agent, so it is imported from a tar as deckhouse.local/images:registry-agent (step 034). That tag
# never changes, so without the annotation neither does this file: a new build reinstalls the
# package, re-imports the tar, and leaves kubelet with a manifest identical to the one it already
# ran, so the OLD container keeps serving. The digest makes the file differ exactly when the image
# does, and bb-sync-file rewriting it is what makes kubelet restart the pod.
#
# Requests, and no limits. Without requests the pod is BestEffort, the smallest CPU weight on the
# node, and under load the agent got next to no CPU while every pull on the node waited on it. A CPU
# limit would throttle the same path from the other side: every byte of every image crosses the
# agent's TLS. Memory stays unlimited until the agent's peak under parallel pulls is measured, and
# system-node-critical already makes it the OOM killer's last choice. The values are AgentCPURequest
# and AgentMemoryRequest in go_lib/registry/const, which a test holds this manifest to.
#
# The store's directory, read-only. The agent measures what the in-cluster cache left on the node
# once the cache is turned off, and reports it as staleStorageDataBytes, which is what
# D8RegistryStaleCacheData fires on. Without the mount it measured an empty directory inside its own
# container and reported zero on every node bashible configures. HostToContainer so that a disk
# mounted on that directory after the agent started is what it measures, rather than the empty
# directory under it. DirectoryOrCreate: a node that never held a store has nothing there, which
# measures as nothing left behind.
#
# The node exporter's textfile directory, writable. The agent is never scraped directly — a
# kube-rbac-proxy beside it would authenticate against the API server it exists to outlive — so it
# keeps its own metrics in a file there, and the node exporter reports them under its own job with
# the node's name. The agent removes the file when it stops, and the branch below removes it when
# the agent is taken off the node.
#
# Probes, on the port the runtime pulls through and over TLS, not on the plain-HTTP metrics
# listener: a probe that passes has completed the same handshake every pull does, with the
# certificate read from disk as it is on every handshake. Loopback, because that is the only
# address the agent listens on, and the kubelet shares the host's network namespace with it.
#
# Liveness restarts an agent that has not answered for a minute. A frozen agent still accepts
# connections into the kernel's backlog but completes no handshake, so containerd waits out its
# handshake timeout on every pull and then pulls without the agent, where platform images cannot be
# pulled; without liveness nothing would restart it, and the node would stay like that. A minute,
# not seconds, because a restart drops every pull in flight. The startup probe holds liveness off
# for up to two minutes, so an agent slow to start on a node that is still booting is not killed for
# it. Readiness is quicker and costs nothing to fail: the pod has no Service, and its Ready
# condition is read only by the D8RegistryAgentNotReady alert. The same probes are in the Engine
# manifest, which a test holds this one to.
#
# This prose lives HERE and not inside the heredoc below, and neither does any backtick: the
# delimiter cannot be quoted — the body needs ${drop_in_root} expanded — so everything inside it is
# subject to command substitution, comments included.
bb-sync-file /etc/kubernetes/manifests/registry-agent.yaml - << EOF
apiVersion: v1
kind: Pod
metadata:
  annotations:
    # The digest of the image this manifest was written for. See the step's own comment above it.
    registry.deckhouse.io/agent-image-digest: "{{ index .images.registry "registryAgent" }}"
  labels:
    component: registry-agent
    tier: control-plane
    # Defined by the registry module, in its templates/v2/agent-security-policy-exception.yaml.
    security.deckhouse.io/security-policy-exception: registry-agent
  name: registry-agent
  namespace: kube-system
spec:
  # Host networking because the runtime is configured to reach the agent at
  # {{ .registry.agent.endpoint }}, and it resolves that address in the host's network
  # namespace. No hostPort: the agent listens on the loopback address of the node
  # itself, which is the only place the runtime looks.
  hostNetwork: true
  # The node's own resolver, never the cluster's. This agent exists so that a node can pull
  # images when the cluster cannot help it, and the names it has to resolve are upstream
  # registries outside the cluster. Cluster DNS would make it depend on the very thing it
  # is there to outlive, and on a node joining the cluster that dependency is circular:
  # kube-dns cannot start until its image is pulled, the image cannot be pulled until this
  # agent resolves the upstream, and the agent cannot resolve it without kube-dns. The node
  # then never becomes Ready and is replaced, forever.
  dnsPolicy: Default
  priorityClassName: system-node-critical
  priority: 2000001000
  # A frozen agent cannot act on SIGTERM, so the kubelet kills it only when the grace period
  # ends: with the default 30 s, liveness gave up on it after a minute and the node waited half a
  # minute more. A running agent exits within a second of SIGTERM, so this is headroom, not a wait.
  terminationGracePeriodSeconds: 15
  containers:
  - name: registry-agent
    image: {{ $agent_image }}
    imagePullPolicy: IfNotPresent
    args:
    - --listen-address={{ .registry.agent.endpoint }}
    - --containerd-registry-dir=${drop_in_root}
    - --pki-dir=${pki_path}
    - --bootstrap-layout=${bootstrap_layout}
    - --kubeconfig=${agent_kubeconfig}
    - --layout-cache=${cache_path}/layout.json
    - --trust-dir=${trust_path}
    - --store-path=${store_path}
    - --metrics-textfile=${metrics_textfile_dir}/registry-agent.prom
    env:
    - name: NODE_NAME
      valueFrom:
        fieldRef:
          fieldPath: spec.nodeName
${proxy_env}
    securityContext:
      allowPrivilegeEscalation: false
      capabilities:
        drop:
        - ALL
      readOnlyRootFilesystem: true
      runAsGroup: 0
      runAsNonRoot: false
      runAsUser: 0
      seccompProfile:
        type: RuntimeDefault
    resources:
      requests:
        cpu: 50m
        memory: 64Mi
    startupProbe:
      httpGet:
        scheme: HTTPS
        host: {{ $agent_probe_host }}
        port: {{ $agent_probe_port }}
        path: /healthz
      periodSeconds: 5
      timeoutSeconds: 5
      failureThreshold: 24
    livenessProbe:
      httpGet:
        scheme: HTTPS
        host: {{ $agent_probe_host }}
        port: {{ $agent_probe_port }}
        path: /healthz
      periodSeconds: 10
      timeoutSeconds: 5
      failureThreshold: 6
    readinessProbe:
      httpGet:
        scheme: HTTPS
        host: {{ $agent_probe_host }}
        port: {{ $agent_probe_port }}
        path: /healthz
      periodSeconds: 5
      timeoutSeconds: 3
      failureThreshold: 3
    volumeMounts:
    # Written by the agent, and by nothing else.
    - mountPath: ${drop_in_root}
      name: containerd-registry-d
    - mountPath: ${agent_path}
      name: agent-material
      readOnly: true
    - mountPath: ${cache_path}
      name: layout-cache
    # Read-only: these are the node's, staged by a bashible step, and the agent only ever
    # reads them.
    - mountPath: ${trust_path}
      name: trusted-authorities
      readOnly: true
    - mountPath: ${store_path}
      name: store-data
      readOnly: true
      mountPropagation: HostToContainer
    - mountPath: ${metrics_textfile_dir}
      name: metrics-textfile
${kubeconfig_mount}
  volumes:
  - hostPath:
      path: ${drop_in_root}
      type: DirectoryOrCreate
    name: containerd-registry-d
  - hostPath:
      path: ${agent_path}
      type: Directory
    name: agent-material
  - hostPath:
      path: ${cache_path}
      type: DirectoryOrCreate
    name: layout-cache
  # DirectoryOrCreate: on a cluster whose module sources all use public authorities the
  # directory holds nothing, and the agent must start anyway.
  - hostPath:
      path: ${trust_path}
      type: DirectoryOrCreate
    name: trusted-authorities
  - hostPath:
      path: ${store_path}
      type: DirectoryOrCreate
    name: store-data
  - hostPath:
      path: ${metrics_textfile_dir}
      type: DirectoryOrCreate
    name: metrics-textfile
${kubeconfig_volume}
EOF

{{- else }}

# The agent does not own the runtime configuration here, so nothing of it should remain.
#
# Its drop-in in the runtime's directory is not removed from this side, and does not have
# to be: step 030 is the writer again now, and it removes every host directory listed in
# `deckhouse_hosts_state.json` that its own configuration does not name. The agent
# records the directory it created in that same file for exactly this reason, so the
# handover works in both directions without either step reaching into the other's
# output.
#
# What is removed here is the pod, the material only the agent reads, and the agent's metrics
# file. The agent removes that file itself when the kubelet stops it, but not when it is killed
# or the node goes down under it, and a file left behind is a set of series the node exporter
# goes on reporting as current. This removal runs before the kubelet has noticed the manifest
# is gone, so a running agent may write the file once more after it; on a clean stop it is the
# agent's own removal that counts, and this one is for a file an agent no longer running left.
rm -f /etc/kubernetes/manifests/registry-agent.yaml
rm -rf /etc/kubernetes/registry-agent
rm -rf /var/lib/deckhouse/registry-agent
rm -f /var/run/node-exporter-textfile/registry-agent.prom

{{- end }}
