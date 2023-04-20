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

bb-event-on 'selinux_deckhouse_policy_changed' '_on_selinux_deckhouse_policy_changed'
_on_selinux_deckhouse_policy_changed() {
  checkmodule -M -m -o /var/lib/bashible/policies/deckhouse.mod /var/lib/bashible/policies/deckhouse.te
  semodule_package -o /var/lib/bashible/policies/deckhouse.pp -m /var/lib/bashible/policies/deckhouse.mod
  semodule -i /var/lib/bashible/policies/deckhouse.pp
}

mkdir -p /var/lib/bashible/policies
bb-sync-file /var/lib/bashible/policies/deckhouse.te - selinux_deckhouse_policy_changed << "EOF"
module deckhouse 1.0;

require {
  type unlabeled_t;
  type httpd_t;
  type http_port_t;
  type init_t;
  type var_lib_t;
  type sge_port_t;
  type load_policy_t;
  type var_lock_t;
  type setfiles_t;
  type unreserved_port_t;
  class tcp_socket name_connect;
  class capability sys_resource;
  class process setrlimit;
  class file { getattr open read write execute_no_trans execute };
  class tcp_socket name_bind;
}

#============= httpd_t ==============

#!!!! This avc can be allowed using one of the these booleans:
#     httpd_run_stickshift, httpd_setrlimit
allow httpd_t self:capability sys_resource;

#!!!! This avc can be allowed using the boolean 'httpd_setrlimit'
allow httpd_t self:process setrlimit;
allow httpd_t sge_port_t:tcp_socket name_bind;
allow httpd_t unreserved_port_t:tcp_socket name_connect;
allow httpd_t unlabeled_t:file getattr;
allow httpd_t http_port_t:tcp_socket name_connect;

#!!!! This avc is allowed in the current policy
allow httpd_t unlabeled_t:file { open read };

#============= init_t ==============
allow init_t unlabeled_t:file write;
allow init_t var_lib_t:file { execute execute_no_trans };

#============= load_policy_t ==============
allow load_policy_t var_lib_t:file read;
allow load_policy_t var_lock_t:file write;

#============= setfiles_t ==============
allow setfiles_t var_lib_t:file read;
EOF

{{- if eq .cri "Containerd" }}
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
