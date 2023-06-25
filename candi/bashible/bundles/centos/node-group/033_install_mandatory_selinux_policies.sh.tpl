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

{{- if eq .cri "Containerd" }}
mkdir -p /var/lib/bashible/policies

bb-event-on 'selinux_cilium_policy_changed' '_on_selinux_cilium_policy_changed'
_on_selinux_cilium_policy_changed() {
  checkmodule -M -m -o /var/lib/bashible/policies/cilium.mod /var/lib/bashible/policies/cilium.te
  semodule_package -o /var/lib/bashible/policies/cilium.pp -m /var/lib/bashible/policies/cilium.mod
  semodule -i /var/lib/bashible/policies/cilium.pp
}

if crictl ps | grep -q "cilium-agent"; then
  bb-sync-file /var/lib/bashible/policies/cilium.te - selinux_cilium_policy_changed << "EOF"
module cilium 1.0;

require {
  type init_t;
  type spc_t;
  type container_runtime_t;
  class bpf prog_run;
}

#============= spc_t ==============
allow spc_t container_runtime_t:bpf prog_run;
allow spc_t init_t:bpf prog_run;
EOF
fi
{{- end }}
