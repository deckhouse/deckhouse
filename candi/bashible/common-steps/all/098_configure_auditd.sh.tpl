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

{{- if or ( eq .cri "Containerd") ( eq .cri "ContainerdV2") }}

bb-event-on 'bb-sync-file-changed' '_on_audit_rules_changed'
_on_audit_rules_changed() {
  if augenrules --check; then
    augenrules --load
    bb-flag-unset auditd-need-reload
  else
    bb-log-error "failed to reload auditd rules"
    exit 1
  fi
}

if [ -d /etc/audit/rules.d ]; then
  bb-sync-file /etc/audit/rules.d/containerd-deckhouse.rules - << "EOF"

# exclude containerd internal operations such as image unpacking and others. A large number of operations can create a heavy load on auditd.
-a never,exit -F dir=/var/lib/containerd -F exe=/opt/deckhouse/bin/containerd
# exclude runc internal operations such as image unpacking and others. A large number of operations can create a heavy load on auditd.
-a never,exit -F dir=/var/lib/containerd -F exe=/opt/deckhouse/bin/runc
# watch containerd config changes
-w /etc/containerd/config.toml -p wa -k containerd
# record containerd binary exec
-a always,exit -F arch=b64 -S execve -F path=/opt/deckhouse/bin/containerd -k containerd
# detect data modifications
-a always,exit -F arch=b64 -F dir=/var/lib/containerd -F perm=w -k containerd
# watch containerd binary modifications
-w /opt/deckhouse/bin/containerd -p wa -k containerd
# watch runc binary modifications
-w /opt/deckhouse/bin/runc -p wa -k containerd

EOF
  bb-sync-file /etc/audit/rules.d/z99-deckhouse.rules - << "EOF"
-b 65536
--backlog_wait_time 0
EOF
fi
if command -v auditctl &>/dev/null; then
  auditctl --backlog_wait_time 0 -b 65536 || bb-log-warning "failed to configure auditctl backlog; continuing with defaults"
else
  bb-log-info "auditctl is not installed; skipping backlog configuration"
fi

{{- end }}
