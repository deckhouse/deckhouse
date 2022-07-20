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
  {{ if eq .runType "ClusterBootstrap" }}
  systemctl restart "kubelet.service"
  {{ else }}
  if ! bb-flag? reboot; then
    {{- if eq .cri "Docker" }}
    # This hack need for prevent high load by kubelet to docker when restarts kubelet double time
    if major_docker_version="$(dockerd -v | sed -nr 's/Docker version ([0-9]{2}).+/\1/p')"; then
      if [[ "$major_docker_version" == "18" ]]; then
        sleep_seconds=300
        echo "You're using old docker major version ${major_docker_version}. We should sleep ${sleep_seconds} seconds before kubelet restart"
        sleep "$sleep_seconds"
      fi
    fi
    {{- end }}
    systemctl restart "kubelet.service"
    # Issue with oscillating cloud LoadBalancer targets is tracked here.
    # https://github.com/kubernetes/kubernetes/issues/102367
    # Remove the sleep once a solution is devised.
    sleep 60
  fi
  {{- end }}
{{- end }}

  bb-flag-unset kubelet-need-restart
fi
