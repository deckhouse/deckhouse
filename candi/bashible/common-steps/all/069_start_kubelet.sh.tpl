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

function wait-kubelet-client-certificate() {
  # Waiting till the file /var/lib/kubelet/pki/kubelet-client-current.pem generated
  {{ if eq .runType "Normal" }}
  attempt=0
  until [ -f /var/lib/kubelet/pki/kubelet-client-current.pem ]; do
    attempt=$(( attempt + 1 ))
    if [ "$attempt" -gt "30" ]; then
      bb-log-error "The file /var/lib/kubelet/pki/kubelet-client-current.pem was not generated in 5 minutes."
      break
    fi
    bb-log-info "Waiting till the file /var/lib/kubelet/pki/kubelet-client-current.pem generated (10sec)..."
    sleep 10
  done
  {{- else }}
  true
  {{- end }}
}

if bb-flag? kubelet-need-restart; then

  bb-log-warning "'kubelet-need-restart' flag was set, restarting kubelet."
  if [ -f /var/lib/kubelet/cpu_manager_state ]; then rm /var/lib/kubelet/cpu_manager_state; fi
  if [ -f /var/lib/kubelet/memory_manager_state ]; then rm /var/lib/kubelet/memory_manager_state; fi
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

  bb-flag-unset kubelet-need-restart
fi

if bb-flag? reboot; then
  exit 0
fi

if systemctl is-active --quiet "kubelet.service"; then
  wait-kubelet-client-certificate
  exit 0
fi

bb-log-warning "Kubelet service is not running. Starting it..."
if systemctl start "kubelet.service"; then
  bb-log-info "Kubelet has started."
  wait-kubelet-client-certificate
else
  bb-log-error "Kubelet has not started. Exit"
  exit 1
fi
