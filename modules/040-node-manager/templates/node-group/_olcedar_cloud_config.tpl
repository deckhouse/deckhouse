{{- /*
  Bootstrap userdata for a node of an olcedar (systemType=Immutable) group.

  Such a node has no bashible: the whole bootstrap is the nodeconfig.yaml the
  olcedar initramfs picks out of the userdata by basename — the desired state
  nodelet reconciles. The format is #cloud-config, but there is no cloud-init
  on the node: only write_files is read, so runcmd and friends would be ignored.

  One Secret serves every machine of the group, so the node name cannot be
  rendered here: __NODE_NAME__ is substituted on the node from the NoCloud
  meta-data. The placeholder is not a valid DNS1123 subdomain, so a node that
  failed to substitute it refuses the config instead of registering under a
  name shared with its neighbours.

  The install disk is not named either: the platform decides whether the
  cloud-init CDROM or the root disk comes first, so the disk is selected on the
  node — an empty diskSelector takes the first whole disk that is not a CDROM.
*/ -}}
{{- define "node_group_olcedar_cloud_config" }}
  {{- $context := index . 0 }}
  {{- $ng := index . 1 }}
  {{- $bootstrap_token := index . 2 -}}
  {{- $digests := $context.Values.global.modulesImages.digests.registrypackages -}}
  {{- /* kubeletSysext1356 carries the kubelet of Kubernetes 1.35.6. The values
       already carry the sha256: prefix the NodeConfig schema requires. */ -}}
  {{- $kubelet_sysext_key := printf "kubeletSysext%s" (replace "." "" $context.Values.global.discovery.kubernetesVersion) -}}
  {{- $kubelet_digest := index $digests $kubelet_sysext_key -}}
  {{- if not $kubelet_digest }}
    {{- fail (printf "no kubelet sysext package for Kubernetes %s (looked up %s)" $context.Values.global.discovery.kubernetesVersion $kubelet_sysext_key) }}
  {{- end -}}
  {{- /* The node reads the first line to decide whether this is userdata it can
         parse, so the document must start with the header and nothing before it. */ -}}
#cloud-config
write_files:
- path: /config/nodeconfig.yaml
  content: |
    apiVersion: internal.deckhouse.io/v1alpha1
    kind: NodeConfig
    metadata:
      name: __NODE_NAME__
      labels:
        node.deckhouse.io/group: {{ $ng.name }}
    spec:
      nodeName: __NODE_NAME__
      storage:
        diskSelector: {}
      # TODO: resolve the OS image from the release channel once it is published
      # there; the same pin lives in node-controller's nodeconfig controller.
      osImage: registry.deckhouse.io/deckhouse/olcedar@v0.1
      extensions:
      - name: containerd
        digest: {{ index $digests "containerdSysext224" }}
        requestedBy: node-manager
      - name: kubernetes-cni
        digest: {{ index $digests "kubernetesCniSysext162" }}
        requestedBy: node-manager
      - name: kubelet
        digest: {{ $kubelet_digest }}
        requestedBy: node-manager
      kernel:
        sysctl:
          net.ipv4.ip_forward: "1"
          vm.max_map_count: "262144"
          # kubelet refuses to start without these (protect-kernel-defaults).
          kernel.panic: "10"
          kernel.panic_on_oops: "1"
      network:
        hostname: __NODE_NAME__
        interfaces:
        - name: eth0
          dhcp: true
      kubelet:
        clusterDomain: {{ $context.Values.global.discovery.clusterDomain }}
        clusterDNS: ["{{ $context.Values.global.discovery.clusterDNSAddress }}"]
        caCert: {{ $context.Values.nodeManager.internal.kubernetesCA | b64enc }}
        bootstrapToken: {{ $bootstrap_token }}
        # Without it the node never gets a providerID, and CAPI cannot match the
        # Machine it ordered to the Node that registered.
        externalCloudProvider: true
        registerWithTaints:
        - key: node.deckhouse.io/uninitialized
          effect: NoSchedule
        nodeLabels:
          node.deckhouse.io/group: {{ $ng.name }}
          node.deckhouse.io/type: {{ $ng.nodeType }}
      apiServerEndpoints:
      {{- range $context.Values.nodeManager.internal.clusterMasterAddresses }}
      - "https://{{ . }}"
      {{- end }}
      registryPackagesProxyAccessTokenB64: {{ $context.Values.nodeManager.internal.packagesProxy.token | b64enc }}
{{- end }}
