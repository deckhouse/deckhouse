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

{{- $containerd := "containerd1732"}}
{{- if eq .cri "ContainerdV2" }}
  {{- $containerd = "containerd224" }}
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
env_file="/run/rpp-prefetch.env"

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

# systemd-run launches a transient system service with a clean environment, so
# persist the RPP auth/context explicitly for the background fetch. Keep the
# token out of the unit definition by sourcing a root-only env file at runtime.
if ! (umask 077 && : > "$env_file"); then
  bb-log-warning "failed to create $env_file; skip prefetch — 007 will fetch inline"
  return 0 2>/dev/null || exit 0
fi

for var in PACKAGES_PROXY_ADDRESSES PACKAGES_PROXY_TOKEN PACKAGES_PROXY_KUBE_APISERVER_ENDPOINTS; do
  if [[ -n "${!var-}" ]]; then
    printf 'export %s=%q\n' "$var" "${!var}" >>"$env_file"
  fi
done

if ! systemd-run \
    --unit="$unit" \
    --description="Prefetch deckhouse registry packages (parallel with bashible system prep)" \
    --collect \
    /bin/bash -c ". \"$env_file\" && exec /opt/deckhouse/bin/rpp-get fetch \
      \"d8:{{ .images.registrypackages.d8 }}\" \
      \"yq:{{ .images.registrypackages.yq4471 }}\" \
      \"curl:{{ .images.registrypackages.d8Curl891 }}\" \
      \"e2fsprogs:{{ .images.registrypackages.e2fsprogs1472 }}\" \
      \"iptables:{{ .images.registrypackages.iptables189 }}\" \
      \"socat:{{ .images.registrypackages.socat1734 }}\" \
      \"jq:{{ .images.registrypackages.jq171 }}\" \
      \"nfs-mount:{{- .images.registrypackages.nfsMount282 }}\" \
      \"lsblk:{{- index .images.registrypackages "lsblk2402" }}\" \
      \"which:{{ .images.registrypackages.which223 }}\" \
      \"growpart:{{ .images.registrypackages.growpart033 }}\" \
      \"virt-what:{{ .images.registrypackages.virtWhat125 }}\" \
      \"containerd:{{ index .images.registrypackages $containerd }}\" \
      \"kubernetes-cni:{{ index .images.registrypackages (printf "kubernetesCni%s" $kubernetesCniVersion) | toString }}\" \
      \"kubelet:{{ index .images.registrypackages (printf "kubelet%s" $kubernetesVersion) | toString }}\" \
      \"crictl:{{ index .images.registrypackages (printf "crictl%s" (.kubernetesVersion | replace "." "")) | toString }}\" \
      \"registry-proxy:{{ .images.registrypackages.registryProxy }}\" \
      \"kubernetes-api-proxy:{{ .images.registrypackages.kubernetesApiProxy }}\" \
      \"toml-merge:{{ .images.registrypackages.tomlMerge01 }}\" \
      \"pause:{{ .images.registrypackages.pause }}\"" \
    >/dev/null 2>&1; then
  bb-log-warning "systemd-run failed to launch $unit; 007 will fetch inline"
  return 0 2>/dev/null || exit 0
fi

bb-log-info "$unit launched; subsequent step 007 will wait for it"
