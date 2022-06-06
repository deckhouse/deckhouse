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

{{- if eq .cri "Docker" }}
if [[ "${FIRST_BASHIBLE_RUN}" != "yes" ]]; then
  exit 0
fi

pause_container="registry.k8s.io/pause:3.2"
if [[ "$(docker image ls -q "${pause_container}" | wc -l)" -eq "0" ]]; then
  if ! docker pull "${pause_container}" >/dev/null 2>/dev/null; then
    docker pull registry.deckhouse.io/deckhouse/ce:pause-3.2
    docker tag registry.deckhouse.io/deckhouse/ce:pause-3.2 "${pause_container}"
  fi
fi
{{- end }}
