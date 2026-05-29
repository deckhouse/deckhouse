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
# bashible: parallel-group=pkg-batch

{{- if or ( eq .cri "Containerd") ( eq .cri "ContainerdV2") }}

  {{- $sandbox_image := "deckhouse.local/images:pause" -}}
  {{- $kubernetes_api_proxy_image := "deckhouse.local/images:kubernetes-api-proxy" }}
  {{- $registry_proxy_image := "deckhouse.local/images:registry-proxy" }}

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

  if [[ "${PACKAGE}" == "registry-proxy" ]]; then
    ctr_import_image {{ $registry_proxy_image }} "/opt/deckhouse/images/registry-proxy.tar"
    return 0
  fi
}

bb-event-on 'bb-package-installed' 'post-install-import'

__sec_start=$(date +%s.%N)
__sec() {
  local now dur
  now=$(date +%s.%N)
  dur=$(awk -v s="$__sec_start" -v e="$now" 'BEGIN{printf "%.3f", e-s}')
  echo "[bashible-timing] step=034_ctr_import_local_images.sh section=$1 dur=${dur}s"
  __sec_start=$now
}

# Per-step prefetch wait: all three packages below are prefetched by step 001.
bb-rpp-wait-fetched "pause" "{{ $.images.registrypackages.pause }}" || true
bb-rpp-wait-fetched "kubernetes-api-proxy" "{{ $.images.registrypackages.kubernetesApiProxy }}" || true
bb-rpp-wait-fetched "registry-proxy" "{{ $.images.registrypackages.registryProxy }}" || true
__sec wait_prefetch

bb-package-install "pause:{{ $.images.registrypackages.pause }}"
bb-package-install "kubernetes-api-proxy:{{ $.images.registrypackages.kubernetesApiProxy }}"
bb-package-install "registry-proxy:{{ $.images.registrypackages.registryProxy }}"
__sec pkg_install

if bb-flag? need-local-images-import; then
  post-install-import pause
  post-install-import kubernetes-api-proxy
  post-install-import registry-proxy
  bb-flag-unset need-local-images-import
  __sec ctr_import
fi
{{- end }}
