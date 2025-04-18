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

{{- if eq $.cri "Containerd" }}

{{- $sandbox_image := printf "%s@%s" .registry.imagesBase (index $.images.common "pause") }}

_get_local_images_list() {
  repo_digests=$(/opt/deckhouse/bin/crictl images -o json | jq -r '.images[].repoDigests[]?')
  echo $repo_digests
}

if [ "$FIRST_BASHIBLE_RUN" != "yes" ]; then
  local_images_list=$(_get_local_images_list)
  if ! echo $local_images_list | grep -q {{ $sandbox_image | quote }}; then
    /opt/deckhouse/bin/ctr --namespace=k8s.io images pull --hosts-dir="/etc/containerd/registry_prepull.d" {{ $sandbox_image }}
  fi
fi

{{- end }}
