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
  bb-log-warning "'kubelet-need-restart' flag was set. Kubelet should be restarted!"
  {{ if eq .runType "ClusterBootstrap" }}
  bb-log-info "Restart kubelet service..."

  systemctl restart "kubelet.service"

  bb-log-info "Kubelet service was restarted."
  {{ else }}
  if ! bb-flag? reboot; then
    bb-log-info "Restart kubelet service..."

    systemctl restart "kubelet.service"

    bb-log-info "Kubelet service was restarted. Sleep 60 seconds to prevent oscillation in Cloud LoadBalancer targets."
    # Issue with oscillating cloud LoadBalancer targets is tracked here.
    # https://github.com/kubernetes/kubernetes/issues/102367
    # Remove the sleep once a solution is devised.
    sleep 60
  else
     bb-log-info "Skip restarting kubelet because node will be rebooted."
  fi
  {{- end }}
{{- end }}

  bb-flag-unset kubelet-need-restart
fi

{{- if ne .runType "ImageBuilding" }}
if ! systemctl is-active --quiet "kubelet.service" && ! bb-flag? reboot; then
  bb-log-warning "Kubelet service is not running. Start it..."
  if systemctl start "kubelet.service"; then
    bb-log-info "Kubelet has started."
  else
    bb-log-error "Kubelet has not started. Exit"
    exit 1
  fi
fi
{{- end }}
