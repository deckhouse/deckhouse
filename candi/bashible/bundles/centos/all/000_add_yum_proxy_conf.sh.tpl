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

bb-yum-install yum-utils

proxy="--setopt=proxy="
if bb-is-centos-version? 7; then
  proxy="--setopt=proxy=_none_"
fi

if yum --version | grep -q dnf; then
  proxy="--setopt=proxy="
fi

{{- if .packagesProxy.uri }}
proxy="--setopt=proxy={{ .packagesProxy.uri }} main"
{{- end }}

${config_manager} --save ${proxy}

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
