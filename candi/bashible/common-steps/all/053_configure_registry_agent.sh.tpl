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

agent_path="/etc/kubernetes/registry-agent"
pki_path="${agent_path}/pki"
bootstrap_layout="${agent_path}/bootstrap-layout.json"
cache_path="/var/lib/deckhouse/registry-agent"

drop_in_root="$(dirname "$(dirname "{{ .registry.agent.dropInFile }}")")"

mkdir -p "${pki_path}" "${cache_path}" /etc/kubernetes/manifests "${drop_in_root}"
chmod 0700 "${cache_path}"

bb-package-install "cfssl:{{ .images.registrypackages.cfssl165 }}"

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

# The kubelet's own credentials are how the agent reads this node's layout, so that
# nothing has to be distributed to the node for it. They are mounted only once they
# exist, and the two obvious alternatives are both wrong:
#
#   - FileOrCreate would have the kubelet create an empty /etc/kubernetes/kubelet.conf,
#     and step 061 skips generating the bootstrap kubeconfig when that file is present —
#     so the node would never complete its TLS bootstrap;
#   - File would leave the pod pending until the file appears, which is precisely the
#     window in which a node being bootstrapped has nothing else able to pull images.
#
# So the agent starts without API access and routes from the layout the node was
# installed with. The mount appears on a later bashible pass, the kubelet restarts the
# pod, and from then on the cluster's layout wins.
kubeconfig_mount=""
kubeconfig_volume=""
if [[ -f /etc/kubernetes/kubelet.conf ]]; then
  kubeconfig_mount="$(cat << "MOUNT"
    - mountPath: /etc/kubernetes/kubelet.conf
      name: kubeconfig
      readOnly: true
    - mountPath: /var/lib/kubelet/pki
      name: kubelet-pki
      readOnly: true
MOUNT
)"
  kubeconfig_volume="$(cat << "VOLUME"
  - hostPath:
      path: /etc/kubernetes/kubelet.conf
      type: File
    name: kubeconfig
  - hostPath:
      path: /var/lib/kubelet/pki
      type: DirectoryOrCreate
    name: kubelet-pki
VOLUME
)"
fi

bb-sync-file /etc/kubernetes/manifests/registry-agent.yaml - << EOF
apiVersion: v1
kind: Pod
metadata:
  labels:
    component: registry-agent
    tier: control-plane
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
  containers:
  - name: registry-agent
    image: {{ $agent_image }}
    imagePullPolicy: IfNotPresent
    args:
    - --listen-address={{ .registry.agent.endpoint }}
    - --containerd-registry-dir=${drop_in_root}
    - --pki-dir=${pki_path}
    - --bootstrap-layout=${bootstrap_layout}
    - --layout-cache=${cache_path}/layout.json
    env:
    - name: NODE_NAME
      valueFrom:
        fieldRef:
          fieldPath: spec.nodeName
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
    volumeMounts:
    # Written by the agent, and by nothing else.
    - mountPath: ${drop_in_root}
      name: containerd-registry-d
    - mountPath: ${pki_path}
      name: pki
      readOnly: true
    - mountPath: ${bootstrap_layout}
      name: bootstrap-layout
      readOnly: true
    - mountPath: ${cache_path}
      name: layout-cache
${kubeconfig_mount}
  volumes:
  - hostPath:
      path: ${drop_in_root}
      type: DirectoryOrCreate
    name: containerd-registry-d
  - hostPath:
      path: ${pki_path}
      type: Directory
    name: pki
  - hostPath:
      path: ${bootstrap_layout}
      type: File
    name: bootstrap-layout
  - hostPath:
      path: ${cache_path}
      type: DirectoryOrCreate
    name: layout-cache
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
# What is removed here is the pod and the material only the agent reads.
rm -f /etc/kubernetes/manifests/registry-agent.yaml
rm -rf /etc/kubernetes/registry-agent
rm -rf /var/lib/deckhouse/registry-agent

{{- end }}
