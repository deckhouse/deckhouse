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
-a always,exit -F arch=b64 -F dir=/etc/containerd -F perm=wa -k containerd
-a always,exit -F arch=b32 -F dir=/etc/containerd -F perm=wa -k containerd
-a always,exit -F arch=b64 -F dir=/var/lib/containerd -F perm=wa -k containerd
-a always,exit -F arch=b32 -F dir=/var/lib/containerd -F perm=wa -k containerd
-a always,exit -F arch=b64 -F path=/opt/deckhouse/bin/containerd -F perm=xwa -k containerd
-a always,exit -F arch=b32 -F path=/opt/deckhouse/bin/containerd -F perm=xwa -k containerd
-a always,exit -F arch=b64 -F path=/run/containerd/containerd.sock -F perm=rw  -k containerd
-a always,exit -F arch=b32 -F path=/run/containerd/containerd.sock -F perm=rw  -k containerd
EOF
  bb-sync-file /etc/audit/rules.d/z99-deckhouse.rules - << "EOF"
-b 65536
--backlog_wait_time 0
EOF
fi

{{- end }}

