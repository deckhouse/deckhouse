{{- /*
# Copyright 2024 Flant JSC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
*/}}
#!/bin/bash
set -Eeo pipefail

{{- /*
# dhctl and node-manager renders have different helm root dir and .Files.Get should use different paths:
# '/deckhouse/candi/bashible/...' - dhctl
# 'candi/bashible/...' - node-manager
# For dhctl render we include 'bb_package_install'.
# For node-manager render this file include to place, where 'bb_package_install' already included on previous lines.
*/}}

{{- if $bb_package_install := .Files.Get "deckhouse/candi/bashible/bb_package_install.sh.tpl" -}}
  {{- tpl ( $bb_package_install ) . | nindent 0 }}
{{- end }}

{{ with .images.registrypackages }}
bb-package-install "jq:{{ .jq171 }}" "curl:{{ .d8Curl891 }}" "netcat:{{ .netcat110481 }}"
{{- end }}
