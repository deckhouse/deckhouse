{{- if eq .runType "Normal" }}
if bb-flag? is-bootstrapped; then exit 0; fi

{{ if eq .nodeGroup.nodeType "Static" }}
function is_ip_in_net() {
  ip="$1"
  IFS="/" read net_address net_prefix <<< "$2"

  IFS=. read -r a b c d <<< "$ip"
  ip_dec="$((a * 256 ** 3 + b * 256 ** 2 + c * 256 + d))"

  IFS=. read -r a b c d <<< "$net_address"
  net_address_dec="$((a * 256 ** 3 + b * 256 ** 2 + c * 256 + d))"

  netmask=$(((0xFFFFFFFF << (32 - net_prefix)) & 0xFFFFFFFF))

  test $((netmask & ip_dec)) -eq $((netmask & net_address_dec))
}

ip_in_system=$(ip -f inet -br -j addr | jq -r '.[] | .addr_info[] | .local')

node_ip=""
for ip in $ip_in_system; do
  if is_ip_in_net "$ip" "{{ .nodeGroup.internalNetworkCIDRs | join " " }}"; then
    node_ip=$ip
  fi
done

if [ -z "$node_ip" ]; then
  >&2 echo "ERROR: Can't find any local ip in CIDR {{ .nodeGroup.internalNetworkCIDRs }}"
  exit 1
fi
{{ end }}

cat << "EOF" > /etc/systemd/system/kubelet.service.d/cim.conf
[Service]
EnvironmentFile=-/etc/sysconfig/kubelet
ExecStart=
ExecStart=/usr/bin/kubelet \
{{- if ne .nodeGroup.nodeType "Static" }}
    --register-with-taints=node.flant.com/csi-not-bootstrapped=:NoSchedule \
{{- end }}
    --bootstrap-kubeconfig=/etc/kubernetes/bootstrap-kubelet.conf \
    --config=/var/lib/kubelet/config.yaml \
    --cni-bin-dir=/opt/cni/bin/ \
    --cni-conf-dir=/etc/cni/net.d/ \
    --kubeconfig=/etc/kubernetes/kubelet.conf \
    --network-plugin=cni \
{{- if eq .nodeGroup.nodeType "Static" }}
    --node-ip=$node_ip \
{{- else }}
    --cloud-provider=external \
    --pod-manifest-path=/etc/kubernetes/manifests \
{{- end }}
    --v=2 $KUBELET_EXTRA_ARGS
EOF

cat << "EOF" > /var/lib/kubelet/config.yaml
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
clusterDomain: {{ .normal.clusterDomain }}
clusterDNS:
- {{ .normal.clusterDNSAddress }}
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
resolvConf: /etc/resolv.conf
rotateCertificates: true
runtimeRequestTimeout: 2m0s
serializeImagePulls: true
syncFrequency: 1m0s
volumeStatsAggPeriod: 1m0s
EOF
{{- end }}
