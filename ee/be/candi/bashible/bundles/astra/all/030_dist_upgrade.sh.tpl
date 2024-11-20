# Copyright 2022 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE.

{{- if eq .runType "ImageBuilding" }}
bb-set-proxy
bb-apt-dist-upgrade
bb-unset-proxy
{{- end }}
