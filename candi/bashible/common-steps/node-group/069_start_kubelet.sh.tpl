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

if bb-flag? kubelet-need-restart; then

{{- if ne .runType "ImageBuilding" }}
  bb-log-warning "'kubelet-need-restart' flag was set, restarting kubelet."
  systemctl restart "kubelet.service"
  {{ if ne .runType "ClusterBootstrap" }}
  if [[ "${FIRST_BASHIBLE_RUN}" != "yes" ]] && ! bb-flag? reboot; then
    bb-log-info "Kubelet service was restarted. Sleep 60 seconds to prevent oscillation in Cloud LoadBalancer targets."
    # Issue with oscillating cloud LoadBalancer targets is tracked here.
    # https://github.com/kubernetes/kubernetes/issues/102367
    # Remove the sleep once a solution is devised.
    sleep 60
  fi
  {{- end }}
{{- end }}

  bb-flag-unset kubelet-need-restart
fi

{{- if ne .runType "ImageBuilding" }}
if bb-flag? reboot; then
  exit 0
fi

if systemctl is-active --quiet "kubelet.service"; then
  exit 0
fi

bb-log-warning "Kubelet service is not running. Starting it..."
if systemctl start "kubelet.service"; then
  bb-log-info "Kubelet has started."
else
  bb-log-error "Kubelet has not started. Exit"
  exit 1
fi
{{- end }}
