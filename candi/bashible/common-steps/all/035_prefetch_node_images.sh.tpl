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

# Master-only background image prefetch.
#
# Right after containerd is configured (030-033) and local images are imported (034),
# fires a transient systemd unit that pulls a curated set of images in parallel via
# `crictl pull`. By the time kubelet starts scheduling pods, most cold-start images are
# already in the containerd image store. Using crictl (the CRI image service) means the
# pull path, registry auth and mirror config are exactly the ones kubelet itself uses —
# no --hosts-dir, no manual namespace.
#
# The list is rendered below in start-priority order (xargs -P workers take lines
# top-to-bottom): deckhouse-controller (tag-based `.deckhouseImageRef`, ~200 MiB, the
# bootstrap pod's own image), deckhouse/*, cniCilium/*, coredns, controlPlaneManager,
# nodeManager (minus nvidia*/nodeFeatureDiscovery/stale clusterAutoscaler), current
# cloudProvider/*, common runtime bits, kubeProxy, csi-sidecars, common misc, then
# chrony/*, registryPackagesProxy/*, monitoringKubernetes/* and kubeDns/*.
#
# Each line is "<section>/<imageKey> <ref>"; pull_one splits on the first space.
# Fire-and-forget: failures never block bashible; kubelet fetches anything missing later.

{{- if and (eq .nodeGroup.name "master") (or (eq .cri "Containerd") (eq .cri "ContainerdV2")) }}

if ! command -v systemd-run >/dev/null 2>&1 || ! command -v systemctl >/dev/null 2>&1; then
  bb-log-warning "systemd-run/systemctl not available; skip image prefetch"
  return 0 2>/dev/null || exit 0
fi

case "$(systemctl is-system-running 2>/dev/null || true)" in
  running|degraded|starting|initializing|maintenance) ;;
  *)
    bb-log-warning "systemd not in usable state; skip image prefetch"
    return 0 2>/dev/null || exit 0
    ;;
esac

if ! command -v crictl >/dev/null 2>&1; then
  bb-log-warning "crictl not available; skip image prefetch"
  return 0 2>/dev/null || exit 0
fi

unit="ctr-prefetch.service"

case "$(systemctl is-active "$unit" 2>/dev/null || true)" in
  active|activating)
    bb-log-info "$unit already running; nothing to do"
    return 0 2>/dev/null || exit 0
    ;;
esac
systemctl reset-failed "$unit" >/dev/null 2>&1 || true

# Materialize the prefetch driver as a standalone script so systemd-run only sees a
# path. Passing it inline via `bash -c "$VAR"` is unsafe: systemd's Exec= syntax expands
# ${VAR} itself before bash runs (eating ${log_dir}, ${item#* } as parameter expansions)
# and serializes newlines as literal `\n`. The image list is embedded in the script as a
# quoted heredoc — go-template still substitutes its actions while rendering this file.
umask 077
script_file="/run/ctr-prefetch.sh"
{{- $k8s := .kubernetesVersion | toString | replace "." "" }}
{{- $base := $.registry.imagesBase }}
{{- $caCurrent := printf "clusterAutoscaler%s" $k8s }}
{{- $cpmNames := list "etcd" (printf "controlPlaneManager%s" $k8s) (printf "kubeApiserver%s" $k8s) (printf "kubeControllerManager%s" $k8s) (printf "kubeScheduler%s" $k8s) }}
{{- $commonRest := list "kubeRbacProxy" "iptablesWrapper" "init" "shellOperator" "distroless" }}
{{- $kpNames := list (printf "kubeProxy%s" $k8s) "iptablesWrapperInit" "initContainer" }}
{{- $commonNamed := list (printf "csiExternalProvisioner%s" $k8s) (printf "csiExternalAttacher%s" $k8s) (printf "csiExternalResizer%s" $k8s) (printf "csiExternalSnapshotter%s" $k8s) (printf "csiLivenessprobe%s" $k8s) (printf "csiNodeDriverRegistrar%s" $k8s) "checkKernelVersion" "cniMigrationInitChecker" "vxlanOffloadingFixer" }}
cat > "$script_file" <<'EOSCRIPT'
#!/bin/bash
set -u

script_file="/run/ctr-prefetch.sh"
results_file="/run/ctr-prefetch.results"
log_dir="/var/log/d8/bashible"
log_file="${log_dir}/ctr-prefetch.log"
runtime_endpoint="unix:///run/containerd/containerd.sock"
parallelism=4
per_image_total_deadline=600
cri_ready_deadline=120

# Image list, one "<section>/<imageKey> <ref>" per line. The single-quoted heredoc
# disables bash expansion; go-template substitutes its actions when rendering this file.
images=$(cat <<'PREFETCH_EOF'
{{- with .deckhouseImageRef }}
deckhouse/controller {{ . }}
{{- end }}
{{- range $section := (list "deckhouse" "cniCilium") }}
{{- range $name, $digest := (index $.images $section | default dict) }}
{{ $section }}/{{ $name }} {{ $base }}@{{ $digest }}
{{- end }}
{{- end }}
{{- with index ($.images.common | default dict) "coredns" }}
common/coredns {{ $base }}@{{ . }}
{{- end }}
{{- range $name := $cpmNames }}
{{- $digest := index ($.images.controlPlaneManager | default dict) $name }}
{{- if $digest }}
controlPlaneManager/{{ $name }} {{ $base }}@{{ $digest }}
{{- end }}
{{- end }}
{{- range $name, $digest := (index $.images "nodeManager" | default dict) }}
{{- if and (not (hasPrefix "nvidia" $name)) (ne $name "nodeFeatureDiscovery") (or (not (hasPrefix "clusterAutoscaler" $name)) (eq $name $caCurrent)) }}
nodeManager/{{ $name }} {{ $base }}@{{ $digest }}
{{- end }}
{{- end }}
{{- if .provider }}
{{- $cpSection := printf "cloudProvider%s" (.provider | title) }}
{{- range $name, $digest := (index $.images $cpSection | default dict) }}
{{ $cpSection }}/{{ $name }} {{ $base }}@{{ $digest }}
{{- end }}
{{- end }}
{{- range $name := $commonRest }}
{{- $digest := index ($.images.common | default dict) $name }}
{{- if $digest }}
common/{{ $name }} {{ $base }}@{{ $digest }}
{{- end }}
{{- end }}
{{- range $name := $kpNames }}
{{- $digest := index ($.images.kubeProxy | default dict) $name }}
{{- if $digest }}
kubeProxy/{{ $name }} {{ $base }}@{{ $digest }}
{{- end }}
{{- end }}
{{- range $name := $commonNamed }}
{{- $digest := index ($.images.common | default dict) $name }}
{{- if $digest }}
common/{{ $name }} {{ $base }}@{{ $digest }}
{{- end }}
{{- end }}
{{- range $section := (list "chrony" "registryPackagesProxy" "monitoringKubernetes" "kubeDns") }}
{{- range $name, $digest := (index $.images $section | default dict) }}
{{ $section }}/{{ $name }} {{ $base }}@{{ $digest }}
{{- end }}
{{- end }}
PREFETCH_EOF
)

mkdir -p "$log_dir"
: > "$results_file"

ts() { date -u +%Y-%m-%dT%H:%M:%SZ; }
# Single short append is atomic w.r.t. concurrent xargs workers (POSIX O_APPEND, our
# lines are far below PIPE_BUF), so no flock is needed — same for $results_file below.
log() { printf '%s\n' "$*" >> "$log_file"; echo "$*"; }

total_count=$(printf '%s\n' "$images" | grep -c . || true)
if [ "${total_count:-0}" -eq 0 ]; then
  log "$(ts) image prefetch: empty list; nothing to do"
  rm -f "$script_file" "$results_file"
  exit 0
fi

session_start=$(date +%s.%N)
log "=== ctr-prefetch session started $(ts) ==="
log "$(ts) START parallelism=${parallelism} count=${total_count}"

deadline=$((SECONDS + cri_ready_deadline))
until crictl --runtime-endpoint="$runtime_endpoint" info >/dev/null 2>&1; do
  if [ $SECONDS -ge $deadline ]; then
    log "$(ts) ABORT CRI not ready within ${cri_ready_deadline}s"
    exit 0
  fi
  sleep 1
done

# Image size in bytes via a targeted CRI inspect; "-" on failure or jq absent.
get_image_size() {
  local ref="$1"
  command -v jq >/dev/null 2>&1 || { echo "-"; return 0; }
  crictl --runtime-endpoint="$runtime_endpoint" inspecti -o json "$ref" 2>/dev/null \
    | jq -r '.status.size // "-"' 2>/dev/null \
    | { read -r v || true; printf '%s\n' "${v:--}"; }
}

pull_one() {
  local item="$1"
  local name="${item%% *}"
  local ref="${item#* }"
  local start_ts size_bytes dur
  start_ts=$(date +%s.%N)
  local start_s=$SECONDS
  local delay=2
  while :; do
    if crictl --runtime-endpoint="$runtime_endpoint" pull "$ref" >/dev/null 2>&1; then
      dur=$(awk -v s="$start_ts" -v e="$(date +%s.%N)" 'BEGIN{printf "%.3f", e-s}')
      size_bytes=$(get_image_size "$ref")
      log "$(ts) OK    dur=${dur}s size=${size_bytes}B name=${name} ref=${ref}"
      printf 'OK %s\n' "$size_bytes" >> "$results_file"
      return 0
    fi
    if [ $((SECONDS - start_s)) -ge $per_image_total_deadline ]; then
      dur=$(awk -v s="$start_ts" -v e="$(date +%s.%N)" 'BEGIN{printf "%.3f", e-s}')
      log "$(ts) FAIL  dur=${dur}s size=- name=${name} ref=${ref} reason=timeout"
      printf 'FAIL 0\n' >> "$results_file"
      return 0
    fi
    sleep "$delay"
    if [ "$delay" -lt 30 ]; then
      delay=$((delay * 2))
    fi
  done
}
export -f ts log get_image_size pull_one
export log_file results_file runtime_endpoint per_image_total_deadline

printf '%s\n' "$images" | grep -v '^$' \
  | xargs -d '\n' -P "$parallelism" -I {} bash -c 'pull_one "$@"' _ {}

elapsed=$(awk -v s="$session_start" -v e="$(date +%s.%N)" 'BEGIN{printf "%.3f", e-s}')

ok_count=$(grep -c '^OK ' "$results_file" 2>/dev/null || echo 0)
fail_count=$(grep -c '^FAIL ' "$results_file" 2>/dev/null || echo 0)
total_bytes=$(awk '$1=="OK" && $2 ~ /^[0-9]+$/ { s+=$2 } END { printf "%.0f", s+0 }' "$results_file")
log "$(ts) DONE  elapsed=${elapsed}s ok=${ok_count} fail=${fail_count} bytes=${total_bytes}"

rm -f "$script_file" "$results_file"
EOSCRIPT
chmod 700 "$script_file"

# /opt/deckhouse/bin (crictl, jq) is NOT on a transient unit's default PATH, so declare
# it at launch time instead of re-exporting inside the script.
if ! systemd-run \
    --unit="$unit" \
    --description="Prefetch node images (parallel with bashible system prep)" \
    --setenv=PATH="/opt/deckhouse/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin" \
    --collect \
    /bin/bash "$script_file" \
    >/dev/null 2>&1; then
  bb-log-warning "systemd-run failed to launch $unit"
  rm -f "$script_file"
  return 0 2>/dev/null || exit 0
fi

bb-log-info "$unit launched"
echo "[bashible-timing] step=035_prefetch_node_images.sh section=launched"

{{- end }}
