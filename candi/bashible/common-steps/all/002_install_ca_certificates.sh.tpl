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

bb-package-install "d8-ca-updater:{{ .images.registrypackages.d8CaUpdater060824 }}"

REGISTRY_CACERT_PATH="/opt/deckhouse/share/ca-certificates/registry-ca.crt"

{{- if .registry.ca }}
bb-sync-file $REGISTRY_CACERT_PATH - << "EOF"
{{ .registry.ca }}
EOF
{{- else }}
if [ -f $REGISTRY_CACERT_PATH ]; then
  rm -f $REGISTRY_CACERT_PATH
fi
{{- end }}
