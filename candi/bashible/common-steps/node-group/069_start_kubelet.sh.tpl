{{- /* CSI socket migration. fox MR !2179 */}}
{{- if ne .nodeGroup.nodeType "Static" }}
if [[ -d /var/lib/kubelet/plugins/ebs.csi.aws.com ]]; then
  if [[ -d /var/lib/kubelet/csi-plugins/ebs.csi.aws.com ]]; then
    rm -rf /var/lib/kubelet/plugins/ebs.csi.aws.com
    bb-flag-set kubelet-need-restart
  else
    bb-log-error '"/var/lib/kubelet/csi-plugins/ebs.csi.aws.com" is not created yet'
    exit 1
  fi
fi

if [[ -d /var/lib/kubelet/plugins/pd.csi.storage.gke.io ]]; then
  if [[ -d /var/lib/kubelet/csi-plugins/pd.csi.storage.gke.io ]]; then
    rm -rf /var/lib/kubelet/plugins/pd.csi.storage.gke.io
    bb-flag-set kubelet-need-restart
  else
    bb-log-error '"/var/lib/kubelet/csi-plugins/pd.csi.storage.gke.io" is not created yet'
    exit 1
  fi
fi

if [[ -d /var/lib/kubelet/plugins/cinder.csi.openstack.org ]]; then
  if [[ -d /var/lib/kubelet/csi-plugins/cinder.csi.openstack.org ]]; then
    rm -rf /var/lib/kubelet/plugins/cinder.csi.openstack.org
    bb-flag-set kubelet-need-restart
  else
    bb-log-error '"/var/lib/kubelet/csi-plugins/cinder.csi.openstack.org" is not created yet'
    exit 1
  fi
fi

if [[ -d /var/lib/kubelet/plugins/vsphere.csi.vmware.com ]]; then
  if [[ -d /var/lib/kubelet/csi-plugins/vsphere.csi.vmware.com ]]; then
    rm -rf /var/lib/kubelet/plugins/vsphere.csi.vmware.com
    bb-flag-set kubelet-need-restart
  else
    bb-log-error '"/var/lib/kubelet/csi-plugins/vsphere.csi.vmware.com" is not created yet'
    exit 1
  fi
fi

if [[ -d /var/lib/kubelet/plugins/yandex.csi.flant.com ]]; then
  if [[ -d /var/lib/kubelet/csi-plugins/yandex.csi.flant.com ]]; then
    rm -rf /var/lib/kubelet/plugins/yandex.csi.flant.com
    bb-flag-set kubelet-need-restart
  else
    bb-log-error '"/var/lib/kubelet/csi-plugins/yandex.csi.flant.com" is not created yet'
    exit 1
  fi
fi
{{- end }}

if bb-flag? kubelet-need-restart; then
{{- if ne .runType "ImageBuilding" }}
  {{ if eq .runType "ClusterBootstrap" }}
  systemctl restart "kubelet.service"
  {{ else }}
  if ! bb-flag? reboot; then
    systemctl restart "kubelet.service"
  fi
  {{- end }}
{{- end }}

  bb-flag-unset kubelet-need-restart
fi
