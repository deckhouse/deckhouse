# Миграция!!!!! Удалить после выката
rm -f /etc/systemd/system/kubelet.service.d/cim.conf
rm -rf /var/lib/kubelet/manifests

# In case we adopting node bootstrapped by kubeadm
rm -f /etc/systemd/system/kubelet.service.d/10-kubeadm.conf
rm -f /var/lib/kubelet/kubeadm-flags.env

# Read previously discovered IP
discovered_node_ip="$(</var/lib/bashible/discovered-node-ip)"

bb-event-on 'bb-sync-file-changed' '_enable_kubelet_service'
function _enable_kubelet_service() {
{{- if ne .runType "ImageBuilding" }}
  systemctl daemon-reload
{{- end }}
  systemctl enable kubelet.service
  bb-flag-set kubelet-need-restart
}

# Generate kubelet unit
bb-sync-file /etc/systemd/system/kubelet.service.d/10-deckhouse.conf - << EOF
[Service]
ExecStart=
ExecStart=/usr/bin/kubelet \\
{{- if not (eq .nodeGroup.nodeType "Static") }}
    --register-with-taints=node.deckhouse.io/uninitialized=:NoSchedule,node.deckhouse.io/csi-not-bootstrapped=:NoSchedule \\
{{- else }}
    --register-with-taints=node.deckhouse.io/uninitialized=:NoSchedule \\
{{- end }}
    --node-labels=node.deckhouse.io/group={{ .nodeGroup.name }} \\
    --node-labels=node.deckhouse.io/type={{ .nodeGroup.nodeType }} \\
    --bootstrap-kubeconfig=/etc/kubernetes/bootstrap-kubelet.conf \\
    --config=/var/lib/kubelet/config.yaml \\
    --cni-bin-dir=/opt/cni/bin/ \\
    --cni-conf-dir=/etc/cni/net.d/ \\
    --kubeconfig=/etc/kubernetes/kubelet.conf \\
    --network-plugin=cni \\
    --address=${discovered_node_ip:-0.0.0.0} \\
{{- if eq .nodeGroup.nodeType "Static" -}}
$([ -n "$discovered_node_ip" ] && echo "    --node-ip=${discovered_node_ip} \\")
{{- else }}
    --cloud-provider=external \\
{{- end }}
    --pod-manifest-path=/etc/kubernetes/manifests \\
{{- if hasKey .nodeGroup "kubelet" }}
    --root-dir={{ .nodeGroup.kubelet.rootDir | default "/var/lib/kubelet" }} \\
{{- end }}
    --v=2
EOF
