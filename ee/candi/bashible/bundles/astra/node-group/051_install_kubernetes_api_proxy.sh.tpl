# Copyright 2021 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

if bb-is-astra-version? 2.12.+ || bb-is-astra-version? 1.7.+ ; then
  bb-rp-install "nginx:{{ .images.registrypackages.nginxDebian1180Stretch }}"
fi
