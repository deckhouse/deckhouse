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

bb-set-proxy

{{- $kubernetes_api_proxy_image := printf "%s@%s" .registry.imagesBase ( index .images.controlPlaneManager "kubernetesApiProxy" ) }}

{{- if or ( eq .cri "Containerd") ( eq .cri "ContainerdV2") }}
  {{- $kubernetes_api_proxy_image = "deckhouse.local/images:kubernetes-api-proxy" }}
{{- end }}

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
    image: {{ $kubernetes_api_proxy_image }}
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
    image: {{ $kubernetes_api_proxy_image }}
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
  priority: 2000001000
  volumes:
  - hostPath:
      path: /etc/kubernetes/kubernetes-api-proxy
      type: DirectoryOrCreate
    name: kubernetes-api-proxy-conf
  - name: tmp
    emptyDir: {}
EOF

bb-unset-proxy
