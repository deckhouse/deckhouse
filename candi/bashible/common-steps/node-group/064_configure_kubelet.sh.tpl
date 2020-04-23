bb-event-on 'bb-sync-file-changed' 'bb-flag-set kubelet-need-restart'

bb-sync-file /var/lib/kubelet/config.yaml - << "EOF"
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
  imagefs.available: 5%
  imagefs.inodesFree: 5%
  memory.available: 1%
  nodefs.available: 5%
  nodefs.inodesFree: 5%
evictionSoft:
  imagefs.available: 10%
  imagefs.inodesFree: 10%
  memory.available: 2%
  nodefs.available: 10%
  nodefs.inodesFree: 10%
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
maxPods: 110
nodeStatusUpdateFrequency: 10s
podsPerCore: 0
podPidsLimit: -1
readOnlyPort: 0
registryPullQPS: 5
registryBurst: 10
resolvConf: /run/systemd/resolve/resolv.conf
rotateCertificates: true
runtimeRequestTimeout: 2m0s
serializeImagePulls: true
syncFrequency: 1m0s
volumeStatsAggPeriod: 1m0s
EOF
