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

{{- if eq .cri "Containerd" }}
_on_audit_rules_changed() {
  bb-flag-set auditd-need-restart
}
bb-event-on 'containerd-audit-rules-changed' '_on_audit_rules_changed'

if [ -d /etc/audit/rules.d ]; then
  bb-sync-file /etc/audit/rules.d/containerd-deckhouse.rules - containerd-audit-rules-changed << "EOF"
-w /etc/containerd -p rwxa -k containerd
-w /var/lib/containerd -p rwxa -k containerd
-w /opt/deckhouse/bin/containerd -p rwxa -k containerd
-w /run/containerd/containerd.sock -p rwxa -k containerd
EOF
fi

if bb-flag? auditd-need-restart; then
  bb-log-warning "'auditd-need-restart' flag was set, restarting auditd."
  if systemctl restart auditd; then
    bb-flag-unset auditd-need-restart
  else
    bb-log-error "failed to restart auditd"
    exit 1
  fi
fi
{{- end }}

