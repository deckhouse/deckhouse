# Миграция!!!!! Удалить после выката
rm -f /etc/systemd/system/kubelet.service.d/cim.conf
rm -rf /var/lib/kubelet/manifests

# In case we adopting node bootstrap by kubeadm
rm -f /etc/systemd/system/kubelet.service.d/10-kubeadm.conf
rm -f /var/lib/kubelet/kubeadm-flags.env

# Read previously discovered IP
discovered_node_ip="$(</var/lib/bashible/discovered-node-ip)"

# Generate kubelet unit
cat << EOF > /etc/systemd/system/kubelet.service.d/10-deckhouse.conf
[Service]
ExecStart=
ExecStart=/usr/bin/kubelet \\
{{- if not (eq .nodeGroup.nodeType "Static") }}
    --register-with-taints=node.flant.com/csi-not-bootstrapped=:NoSchedule \\
{{- end }}
    --bootstrap-kubeconfig=/etc/kubernetes/bootstrap-kubelet.conf \\
    --config=/var/lib/kubelet/config.yaml \\
    --cni-bin-dir=/opt/cni/bin/ \\
    --cni-conf-dir=/etc/cni/net.d/ \\
    --kubeconfig=/etc/kubernetes/kubelet.conf \\
    --network-plugin=cni \\
    --address=${discovered_node_ip:-0.0.0.0} \\
{{- if eq .nodeGroup.nodeType "Static" }}
$([ -n "$discovered_node_ip" ] && echo "    --node-ip=${discovered_node_ip} \\")
{{- else }}
    --cloud-provider=external \\
{{- end }}
    --pod-manifest-path=/etc/kubernetes/manifests \\
    --v=2
EOF

systemctl daemon-reload
