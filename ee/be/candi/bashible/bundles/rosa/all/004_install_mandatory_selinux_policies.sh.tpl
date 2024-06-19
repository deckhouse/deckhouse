# Copyright 2024 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE.

if [[ "$(getenforce)" != "Enforcing" ]]; then
  exit 0
fi

bb-event-on 'selinux_deckhouse_policy_changed' '_on_selinux_deckhouse_policy_changed'
_on_selinux_deckhouse_policy_changed() {
  checkmodule -M -m -o /var/lib/bashible/policies/deckhouse.mod /var/lib/bashible/policies/deckhouse.te
  semodule_package -o /var/lib/bashible/policies/deckhouse.pp -m /var/lib/bashible/policies/deckhouse.mod
  semodule -i /var/lib/bashible/policies/deckhouse.pp
}

mkdir -p /var/lib/bashible/policies
bb-sync-file /var/lib/bashible/policies/deckhouse.te - selinux_deckhouse_policy_changed << "EOF"
module deckhouse 1.1;

require {
  type unlabeled_t;
  type init_t;
  type var_lib_t;
  type load_policy_t;
  type var_lock_t;
  type setfiles_t;
  class file { getattr open read write execute_no_trans execute };
}

#============= init_t ==============
allow init_t unlabeled_t:file write;
allow init_t var_lib_t:file { execute execute_no_trans };

#============= load_policy_t ==============
allow load_policy_t var_lib_t:file read;
allow load_policy_t var_lock_t:file write;

#============= setfiles_t ==============
allow setfiles_t var_lib_t:file read;
EOF
