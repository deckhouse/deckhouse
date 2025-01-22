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


function get_node_proxy_cert() {

  # kubectl exist
  if type kubectl >/dev/null 2>&1 && test -f /etc/kubernetes/kubelet.conf; then
    bb-log-info "Trying to get node-proxy certificates using kubectl"
    
    if ! secret_json=$(kubectl get secret -n kube-system node-proxy -o json 2>/dev/null); then
      bb-log-error "Failed to get secret using kubectl"
      return 1
    fi
    #  haproxy.pem
    if ! echo "$secret_json" | jq -r '.data["haproxy.pem"]' | base64 -d > haproxy.pem; then
      bb-log-error "Failed to extract haproxy.pem from kubectl secret"
      return 1
    fi
    #  ca.crt
    if ! echo "$secret_json" | jq -r '.data["ca.crt"]' | base64 -d > ca.crt; then
      bb-log-error "Failed to extract ca.crt from kubectl secret"
      return 1
    fi

    bb-log-info "Successfully retrieved certificates using kubectl"
    return 0
  fi
  # kubectl does not exist
  for server in {{ .normal.apiserverEndpoints | join " " }}; do
    bb-log-info "Trying to get certificates from $server"
    
    if ! response=$(d8-curl -sS --fail -X GET "https://$server/api/v1/namespaces/kube-system/secrets/node-proxy" \
      -H "Authorization: Bearer $(</var/lib/bashible/bootstrap-token)" \
      --cacert "$BOOTSTRAP_DIR/ca.crt"); then
      bb-log-error "Request to $server failed"
      continue
    fi

    #  haproxy.pem
    if ! echo "$response" | jq -r '.data["haproxy.pem"]' | base64 -d > haproxy.pem; then
      bb-log-error "Failed to extract haproxy.pem from $server response"
      continue
    fi

    # ca.crt
    if ! echo "$response" | jq -r '.data["ca.crt"]' | base64 -d > ca.crt; then
      bb-log-error "Failed to extract ca.crt from $server response"
      continue
    fi

    bb-log-info "Successfully retrieved certificates from $server"
    return 0
  done

  bb-log-error "All attempts to get certificates failed"
  return 1
}

# Master nodes: generate certificate locally
if [ -f /etc/kubernetes/pki/ca.key ]; then
  if ! openssl verify -CAfile /etc/kubernetes/pki/ca.crt haproxy.pem >/dev/null 2>&1; then
    bb-log-info "Generating new node-proxy certificate"
    cp /etc/kubernetes/pki/ca.crt ca.crt
    openssl genrsa -out key.pem 2048
    openssl req -new -key key.pem -out key.csr -subj "/CN=health-user/O=health-group"
    openssl x509 -req -in key.csr -CA ca.crt -CAkey /etc/kubernetes/pki/ca.key -CAcreateserial -out cert.pem -days 3650 -sha256
    cat key.pem cert.pem > haproxy.pem
    rm -f key.csr cert.pem key.pem
  fi

# Worker nodes: get certificate from secret
else
  set +e
  openssl verify -CAfile /etc/kubernetes/pki/ca.crt /etc/kubernetes/node-proxy/haproxy.pem 2>/dev/null
  exit_code=$?
  set -e

  if [ ! -f /etc/kubernetes/node-proxy/haproxy.pem ] || [ $exit_code -ne 0 ]; then
    bb-log-error "Node-proxy certificate verification failed, fetching new certificate"
    rm -f /etc/kubernetes/node-proxy/haproxy.pem
    get_node_proxy_cert || exit 1
  fi
fi

# Set permissions
chown deckhouse:deckhouse -R /etc/kubernetes/node-proxy
chmod 700 /etc/kubernetes/node-proxy
chmod 600 /etc/kubernetes/node-proxy/*

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
