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

REGISTRY_CACERT_PATH="/opt/deckhouse/share/ca-certificates/registry-ca.crt"
REGISTRY_MODE="{{ $.registry.registryMode | default ""  }}"
REGISTRY_AUTH="$(base64 -d <<< "{{ $.registry.auth | default "" }}")"
REGISTRY_CA_EXISTS={{ if and $.registry.ca (ne $.registry.ca "") }}true{{ else }}false{{ end }}

mkdir -p /etc/kubernetes/manifests

pull_and_re_tag_image() {
    local PROXY_IMG_ADDRESS=$1
    local ACTUAL_IMAGE_ADDRESS=$2

    if [ "$REGISTRY_CA_EXISTS" = true ]; then
        /opt/deckhouse/bin/ctr --namespace=k8s.io images pull -u "$REGISTRY_AUTH" --tlscacert "$REGISTRY_CACERT_PATH" "$PROXY_IMG_ADDRESS" || return 1
    else
        /opt/deckhouse/bin/ctr --namespace=k8s.io images pull -u "$REGISTRY_AUTH" "$PROXY_IMG_ADDRESS" || return 1
    fi

    /opt/deckhouse/bin/ctr --namespace=k8s.io image tag "$PROXY_IMG_ADDRESS" "$ACTUAL_IMAGE_ADDRESS" || return 1
    /opt/deckhouse/bin/ctr --namespace=k8s.io image rm "$PROXY_IMG_ADDRESS" || return 1
}

pull_using_proxies() {
    local IMAGE_PATH=$1
    local REGISTRY_ACTUAL_ADDRESS=$2
    local REGISTRY_PROXY_ADDRESSES=$3

    local ACTUAL_IMAGE_ADDRESS="${REGISTRY_ACTUAL_ADDRESS}${IMAGE_PATH}"

    IFS=',' read -ra PROXY_ADDR <<< "$REGISTRY_PROXY_ADDRESSES"
    for REGISTRY_PROXY_ADDRESS in "${PROXY_ADDR[@]}"; do
        local PROXY_IMG_ADDRESS="${REGISTRY_PROXY_ADDRESS}${IMAGE_PATH}"

        if pull_and_re_tag_image "$PROXY_IMG_ADDRESS" "$ACTUAL_IMAGE_ADDRESS"; then
            echo "The image '$ACTUAL_IMAGE_ADDRESS' was correctly pulling from '$PROXY_IMG_ADDRESS'"
            return 0
        fi
    done

    >&2 echo "Failed to pull image '$ACTUAL_IMAGE_ADDRESS' using addresses '$REGISTRY_PROXY_ADDRESSES'"
    exit 1
}

if crictl version >/dev/null 2>/dev/null; then
  {{- $registryProxyAddresses := "" }}
  {{- if $.systemRegistry.addresses }}
    {{- $registryProxyAddresses = $.systemRegistry.addresses | join "," }}
  {{- end }}

  # Registry vars
  REGISTRY_ACTUAL_ADDRESS="{{ $.registry.address }}"
  REGISTRY_PROXY_ADDRESSES="{{ $registryProxyAddresses }}"
  EMBEDDED_REGISTRY_ACTUAL_ADDRESS="{{ $.systemRegistry.registryAddress | default "" }}"

  # Images refs
  IMAGE_PATH_FOR_KUBERNETES_API_PROXY={{ printf "%s@%s" $.registry.path (index $.images.controlPlaneManager "kubernetesApiProxy") }}
  IMAGE_PATH_FOR_PAUSE={{ printf "%s@%s" $.registry.path (index $.images.common "pause") }}

  # Bootstrap for registry mode != "Direct" and embedded registry address == current registry address
  if [ "$FIRST_BASHIBLE_RUN" == "yes" ] && [ -n "$REGISTRY_PROXY_ADDRESSES" ] && \
    [ -n "$REGISTRY_MODE" ] && [ "$REGISTRY_MODE" != "Direct" ] && \
    [ "$EMBEDDED_REGISTRY_ACTUAL_ADDRESS" == "$REGISTRY_ACTUAL_ADDRESS" ]; then
    pull_using_proxies "$IMAGE_PATH_FOR_KUBERNETES_API_PROXY" "$REGISTRY_ACTUAL_ADDRESS" "$REGISTRY_PROXY_ADDRESSES"
    pull_using_proxies "$IMAGE_PATH_FOR_PAUSE" "$REGISTRY_ACTUAL_ADDRESS" "$REGISTRY_PROXY_ADDRESSES"
  else
    /opt/deckhouse/bin/crictl pull "${REGISTRY_ACTUAL_ADDRESS}${IMAGE_PATH_FOR_KUBERNETES_API_PROXY}"
    /opt/deckhouse/bin/crictl pull "${REGISTRY_ACTUAL_ADDRESS}${IMAGE_PATH_FOR_PAUSE}"
  fi
fi

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
  shareProcessNamespace: true
  containers:
  - name: kubernetes-api-proxy
    image: {{ printf "%s%s@%s" $.registry.address $.registry.path (index $.images.controlPlaneManager "kubernetesApiProxy") }}
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
    image: {{ printf "%s%s@%s" $.registry.address $.registry.path (index $.images.controlPlaneManager "kubernetesApiProxy") }}
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
