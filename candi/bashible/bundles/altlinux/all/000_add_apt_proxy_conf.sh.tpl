# Copyright 2022 Flant JSC
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

proxy=""
{{- if .packagesProxy }}
authstring=""
  {{- if .packagesProxy.username }}
authstring="{{ .packagesProxy.username }}"
  {{- end }}
  {{- if .packagesProxy.password }}
authstring="${authstring}:{{ .packagesProxy.password }}"
  {{- end }}
if [[ -n $authstring ]]; then
 proxy="$(echo "{{ .packagesProxy.uri }}" | sed "s/:\/\//:\/\/${authstring}@/")"
else
 proxy="{{ .packagesProxy.uri }}"
fi
{{- end }}

if [[ -n $proxy ]]; then
  bb-sync-file /etc/apt/apt.conf.d/00proxy - << EOF
Acquire {
  HTTP::proxy "$proxy";
  HTTPS::proxy "$proxy";
}
EOF
else
  rm -f /etc/apt/apt.conf.d/00proxy
fi
