# Copyright 2022 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE.

SYSTEM_PACKAGES="curl wget bash-completion lvm2 parted apt-transport-https sudo nfs-common vim virt-what"
KUBERNETES_DEPENDENCIES="iptables iproute2 socat util-linux mount ebtables ethtool conntrack"

bb-apt-install ${SYSTEM_PACKAGES} ${KUBERNETES_DEPENDENCIES}

bb-package-install "jq:{{ .images.registrypackages.jq16 }}" "curl:{{ .images.registrypackages.d8Curl821 }}"
