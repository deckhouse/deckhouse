{{- if ne .nodeGroup.nodeType "Static" }}
# Copyright 2022 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE.

bb-yum-install cloud-utils-growpart
{{- end }}
