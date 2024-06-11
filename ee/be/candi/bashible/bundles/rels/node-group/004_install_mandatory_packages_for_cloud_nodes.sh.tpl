{{- if ne .nodeGroup.nodeType "Static" }}
# Copyright 2022 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE.

bb-yum-install http://mirror.centos.org/centos/7.9.2009/os/x86_64/Packages/cloud-utils-growpart-0.29-5.el7.noarch.rpm
{{- end }}
