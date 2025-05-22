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

{{- $inhibitorPkgName := "d8-shutdown-inhibitor" }}
{{- $inhibitorIndex := "d8ShutdownInhibitor" }}
{{- $inhibitorVersion := "0.1" | replace "." "" }}


old_inhibitor_hash=""
if [ -f "${BB_RP_INSTALLED_PACKAGES_STORE}/{{ $inhibitorPkgName }}/digest" ]; then
  old_inhibitor_hash=$(<"${BB_RP_INSTALLED_PACKAGES_STORE}/{{ $inhibitorPkgName }}/digest")
fi

bb-package-install "{{ $inhibitorPkgName }}:{{ index .images.registrypackages (printf "%s%s" $inhibitorIndex $inhibitorVersion) | toString }}"

new_inhibitor_hash=$(<"${BB_RP_INSTALLED_PACKAGES_STORE}/{{ $inhibitorPkgName }}/digest")
if [[ "${old_inhibitor_hash}" != "${new_inhibitor_hash}" ]]; then
  bb-flag-set inhibitor-need-restart
fi

if bb-flag? inhibitor-need-restart; then
  bb-log-warning "'inhibitor-need-restart' flag was set, restarting {{ $inhibitorPkgName }}."

  systemctl restart "{{ $inhibitorPkgName }}.service"

  bb-flag-unset inhibitor-need-restart
fi

# Step 1/2: enable systemd unit.
if ! systemctl is-enabled "d8-shutdown-inhibitor.service"; then
  bb-log-info "Deckhouse shutdown inhibitor service is disabled. Enable it..."
  if systemctl enable "d8-shutdown-inhibitor.service"; then
    bb-log-info "Deckhouse shutdown inhibitor was enabled."
  else
    systemctl status "d8-shutdown-inhibitor.service"
    bb-log-error "Deckhouse shutdown inhibitor has not been enabled."
  fi
fi

# Inhibitor will start after reboot, no need to start it right now.
if bb-flag? reboot; then
  exit 0
fi

# Step 2/2: start systemd unit if needed.
if systemctl is-active --quiet "d8-shutdown-inhibitor.service"; then
  bb-log-warning "Deckhouse shutdown inhibitor service is already running."
  exit 0
fi

bb-log-warning "Deckhouse shutdown inhibitor service is not running. Starting it..."
if systemctl start "d8-shutdown-inhibitor.service"; then
  bb-log-info "Deckhouse shutdown inhibitor has been started."
else
  systemctl status "d8-shutdown-inhibitor.service"
  bb-log-error "Deckhouse shutdown inhibitor has not been started. Exit"
  exit 1
fi

