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

if crictl version >/dev/null 2>/dev/null; then
  {{- $registryProxyAddress := "" }}
  {{- if .normal.apiserverEndpoints }}
    {{- range $key, $value := .normal.apiserverEndpoints }}
      {{- if eq $key 0 }}
        {{- $ipAddressAndPort := splitList ":" $value }}
        {{- $ipAddress := index $ipAddressAndPort 0 }}
        {{- $registryProxyAddress = printf "%s:5001" $ipAddress }}
      {{- end }}
    {{- end }}
  {{- end }}

  # Registry vars
  REGISTRY_MODE="{{ $.registry.registryMode }}"
  REGISTRY_PROXY_ADDRESS="{{ $registryProxyAddress }}"

  # Images vars
  PROXY_RETRIEVED_IMAGE_FOR_KUBERNETES_API_PROXY={{ printf "%s%s@%s" $registryProxyAddress $.registry.path (index $.images.controlPlaneManager "kubernetesApiProxy") }}
  PROXY_RETRIEVED_IMAGE_FOR_PAUSE={{ printf "%s%s@%s" $registryProxyAddress $.registry.path (index $.images.common "pause") }}

  ACTUAL_IMAGE_NAME_FOR_KUBERNETES_API_PROXY={{ printf "%s%s@%s" $.registry.address $.registry.path (index $.images.controlPlaneManager "kubernetesApiProxy") }}
  ACTUAL_IMAGE_NAME_FOR_PAUSE={{ printf "%s%s@%s" $.registry.address $.registry.path (index $.images.common "pause") }}

  # Bootstrap for registry mode != "Direct"
  if [ "$FIRST_BASHIBLE_RUN" == "yes" ] && [ -n "${REGISTRY_PROXY_ADDRESS+x}" ] && [ -n "${REGISTRY_MODE+x}" ] && [ "$REGISTRY_MODE" != "Direct" ]; then
    crictl pull $PROXY_RETRIEVED_IMAGE_FOR_KUBERNETES_API_PROXY
    crictl pull $PROXY_RETRIEVED_IMAGE_FOR_PAUSE
    ctr --namespace=k8s.io image tag $PROXY_RETRIEVED_IMAGE_FOR_KUBERNETES_API_PROXY $ACTUAL_IMAGE_NAME_FOR_KUBERNETES_API_PROXY
    ctr --namespace=k8s.io image tag $PROXY_RETRIEVED_IMAGE_FOR_PAUSE $ACTUAL_IMAGE_NAME_FOR_PAUSE
    crictl rmi $PROXY_RETRIEVED_IMAGE_FOR_KUBERNETES_API_PROXY $PROXY_RETRIEVED_IMAGE_FOR_PAUSE
  else
    crictl pull $ACTUAL_IMAGE_NAME_FOR_KUBERNETES_API_PROXY
    crictl pull $ACTUAL_IMAGE_NAME_FOR_PAUSE
  fi
fi
