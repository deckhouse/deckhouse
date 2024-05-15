# Copyright 2022 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE.

bb-package-install "jq:{{ .images.registrypackages.jq16 }}" "curl:{{ .images.registrypackages.d8Curl821 }}" "virt-what:{{ .images.registrypackages.virtWhat125 }}" "socat:{{ .images.registrypackages.socat1734 }}" "e2fsprogs:{{ .images.registrypackages.e2fsprogs1470 }}" "mount:{{ .images.registrypackages.utilLinux2401 }}" "xfsprogs:{{ .images.registrypackages.xfsprogs670 }}" "netcat:{{ .images.registrypackages.netcat110481 }}"
