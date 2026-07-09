#!/usr/bin/env bash
set -Eeuo pipefail

if [[ $EUID -ne 0 ]]; then echo "run as root" >&2; exit 1; fi
if [[ -f /etc/kubernetes/kubelet.conf ]]; then echo "already joined"; exit 0; fi

BIN_DIR=/opt/deckhouse/bin
mkdir -p "${BIN_DIR}"

# --- preflight ---
swapoff -a || true
modprobe br_netfilter || true
sysctl -w net.ipv4.ip_forward=1 >/dev/null || true
sysctl -w net.bridge.bridge-nf-call-iptables=1 >/dev/null || true

# --- resolve VCP endpoints to the ALB VIP (ALB routes them by SNI) ---
if ! grep -q "${VCP_API_HOST}" /etc/hosts; then
  echo "${VCP_ALB_VIP} ${VCP_API_HOST} ${VCP_KONN_HOST} ${VCP_PKG_HOST}" >> /etc/hosts
fi

# --- CA of the tenant cluster (used by kubelet) ---
mkdir -p /etc/kubernetes/pki
echo -n "${VCP_CA_CRT_B64}" | base64 -d > /etc/kubernetes/pki/ca.crt

# --- packages: rpp-get itself forces HTTPS, so package installs go to the token-gated ALB :443 ---
export PACKAGES_PROXY_ADDRESSES="${VCP_PKG_HOST}:443"
export PACKAGES_PROXY_TOKEN="${VCP_RPP_TOKEN}"

# --- bootstrap rpp-get via embedded minget over the ALB plaintext port (tokenless) ---
echo -n "${VCP_MINGET_B64}" | base64 -d > "${BIN_DIR}/minget"
[[ -s "${BIN_DIR}/minget" ]] || { echo "embedded minget is empty (missing candi/bashible/bootstrap/minget)" >&2; exit 1; }
chmod +x "${BIN_DIR}/minget"
RPP_GET_DIGEST="${VCP_RPP_GET_DIGEST}"
RPP_GET_TMP="${BIN_DIR}/rpp-get.tmp"
"${BIN_DIR}/minget" "${VCP_ALB_VIP}:80/${VCP_CLUSTER_UUID}/rpp-get?digest=${RPP_GET_DIGEST}" > "${RPP_GET_TMP}"
chmod +x "${RPP_GET_TMP}"
"${RPP_GET_TMP}" version >/dev/null \
  || { echo "downloaded rpp-get is not executable" >&2; exit 1; }
mv -f "${RPP_GET_TMP}" "${BIN_DIR}/rpp-get"

# --- install core packages (rpp-get talks HTTPS + bearer to the ALB itself) ---
"${BIN_DIR}/rpp-get" install \
  "containerd:${VCP_CONTAINERD_DIGEST}" \
  "crictl:${VCP_CRICTL_DIGEST}" \
  "kubelet:${VCP_KUBELET_DIGEST}" \
  "pause:${VCP_PAUSE_DIGEST}" \
  "kubernetes-api-proxy:${VCP_KUBERNETES_API_PROXY_DIGEST}" \
  "registry-proxy:${VCP_REGISTRY_PROXY_DIGEST}"

# The containerd package ships a minimal config. Bashible normally expands it and
# switches CRI sandbox image to the locally imported pause image; do the same here.
if [[ -f /etc/containerd/config.toml ]] && ! grep -q 'deckhouse.local/images:pause' /etc/containerd/config.toml; then
  if grep -q '^version = 3' /etc/containerd/config.toml; then
    cat >> /etc/containerd/config.toml <<'EOF'

  [plugins.'io.containerd.cri.v1.images']
    [plugins.'io.containerd.cri.v1.images'.pinned_images]
      sandbox = "deckhouse.local/images:pause"
EOF
  else
    sed -i '/enable_selinux =/a\    sandbox_image = "deckhouse.local/images:pause"' /etc/containerd/config.toml
  fi
fi

# --- containerd (service imports local pause/kubernetes-api-proxy/registry-proxy images on start) ---
systemctl start containerd-deckhouse.service

# --- local apiserver proxy expected by Deckhouse, cni-cilium, kubelet and node-manager ---
mkdir -p /etc/kubernetes/kubernetes-api-proxy /etc/kubernetes/manifests
cat > /etc/kubernetes/kubernetes-api-proxy/upstreams.json <<EOF
["${VCP_API_PROXY_UPSTREAM}"]
EOF
cp /etc/kubernetes/pki/ca.crt /etc/kubernetes/kubernetes-api-proxy/ca.crt
touch /etc/kubernetes/kubernetes-api-proxy/cl.crt /etc/kubernetes/kubernetes-api-proxy/cl.key
chown -R 0:64535 /etc/kubernetes/kubernetes-api-proxy
chmod 750 /etc/kubernetes/kubernetes-api-proxy
chmod 640 /etc/kubernetes/kubernetes-api-proxy/*

cat > /etc/kubernetes/manifests/kubernetes-api-proxy.yaml <<'EOF'
apiVersion: v1
kind: Pod
metadata:
  labels:
    component: kubernetes-api-proxy
    tier: control-plane
  name: kubernetes-api-proxy
  namespace: kube-system
spec:
  priorityClassName: system-node-critical
  hostNetwork: true
  dnsPolicy: ClusterFirstWithHostNet
  securityContext:
    fsGroup: 64535
  volumes:
  - name: certs
    hostPath:
      path: /etc/kubernetes/kubernetes-api-proxy
      type: Directory
  - name: upstreams
    hostPath:
      path: /etc/kubernetes/kubernetes-api-proxy/upstreams.json
      type: FileOrCreate
  containers:
  - name: kubernetes-api-proxy
    image: deckhouse.local/images:kubernetes-api-proxy
    imagePullPolicy: IfNotPresent
    args:
    - --listen-address=127.0.0.1
    - --listen-port=6445
    - --health-listen=127.0.0.1:6480
    - --log-level=debug
    - --as-static-pod=true
    - --fallback-upstreams=${VCP_API_PROXY_UPSTREAM}
    - --fallback-file=/var/run/kubernetes.io/kubernetes-api-proxy/upstreams.json
    ports:
    - name: https
      containerPort: 6445
      hostPort: 6445
      protocol: TCP
    - name: health
      containerPort: 6480
      protocol: TCP
    readinessProbe:
      httpGet:
        path: /readyz
        port: health
        host: 127.0.0.1
      initialDelaySeconds: 2
      periodSeconds: 5
    livenessProbe:
      httpGet:
        path: /healthz
        port: health
        host: 127.0.0.1
      initialDelaySeconds: 2
      periodSeconds: 10
    resources:
      requests:
        cpu: 50m
        memory: 64Mi
      limits:
        cpu: 500m
        memory: 256Mi
    securityContext:
      allowPrivilegeEscalation: false
      capabilities:
        drop:
        - ALL
        add:
        - DAC_OVERRIDE
        - SETGID
        - SETUID
      readOnlyRootFilesystem: true
      runAsGroup: 0
      runAsNonRoot: false
      runAsUser: 0
      seccompProfile:
        type: RuntimeDefault
    volumeMounts:
    - name: certs
      mountPath: /var/run/kubernetes.io/kubernetes-api-proxy
      readOnly: true
    - name: upstreams
      mountPath: /var/run/kubernetes.io/kubernetes-api-proxy/upstreams.json
      readOnly: false
EOF

# --- kubelet config (ca.crt already written above) ---
mkdir -p /var/lib/kubelet

cat > /var/lib/kubelet/config.yaml <<EOF
apiVersion: kubelet.config.k8s.io/v1beta1
kind: KubeletConfiguration
cgroupDriver: systemd
clusterDomain: ${VCP_CLUSTER_DOMAIN}
clusterDNS:
- ${VCP_CLUSTER_DNS}
rotateCertificates: true
authentication:
  x509:
    clientCAFile: /etc/kubernetes/pki/ca.crt
  anonymous:
    enabled: false
authorization:
  mode: Webhook
EOF

cat > /etc/kubernetes/bootstrap-kubelet.conf <<'EOF'
${VCP_BOOTSTRAP_KUBECONFIG}
EOF
sed -i 's#server: .*#server: https://127.0.0.1:6445/#' /etc/kubernetes/bootstrap-kubelet.conf
chmod 0600 /etc/kubernetes/bootstrap-kubelet.conf

# --- kubelet flags via drop-in (base kubelet.service ships with the package) ---
mkdir -p /etc/systemd/system/kubelet.service.d
cat > /etc/systemd/system/kubelet.service.d/10-vcp.conf <<EOF
[Service]
ExecStart=
ExecStart=${BIN_DIR}/kubelet \\
  --bootstrap-kubeconfig=/etc/kubernetes/bootstrap-kubelet.conf \\
  --kubeconfig=/etc/kubernetes/kubelet.conf \\
  --config=/var/lib/kubelet/config.yaml \\
  --pod-manifest-path=/etc/kubernetes/manifests \\
  --container-runtime-endpoint=unix:///run/containerd/containerd.sock
EOF

systemctl daemon-reload
systemctl restart kubelet.service
echo "kubelet started; waiting for TLS bootstrap + CSR approval"
