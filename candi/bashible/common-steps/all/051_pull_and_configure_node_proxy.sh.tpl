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

if crictl version >/dev/null 2>/dev/null; then
  crictl pull {{ printf "%s%s@%s" $.registry.address $.registry.path (index $.images.controlPlaneManager "nodeProxy") }}
fi

bb-sync-file /etc/kubernetes/manifests/node-proxy.yaml - << EOF
apiVersion: v1
kind: Pod
metadata:
  labels:
    component: node-proxy
    tier: control-plane
  name: node-proxy
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
  - name: node-proxy
    image: {{ printf "%s%s@%s" $.registry.address $.registry.path (index $.images.controlPlaneManager "nodeProxy") }}
    imagePullPolicy: IfNotPresent
    command: ["/opt/nginx-static/sbin/nginx", "-c", "/etc/nginx/config/nginx.conf", "-g", "daemon off;"]
    env:
    - name: PATH
      value: /opt/nginx-static/sbin
  priorityClassName: system-node-critical
EOF

bb-unset-proxy
