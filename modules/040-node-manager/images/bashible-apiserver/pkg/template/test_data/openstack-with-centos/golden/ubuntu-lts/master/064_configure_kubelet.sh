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
rootDir="/var/lib/kubelet"

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
criDir=$(/usr/local/bin/crictl info -o json | jq -r '.config.containerdRootDir')

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
cgroupDriver: systemd
clusterDomain: cluster.local
clusterDNS:
- 10.222.0.10
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
# serverTLSBootstrap flag should be enable after bootstrap of first master.
# This flag affects logs from kubelet, for period of time between kubelet start and certificate request approve by Deckhouse hook.
serverTLSBootstrap: true
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
maxPods: 110
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
protectKernelDefaults: true
containerLogMaxSize: 10Mi
containerLogMaxFiles: 5
EOF
