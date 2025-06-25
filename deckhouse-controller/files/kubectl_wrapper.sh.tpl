#!/bin/bash

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

set -Eeuo pipefail

if [ -s /tmp/kubectl_version ]; then
 kubernetes_version="$(cat /tmp/kubectl_version)"
else
 # Workaround for running kubectl before global hook global-hooks/discovery/kubernetes_version running
 kubernetes_version="$(/usr/local/bin/kubectl-{{ index (index . 0) "kubectl" }} version -o json 2>/dev/null | jq -r '.serverVersion.gitVersion | ltrimstr("v")')"
fi

case "$kubernetes_version" in
{{- range . }}
  {{- $versions := list }}
  {{- range .version }}
    {{- $versions = append $versions (printf "%s.*" .) }}
  {{- end }}
  {{ join " | " $versions }} )
    kubectl_version="{{ .kubectl }}"
    ;;
{{- end }}
  *)
    >&2 echo "ERROR: unsupported kubernetes version $kubernetes_version"
    exit 1
    ;;
esac

exec "/usr/bin/kubectl-$kubectl_version" "$@"
