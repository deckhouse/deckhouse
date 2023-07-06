# Copyright 2023 Flant JSC
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
	type var_lib_t;
	type init_t;
	class file execute;
}

#============= init_t ==============

#!!!! This avc is allowed in the current policy
allow init_t var_lib_t:file execute;
EOF
