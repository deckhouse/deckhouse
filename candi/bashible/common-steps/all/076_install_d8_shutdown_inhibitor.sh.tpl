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

{{- $moduleKey := "nodeManager" }}
{{- $inhibitorImageKey := "d8ShutdownInhibitor" }}
{{- $inhibitorPkgName := "d8-shutdown-inhibitor" }}

{{- $imagePresent := true }}
{{- $inhibitorDigest := dig $moduleKey $inhibitorImageKey "<missing>" .images }}
{{- $inhibitorPackage := "" }}
{{- if ne $inhibitorDigest "<missing>" }}
{{    $inhibitorPackage = printf "%s:%s" $inhibitorPkgName $inhibitorDigest }}
{{- else }}
{{-   $imagePresent = false }}
{{- end }}

bb-log-warning "package = {{ $inhibitorPackage }}, present = {{ $imagePresent }}"

inhibitor_service_name="d8-shutdown-inhibitor.service"
extra_logind_conf="/etc/systemd/logind.conf.d/99-node-d8-shutdown-inhibitor.conf"

bb-event-on 'restart-inhibitor-if-needed' '_restart_inhibitor_if_needed'
_restart_inhibitor_if_needed() {
  # Machine reboot is scheduled, no need to restart service right now.
  if bb-flag? reboot; then
    exit 0
  fi
  if bb-flag? inhibitor-need-restart; then
    bb-log-warning "'inhibitor-need-restart' flag was set, restarting {{ $inhibitorPkgName }}."
    systemctl restart "${inhibitor_service_name}"
    bb-flag-unset inhibitor-need-restart
  fi
}

bb-event-on 'd8-shutdown-inhibitor-cleanup' '_shutdown-inhibitor-cleanup'
function _shutdown-inhibitor-cleanup() {
  rm -rf "${extra_logind_conf}"
  # Send SIGHUP to logind to reload its configuration.
  systemctl -s SIGHUP kill systemd-logind
}


function inhibitor::install() {
  digest_path="${BB_RP_INSTALLED_PACKAGES_STORE}/{{ $inhibitorPkgName }}/digest"

  old_inhibitor_hash=""
  if [ -f "${digest_path}" ]; then
    old_inhibitor_hash=$(<"${digest_path}")
  fi

  bb-package-install "{{ $inhibitorPackage }}"

  new_inhibitor_hash=$(<"${digest_path}")
  if [[ "${old_inhibitor_hash}" != "${new_inhibitor_hash}" ]]; then
    bb-flag-set inhibitor-need-restart
  fi
}

function inhibitor::uninstall() {
  bb-package-remove "{{ $inhibitorPkgName }}"
}


function inhibitor::enable() {
  if systemctl is-enabled "${inhibitor_service_name}"; then
    # Already enabled, do nothing.
    return 0
  fi

  bb-log-info "Deckhouse shutdown inhibitor service is disabled. Enable it..."
  if systemctl enable "${inhibitor_service_name}"; then
    bb-log-info "Deckhouse shutdown inhibitor was enabled."
  else
    systemctl status "${inhibitor_service_name}"
    bb-log-error "Deckhouse shutdown inhibitor has not been enabled."
  fi
}

function inhibitor::disable() {
  if ! systemctl is-enabled "${inhibitor_service_name}"; then
    # Already disabled, do nothing.
    return 0
  fi

  bb-log-info "Deckhouse shutdown inhibitor service is enabled. Disable it..."
  if systemctl disable "${inhibitor_service_name}"; then
    bb-log-info "Deckhouse shutdown inhibitor was disabled."
  else
    systemctl status "${inhibitor_service_name}"
    bb-log-error "Deckhouse shutdown inhibitor has not been disabled."
  fi
}

function inhibitor::start() {
  # Inhibitor will start after reboot, no need to start it right now.
  if bb-flag? reboot; then
    exit 0
  fi
  bb-event-fire 'restart-inhibitor-if-needed'  

  # Do nothing if already started.
  if systemctl is-active --quiet "${inhibitor_service_name}"; then
    bb-log-warning "Deckhouse shutdown inhibitor service is already running."
    exit 0
  fi

  bb-log-warning "Deckhouse shutdown inhibitor service is not running. Starting it..."
  if systemctl start "${inhibitor_service_name}"; then
    bb-log-info "Deckhouse shutdown inhibitor has been started."
  else
    systemctl status "${inhibitor_service_name}"
    bb-log-error "Deckhouse shutdown inhibitor has not been started. Exit"
    exit 1
  fi
}

function inhibitor::stop() {
  # Do nothing if already stopped.
  if ! systemctl is-active --quiet "${inhibitor_service_name}"; then
    bb-log-warning "Deckhouse shutdown inhibitor service is already stopped."
    # Cleanup logind configuration if not done previously.
    bb-event-fire 'd8-shutdown-inhibitor-cleanup'
    return
  fi

  bb-log-warning "Deckhouse shutdown inhibitor service is running. Stop it..."
  if systemctl stop "${inhibitor_service_name}"; then
    bb-log-info "Deckhouse shutdown inhibitor has been stopped."
  else
    systemctl status "${inhibitor_service_name}"
    bb-log-error "Deckhouse shutdown inhibitor has not been stopped. Exit"
    return
  fi

  # Cleanup logind configuration.
  bb-event-fire 'd8-shutdown-inhibitor-cleanup'
}


{{- if $imagePresent }}
  inhibitor::install
  inhibitor::enable
  inhibitor::start
{{- else }}
  inhibitor::disable
  inhibitor::stop
  inhibitor::uninstall
{{- end }}
