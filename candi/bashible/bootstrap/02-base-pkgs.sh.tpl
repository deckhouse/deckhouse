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

{{- if $check_python := .Files.Get "/deckhouse/candi/bashible/check_python.sh" | default (.Files.Get "candi/bashible/check_python.sh") -}}
  {{- tpl ( $check_python ) . | nindent 0 }}
{{- end }}

{{- if $bb_package_install := .Files.Get "/deckhouse/candi/bashible/bb_package_install.sh.tpl" | default (.Files.Get "candi/bashible/bb_package_install.sh.tpl") -}}
  {{- tpl ( $bb_package_install ) . | nindent 0 }}
{{- end }}

{{ with .images.registrypackages }}
bb-package-install "jq:{{ .jq16 }}" "curl:{{ .d8Curl821 }}" "netcat:{{ .netcat110481 }}"
{{- end }}
