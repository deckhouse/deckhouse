# Copyright 2025 Flant JSC
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

bb-log-info "check runType == normal"
{{- if eq .runType "Normal" }}
{{ $kubernetesVersion := .kubernetesVersion | toString }}

bb-log-info "check FIRST_BASHIBLE_RUN == no"
if [ "$FIRST_BASHIBLE_RUN" == "no" ]; then
  currentVersion=$(kubelet --version |egrep -o "1.[0-9]+")
  desiredVersion={{ $kubernetesVersion }}

  if [[ "${desiredVersion}" = "1.31" && -n "$currentVersion" && "$currentVersion" = "1.30" ]]
    then
      bb-deckhouse-get-disruptive-update-approval
  fi
  if [[ "${desiredVersion}" = "1.30" && -n "$currentVersion" && "$currentVersion" = "1.31" ]]
    then
      bb-deckhouse-get-disruptive-update-approval
  fi

  bb-log-info "$(kubectl version)"
  bb-log-info "$(kubelet --version)"

  bb-log-info "check desiredVersion == 1.32"
  # https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG/CHANGELOG-1.32.md#no-really-you-must-read-this-before-you-upgrade
  if [[ "${desiredVersion}" = "1.32" && -n "$currentVersion" && "$currentVersion" = "1.31" ]]; then
    systemctl stop kubelet
    rm -f /var/lib/kubelet/pod_status_manager_state
    bb-log-info "Removed /var/lib/kubelet/pod_status_manager_state"
    systemctl start kubelet
  fi
fi

{{- end }}
