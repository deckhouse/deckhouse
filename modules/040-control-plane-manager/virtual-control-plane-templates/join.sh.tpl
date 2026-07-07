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
"${BIN_DIR}/minget" "${VCP_ALB_VIP}:80/${VCP_CLUSTER_UUID}/rpp-get?digest=${VCP_RPP_GET_DIGEST}" > "${BIN_DIR}/rpp-get"
echo "${VCP_RPP_GET_DIGEST#sha256:}  ${BIN_DIR}/rpp-get" | sha256sum -c - \
  || { echo "rpp-get digest mismatch" >&2; exit 1; }
chmod +x "${BIN_DIR}/rpp-get"

# --- install core packages (rpp-get talks HTTPS + bearer to the ALB itself) ---
"${BIN_DIR}/rpp-get" install \
  "containerd:${VCP_CONTAINERD_DIGEST}" \
  "crictl:${VCP_CRICTL_DIGEST}" \
  "kubelet:${VCP_KUBELET_DIGEST}"

# --- containerd (package ships binaries + config + containerd-deckhouse.service, enabled) ---
systemctl start containerd-deckhouse.service

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
  --container-runtime-endpoint=unix:///run/containerd/containerd.sock
EOF

systemctl daemon-reload
systemctl restart kubelet.service
echo "kubelet started; waiting for TLS bootstrap + CSR approval"
