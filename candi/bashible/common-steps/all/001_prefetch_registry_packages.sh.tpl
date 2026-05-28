# Copyright 2026 Flant JSC
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

# Prefetch registry packages in the background so steps 003-006 can run in parallel with
# the network download. Step 007 (007_fetch_registry_packages.sh) waits for this systemd
# unit to finish; if for any reason this prefetch doesn't run (missing systemd, missing
# rpp-get, etc.) step 007 falls back to running the fetch inline as before.
#
# Package list MUST stay identical to 007_fetch_registry_packages.sh.tpl.

{{- $kubernetesVersion := printf "%s%s" (.kubernetesVersion | toString) (index .k8s .kubernetesVersion "patch" | toString) | replace "." "" }}
{{- $kubernetesCniVersion := "1.6.2" | replace "." "" }}

{{- $containerd := "containerd1730"}}
{{- if eq .cri "ContainerdV2" }}
  {{- $containerd = "containerd223" }}
{{- end }}

if ! command -v systemd-run >/dev/null 2>&1 || ! command -v systemctl >/dev/null 2>&1; then
  bb-log-warning "systemd-run/systemctl not available; skip prefetch — 007 will fetch inline"
  return 0 2>/dev/null || exit 0
fi

# Only attempt if systemd is in a usable state.
case "$(systemctl is-system-running 2>/dev/null || true)" in
  running|degraded|starting|initializing|maintenance) ;;
  *)
    bb-log-warning "systemd not in usable state; skip prefetch"
    return 0 2>/dev/null || exit 0
    ;;
esac

if ! command -v rpp-get >/dev/null 2>&1; then
  bb-log-warning "rpp-get not yet available; skip prefetch — 007 will fetch inline"
  return 0 2>/dev/null || exit 0
fi

unit="rpp-prefetch.service"

# Idempotency:
#   - active           → already running, leave it
#   - inactive/failed  → reset and (re)launch
case "$(systemctl is-active $unit 2>/dev/null || true)" in
  active|activating)
    bb-log-info "$unit already running; nothing to do"
    return 0 2>/dev/null || exit 0
    ;;
  failed)
    systemctl reset-failed $unit >/dev/null 2>&1 || true
    ;;
  inactive|"")
    # Clear any leftover unit definition so systemd-run can reuse the name.
    systemctl stop $unit >/dev/null 2>&1 || true
    systemctl reset-failed $unit >/dev/null 2>&1 || true
    ;;
esac

if ! systemd-run \
    --unit="$unit" \
    --description="Prefetch deckhouse registry packages (parallel with bashible system prep)" \
    --collect \
    /bin/bash -c "rpp-get fetch \
      \"kubernetes-cni:{{ index .images.registrypackages (printf "kubernetesCni%s" $kubernetesCniVersion) | toString }}\" \
      \"kubelet:{{ index .images.registrypackages (printf "kubelet%s" $kubernetesVersion) | toString }}\" \
      \"containerd:{{ index .images.registrypackages $containerd }}\" \
      \"crictl:{{ index .images.registrypackages (printf "crictl%s" (.kubernetesVersion | replace "." "")) | toString }}\" \
      \"toml-merge:{{ .images.registrypackages.tomlMerge01 }}\" \
      \"d8:{{ .images.registrypackages.d8 }}\" \
      \"pause:{{ .images.registrypackages.pause }}\" \
      \"kubernetes-api-proxy:{{ .images.registrypackages.kubernetesApiProxy }}\" \
      \"registry-proxy:{{ .images.registrypackages.registryProxy }}\" \
      \"jq:{{ .images.registrypackages.jq171 }}\" \
      \"yq:{{ .images.registrypackages.yq4471 }}\" \
      \"curl:{{ .images.registrypackages.d8Curl891 }}\" \
      \"which:{{ .images.registrypackages.which223 }}\" \
      \"virt-what:{{ .images.registrypackages.virtWhat125 }}\" \
      \"socat:{{ .images.registrypackages.socat1734 }}\" \
      \"e2fsprogs:{{ .images.registrypackages.e2fsprogs1472 }}\" \
      \"iptables:{{ .images.registrypackages.iptables189 }}\" \
      \"growpart:{{ .images.registrypackages.growpart033 }}\" \
      \"lsblk:{{- index .images.registrypackages "lsblk2402" }}\" \
      \"nfs-mount:{{- .images.registrypackages.nfsMount282 }}\"" \
    >/dev/null 2>&1; then
  bb-log-warning "systemd-run failed to launch $unit; 007 will fetch inline"
  return 0 2>/dev/null || exit 0
fi

bb-log-info "$unit launched; subsequent step 007 will wait for it"
