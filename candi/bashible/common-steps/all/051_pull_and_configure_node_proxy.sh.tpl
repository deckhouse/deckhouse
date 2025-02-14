# Copyright 2025 Flant JSC
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

function wait_for_certificate() {
  local csr_name="$1"
  local key_file="$2"
  local timeout=300
  local interval=5
  local elapsed=0
  local cert=""
  while true; do
    cert=$(kubectl get csr "${csr_name}" -o jsonpath='{.status.certificate}' 2>/dev/null || true)
    if [ -n "$cert" ]; then
      echo "$cert" | base64 -d > cert.pem
      break
    fi
    if [ "$elapsed" -ge "$timeout" ]; then
      bb-log-error "Timeout waiting for certificate for CSR ${csr_name}"
      return 1
    fi
    sleep "$interval"
    elapsed=$((elapsed + interval))
  done
  cat "$key_file" cert.pem > haproxy.pem
  rm -f key.pem cert.pem health-user.csr
}

function get_node_proxy_cert() {
  openssl genrsa -out key.pem 2048
  openssl req -new -key key.pem -subj "/CN=health-user/O=health-group" -out health-user.csr
  CSR_BASE64=$(base64 -w 0 health-user.csr)
  csr_name="health-user-csr-$(hostname -s)"
  cat <<EOF | kubectl apply -f -
apiVersion: certificates.k8s.io/v1
kind: CertificateSigningRequest
metadata:
  name: ${csr_name}
spec:
  request: ${CSR_BASE64}
  signerName: kubernetes.io/kube-apiserver-client
  usages:
  - client auth
EOF
  wait_for_certificate "${csr_name}" "key.pem"
}

if [ -f /etc/kubernetes/pki/ca.key ]; then
  if ! openssl verify -CAfile /etc/kubernetes/pki/ca.crt haproxy.pem >/dev/null 2>&1; then
    cp /etc/kubernetes/pki/ca.crt ca.crt
    openssl genrsa -out key.pem 2048
    openssl req -new -key key.pem -out key.csr -subj "/CN=health-user/O=health-group"
    openssl x509 -req -in key.csr -CA ca.crt -CAkey /etc/kubernetes/pki/ca.key -CAcreateserial -out cert.pem -days 3650 -sha256
    cat key.pem cert.pem > haproxy.pem
    rm -f key.csr cert.pem key.pem ca.crt
  fi
else
  set +e
  openssl verify -CAfile /etc/kubernetes/pki/ca.crt /etc/kubernetes/node-proxy/haproxy.pem 2>/dev/null
  exit_code=$?
  set -e
  if [ ! -f /etc/kubernetes/node-proxy/haproxy.pem ] || [ $exit_code -ne 0 ]; then
    rm -f /etc/kubernetes/node-proxy/haproxy.pem
    get_node_proxy_cert || exit 1
  fi
fi

chown deckhouse:deckhouse -R /etc/kubernetes/node-proxy
chmod 700 /etc/kubernetes/node-proxy
chmod 600 /etc/kubernetes/node-proxy/*

bb-set-proxy

if crictl version >/dev/null 2>&1; then
  crictl pull {{ printf "%s%s@%s" $.registry.address $.registry.path (index $.images.controlPlaneManager "nodeProxy") }}
fi

bb-sync-file /etc/kubernetes/manifests/node-proxy.yaml - <<EOF
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
    command: ["/bin/haproxy", "-W", "-db", "-f", "/etc/kubernetes/node-proxy/config.cfg"]
    volumeMounts:
      - name: certs
        mountPath: /etc/kubernetes/node-proxy
        readOnly: true
      - name: socket
        mountPath: /socket
  - name: sidecar
    image: {{ printf "%s%s@%s" $.registry.address $.registry.path (index $.images.controlPlaneManager "nodeProxy") }}
    imagePullPolicy: IfNotPresent
    command:
      - "/bin/node-proxy-sidecar"
      - "--config=/config/discovery.yaml"
      - {{ printf "--api-host=%s" (join "," .normal.apiserverEndpoints) }}
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

bb-sync-file /etc/kubernetes/node-proxy/rpp_backend_backend.cfg - <<'EOF'
{{- if .packagesProxy }}
{{- range $index, $addr := .packagesProxy.addresses }}
server srv{{ add $index 1 }} {{ $addr }}
{{- end }}
{{- end }}
EOF

bb-sync-file /etc/kubernetes/node-proxy/kube_api_backend.cfg - <<'EOF'
{{- range $index, $addr := .normal.apiserverEndpoints }}
server srv{{ add $index 1 }} {{ $addr }}
{{- end }}
EOF

grep -q ":6445" /etc/kubernetes/admin.conf && sed -i 's/6445/3994/' /etc/kubernetes/admin.conf
