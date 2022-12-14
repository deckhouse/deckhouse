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
# cgroup default is `systemd`, only for docker cri we use `cgroupfs`.
cgroup_driver="systemd"
{{- if eq .cri "Containerd" }}
# Overriding cgroup type from external config file
if [ -f /var/lib/bashible/cgroup_config ]; then
  cgroup_driver="$(cat /var/lib/bashible/cgroup_config)"
fi
{{- end }}

{{- if eq .cri "NotManaged" }}
  {{- if .nodeGroup.cri.notManaged.criSocketPath }}
cri_socket_path={{ .nodeGroup.cri.notManaged.criSocketPath | quote }}
  {{- else }}
for socket_path in /var/run/docker.sock /run/containerd/containerd.sock; do
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

if grep -q "docker" <<< "${cri_socket_path}"; then
  cri_type="NotManagedDocker"
else
  cri_type="NotManagedContainerd"
fi
{{- else if eq .cri "Docker" }}
cri_type="Docker"
{{- else }}
cri_type="Containerd"
{{- end }}

if [[ "${cri_type}" == "Docker" || "${cri_type}" == "NotManagedDocker" ]]; then
  cgroup_driver="cgroupfs"
  criDir=$(docker info --format '{{`{{.DockerRootDir}}`}}')
  if [ -d "${criDir}/overlay2" ]; then
    criDir="${criDir}/overlay2"
  else
    if [ -d "${criDir}/aufs" ]; then
      criDir="${criDir}/aufs"
    fi
  fi
fi

if [[ "${cri_type}" == "Containerd" || "${cri_type}" == "NotManagedContainerd" ]]; then
  criDir=$(crictl info -o json | jq -r '.config.containerdRootDir')
fi

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

imagefsSize=$(df --output=size $criDir | tail -n1)
imagefsSizeGFivePercent=$((imagefsSize/(1000*1000)*5/100))
if [ "$imagefsSizeGFivePercent" -gt "$maxAvailableReservedSpace" ]; then
  evictionHardThresholdImagefsAvailable="${maxAvailableReservedSpace}G"
fi
if [ "$(($imagefsSizeGFivePercent*2))" -gt "$(($maxAvailableReservedSpace*2))" ]; then
  evictionSoftThresholdImagefsAvailable="$(($maxAvailableReservedSpace*2))G"
fi

imagefsInodes=$(df --output=itotal $criDir | tail -n1)
imagefsInodesKFivePercent=$((imagefsInodes/1000*5/100))
if [ "$imagefsInodesKFivePercent" -gt "$needInodesFree" ]; then
  evictionHardThresholdImagefsInodesFree="${needInodesFree}k"
fi
if [ "$(($imagefsInodesKFivePercent*2))" -gt "$(($needInodesFree*2))" ]; then
  evictionSoftThresholdImagefsInodesFree="$(($needInodesFree*2))k"
fi

shutdownGracePeriod="2m"
shutdownGracePeriodCriticalPods="15s"

if [[ -f /var/lib/bashible/cloud-provider-variables ]]; then
  source /var/lib/bashible/cloud-provider-variables

  if [[ -n "$shutdown_grace_period" ]]; then
    shutdownGracePeriod="$shutdown_grace_period"
  fi
  if [[ -n "$shutdown_grace_period_critical_pods" ]]; then
    shutdownGracePeriodCriticalPods="$shutdown_grace_period_critical_pods"
  fi
fi

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
  memory.available: 1%
  nodefs.available: $evictionHardThresholdNodefsAvailable
  nodefs.inodesFree: $evictionHardThresholdNodefsInodesFree
evictionSoft:
  imagefs.available: $evictionSoftThresholdImagefsAvailable
  imagefs.inodesFree: $evictionSoftThresholdImagefsInodesFree
  memory.available: 2%
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
failSwapOn: true
tlsCipherSuites: ["TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256","TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256","TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305","TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384","TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305","TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384","TLS_RSA_WITH_AES_256_GCM_SHA384","TLS_RSA_WITH_AES_128_GCM_SHA256"]
{{- if ne .runType "ClusterBootstrap" }}
# serverTLSBootstrap flag should be enable after bootstrap of first master.
# This flag affects logs from kubelet, for period of time between kubelet start and certificate request approve by Deckhouse hook.
serverTLSBootstrap: true
{{- end }}
featureGates:
  ExpandCSIVolumes: true
{{- if semverCompare "<1.23" .kubernetesVersion }}
  EphemeralContainers: true
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
{{- $max_pods := 110 }}
{{- if hasKey .nodeGroup "kubelet" }}
  {{- $max_pods = .nodeGroup.kubelet.maxPods | default $max_pods }}
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
volumeStatsAggPeriod: 1m0s
healthzBindAddress: 127.0.0.1
healthzPort: 10248
protectKernelDefaults: true
{{- if or (eq .cri "Containerd") (eq .cri "NotManaged") }}
containerLogMaxSize: {{ .nodeGroup.kubelet.containerLogMaxSize | default "50Mi" }}
containerLogMaxFiles: {{ .nodeGroup.kubelet.containerLogMaxFiles | default 4 }}
{{- end }}
allowedUnsafeSysctls:  ["net.*"]
shutdownGracePeriod: ${shutdownGracePeriod}
shutdownGracePeriodCriticalPods: ${shutdownGracePeriodCriticalPods}
EOF
