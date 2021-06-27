# Copyright 2021 Flant CJSC
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

{{- range $key, $value := index .k8s .kubernetesVersion "bashible" "ubuntu" }}
  {{- $ubuntuVersion := toString $key }}
  {{- if or $value.kernel.gcp.desiredVersion $value.kernel.gcp.allowedPattern }}
if bb-is-ubuntu-version? {{ $ubuntuVersion }} ; then
  cat <<EOF > /var/lib/bashible/kernel_version_config_by_cloud_provider
desired_version={{ $value.kernel.gcp.desiredVersion | quote }}
allowed_versions_pattern={{ $value.kernel.gcp.allowedPattern | quote }}
EOF
fi
  {{- end }}
{{- end }}
