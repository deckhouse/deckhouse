# Copyright 2026 Flant JSC
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

{{- if has (.registry).mode (list "Local") }}

SYNCER_CONFIG_PATH="$(bb-tmp-file)"

bb-sync-file $SYNCER_CONFIG_PATH - << "EOF"
source:
  address: localhost:5511
destination:
  address: localhost:5001
  ca: |
    {{ .registry.bootstrap.init.ca.cert | nindent 4 }}
  user:
    name: {{ .registry.bootstrap.init.rw_user.name | quote }}
    password: {{ .registry.bootstrap.init.rw_user.password | quote }}
EOF

syncer $SYNCER_CONFIG_PATH | bb-log-stream-dhctl

{{- end }}
