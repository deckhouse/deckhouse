# Copyright 2021 Flant JSC
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

{{- if .packagesProxy.uri }}
yum-config-manager --save --setopt=proxy={{ .packagesProxy.uri }} main
{{- else }}
yum-config-manager --save --setopt=proxy=_none_
{{- end }}
{{- if .packagesProxy.username }}
yum-config-manager --save --setopt=proxy_username={{ .packagesProxy.username }} main
{{- else }}
yum-config-manager --save --setopt=proxy_username=
{{- end }}
{{- if .packagesProxy.password }}
yum-config-manager --save --setopt=proxy_password={{ .packagesProxy.password }} main
{{- else }}
yum-config-manager --save --setopt=proxy_password=
{{- end }}
