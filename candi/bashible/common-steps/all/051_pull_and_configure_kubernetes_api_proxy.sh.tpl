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

bb-set-proxy

{{- define "is_containerd_cri_and_embedded_registry" }}
  {{- if eq $.cri "Containerd" }}
    {{- if and $.registry.registryMode (ne $.registry.registryMode "Direct") }}
      {{- $system_registry_address := $.systemRegistry.registryAddress | default "" }}
      {{- if eq $.registry.address $system_registry_address }}
        {{- printf "%s" "true" -}}
      {{- end }}
    {{- end }}
  {{- end }}
{{- end }}

{{- $target_registry_address := $.registry.address }}

{{- $sandbox_image_path := printf "%s@%s" $.registry.path (index $.images.common "pause") }}
{{- $target_sandbox_image := printf "%s%s" $target_registry_address $sandbox_image_path }}

{{- $kubernetes_api_proxy_image_path := printf "%s@%s" $.registry.path (index $.images.controlPlaneManager "kubernetesApiProxy") }}
{{- $target_kubernetes_api_proxy_image := printf "%s%s" $target_registry_address $kubernetes_api_proxy_image_path }}

{{- if (include "is_containerd_cri_and_embedded_registry" .) }}

# If embedded registry
{{- $source_registry_port := "5001" }}
{{- $source_registry_addresses := $.systemRegistry.addresses | join "," }}
{{- $source_registry_cacert_path := "" }}
{{- if $.registry.ca }}
  {{- $source_registry_cacert_path = "/opt/deckhouse/share/ca-certificates/registry-ca.crt" }}
{{- end }}
{{- $source_registry_user_and_password := "" }}
{{- if $.registry.auth }}
  {{- $source_registry_user_and_password = $.registry.auth | b64dec }}
{{- end }}

_pull_img_from_source_and_re_tag() {
    local source_registry_image=$1
    local target_registry_image=$2

    /opt/deckhouse/bin/ctr \
        --namespace=k8s.io \
        images pull \
        {{- if $source_registry_user_and_password }}
        --user {{ $source_registry_user_and_password | quote }} \
        {{- end }}
        {{- if $source_registry_cacert_path }}
        --tlscacert {{ $source_registry_cacert_path | quote }} \
        {{- end }}
        "$source_registry_image" || return 1
    /opt/deckhouse/bin/ctr --namespace=k8s.io images tag "$source_registry_image" "$target_registry_image" || return 1
    /opt/deckhouse/bin/ctr --namespace=k8s.io images rm "$source_registry_image" || return 1
}

_pull_img_from_several_sources_and_re_tag() {
    local image_path=$1
    local target_registry_address=$2
    local source_registry_addresses=$3
    local target_registry_image="${target_registry_address}${image_path}"

    IFS=',' read -ra source_registry_addresses_list <<< "$source_registry_addresses"
    for source_registry_address in "${source_registry_addresses_list[@]}"; do
        local source_registry_image="${source_registry_address}${image_path}"
        if _pull_img_from_source_and_re_tag "$source_registry_image" "$target_registry_image"; then
            echo "The image '$target_registry_image' was correctly pulled from '$source_registry_image'"
            return 0
        fi
    done
    >&2 echo "Failed to pull image '$target_registry_image' using addresses '$source_registry_addresses'"
    exit 1
}

_get_local_images_list() {
  repo_digests=$(/opt/deckhouse/bin/crictl images -o json | jq -r '.images[].repoDigests[]?')
  echo $repo_digests
}

if [ "$FIRST_BASHIBLE_RUN" == "yes" ]; then
  # Pulling images from the embedded registry during the first node startup.  
  # Images are pulled from the following sources:  
  # - Local embedded registry (localhost)  
  # - Proxy embedded registry  
  # - Addresses of neighboring master nodes  

  discovered_node_ip="$(</var/lib/bashible/discovered-node-ip)"
  list_of_local_imgs=$(_get_local_images_list)
  target_registry_address={{ $target_registry_address | quote }}
  source_registry_addresses="127.0.0.1:{{- $source_registry_port -}},$discovered_node_ip:{{- $source_registry_port -}}"
  {{- if $source_registry_addresses }}
  source_registry_addresses="$source_registry_addresses,{{- $source_registry_addresses -}}"
  {{- end }}

  if ! echo $list_of_local_imgs | grep -q {{ $target_sandbox_image | quote }}; then
    _pull_img_from_several_sources_and_re_tag {{ $sandbox_image_path | quote }} $target_registry_address $source_registry_addresses
  fi

  if ! echo $list_of_local_imgs | grep -q {{ $target_kubernetes_api_proxy_image | quote }}; then
    _pull_img_from_several_sources_and_re_tag {{ $kubernetes_api_proxy_image_path | quote }} $target_registry_address $source_registry_addresses
  fi
else
  # If it's not the first run, information from containerd (CRI) is used for pulling images.
  if ! echo $list_of_local_imgs | grep -q {{ $target_sandbox_image | quote }}; then
    /opt/deckhouse/bin/crictl pull {{ $target_sandbox_image | quote }}
  fi

  if ! echo $list_of_local_imgs | grep -q {{ $target_kubernetes_api_proxy_image | quote }}; then
    /opt/deckhouse/bin/crictl pull {{ $target_kubernetes_api_proxy_image | quote }}
  fi
fi

{{- else }}

# Use cri info for pulling images if crictl exist (for cri NotManaged or Containerd)
if crictl version >/dev/null 2>/dev/null; then
  crictl pull {{ $target_sandbox_image | quote }}
  crictl pull {{ $target_kubernetes_api_proxy_image | quote }}
fi

{{- end }}

mkdir -p /etc/kubernetes/manifests
bb-sync-file /etc/kubernetes/manifests/kubernetes-api-proxy.yaml - << EOF
apiVersion: v1
kind: Pod
metadata:
  labels:
    component: kubernetes-api-proxy
    tier: control-plane
  name: kubernetes-api-proxy
  namespace: kube-system
spec:
  dnsPolicy: ClusterFirstWithHostNet
  hostNetwork: true
  securityContext:
    runAsNonRoot: false
    runAsUser: 0
    runAsGroup: 0
  shareProcessNamespace: true
  containers:
  - name: kubernetes-api-proxy
    image: {{ $target_kubernetes_api_proxy_image }}
    imagePullPolicy: IfNotPresent
    command: ["/opt/nginx-static/sbin/nginx", "-c", "/etc/nginx/config/nginx.conf", "-g", "daemon off;"]
    env:
    - name: PATH
      value: /opt/nginx-static/sbin
    volumeMounts:
    - mountPath: /etc/nginx/config
      name: kubernetes-api-proxy-conf
    - mountPath: /tmp
      name: tmp
  - name: kubernetes-api-proxy-reloader
    image: {{ $target_kubernetes_api_proxy_image }}
    imagePullPolicy: IfNotPresent
    command: ["/kubernetes-api-proxy-reloader"]
    env:
    - name: PATH
      value: /opt/nginx-static/sbin
    volumeMounts:
    - mountPath: /etc/nginx/config
      name: kubernetes-api-proxy-conf
    - mountPath: /tmp
      name: tmp
  priorityClassName: system-node-critical
  volumes:
  - hostPath:
      path: /etc/kubernetes/kubernetes-api-proxy
      type: DirectoryOrCreate
    name: kubernetes-api-proxy-conf
  - name: tmp
    emptyDir: {}
EOF

bb-unset-proxy
