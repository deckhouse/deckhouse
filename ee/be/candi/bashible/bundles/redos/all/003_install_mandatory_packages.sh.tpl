# Copyright 2022 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE.
# policycoreutils-python libseccomp - containerd.io dependencies
SYSTEM_PACKAGES="curl wget virt-what bash-completion lvm2 parted sudo yum-utils nfs-utils tar xz device-mapper-persistent-data net-tools libseccomp checkpolicy"
KUBERNETES_DEPENDENCIES="conntrack-tools ebtables ethtool iproute iptables socat util-linux"
if bb-is-redos-version? 7.3; then
  SYSTEM_PACKAGES="${SYSTEM_PACKAGES} policycoreutils-python"
fi

bb-var BB_YUM_INSTALL_EXTRA_ARGS "--allowerasing"

bb-yum-install ${SYSTEM_PACKAGES} ${KUBERNETES_DEPENDENCIES}

bb-package-install "jq:{{ .images.registrypackages.jq16 }}" "curl:{{ .images.registrypackages.d8Curl821 }}"

bb-yum-remove yum-cron
