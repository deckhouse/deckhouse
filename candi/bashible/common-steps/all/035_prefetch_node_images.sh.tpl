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
# Fires a transient systemd unit `ctr-prefetch.service` right after containerd is
# configured and restarted (030-033) and the local images are imported (034). The
# unit pulls a curated set of images in parallel via `ctr pull` so that by the time
# kubelet starts scheduling pods, the bulk of cold-start images is already in the
# containerd content store.
#
# Image whitelist (order = start-priority; xargs -P workers pick lines top-to-bottom,
# so earlier sections get scheduled first):
#    0. deckhouse-controller (tag-based ref: `<imagesBase>:<tag>`). Injected by
#       dhctl as `.deckhouseImageRef` and built from the same source the
#       bootstrap Deployment uses for `params.Registry`
#       (DeckhouseInstaller.GetInclusterImage(true)). Prefetching it here means
#       kubelet's IfNotPresent pull of the bootstrap pod finds the ~200 MiB
#       image already in containerd. Skipped when the ref is unavailable.
#    1. deckhouse/*
#    2. cniCilium/*
#    3. common/coredns
#    4. controlPlaneManager: etcd + kubeApiserver/kubeControllerManager/kubeScheduler/
#                            controlPlaneManager for the current kubernetes version
#    5. nodeManager/* (filtered: skip nvidia*, nodeFeatureDiscovery, non-current
#                      clusterAutoscaler<N>)
#    6. cloudProvider<Provider>/*
#    7. common: kubeRbacProxy, iptablesWrapper, init, shellOperator, distroless
#    8. kubeProxy: kubeProxy<N>, iptablesWrapperInit, initContainer
#    9. common csi-sidecars for current k8s: csiExternalProvisioner<N>, csiExternalAttacher<N>,
#       csiExternalResizer<N>, csiExternalSnapshotter<N>, csiLivenessprobe<N>,
#       csiNodeDriverRegistrar<N>
#   10. common misc: checkKernelVersion, cniMigrationInitChecker, vxlanOffloadingFixer
#   11. chrony/*
#   12. registryPackagesProxy/*
#   13. monitoringKubernetes/*
#   14. kubeDns/*
#
# Each line: "<section>/<imageKey> <ref>" — pull_one splits on the first space.
#
# This is fire-and-forget: failures never block bashible. If the unit dies, times out,
# or some image cannot be pulled, kubelet will fetch it later as usual.

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

if ! command -v ctr >/dev/null 2>&1; then
  bb-log-warning "ctr not available; skip image prefetch"
  return 0 2>/dev/null || exit 0
fi

unit="ctr-prefetch.service"

case "$(systemctl is-active $unit 2>/dev/null || true)" in
  active|activating)
    bb-log-info "$unit already running; nothing to do"
    return 0 2>/dev/null || exit 0
    ;;
  failed)
    systemctl reset-failed $unit >/dev/null 2>&1 || true
    ;;
  inactive|"")
    systemctl stop $unit >/dev/null 2>&1 || true
    systemctl reset-failed $unit >/dev/null 2>&1 || true
    ;;
esac

# Render the prefetch list straight into a file via a quoted heredoc — no bash array
# literal, no go-template trim acrobatics. The quoted EOF (single-quoted PREFETCH_EOF)
# disables bash expansion; go-template still substitutes its actions as it produces
# the file.
list_file="/run/ctr-prefetch.list"
umask 077
{{- $k8s := .kubernetesVersion | toString | replace "." "" }}
{{- $cpmNames := list "etcd"
    (printf "controlPlaneManager%s" $k8s)
    (printf "kubeApiserver%s" $k8s)
    (printf "kubeControllerManager%s" $k8s)
    (printf "kubeScheduler%s" $k8s) }}
{{- $commonRest := list "kubeRbacProxy" "iptablesWrapper" "init" "shellOperator" "distroless" }}
cat > "$list_file" <<'PREFETCH_EOF'
{{- with .deckhouseImageRef }}
deckhouse/controller {{ . }}
{{- end }}
{{- range $name, $digest := (index $.images "deckhouse" | default dict) }}
deckhouse/{{ $name }} {{ $.registry.imagesBase }}@{{ $digest }}
{{- end }}
{{- range $name, $digest := (index $.images "cniCilium" | default dict) }}
cniCilium/{{ $name }} {{ $.registry.imagesBase }}@{{ $digest }}
{{- end }}
{{- with index ($.images.common | default dict) "coredns" }}
common/coredns {{ $.registry.imagesBase }}@{{ . }}
{{- end }}
{{- range $name := $cpmNames }}
{{- $digest := index ($.images.controlPlaneManager | default dict) $name }}
{{- if $digest }}
controlPlaneManager/{{ $name }} {{ $.registry.imagesBase }}@{{ $digest }}
{{- end }}
{{- end }}
{{- $caCurrent := printf "clusterAutoscaler%s" $k8s }}
{{- range $name, $digest := (index $.images "nodeManager" | default dict) }}
{{- if not (hasPrefix "nvidia" $name) }}
{{- if ne $name "nodeFeatureDiscovery" }}
{{- if or (not (hasPrefix "clusterAutoscaler" $name)) (eq $name $caCurrent) }}
nodeManager/{{ $name }} {{ $.registry.imagesBase }}@{{ $digest }}
{{- end }}
{{- end }}
{{- end }}
{{- end }}
{{- if .provider }}
{{- $cpSection := printf "cloudProvider%s" (.provider | title) }}
{{- range $name, $digest := (index $.images $cpSection | default dict) }}
{{ $cpSection }}/{{ $name }} {{ $.registry.imagesBase }}@{{ $digest }}
{{- end }}
{{- end }}
{{- range $name := $commonRest }}
{{- $digest := index ($.images.common | default dict) $name }}
{{- if $digest }}
common/{{ $name }} {{ $.registry.imagesBase }}@{{ $digest }}
{{- end }}
{{- end }}
{{- $kpNames := list (printf "kubeProxy%s" $k8s) "iptablesWrapperInit" "initContainer" }}
{{- range $name := $kpNames }}
{{- $digest := index ($.images.kubeProxy | default dict) $name }}
{{- if $digest }}
kubeProxy/{{ $name }} {{ $.registry.imagesBase }}@{{ $digest }}
{{- end }}
{{- end }}
{{- $csiNames := list
    (printf "csiExternalProvisioner%s" $k8s)
    (printf "csiExternalAttacher%s" $k8s)
    (printf "csiExternalResizer%s" $k8s)
    (printf "csiExternalSnapshotter%s" $k8s)
    (printf "csiLivenessprobe%s" $k8s)
    (printf "csiNodeDriverRegistrar%s" $k8s) }}
{{- range $name := $csiNames }}
{{- $digest := index ($.images.common | default dict) $name }}
{{- if $digest }}
common/{{ $name }} {{ $.registry.imagesBase }}@{{ $digest }}
{{- end }}
{{- end }}
{{- range $name := (list "checkKernelVersion" "cniMigrationInitChecker" "vxlanOffloadingFixer") }}
{{- $digest := index ($.images.common | default dict) $name }}
{{- if $digest }}
common/{{ $name }} {{ $.registry.imagesBase }}@{{ $digest }}
{{- end }}
{{- end }}
{{- range $name, $digest := (index $.images "chrony" | default dict) }}
chrony/{{ $name }} {{ $.registry.imagesBase }}@{{ $digest }}
{{- end }}
{{- range $name, $digest := (index $.images "registryPackagesProxy" | default dict) }}
registryPackagesProxy/{{ $name }} {{ $.registry.imagesBase }}@{{ $digest }}
{{- end }}
{{- range $name, $digest := (index $.images "monitoringKubernetes" | default dict) }}
monitoringKubernetes/{{ $name }} {{ $.registry.imagesBase }}@{{ $digest }}
{{- end }}
{{- range $name, $digest := (index $.images "kubeDns" | default dict) }}
kubeDns/{{ $name }} {{ $.registry.imagesBase }}@{{ $digest }}
{{- end }}
PREFETCH_EOF

list_count=$(grep -c . "$list_file" 2>/dev/null || echo 0)
if [ "$list_count" -eq 0 ]; then
  rm -f "$list_file"
  bb-log-info "image prefetch: empty list; nothing to do"
  return 0 2>/dev/null || exit 0
fi

# Materialize the prefetch driver as a standalone script so systemd-run only sees a
# path. Passing the script inline via `bash -c "$VAR"` is unsafe: systemd's Exec=
# syntax expands ${VAR} *itself* before bash runs (eating ${log_dir}, ${parallelism},
# ${item#* } as bash parameter expansions etc.) and serializes newlines as literal
# `\n` in the cmdline.
script_file="/run/ctr-prefetch.sh"
cat > "$script_file" <<'EOSCRIPT'
#!/bin/bash
set -u

# Deckhouse ships ctr/crictl/jq under /opt/deckhouse/bin, which is NOT on the default
# PATH of a transient systemd unit. Without this, `command -v ctr` fails inside the
# unit and the `until ctr version` loop spins until the deadline.
export PATH="/opt/deckhouse/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

list_file="/run/ctr-prefetch.list"
script_file="/run/ctr-prefetch.sh"
hosts_dir="/etc/containerd/registry.d"
log_dir="/var/log/d8/bashible"
log_file="${log_dir}/ctr-prefetch.log"
runtime_endpoint="unix:///run/containerd/containerd.sock"
parallelism=4
per_image_total_deadline=600
ctr_ready_deadline=120

mkdir -p "$log_dir"

ts() { date -u +%Y-%m-%dT%H:%M:%SZ; }
# Single short append is atomic w.r.t. concurrent xargs workers (POSIX O_APPEND,
# our lines are far below PIPE_BUF), so no flock is needed.
log() { printf '%s\n' "$*" >> "$log_file"; echo "$*"; }

session_start=$(date +%s.%N)
total_count=$(grep -c . "$list_file" 2>/dev/null || echo 0)
log "=== ctr-prefetch session started $(ts) ==="
log "$(ts) START parallelism=${parallelism} count=${total_count}"

deadline=$((SECONDS + ctr_ready_deadline))
until ctr version >/dev/null 2>&1; do
  if [ $SECONDS -ge $deadline ]; then
    log "$(ts) ABORT containerd not ready within ${ctr_ready_deadline}s"
    exit 0
  fi
  sleep 1
done

# Returns image size in bytes via CRI; "-" on failure or jq/crictl absent.
get_image_size() {
  local ref="$1"
  if ! command -v crictl >/dev/null 2>&1 || ! command -v jq >/dev/null 2>&1; then
    echo "-"; return 0
  fi
  crictl --runtime-endpoint="$runtime_endpoint" images -o json 2>/dev/null \
    | jq -r --arg ref "$ref" '
        (.images // [])[]
        | select((.repoDigests // []) | index($ref))
        | .size
      ' 2>/dev/null \
    | head -n1 \
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
    if ctr -n k8s.io images pull --hosts-dir "$hosts_dir" "$ref" >/dev/null 2>&1; then
      ctr -n k8s.io images label "$ref" io.cri-containerd.pinned=pinned >/dev/null 2>&1 || true
      dur=$(awk -v s="$start_ts" -v e="$(date +%s.%N)" 'BEGIN{printf "%.3f", e-s}')
      size_bytes=$(get_image_size "$ref")
      log "$(ts) OK    dur=${dur}s size=${size_bytes}B name=${name} ref=${ref}"
      return 0
    fi
    if [ $((SECONDS - start_s)) -ge $per_image_total_deadline ]; then
      dur=$(awk -v s="$start_ts" -v e="$(date +%s.%N)" 'BEGIN{printf "%.3f", e-s}')
      log "$(ts) FAIL  dur=${dur}s size=- name=${name} ref=${ref} reason=timeout"
      return 0
    fi
    sleep "$delay"
    if [ "$delay" -lt 30 ]; then
      delay=$((delay * 2))
    fi
  done
}
export -f ts log get_image_size pull_one
export hosts_dir log_file runtime_endpoint per_image_total_deadline

# Skip blank lines from xargs input (defensive against accidental empties).
grep -v '^$' "$list_file" \
  | xargs -d '\n' -P "$parallelism" -I {} bash -c 'pull_one "$@"' _ {}

elapsed=$(awk -v s="$session_start" -v e="$(date +%s.%N)" 'BEGIN{printf "%.3f", e-s}')

# Count results only within the current session (log file accumulates across runs).
session_start_lineno=$(grep -n '^=== ctr-prefetch session started' "$log_file" 2>/dev/null | tail -n1 | cut -d: -f1)
: "${session_start_lineno:=1}"
session_log=$(tail -n "+${session_start_lineno}" "$log_file" 2>/dev/null)

ok_count=$(printf '%s\n' "$session_log" | grep -c '^[^ ]* OK ')
fail_count=$(printf '%s\n' "$session_log" | grep -c '^[^ ]* FAIL ')
total_bytes=$(printf '%s\n' "$session_log" | awk '
  /^[^ ]* OK / {
    for (i=1; i<=NF; i++) if ($i ~ /^size=[0-9]+B$/) {
      gsub(/^size=|B$/, "", $i); sum += $i
    }
  }
  END { printf "%.0f", sum+0 }')
log "$(ts) DONE  elapsed=${elapsed}s ok=${ok_count} fail=${fail_count} bytes=${total_bytes}"

rm -f "$list_file" "$script_file"
EOSCRIPT
chmod 700 "$script_file"

if ! systemd-run \
    --unit="$unit" \
    --description="Prefetch node images (parallel with bashible system prep)" \
    --collect \
    /bin/bash "$script_file" \
    >/dev/null 2>&1; then
  bb-log-warning "systemd-run failed to launch $unit"
  rm -f "$list_file" "$script_file"
  return 0 2>/dev/null || exit 0
fi

bb-log-info "$unit launched (${list_count} images, parallelism=4)"
echo "[bashible-timing] step=035_prefetch_node_images.sh section=launched count=${list_count}"

{{- end }}
