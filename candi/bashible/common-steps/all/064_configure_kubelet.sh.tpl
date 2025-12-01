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

{{- $topologyManagerEnabled := dig "kubelet" "topologyManager" "enabled" false .nodeGroup }}
{{- if eq $topologyManagerEnabled true }}
function reserved_memory {
  return=$(echo $(dynamic_memory_sizing) $(eviction_hard_threshold_memory_available) | awk '{print $1 + $2}')
  echo -n "${return}"
}
{{- end }}

bb-sync-file /var/lib/kubelet/config.yaml - << EOF
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
  memory.available: "$(eviction_hard_threshold_memory_available)"
  nodefs.available: $evictionHardThresholdNodefsAvailable
  nodefs.inodesFree: $evictionHardThresholdNodefsInodesFree
evictionSoft:
  imagefs.available: $evictionSoftThresholdImagefsAvailable
  imagefs.inodesFree: $evictionSoftThresholdImagefsInodesFree
  memory.available: "$(eviction_soft_threshold_memory_available)"
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
{{- $swapBehavior := dig "kubelet" "memorySwap" "swapBehavior" "" .nodeGroup }}
{{- if eq $swapBehavior "" }}
failSwapOn: true
{{- else }}
failSwapOn: false
memorySwap:
  swapBehavior: {{ $swapBehavior }}
{{- end }}
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
{{- if semverCompare "< 1.30" .kubernetesVersion }}
  ValidatingAdmissionPolicy: true
{{- end }}
  RotateKubeletServerCertificate: true
{{- if eq $topologyManagerEnabled true }}
  MemoryManager: true
{{- end }}
{{- if semverCompare ">=1.32 <1.34" .kubernetesVersion }}
  DynamicResourceAllocation: true
{{- end }}
{{- if and (ne $swapBehavior "") (semverCompare "< 1.34" .kubernetesVersion) }}
  NodeSwap: true
{{- end }}
{{- range .allowedKubeletFeatureGates }}
  {{ . }}: true
{{- end }}
fileCheckFrequency: 20s
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
nodeStatusUpdateFrequency: {{ .nodeStatusUpdateFrequency | default "10" }}s
podsPerCore: 0
podPidsLimit: -1
readOnlyPort: 0
registryPullQPS: 10
registryBurst: 20
resolvConf: ${resolvConfPath}
rotateCertificates: true
runtimeRequestTimeout: 2m0s
serializeImagePulls: true
syncFrequency: 1m0s
{{- if eq $resourceReservationMode "Auto" }}
systemReserved:
  cpu: 70m
  memory: "$(dynamic_memory_sizing)"
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
  memory: "$(dynamic_memory_sizing)"
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
  shutdownGracePeriodSeconds: 5
- priority: 1999999999
  shutdownGracePeriodSeconds: ${shutdownGracePeriodCriticalPods}
- priority: 0
  shutdownGracePeriodSeconds: ${shutdownGracePeriod}
{{- if hasKey .nodeGroup "staticInstances" }}
providerID: $(cat /var/lib/bashible/node-spec-provider-id)
{{- end }}
{{- if eq $topologyManagerEnabled true }}
cpuManagerPolicy: static
memoryManagerPolicy: Static
reservedMemory:
- numaNode: 0
  limits:
    memory: "$(reserved_memory)"
topologyManagerScope: {{ dig "kubelet" "topologyManager" "scope" "Container" .nodeGroup | kebabcase }}
topologyManagerPolicy: {{ dig "kubelet" "topologyManager" "policy" "None" .nodeGroup | kebabcase }}
{{- end }}
EOF

# CIS becnhmark purposes
chmod 600 /var/lib/kubelet/config.yaml
