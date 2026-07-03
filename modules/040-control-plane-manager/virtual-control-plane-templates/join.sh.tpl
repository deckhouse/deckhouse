#!/usr/bin/env bash
set -Eeuo pipefail
# VCP node join (Phase 1). Idempotent one-shot. Package install mirrors the parent
# cluster path via registry-packages-proxy; install details are hardened on live nodes.

if [[ $EUID -ne 0 ]]; then echo "run as root" >&2; exit 1; fi
if [[ -f /etc/kubernetes/kubelet.conf ]]; then echo "already joined"; exit 0; fi

BIN_DIR=/opt/deckhouse/bin
mkdir -p "${BIN_DIR}"

# --- preflight ---
swapoff -a || true
modprobe br_netfilter || true
sysctl -w net.ipv4.ip_forward=1 >/dev/null || true
sysctl -w net.bridge.bridge-nf-call-iptables=1 >/dev/null || true

export PACKAGES_PROXY_ADDRESSES="${VCP_RPP_ADDRESSES}"
export PACKAGES_PROXY_BOOTSTRAP_ADDRESSES="${VCP_RPP_BOOTSTRAP_ADDRESSES}"
export PACKAGES_PROXY_TOKEN="${VCP_RPP_TOKEN}"
export PACKAGES_PROXY_BOOTSTRAP_CLUSTER_UUID="${VCP_CLUSTER_UUID}"

# --- fetch rpp-get via embedded minget ---
echo -n "${VCP_MINGET_B64}" | base64 -d > "${BIN_DIR}/minget"
chmod +x "${BIN_DIR}/minget"
first_bootstrap="${PACKAGES_PROXY_BOOTSTRAP_ADDRESSES%% *}"
"${BIN_DIR}/minget" "http://${first_bootstrap}/${VCP_CLUSTER_UUID}/rpp-get?digest=${VCP_RPP_GET_DIGEST}" > "${BIN_DIR}/rpp-get"
chmod +x "${BIN_DIR}/rpp-get"

# --- install core packages ---
"${BIN_DIR}/rpp-get" install \
  "containerd:${VCP_CONTAINERD_DIGEST}" \
  "crictl:${VCP_CRICTL_DIGEST}" \
  "kubelet:${VCP_KUBELET_DIGEST}"

# --- containerd (package ships binaries + config + containerd-deckhouse.service, enabled) ---
systemctl start containerd-deckhouse.service

# --- kubelet config ---
mkdir -p /etc/kubernetes/pki /var/lib/kubelet
echo -n "${VCP_CA_CRT_B64}" | base64 -d > /etc/kubernetes/pki/ca.crt

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
