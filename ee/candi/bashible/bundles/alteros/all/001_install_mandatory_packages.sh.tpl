# Copyright 2022 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE.
# policycoreutils-python libseccomp - containerd.io dependencies
SYSTEM_PACKAGES="curl wget virt-what bash-completion lvm2 parted sudo yum-utils nfs-utils tar xz device-mapper-persistent-data net-tools libseccomp checkpolicy policycoreutils-python"
KUBERNETES_DEPENDENCIES="conntrack ebtables ethtool iproute iptables socat util-linux"

bb-yum-install yum-plugin-versionlock

bb-yum-install ${SYSTEM_PACKAGES} ${KUBERNETES_DEPENDENCIES}

bb-rp-install "jq:{{ .images.registrypackages.jq16 }}" "curl:{{ .images.registrypackages.d8Curl7800 }}"

bb-yum-remove yum-cron
