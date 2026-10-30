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

bb-event-on 'bb-sync-file-changed' 'bb-flag-set kubelet-need-restart'

# Detect systemd-resolved
if grep -q '^nameserver 127.0.0.53' /etc/resolv.conf ; then
  resolvConfPath="/run/systemd/resolve/resolv.conf"
else
  resolvConfPath="/etc/resolv.conf"
fi

# This folder doesn't have time to create before we stop freshly, unconfigured kubelet during bootstrap (step 034_install_kubelet_and_his_friends.sh).
mkdir -p /var/lib/kubelet

# Check CRI type and set appropriated parameters.
# cgroup default is `systemd`.
cgroup_driver="systemd"
{{- if or (eq .cri "Containerd") (eq .cri "ContainerdV2") }}
# Overriding cgroup type from external config file
if [ -f /var/lib/bashible/cgroup_config ]; then
  cgroup_driver="$(cat /var/lib/bashible/cgroup_config)"
fi
{{- end }}

{{- if eq .cri "NotManaged" }}
  {{- if .nodeGroup.cri.notManaged.criSocketPath }}
cri_socket_path={{ .nodeGroup.cri.notManaged.criSocketPath | quote }}
  {{- else }}
for socket_path in /run/containerd/containerd.sock; do
  if [[ -S "${socket_path}" ]]; then
    cri_socket_path="${socket_path}"
    break
  fi
done
  {{- end }}

if [[ -z "${cri_socket_path}" ]]; then
  bb-log-error 'CRI socket is not found, need to manually set "nodeGroup.cri.notManaged.criSocketPath"'
  exit 1
fi
{{- end }}

# Calculate eviction thresholds.

# We don't need more free space on partition.
maxAvailableReservedSpace="20" # `Gb` units
# ~ 61046 inodes are needed for each Gb. Average value was manually calculated from multiple clusters.
needInodesFree=$((maxAvailableReservedSpace*61046/1000)) # `k` units

evictionHardThresholdNodefsAvailable="5%"
evictionHardThresholdNodefsInodesFree="5%"
evictionHardThresholdImagefsAvailable="5%"
evictionHardThresholdImagefsInodesFree="5%"

evictionSoftThresholdNodefsAvailable="10%"
evictionSoftThresholdNodefsInodesFree="10%"
evictionSoftThresholdImagefsAvailable="10%"
evictionSoftThresholdImagefsInodesFree="10%"

{{- if hasKey .nodeGroup "kubelet" }}
rootDir="{{ .nodeGroup.kubelet.rootDir | default "/var/lib/kubelet" }}"
{{- else }}
rootDir="/var/lib/kubelet"
{{- end }}

nodefsSize=$(df --output=size $rootDir | tail -n1)
nodefsSizeGFivePercent=$((nodefsSize/(1000*1000)*5/100))
if [ "$nodefsSizeGFivePercent" -gt "$maxAvailableReservedSpace" ]; then
  evictionHardThresholdNodefsAvailable="${maxAvailableReservedSpace}G"
fi
if [ "$(($nodefsSizeGFivePercent*2))" -gt "$(($maxAvailableReservedSpace*2))" ]; then
  evictionSoftThresholdNodefsAvailable="$(($maxAvailableReservedSpace*2))G"
fi

nodefsInodes=$(df --output=itotal $rootDir | tail -n1)
nodefsInodesKFivePercent=$((nodefsInodes/1000*5/100))
if [ "$nodefsInodesKFivePercent" -gt "$needInodesFree" ]; then
  evictionHardThresholdNodefsInodesFree="${needInodesFree}k"
fi
if [ "$(($nodefsInodesKFivePercent*2))" -gt "$(($needInodesFree*2))" ]; then
  evictionSoftThresholdNodefsInodesFree="$(($needInodesFree*2))k"
fi

{{- if not (eq .cri "NotManaged") }}
# Get CRI directory for eviction thresholds calculation
criDir=$(crictl info -o json | jq -r '.config.containerdRootDir')
# fallback
if [ -z "$criDir" ] || [ "$criDir" = "null" ]; then
  criDir="/var/lib/containerd"
fi
imagefsSize=$(df --output=size "$criDir" | tail -n1)
imagefsInodes=$(df --output=itotal "$criDir" | tail -n1)

imagefsSizeGFivePercent=$((imagefsSize/(1000*1000)*5/100))
if [ "$imagefsSizeGFivePercent" -gt "$maxAvailableReservedSpace" ]; then
  evictionHardThresholdImagefsAvailable="${maxAvailableReservedSpace}G"
fi
if [ "$(($imagefsSizeGFivePercent*2))" -gt "$(($maxAvailableReservedSpace*2))" ]; then
  evictionSoftThresholdImagefsAvailable="$(($maxAvailableReservedSpace*2))G"
fi

imagefsInodesKFivePercent=$((imagefsInodes/1000*5/100))
if [ "$imagefsInodesKFivePercent" -gt "$needInodesFree" ]; then
  evictionHardThresholdImagefsInodesFree="${needInodesFree}k"
fi
if [ "$(($imagefsInodesKFivePercent*2))" -gt "$(($needInodesFree*2))" ]; then
  evictionSoftThresholdImagefsInodesFree="$(($needInodesFree*2))k"
fi
{{- else }}
# For NotManaged CRI, use default percentage-based imagefs thresholds
# We don't calculate absolute values since CRI is managed externally
{{- end }}

shutdownGracePeriod="115"
shutdownGracePeriodCriticalPods="15"
shutdownGracePeriodNodeCriticalPods="5"
{{- if eq .nodeGroup.name "master" }}
# kube-apiserver keeps serving for --shutdown-delay-duration (15s) after SIGTERM and exits
# in 17-20s; etcd exits together with it. With 5s a node shutdown kills both mid-drain.
shutdownGracePeriodNodeCriticalPods="25"
{{- end }}

if [[ -f /var/lib/bashible/cloud-provider-variables ]]; then
  source /var/lib/bashible/cloud-provider-variables

  if [[ -n "$shutdown_grace_period" ]]; then
    shutdownGracePeriod="$shutdown_grace_period"
  fi
  if [[ -n "$shutdown_grace_period_critical_pods" ]]; then
    shutdownGracePeriodCriticalPods="$shutdown_grace_period_critical_pods"
  fi
fi

check_python
function resources_management_memory_units_to_bytes {
  $python_binary -c "
import sys
from decimal import Decimal

def numfmt_to_bytes(human_number, multiplier = 1):
    m = float(multiplier)
    units = {
        'Ki': 1024,
        'Mi': 1024**2,
        'Gi': 1024**3,
        'Ti': 1024**4,
        'Pi': 1024**5,
        'Ei': 1024**6,
        'k': 1000,
        'M': 1000**2,
        'G': 1000**3,
        'T': 1000**4,
        'P': 1000**5,
        'E': 1000**6,
        'm': 0.001,
    }
    for unit, factor in units.items():
        if human_number.endswith(unit):
            return Decimal(float(human_number[: -len(unit)]) * factor * m).quantize(1)
    return Decimal(float(human_number) * m).quantize(1)

human_number = sys.argv[1]
multiplier = 1
if len(sys.argv) > 2:
    if len(sys.argv[2]) > 0:
        multiplier = float(sys.argv[2])
print(numfmt_to_bytes(human_number, multiplier))" $1 $2
}

total_memory=$(free -m|awk '/^Mem:/{print $2}')
# floor to GiB: kernel update moves MemTotal by MiBs, keep reserves stable
if (( total_memory >= 1024 )); then
  total_memory=$(( total_memory / 1024 * 1024 ))
fi

{{- $resourceReservationMode := dig "kubelet" "resourceReservation" "mode" "" .nodeGroup }}
{{- if eq $resourceReservationMode "Auto" }}
# https://github.com/openshift/machine-config-operator/blob/bd24f17943eb95309fe78327f8f3eabd104ab577/templates/common/_base/files/kubelet-auto-sizing.yaml / 3
function dynamic_memory_sizing {
    local recommended_systemreserved_memory=0
    local t_memory=$total_memory

    if (($t_memory <= 4096)); then # 8% of the first 4GB of memory
        recommended_systemreserved_memory=$(echo $t_memory 0.08 | awk '{print $1 * $2}')
        t_memory=0
    else
        recommended_systemreserved_memory=333
        t_memory=$((t_memory-4096))
    fi
    if (($t_memory <= 4096)); then # 6% of the next 4GB of memory (up to 8GB)
        recommended_systemreserved_memory=$(echo $recommended_systemreserved_memory $(echo $t_memory 0.06 | awk '{print $1 * $2}') | awk '{print $1 + $2}')
        t_memory=0
    else
        recommended_systemreserved_memory=$(echo $recommended_systemreserved_memory 252 | awk '{print $1 + $2}')
        t_memory=$((t_memory-4096))
    fi
    if (($t_memory <= 8192)); then # 3% of the next 8GB of memory (up to 16GB)
        recommended_systemreserved_memory=$(echo $recommended_systemreserved_memory $(echo $t_memory 0.03 | awk '{print $1 * $2}') | awk '{print $1 + $2}')
        t_memory=0
    else
        recommended_systemreserved_memory=$(echo $recommended_systemreserved_memory 246 | awk '{print $1 + $2}')
        t_memory=$((t_memory-8192))
    fi
    if (($t_memory <= 114688)); then # 2% of the next 112GB of memory (up to 128GB)
        recommended_systemreserved_memory=$(echo $recommended_systemreserved_memory $(echo $t_memory 0.02 | awk '{print $1 * $2}') | awk '{print $1 + $2}')
        t_memory=0
    else
        recommended_systemreserved_memory=$(echo $recommended_systemreserved_memory 2240 | awk '{print $1 + $2}')
        t_memory=$((t_memory-114688))
    fi
    if (($t_memory >= 0)); then # 1% of any memory above 128GB
        recommended_systemreserved_memory=$(echo $recommended_systemreserved_memory $(echo $t_memory 0.01 | awk '{print $1 * $2}') | awk '{print $1 + $2}')
    fi
    recommended_systemreserved_memory=$(resources_management_memory_units_to_bytes "${recommended_systemreserved_memory}Mi")
    echo -n "${recommended_systemreserved_memory}"
}
{{- else if eq $resourceReservationMode "Static" }}
function dynamic_memory_sizing {
  echo -n "$(resources_management_memory_units_to_bytes {{ dig "kubelet" "resourceReservation" "static" "memory" 0 .nodeGroup }})"
}
{{- end }}

function eviction_hard_threshold_memory_available {
  return=$(resources_management_memory_units_to_bytes "${total_memory}Mi" 0.01)
  echo -n "${return}"
}

function eviction_soft_threshold_memory_available {
  return=$(resources_management_memory_units_to_bytes "${total_memory}Mi" 0.02)
  echo -n "${return}"
}

{{- if eq $resourceReservationMode "Auto" }}
# https://cloud.google.com/kubernetes-engine/docs/concepts/plan-node-sizes ("CPU reservations"), also used by EKS:
# 6% of the first core, 1% of the second, 0.5% of the third and fourth, 0.25% of every core above four.
# Online CPUs are counted, as kubelet counts them for the node capacity.
function dynamic_cpu_sizing {
  awk -v cores="$(getconf _NPROCESSORS_ONLN)" 'BEGIN {
    millicores = 60
    if (cores > 1) millicores += 10
    if (cores > 2) millicores += 5 * ((cores < 4 ? cores : 4) - 2)
    if (cores > 4) millicores += 2.5 * (cores - 4)
    printf "%dm", millicores
  }'
}

# CPU requested by the pods of the node, in millicores. Over-counts on purpose: init containers and
# pod-level requests are added instead of taking the larger, so a wrong answer only keeps the old
# reservation longer, it never lets kubelet reject a pod.
function node_cpu_requests {
  local node pods
  node="$(bb-d8-node-name)" && [[ -n "${node}" ]] || return 1
  pods="$(bb-curl-kube /api/v1/pods --get --data-urlencode "fieldSelector=spec.nodeName=${node}")" || return 1
  jq -er '
    def millicores: tostring | if endswith("m") then .[:-1] | tonumber else tonumber * 1000 end;
    [.items[] | select(.status.phase != "Succeeded" and .status.phase != "Failed") | .spec
      | (.containers + (.initContainers // []))[].resources.requests.cpu, .resources.requests.cpu, .overhead.cpu
      | values | millicores] | add // 0 | ceil' <<< "${pods}"
}

# A kubelet restart re-admits every pod bound to the node against the new allocatable and rejects
# (or, for a critical pod, preempts) whatever no longer fits. So a changed reservation is written
# only while the pods fit; otherwise the applied one stays until bashible runs this step again
# (next configuration change or reboot).
function system_reserved_cpu {
  local desired applied requested allocatable
  desired="$(dynamic_cpu_sizing)"
  applied="$(/opt/deckhouse/bin/yq '.systemReserved.cpu // ""' /var/lib/kubelet/config.yaml 2>/dev/null)"
  # Nothing to keep: first run, or no cpu reserved yet (0m would also break the static CPU manager).
  if [[ -z "${applied}" || "${applied}" == "${desired}" ]]; then
    echo -n "${desired}"
    return 0
  fi
  allocatable=$(( $(getconf _NPROCESSORS_ONLN) * 1000 - ${desired%m} ))
  if ! requested="$(node_cpu_requests)"; then
    bb-log-warning "systemReserved.cpu stays at '${applied}' instead of '${desired}': cannot get the pods of the node to check that they fit"
  elif (( requested <= allocatable )); then
    echo -n "${desired}"
    return 0
  else
    bb-log-warning "systemReserved.cpu stays at '${applied}' instead of '${desired}': pods request ${requested}m of ${allocatable}m allocatable; retried on the next configuration change or reboot"
  fi
  echo -n "${applied}"
}
{{- end }}

{{- $topologyManager := dig "kubelet" "topologyManager" (dict) .nodeGroup }}
{{- $topologyPolicy := dig "policy" "None" $topologyManager }}
{{- $pinning := ne $topologyPolicy "None" }}
{{- $fullPCPUs := has $topologyPolicy (list "Restricted" "SingleNumaNode") }}

system_reserved_memory=0
{{- if or (eq $resourceReservationMode "Auto") (eq $resourceReservationMode "Static") }}
system_reserved_memory=$(dynamic_memory_sizing)
{{- end }}
eviction_hard_memory=$(eviction_hard_threshold_memory_available)
eviction_soft_memory=$(eviction_soft_threshold_memory_available)
# failed $() inside these functions leaves "" and kubelet would crashloop
for value in "${system_reserved_memory}" "${eviction_hard_memory}" "${eviction_soft_memory}"; do
  if [[ ! "${value}" =~ ^[0-9]+$ ]]; then
    bb-log-error "cannot calculate kubelet memory reservations"
    exit 1
  fi
done

{{- if $pinning }}
{{- if not (or (eq $resourceReservationMode "Auto") (and (eq $resourceReservationMode "Static") (hasKey (dig "kubelet" "resourceReservation" "static" (dict) .nodeGroup) "cpu"))) }}
bb-log-error "CPU pinning requires resourceReservation Auto, or Static with static.cpu"
exit 1
{{- end }}

numa_sysfs="${D8_NUMA_SYSFS_DIR:-/sys/devices/system/node}"

function numa_node_ids {
  local dir
  for dir in "${numa_sysfs}"/node[0-9]*; do
    if [[ -d "${dir}" ]]; then
      echo "${dir##*/node}"
    fi
  done | sort -n
}
numa_ids=($(numa_node_ids))

# Split reserve by zone ordinary memory (MemTotal minus hugepages).
# Weights in GiB: small MemTotal/hugepages drift must not change reservedMemory, it means drain.
function numa_reserved_memory {
  local reserve="$1" id kb pages size count i share assigned=0 total_weight=0
  local -a ids=() weights=() usable=()
  for id in "${numa_ids[@]}"; do
    # same match as cAdvisor, so a format kubelet still reads is read here too
    kb="$(sed -n 's/.*MemTotal:[[:space:]]*\([0-9][0-9]*\) kB.*/\1/p' "${numa_sysfs}/node${id}/meminfo" 2>/dev/null | head -n 1)" || kb=""
    if [[ ! "${kb}" =~ ^[0-9]+$ ]]; then
      bb-log-error "cannot read MemTotal of NUMA node ${id}"
      return 1
    fi
    for pages in "${numa_sysfs}/node${id}"/hugepages/hugepages-*kB/nr_hugepages; do
      if [[ -f "${pages}" ]]; then
        size="${pages%kB/nr_hugepages}"
        size="${size##*hugepages-}"
        count="$(< "${pages}")" || count=""
        if [[ ! "${size}" =~ ^[0-9]+$ || ! "${count}" =~ ^[0-9]+$ ]]; then
          bb-log-error "cannot read hugepages of NUMA node ${id} from ${pages}"
          return 1
        fi
        kb=$(( kb - size * count ))
      fi
    done
    if (( kb >= 1048576 )); then
      ids+=("${id}")
      weights+=("$(( kb / 1048576 ))")
      usable+=("$(( kb * 1024 ))")
      total_weight=$(( total_weight + kb / 1048576 ))
    fi
  done
  if (( total_weight == 0 )); then
    bb-log-error "no NUMA nodes with ordinary memory in ${numa_sysfs}"
    return 1
  fi
  echo "reservedMemory:"
  for i in "${!ids[@]}"; do
    # last zone takes remainder: kubelet wants exact sum
    if (( i == ${#ids[@]} - 1 )); then
      share=$(( reserve - assigned ))
    else
      share=$(( reserve * weights[i] / total_weight ))
    fi
    if (( share >= usable[i] )); then
      bb-log-error "reserved memory ${share} exhausts NUMA node ${ids[i]}"
      return 1
    fi
    assigned=$(( assigned + share ))
    if (( share > 0 )); then
      printf -- '- numaNode: %s\n  limits:\n    memory: "%s"\n' "${ids[i]}" "${share}"
    fi
  done
}
numa_reserved_memory_yaml="$(numa_reserved_memory "$(( system_reserved_memory + eviction_hard_memory ))")"

function numa_topology_options {
  local count=${#numa_ids[@]} i closest="$1"
  local -a row
  if (( count == 0 )); then
    closest=false
  fi
  # kubelet indexes distances by NUMA ID: need IDs 0..N-1 and full rows, else it fails to start
  for i in "${!numa_ids[@]}"; do
    row=($(cat "${numa_sysfs}/node${i}/distance" 2>/dev/null))
    if [[ "${closest}" == true ]] && { [[ "${numa_ids[i]}" != "${i}" ]] || (( ${#row[@]} != count )); }; then
      bb-log-warning "NUMA distances unusable, prefer-closest-numa-nodes is not set"
      closest=false
    fi
  done
  if [[ "${closest}" == true ]] || (( count > 8 )); then
    echo "topologyManagerPolicyOptions:"
  fi
  if [[ "${closest}" == true ]]; then
    echo '  prefer-closest-numa-nodes: "true"'
  fi
  # kubelet refuses policy != none on >8 zones without it, values <8 are rejected
  if (( count > 8 )); then
    echo "  max-allowable-numa-nodes: \"${count}\""
  fi
}
numa_topology_options_yaml="$(numa_topology_options {{ eq $topologyPolicy "Restricted" }})"
{{- end }}

new_kubelet_config="$(bb-tmp-file)"
cat > "${new_kubelet_config}" << EOF
apiVersion: kubelet.config.k8s.io/v1beta1
kind: KubeletConfiguration
authentication:
  x509:
    clientCAFile: /etc/kubernetes/pki/ca.crt
  webhook:
    enabled: true
    cacheTTL: 2m0s
  anonymous:
    enabled: false
authorization:
  mode: Webhook
  webhook:
    cacheAuthorizedTTL: 5m0s
    cacheUnauthorizedTTL: 30s
cgroupRoot: "/"
cgroupsPerQOS: true
cgroupDriver: ${cgroup_driver}
{{- if eq .runType "Normal" }}
clusterDomain: {{ .normal.clusterDomain }}
clusterDNS:
- {{ .normal.clusterDNSAddress }}
{{- end }}
{{- if eq .runType "ClusterBootstrap" }}
clusterDomain: {{ .clusterBootstrap.clusterDomain }}
clusterDNS:
- {{ .clusterBootstrap.clusterDNSAddress }}
{{- end }}
cpuManagerReconcilePeriod: 10s
enableControllerAttachDetach: true
enableDebuggingHandlers: true
enableServer: true
enforceNodeAllocatable:
- pods
eventRecordQPS: 50
eventBurst: 50
evictionHard:
  imagefs.available: $evictionHardThresholdImagefsAvailable
  imagefs.inodesFree: $evictionHardThresholdImagefsInodesFree
  memory.available: "${eviction_hard_memory}"
  nodefs.available: $evictionHardThresholdNodefsAvailable
  nodefs.inodesFree: $evictionHardThresholdNodefsInodesFree
evictionSoft:
  imagefs.available: $evictionSoftThresholdImagefsAvailable
  imagefs.inodesFree: $evictionSoftThresholdImagefsInodesFree
  memory.available: "${eviction_soft_memory}"
  nodefs.available: $evictionSoftThresholdNodefsAvailable
  nodefs.inodesFree: $evictionSoftThresholdNodefsInodesFree
evictionSoftGracePeriod:
  imagefs.available: 1m30s
  imagefs.inodesFree: 1m30s
  memory.available: 1m30s
  nodefs.available: 1m30s
  nodefs.inodesFree: 1m30s
evictionPressureTransitionPeriod: 4m0s
evictionMaxPodGracePeriod: 90
evictionMinimumReclaim: null
{{- if ((.nodeGroup).kubelet).memorySwap }}
  {{- $swapBehavior := .nodeGroup.kubelet.memorySwap.swapBehavior | default "NoSwap" }}
failSwapOn: false
memorySwap:
  swapBehavior: {{ $swapBehavior }}
{{- else }}
failSwapOn: true
{{- end }}
{{- if semverCompare ">=1.35" .kubernetesVersion }}
failCgroupV1: false
{{- end }}
seccompDefault: {{ dig "kubelet" "seccompDefault" false .nodeGroup }}
tlsCipherSuites: ["TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256","TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256","TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305","TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384","TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305","TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384","TLS_RSA_WITH_AES_256_GCM_SHA384","TLS_RSA_WITH_AES_128_GCM_SHA256"]
{{- if ne .runType "ClusterBootstrap" }}
# serverTLSBootstrap flag should be enable after bootstrap of first master.
# This flag affects logs from kubelet, for period of time between kubelet start and certificate request approve by Deckhouse hook.
serverTLSBootstrap: true
{{- end }}
{{/*
RotateKubeletServerCertificate default is true, but CIS becnhmark wants it to be explicitly enabled
https://kubernetes.io/docs/reference/command-line-tools-reference/feature-gates/
*/}}
featureGates:
  RotateKubeletServerCertificate: true
{{- if semverCompare ">=1.33 <1.34" .kubernetesVersion }}
{{- /* DynamicResourceAllocation: GA default=true since 1.34, explicitly enable for 1.33 */}}
  DynamicResourceAllocation: true
{{- end }}
{{- if semverCompare ">=1.33" .kubernetesVersion }}
{{- /* DRAPartitionableDevices: Alpha in 1.33 (for NodeSelector per device) */}}
  DRAPartitionableDevices: true
  KubeletEnsureSecretPulledImages: true
{{- end }}
{{- if semverCompare ">=1.34" .kubernetesVersion }}
{{- /* DRADeviceBindingConditions, DRAConsumableCapacity: Alpha in 1.34 (multi-allocations: BindsToNode, AllowMultipleAllocations). */}}
  DRADeviceBindingConditions: true
  DRAConsumableCapacity: true
{{- end }}
{{- if semverCompare ">=1.34 <1.37" .kubernetesVersion }}
{{- /* DRAExtendedResource: Alpha in 1.34, GA and locked to default=true since 1.37. */}}
  DRAExtendedResource: true
{{- end }}
{{- range .allowedKubeletFeatureGates }}
  {{ . }}: true
{{- end }}
fileCheckFrequency: 2s
imageMinimumGCAge: 2m0s
imageGCHighThresholdPercent: 70
imageGCLowThresholdPercent: 65
kubeAPIBurst: 50
kubeAPIQPS: 50
hairpinMode: promiscuous-bridge
httpCheckFrequency: 20s
maxOpenFiles: 1000000
{{- $max_pods := 120 }}
{{- if (((.nodeGroup).kubelet).maxPods) }}
  {{- $max_pods = .nodeGroup.kubelet.maxPods | int }}
{{- else }}
  {{- $prefix := .normal.podSubnetNodeCIDRPrefix | default "24" | int }}
  {{- if ge $prefix 24 }}
    {{- $max_pods = 120 }}
  {{- else if eq $prefix 23 }}
    {{- $max_pods = 250 }}
  {{- else if eq $prefix 22 }}
    {{- $max_pods = 500 }}
  {{- else if le $prefix 21 }}
    {{- $max_pods = 1000 }}
  {{- end }}
{{- end }}
maxPods: {{ $max_pods }}
nodeStatusReportFrequency: 1m
nodeStatusUpdateFrequency: {{ .nodeStatusUpdateFrequency | default "10" }}s
podsPerCore: 0
podPidsLimit: -1
readOnlyPort: 0
registryPullQPS: 10
registryBurst: 20
resolvConf: ${resolvConfPath}
rotateCertificates: true
runtimeRequestTimeout: 4m0s
serializeImagePulls: false
syncFrequency: 1m0s
{{- if eq $resourceReservationMode "Auto" }}
systemReserved:
  cpu: "$(system_reserved_cpu)"
  memory: "${system_reserved_memory}"
  ephemeral-storage: 1Gi
{{- else if eq $resourceReservationMode "Static" }}
systemReserved:
  {{- if hasKey .nodeGroup "kubelet" }}
    {{- if hasKey .nodeGroup.kubelet "resourceReservation" }}
      {{- if hasKey .nodeGroup.kubelet.resourceReservation "static" }}
        {{- if hasKey .nodeGroup.kubelet.resourceReservation.static "cpu" }}
  cpu: {{ .nodeGroup.kubelet.resourceReservation.static.cpu | quote }}
        {{- end }}
        {{- if hasKey .nodeGroup.kubelet.resourceReservation.static "memory" }}
  memory: "${system_reserved_memory}"
        {{- end }}
        {{- if hasKey .nodeGroup.kubelet.resourceReservation.static "ephemeralStorage" }}
  ephemeral-storage: {{ .nodeGroup.kubelet.resourceReservation.static.ephemeralStorage | quote }}
        {{- end }}
      {{- end }}
    {{- end }}
  {{- end }}
{{- end }}
volumeStatsAggPeriod: 1m0s
healthzBindAddress: 127.0.0.1
healthzPort: 10248
protectKernelDefaults: true
containerLogMaxSize: {{ .nodeGroup.kubelet.containerLogMaxSize | default "50Mi" }}
containerLogMaxFiles: {{ .nodeGroup.kubelet.containerLogMaxFiles | default 4 }}
allowedUnsafeSysctls:  ["net.*"]
shutdownGracePeriodByPodPriority:
- priority: 2000000999
  shutdownGracePeriodSeconds: ${shutdownGracePeriodNodeCriticalPods}
- priority: 1999999999
  shutdownGracePeriodSeconds: ${shutdownGracePeriodCriticalPods}
- priority: 0
  shutdownGracePeriodSeconds: ${shutdownGracePeriod}
{{- if hasKey .nodeGroup "staticInstances" }}
providerID: $(cat /var/lib/bashible/node-spec-provider-id)
{{- end }}
{{- if $pinning }}
cpuManagerPolicy: static
{{- if $fullPCPUs }}
cpuManagerPolicyOptions:
  full-pcpus-only: "true"
{{- end }}
memoryManagerPolicy: Static
${numa_reserved_memory_yaml}
${numa_topology_options_yaml}
topologyManagerScope: {{ dig "scope" "Pod" $topologyManager | kebabcase }}
topologyManagerPolicy: {{ $topologyPolicy | kebabcase }}
{{- end }}
EOF
{{- if $pinning }}

# Changing these makes kubelet re-admit running pods under the new policy.
# CPU counts as whole reserved CPUs, rounded up as the CPU manager.
function kubelet_pinning_fields {
  /opt/deckhouse/bin/yq -o=json -I=0 '[.cpuManagerPolicy, .cpuManagerPolicyOptions, .memoryManagerPolicy, .reservedMemory, .systemReserved.cpu, .systemReserved.memory, .topologyManagerPolicy, .topologyManagerScope] | sort_keys(..)' "$1" |
    jq -c '.[4] |= (if . == null then 0 elif type == "number" then ceil elif endswith("m") then (sub("m$"; "") | tonumber | . / 1000 | ceil) else (tonumber | ceil) end)'
}

# Unknown answer fails the step before config is written: bashible retries it, no blind drain.
# DaemonSet and mirror pods skipped: drain leaves them anyway.
function node_has_guaranteed_pods {
  local node pods rc=0
  node="$(bb-d8-node-name)"
  if [[ -z "${node}" ]]; then
    bb-log-error "cannot get node name to check Guaranteed pods"
    exit 1
  fi
  if ! pods="$(bb-curl-kube /api/v1/pods --get --data-urlencode "fieldSelector=spec.nodeName=${node}")"; then
    bb-log-error "cannot list pods of node ${node} to decide on drain"
    exit 1
  fi
  jq -e 'any(.items[]; .status.phase == "Running" and .status.qosClass == "Guaranteed"
    and .metadata.annotations["kubernetes.io/config.mirror"] == null
    and all(.metadata.ownerReferences[]?; .kind != "DaemonSet"))' <<< "${pods}" > /dev/null || rc=$?
  if (( rc > 1 )); then
    bb-log-error "cannot parse pods of node ${node} to decide on drain"
    exit 1
  fi
  (( rc == 0 ))
}

# first run: approval is skipped anyway, API may be unreachable
if [[ "${FIRST_BASHIBLE_RUN:-}" != "yes" ]] &&
   [[ -f /var/lib/kubelet/config.yaml ]] &&
   [[ "$(kubelet_pinning_fields /var/lib/kubelet/config.yaml 2>/dev/null)" != "$(kubelet_pinning_fields "${new_kubelet_config}")" ]] &&
   node_has_guaranteed_pods; then
  bb-deckhouse-get-disruptive-update-approval
fi
{{- end }}

bb-sync-file /var/lib/kubelet/config.yaml "${new_kubelet_config}"

# CIS becnhmark purposes
chmod 600 /var/lib/kubelet/config.yaml
