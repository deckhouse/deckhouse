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
mkdir -p /etc/kubernetes/node-proxy

cd /etc/kubernetes/node-proxy


set +e
openssl verify -CAfile /etc/kubernetes/pki/ca.crt /etc/kubernetes/node-proxy/haproxy.pem 2>/dev/null
exit_code=$?
set -e

if [ $exit_code -ne 0 ]; then
  bb-log-error "Node-proxy certificate verification failed, generating a new certificate"
  cp /etc/kubernetes/pki/ca.crt /etc/kubernetes/node-proxy/ca.crt
  openssl genrsa -out key.pem 2048
  openssl req -new -key  key.pem -out key.csr -subj "/CN=health-user/O=health-group"
  openssl x509 -req -in key.csr -CA ca.crt -CAkey /etc/kubernetes/pki/ca.key -CAcreateserial -out cert.pem -days 3650 -sha256
  cat key.pem cert.pem > haproxy.pem
  rm -rf key.csr cert.pem key.pem
  chown deckhouse:deckhouse -R /etc/kubernetes/node-proxy
  chmod 700 /etc/kubernetes/node-proxy
  chmod 600 /etc/kubernetes/node-proxy/*
fi

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
    runAsNonRoot: true
    runAsUser: 64535
    runAsGroup: 64535
  shareProcessNamespace: true
  containers:
  - name: node-proxy
    image: {{ printf "%s%s@%s" $.registry.address $.registry.path (index $.images.controlPlaneManager "nodeProxy") }}
    imagePullPolicy: IfNotPresent
    command: ["/bin/haproxy", "-W", "-db", "-f", "/config/config.cfg"]
    volumeMounts:
      - name: certs
        mountPath: /etc/kubernetes/node-proxy
        readOnly: true
      - name: socket
        mountPath: /socket
  - name: sidecar
    image: {{ printf "%s%s@%s" $.registry.address $.registry.path (index $.images.controlPlaneManager "nodeProxy") }}
    imagePullPolicy: IfNotPresent
    command: ["/bin/node-proxy-sidecar", "--config=/config/discovery.yaml", "--api-host=10.241.44.17:6443"]
    volumeMounts:
      - name: certs
        mountPath: /etc/kubernetes/node-proxy
        readOnly: true
      - name: socket
        mountPath: /socket
  priorityClassName: system-node-critical
  volumes:
    - name: certs
      hostPath:
        path: /etc/kubernetes/node-proxy
        type: Directory
    - name: socket
      emptyDir: {}
EOF

bb-unset-proxy
