# Copyright 2025 Flant JSC
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

{{- if eq .runType "Normal" }}
  {{ if .gpu }}

if ! command -v /usr/bin/nvidia-container-runtime >/dev/null 2>&1; then
    bb-log-error "'/usr/bin/nvidia-container-runtime' doesn't exist. It's require for Nvdia GPU nodes."
    exit 1
fi

if ! command -v /usr/bin/nvidia-smi; then
    bb-log-error "'/usr/bin/nvidia-smi' doesn't exist. It's require for Nvdia GPU nodes."
    exit 1
fi

/usr/bin/nvidia-smi -L

  {{- end }}
{{- end }}
