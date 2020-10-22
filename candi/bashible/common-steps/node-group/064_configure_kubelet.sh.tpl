bb-event-on 'bb-sync-file-changed' 'bb-flag-set kubelet-need-restart'

# Detect systemd-resolved
if grep -q '^nameserver 127.0.0.53' /etc/resolv.conf ; then
  resolvConfPath="/run/systemd/resolve/resolv.conf"
else
  resolvConfPath="/etc/resolv.conf"
fi

# This folder doesn't have time to create before we stop freshly, unconfigured kubelet during bootstrap (step 034_install_kubelet_and_his_friends.sh).
mkdir -p /var/lib/kubelet

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

rootDir="{{ .nodeGroup.kubelet.rootDir | default "/var/lib/kubelet" }}"

dockerDir=$(docker info --format '{{`{{.DockerRootDir}}`}}')
if [ -d "${dockerDir}/overlay2" ]; then
  dockerDir="${dockerDir}/overlay2"
else
  if [ -d "${dockerDir}/aufs" ]; then
    dockerDir="${dockerDir}/aufs"
  fi
fi

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

imagefsSize=$(df --output=size $dockerDir | tail -n1)
imagefsSizeGFivePercent=$((imagefsSize/(1000*1000)*5/100))
if [ "$imagefsSizeGFivePercent" -gt "$maxAvailableReservedSpace" ]; then
  evictionHardThresholdImagefsAvailable="${maxAvailableReservedSpace}G"
fi
if [ "$(($imagefsSizeGFivePercent*2))" -gt "$(($maxAvailableReservedSpace*2))" ]; then
  evictionSoftThresholdImagefsAvailable="$(($maxAvailableReservedSpace*2))G"
fi

imagefsInodes=$(df --output=itotal $dockerDir | tail -n1)
imagefsInodesKFivePercent=$((imagefsInodes/1000*5/100))
if [ "$imagefsInodesKFivePercent" -gt "$needInodesFree" ]; then
  evictionHardThresholdImagefsInodesFree="${needInodesFree}k"
fi
if [ "$(($imagefsInodesKFivePercent*2))" -gt "$(($needInodesFree*2))" ]; then
  evictionSoftThresholdImagefsInodesFree="$(($needInodesFree*2))k"
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
cgroupDriver: cgroupfs
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
configTrialDuration: 10m0s
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
featureGates:
  ExpandCSIVolumes: true
fileCheckFrequency: 20s
imageMinimumGCAge: 2m0s
imageGCHighThresholdPercent: 50
imageGCLowThresholdPercent: 40
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
nodeStatusUpdateFrequency: 10s
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
EOF
