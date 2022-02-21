# Copyright 2021 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

SYSTEM_PACKAGES="curl wget inotify-tools bash-completion lvm2 parted apt-transport-https sudo nfs-common vim"
KUBERNETES_DEPENDENCIES="iptables iproute2 socat util-linux mount ebtables ethtool"

bb-apt-install ${SYSTEM_PACKAGES} ${KUBERNETES_DEPENDENCIES}

bb-rp-install "jq:{{ .images.registrypackages.jq16 }}" "curl:{{ .images.registrypackages.d8Curl7800 }}"

if bb-is-astra-version? 2.12.+ || bb-is-astra-version? 1.7.+; then
  bb-rp-install "virt-what:{{ .images.registrypackages.virtWhatAstra1151Deb9u1 }}" "conntrack:{{ .images.registrypackages.conntrackAstra1462 }}"
fi
