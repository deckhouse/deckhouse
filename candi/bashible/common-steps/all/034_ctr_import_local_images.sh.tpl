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

{{- if or ( eq .cri "Containerd") ( eq .cri "ContainerdV2") }}

  {{- $sandbox_image := "deckhouse.local/images:pause" -}}
  {{- $kubernetes_api_proxy_image := "deckhouse.local/images:kubernetes-api-proxy" }}

ctr_import_image() {
  local image_name="$1"
  local image_path="$2"

  ctr -n k8s.io images import "$image_path"
  ctr -n k8s.io images label "$image_name" io.cri-containerd.pinned=pinned
}


post-install-import() {
  bb-log-info "start crt images import"
  local PACKAGE="$1"
  
  if [[ "${PACKAGE}" == "pause" ]]; then
    ctr_import_image {{ $sandbox_image }} "/opt/deckhouse/images/pause.tar"
    return 0
  fi

  if [[ "${PACKAGE}" == "kubernetes-api-proxy" ]]; then
    ctr_import_image {{ $kubernetes_api_proxy_image }} "/opt/deckhouse/images/kubernetes-api-proxy.tar"
    return 0
  fi
}

bb-event-on 'bb-package-installed' 'post-install-import'

bb-package-install "pause:{{ $.images.registrypackages.pause }}"
bb-package-install "kubernetes-api-proxy:{{ $.images.registrypackages.kubernetesApiProxy }}"

if bb-flag? need-local-images-import; then
  post-install-import pause
  post-install-import kubernetes-api-proxy
  bb-flag-unset need-local-images-import
fi
{{- end }}
