# Copyright 2021 Flant JSC
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

mkdir -p /etc/kubernetes/kubernetes-api-proxy
# Read previously discovered IP
discovered_node_ip="$(</var/lib/bashible/discovered-node-ip)"

bb-sync-file /etc/kubernetes/kubernetes-api-proxy/nginx_new.conf - << EOF
user nginx;

pid /tmp/kubernetes-api-proxy.pid;
error_log stderr notice;

worker_processes 2;
worker_rlimit_nofile 130048;
worker_shutdown_timeout 10s;

events {
  multi_accept on;
  use epoll;
  worker_connections 16384;
}

stream {
  upstream kubernetes {
    least_conn;
{{- if eq .runType "Normal" }}
  {{- range $key,$value := .normal.apiserverEndpoints }}
    server {{ $value }};
  {{- end }}
{{- else if eq .runType "ClusterBootstrap" }}
    server ${discovered_node_ip}:6443;
{{- end }}
  }
  server {
    listen 127.0.0.1:6445;
    proxy_pass kubernetes;
    # Configurator uses 24h proxy_timeout in case of long running jobs like kubectl exec or kubectl logs
    # After time out, nginx will force a client to reconnect
    proxy_timeout 24h;
    proxy_connect_timeout 1s;
  }
}
EOF

if [[ ! -f /etc/kubernetes/kubernetes-api-proxy/nginx.conf ]]; then
  cp /etc/kubernetes/kubernetes-api-proxy/nginx_new.conf /etc/kubernetes/kubernetes-api-proxy/nginx.conf
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
    volumeMounts:
    - mountPath: /etc/nginx
      name: kubernetes-api-proxy-conf
  - name: kubernetes-api-proxy-reloader
    image: {{ printf "%s%s@%s" $.registry.address $.registry.path (index $.images.controlPlaneManager "kubernetesApiProxy") }}
    imagePullPolicy: IfNotPresent
    command: ["/kubernetes-api-proxy-reloader"]
    volumeMounts:
    - mountPath: /etc/nginx
      name: kubernetes-api-proxy-conf
  priorityClassName: system-node-critical
  volumes:
  - hostPath:
      path: /etc/kubernetes/kubernetes-api-proxy
      type: DirectoryOrCreate
    name: kubernetes-api-proxy-conf
EOF
