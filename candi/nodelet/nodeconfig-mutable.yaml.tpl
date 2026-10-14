apiVersion: internal.deckhouse.io/v1alpha1
kind: NodeConfig
spec:
  systemType: Mutable
  featureGates:
    extensions: true
    static-pods: true
  extensions:
  - name: nodelet
    digest: {{ .images.registrypackages.nodelet }}
  - name: d8-ca-updater
    digest: {{ .images.registrypackages.d8CaUpdater }}
  - name: jq
    digest: {{ .images.registrypackages.jq }}
  - name: yq
    digest: {{ .images.registrypackages.yq }}
  - name: curl
    digest: {{ .images.registrypackages.d8Curl }}
  - name: which
    digest: {{ .images.registrypackages.which223 }}
  - name: virt-what
    digest: {{ .images.registrypackages.virtWhat125 }}
  - name: socat
    digest: {{ .images.registrypackages.socat1734 }}
  - name: e2fsprogs
    digest: {{ .images.registrypackages.e2fsprogs }}
  - name: iptables
    digest: {{ .images.registrypackages.iptables }}
  - name: growpart
    digest: {{ .images.registrypackages.growpart033 }}
  - name: lsblk
    digest: {{ .images.registrypackages.lsblk }}
  - name: nfs-mount
    digest: {{ .images.registrypackages.nfsMount }}
  - name: d8
    digest: {{ .images.registrypackages.d8 }}
  - name: kubernetes-cni
    digest: {{ .images.registrypackages.kubernetesCni191 }}
  - name: systemctl-power-commands-wrapper
    digest: {{ .images.registrypackages.systemctlPowerCommandsWrapper02 }}
  staticPods: []
