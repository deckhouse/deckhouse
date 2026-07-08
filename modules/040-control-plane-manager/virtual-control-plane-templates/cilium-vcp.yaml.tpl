---
# Source: cilium/templates/cilium-secrets-namespace.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: "cilium-secrets"
  labels:
    app.kubernetes.io/part-of: cilium
  annotations:

---
# Source: cilium/templates/cilium-agent/serviceaccount.yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: "cilium"
  namespace: kube-system

---
# Source: cilium/templates/cilium-operator/serviceaccount.yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: "cilium-operator"
  namespace: kube-system

---
# Source: cilium/templates/cilium-configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: cilium-config
  namespace: kube-system
data:

  # Identity allocation mode selects how identities are shared between cilium
  # nodes by setting how they are stored. The options are "crd", "kvstore" or
  # "doublewrite-readkvstore" / "doublewrite-readcrd".
  # - "crd" stores identities in kubernetes as CRDs (custom resource definition).
  #   These can be queried with:
  #     kubectl get ciliumid
  # - "kvstore" stores identities in an etcd kvstore, that is
  #   configured below. Cilium versions before 1.6 supported only the kvstore
  #   backend. Upgrades from these older cilium versions should continue using
  #   the kvstore by commenting out the identity-allocation-mode below, or
  #   setting it to "kvstore".
  # - "doublewrite" modes store identities in both the kvstore and CRDs. This is useful
  #   for seamless migrations from the kvstore mode to the crd mode. Consult the
  #   documentation for more information on how to perform the migration.
  identity-allocation-mode: crd

  identity-heartbeat-timeout: "30m0s"
  identity-gc-interval: "15m0s"
  cilium-endpoint-gc-interval: "5m0s"
  nodes-gc-interval: "5m0s"

  # If you want to run cilium in debug mode change this value to true
  debug: "false"
  metrics-sampling-interval: "5m"
  # The agent can be put into the following three policy enforcement modes
  # default, always and never.
  # https://docs.cilium.io/en/latest/security/policy/intro/#policy-enforcement-modes
  enable-policy: "default"
  # Port to expose Envoy metrics (e.g. "9964"). Envoy metrics listener will be disabled if this
  # field is not set.
  proxy-prometheus-port: "9964"
  # If you want metrics enabled in cilium-operator, set the port for
  # which the Cilium Operator will have their metrics exposed.
  # NOTE that this will open the port on the nodes where Cilium operator pod
  # is scheduled.
  operator-prometheus-serve-addr: ":9963"
  enable-metrics: "true"
  enable-policy-secrets-sync: "true"
  policy-secrets-only-from-secrets-namespace: "true"
  policy-secrets-namespace: "cilium-secrets"

  # Enable IPv4 addressing. If enabled, all endpoints are allocated an IPv4
  # address.
  enable-ipv4: "true"

  # Enable IPv6 addressing. If enabled, all endpoints are allocated an IPv6
  # address.
  enable-ipv6: "false"
  # Users who wish to specify their own custom CNI configuration file must set
  # custom-cni-conf to "true", otherwise Cilium may overwrite the configuration.
  custom-cni-conf: "false"
  enable-bpf-clock-probe: "false"
  # If you want cilium monitor to aggregate tracing for packets, set this level
  # to "low", "medium", or "maximum". The higher the level, the less packets
  # that will be seen in monitor output.
  monitor-aggregation: medium

  # The monitor aggregation interval governs the typical time between monitor
  # notification events for each allowed connection.
  #
  # Only effective when monitor aggregation is set to "medium" or higher.
  monitor-aggregation-interval: "5s"

  # The monitor aggregation flags determine which TCP flags which, upon the
  # first observation, cause monitor notifications to be generated.
  #
  # Only effective when monitor aggregation is set to "medium" or higher.
  monitor-aggregation-flags: all
  # Specifies the ratio (0.0-1.0] of total system memory to use for dynamic
  # sizing of the TCP CT, non-TCP CT, NAT and policy BPF maps.
  bpf-map-dynamic-size-ratio: "0.0025"
  # bpf-policy-map-max specifies the maximum number of entries in endpoint
  # policy map (per endpoint)
  bpf-policy-map-max: "16384"
  # bpf-policy-stats-map-max specifies the maximum number of entries in global
  # policy stats map
  bpf-policy-stats-map-max: "65536"
  # bpf-lb-map-max specifies the maximum number of entries in bpf lb service,
  # backend and affinity maps.
  bpf-lb-map-max: "65536"
  bpf-lb-external-clusterip: "false"
  bpf-lb-source-range-all-types: "false"
  bpf-lb-algorithm-annotation: "false"
  bpf-lb-mode-annotation: "false"

  bpf-distributed-lru: "false"
  bpf-events-drop-enabled: "true"
  bpf-events-policy-verdict-enabled: "true"
  bpf-events-trace-enabled: "true"

  # Pre-allocation of map entries allows per-packet latency to be reduced, at
  # the expense of up-front memory allocation for the entries in the maps. The
  # default value below will minimize memory usage in the default installation;
  # users who are sensitive to latency may consider setting this to "true".
  #
  # This option was introduced in Cilium 1.4. Cilium 1.3 and earlier ignore
  # this option and behave as though it is set to "true".
  #
  # If this value is modified, then during the next Cilium startup the restore
  # of existing endpoints and tracking of ongoing connections may be disrupted.
  # As a result, reply packets may be dropped and the load-balancing decisions
  # for established connections may change.
  #
  # If this option is set to "false" during an upgrade from 1.3 or earlier to
  # 1.4 or later, then it may cause one-time disruptions during the upgrade.
  preallocate-bpf-maps: "false"

  # Name of the cluster. Only relevant when building a mesh of clusters.
  cluster-name: "kubernetes"
  # Unique ID of the cluster. Must be unique across all connected clusters and
  # in the range of 1 and 255. Only relevant when building a mesh of clusters.
  cluster-id: "0"

  # Encapsulation mode for communication between nodes
  # Possible values:
  #   - disabled
  #   - vxlan (default)
  #   - geneve

  routing-mode: "tunnel"
  tunnel-protocol: "vxlan"
  tunnel-source-port-range: "0-0"
  service-no-backend-response: "reject"
  policy-deny-response: "none"


  # Enables L7 proxy for L7 policy enforcement and visibility
  enable-l7-proxy: "false"
  enable-ipv4-masquerade: "true"
  enable-ipv4-big-tcp: "false"
  enable-ipv6-big-tcp: "false"
  enable-ipv6-masquerade: "true"
  enable-tcx: "true"
  datapath-mode: "veth"
  enable-masquerade-to-route-source: "false"

  enable-xt-socket-fallback: "true"
  install-no-conntrack-iptables-rules: "false"
  iptables-random-fully: "false"

  auto-direct-node-routes: "false"
  direct-routing-skip-unreachable: "false"



  kube-proxy-replacement: "true"
  kube-proxy-replacement-healthz-bind-address: ""
  enable-no-service-endpoints-routable: "true"
  bpf-lb-sock: "false"
  enable-health-check-nodeport: "true"
  enable-health-check-loadbalancer-ip: "false"
  node-port-bind-protection: "true"
  enable-auto-protect-node-port-range: "true"
  bpf-lb-acceleration: "disabled"
  enable-service-topology: "false"
  enable-l2-neigh-discovery: "false"
  k8s-require-ipv4-pod-cidr: "false"
  k8s-require-ipv6-pod-cidr: "false"
  enable-k8s-networkpolicy: "true"
  enable-endpoint-lockdown-on-policy-overflow: "false"
  # Tell the agent to generate and write a CNI configuration file
  write-cni-conf-when-ready: /host/etc/cni/net.d/05-cilium.conflist
  cni-exclusive: "true"
  cni-log-file: "/var/run/cilium/cilium-cni.log"
  enable-endpoint-health-checking: "true"
  enable-health-checking: "true"
  health-check-icmp-failure-threshold: "3"
  enable-well-known-identities: "false"
  enable-node-selector-labels: "false"
  synchronize-k8s-nodes: "true"
  operator-api-serve-addr: "127.0.0.1:9234"

  enable-hubble: "false"
  ipam: "cluster-pool"
  ipam-cilium-node-update-rate: "15s"
  cluster-pool-ipv4-cidr: "10.244.0.0/16"
  cluster-pool-ipv4-mask-size: "24"

  default-lb-service-ipam: "lbipam"
  egress-gateway-reconciliation-trigger-interval: "1s"
  enable-vtep: "false"
  vtep-endpoint: ""
  vtep-cidr: ""
  vtep-mask: ""
  vtep-mac: ""

  packetization-layer-pmtud-mode: "blackhole"
  procfs: "/host/proc"
  bpf-root: "/sys/fs/bpf"
  cgroup-root: "/run/cilium/cgroupv2"

  identity-management-mode: "agent"
  enable-sctp: "false"
  remove-cilium-node-taints: "true"
  set-cilium-node-taints: "true"
  set-cilium-is-up-condition: "true"
  unmanaged-pod-watcher-interval: "15s"
  # default DNS proxy to transparent mode in non-chaining modes
  dnsproxy-enable-transparent-mode: "true"
  dnsproxy-socket-linger-timeout: "10"
  tofqdns-dns-reject-response-code: "refused"
  tofqdns-enable-dns-compression: "true"
  tofqdns-endpoint-max-ip-per-hostname: "1000"
  tofqdns-idle-connection-grace-period: "0s"
  tofqdns-max-deferred-connection-deletes: "10000"
  tofqdns-proxy-response-max-delay: "100ms"
  tofqdns-preallocate-identities:  "true"
  agent-not-ready-taint-key: "node.cilium.io/agent-not-ready"

  mesh-auth-enabled: "false"
  mesh-auth-queue-size: "1024"
  mesh-auth-rotated-identities-queue-size: "1024"
  mesh-auth-gc-interval: "5m0s"

  proxy-xff-num-trusted-hops-ingress: "0"
  proxy-xff-num-trusted-hops-egress: "0"
  proxy-connect-timeout: "2"
  proxy-initial-fetch-timeout: "30"
  proxy-max-active-downstream-connections: "50000"
  proxy-max-requests-per-connection: "0"
  proxy-max-connection-duration-seconds: "0"
  proxy-idle-timeout-seconds: "60"
  proxy-max-concurrent-retries: "128"
  proxy-use-original-source-address: "true"
  proxy-cluster-max-connections: "1024"
  proxy-cluster-max-requests: "1024"
  http-retry-count: "3"
  http-stream-idle-timeout: "300"

  external-envoy-proxy: "false"
  envoy-base-id: "0"
  envoy-access-log-buffer-size: "4096"
  envoy-keep-cap-netbindservice: "false"
  max-connected-clusters: "255"
  clustermesh-cache-ttl: "0s"
  clustermesh-enable-endpoint-sync: "false"
  clustermesh-enable-mcs-api: "false"
  clustermesh-mcs-api-install-crds: "true"
  policy-default-local-cluster: "true"

  nat-map-stats-entries: "32"
  nat-map-stats-interval: "30s"
  enable-lb-ipam: "true"
  enable-non-default-deny-policies: "true"
  enable-source-ip-verification: "true"
  enable-dynamic-config: "true"
  enable-drift-checker: "true"

# Extra config allows adding arbitrary properties to the cilium config.
# By putting it at the end of the ConfigMap, it's also possible to override existing properties.
---
# Source: cilium/templates/cilium-agent/clusterrole.yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: cilium
  labels:
    app.kubernetes.io/part-of: cilium
rules:
- apiGroups:
  - networking.k8s.io
  resources:
  - networkpolicies
  verbs:
  - get
  - list
  - watch
- apiGroups:
  - discovery.k8s.io
  resources:
  - endpointslices
  verbs:
  - get
  - list
  - watch
- apiGroups:
  - ""
  resources:
  - namespaces
  - services
  - pods
  - endpoints
  - nodes
  verbs:
  - get
  - list
  - watch
- apiGroups:
  - apiextensions.k8s.io
  resources:
  - customresourcedefinitions
  verbs:
  - list
  - watch
  # This is used when validating policies in preflight. This will need to stay
  # until we figure out how to avoid "get" inside the preflight, and then
  # should be removed ideally.
  - get
- apiGroups:
  - cilium.io
  resources:
  - ciliumloadbalancerippools
  - ciliumbgpnodeconfigs
  - ciliumbgpadvertisements
  - ciliumbgppeerconfigs
  - ciliumclusterwideenvoyconfigs
  - ciliumclusterwidenetworkpolicies
  - ciliumegressgatewaypolicies
  - ciliumendpoints
  - ciliumendpointslices
  - ciliumenvoyconfigs
  - ciliumidentities
  - ciliumlocalredirectpolicies
  - ciliumnetworkpolicies
  - ciliumnodes
  - ciliumnodeconfigs
  - ciliumcidrgroups
  - ciliuml2announcementpolicies
  - ciliumpodippools
  verbs:
  - list
  - watch
- apiGroups:
  - cilium.io
  resources:
  - ciliumidentities
  - ciliumendpoints
  - ciliumnodes
  verbs:
  - create
- apiGroups:
  - cilium.io
  # To synchronize garbage collection of such resources
  resources:
  - ciliumidentities
  verbs:
  - update
- apiGroups:
  - cilium.io
  resources:
  - ciliumendpoints
  verbs:
  - delete
  - get
- apiGroups:
  - cilium.io
  resources:
  - ciliumnodes
  - ciliumnodes/status
  verbs:
  - get
  - update
- apiGroups:
  - cilium.io
  resources:
  - ciliumendpoints/status
  - ciliumendpoints
  - ciliuml2announcementpolicies/status
  - ciliumbgpnodeconfigs/status
  verbs:
  - patch

---
# Source: cilium/templates/cilium-operator/clusterrole.yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: cilium-operator
  labels:
    app.kubernetes.io/part-of: cilium
rules:
- apiGroups:
  - ""
  resources:
  - pods
  verbs:
  - get
  - list
  - watch
  # to automatically delete [core|kube]dns pods so that are starting to being
  # managed by Cilium
  - delete
- apiGroups:
  - ""
  resources:
  - configmaps
  resourceNames:
  - cilium-config
  verbs:
   # allow patching of the configmap to set annotations
  - patch
- apiGroups:
  - ""
  resources:
  - nodes
  verbs:
  - list
  - watch
- apiGroups:
  - ""
  resources:
  # To remove node taints
  - nodes
  # To set NetworkUnavailable false on startup
  - nodes/status
  verbs:
  - patch
- apiGroups:
  - discovery.k8s.io
  resources:
  - endpointslices
  verbs:
  - get
  - list
  - watch
- apiGroups:
  - ""
  resources:
  # to perform LB IP allocation for BGP
  - services/status
  verbs:
  - update
  - patch
- apiGroups:
  - ""
  resources:
  # to check apiserver connectivity
  - namespaces
  - secrets
  verbs:
  - get
  - list
  - watch
- apiGroups:
  - ""
  resources:
  # to perform the translation of a CNP that contains `ToGroup` to its endpoints
  - services
  - endpoints
  verbs:
  - get
  - list
  - watch
- apiGroups:
  - cilium.io
  resources:
  - ciliumnetworkpolicies
  - ciliumclusterwidenetworkpolicies
  verbs:
  # Create auto-generated CNPs and CCNPs from Policies that have 'toGroups'
  - create
  - update
  - deletecollection
  # To update the status of the CNPs and CCNPs
  - patch
  - get
  - list
  - watch
- apiGroups:
  - cilium.io
  resources:
  - ciliumnetworkpolicies/status
  - ciliumclusterwidenetworkpolicies/status
  verbs:
  # Update the auto-generated CNPs and CCNPs status.
  - patch
  - update
- apiGroups:
  - cilium.io
  resources:
  - ciliumendpoints
  - ciliumidentities
  verbs:
  # To perform garbage collection of such resources
  - delete
  - list
  - watch
- apiGroups:
  - cilium.io
  resources:
  - ciliumidentities
  verbs:
  # To synchronize garbage collection of such resources
  - update
- apiGroups:
  - cilium.io
  resources:
  - ciliumnodes
  verbs:
  - create
  - update
  - get
  - list
  - watch
    # To perform CiliumNode garbage collector
  - delete
- apiGroups:
  - cilium.io
  resources:
  - ciliumnodes/status
  verbs:
  - update
- apiGroups:
  - cilium.io
  resources:
  - ciliumendpointslices
  - ciliumenvoyconfigs
  - ciliumbgppeerconfigs
  - ciliumbgpadvertisements
  - ciliumbgpnodeconfigs
  verbs:
  - create
  - update
  - get
  - list
  - watch
  - delete
  - patch
- apiGroups:
  - cilium.io
  resources:
  - ciliumbgpclusterconfigs/status
  - ciliumbgppeerconfigs/status
  verbs:
  - update
- apiGroups:
  - apiextensions.k8s.io
  resources:
  - customresourcedefinitions
  verbs:
  - create
  - get
  - list
  - watch
- apiGroups:
  - apiextensions.k8s.io
  resources:
  - customresourcedefinitions
  verbs:
  - update
  resourceNames:
  - ciliumloadbalancerippools.cilium.io
  - ciliumbgpclusterconfigs.cilium.io
  - ciliumbgppeerconfigs.cilium.io
  - ciliumbgpadvertisements.cilium.io
  - ciliumbgpnodeconfigs.cilium.io
  - ciliumbgpnodeconfigoverrides.cilium.io
  - ciliumclusterwideenvoyconfigs.cilium.io
  - ciliumclusterwidenetworkpolicies.cilium.io
  - ciliumegressgatewaypolicies.cilium.io
  - ciliumendpoints.cilium.io
  - ciliumendpointslices.cilium.io
  - ciliumenvoyconfigs.cilium.io
  - ciliumidentities.cilium.io
  - ciliumlocalredirectpolicies.cilium.io
  - ciliumnetworkpolicies.cilium.io
  - ciliumnodes.cilium.io
  - ciliumnodeconfigs.cilium.io
  - ciliumcidrgroups.cilium.io
  - ciliuml2announcementpolicies.cilium.io
  - ciliumpodippools.cilium.io
  - ciliumgatewayclassconfigs.cilium.io
- apiGroups:
  - cilium.io
  resources:
  - ciliumloadbalancerippools
  - ciliumpodippools
  - ciliumbgpclusterconfigs
  - ciliumbgpnodeconfigoverrides
  - ciliumbgppeerconfigs
  verbs:
  - get
  - list
  - watch
- apiGroups:
    - cilium.io
  resources:
    - ciliumpodippools
  verbs:
    - create
- apiGroups:
  - cilium.io
  resources:
  - ciliumloadbalancerippools/status
  verbs:
  - patch
# For cilium-operator running in HA mode.
#
# Cilium operator running in HA mode requires the use of ResourceLock for Leader Election
# between multiple running instances.
# The preferred way of doing this is to use LeasesResourceLock as edits to Leases are less
# common and fewer objects in the cluster watch "all Leases".
- apiGroups:
  - coordination.k8s.io
  resources:
  - leases
  verbs:
  - create
  - get
  - update
- apiGroups:
  - cilium.io
  resources:
  - ciliumendpointslices
  verbs:
  - deletecollection

---
# Source: cilium/templates/cilium-agent/clusterrolebinding.yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: cilium
  labels:
    app.kubernetes.io/part-of: cilium
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cilium
subjects:
- kind: ServiceAccount
  name: "cilium"
  namespace: kube-system

---
# Source: cilium/templates/cilium-operator/clusterrolebinding.yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: cilium-operator
  labels:
    app.kubernetes.io/part-of: cilium
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cilium-operator
subjects:
- kind: ServiceAccount
  name: "cilium-operator"
  namespace: kube-system

---
# Source: cilium/templates/cilium-agent/role.yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: cilium-config-agent
  namespace: kube-system
  labels:
    app.kubernetes.io/part-of: cilium
rules:
- apiGroups:
  - ""
  resources:
  - configmaps
  verbs:
  - get
  - list
  - watch
---
# Source: cilium/templates/cilium-agent/role.yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: cilium-tlsinterception-secrets
  namespace: "cilium-secrets"
  labels:
    app.kubernetes.io/part-of: cilium
rules:
- apiGroups:
  - ""
  resources:
  - secrets
  verbs:
  - get
  - list
  - watch

---
# Source: cilium/templates/cilium-operator/role.yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: cilium-operator-tlsinterception-secrets
  namespace: "cilium-secrets"
  labels:
    app.kubernetes.io/part-of: cilium
rules:
- apiGroups:
  - ""
  resources:
  - secrets
  verbs:
  - create
  - delete
  - update
  - patch
---
# Source: cilium/templates/cilium-operator/role.yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: cilium-operator-ztunnel
  namespace: kube-system
  labels:
    app.kubernetes.io/part-of: cilium
rules:
# ZTunnel DaemonSet management permissions
# Note: These permissions must always be granted (not conditional on encryption.type)
# because the controller needs to clean up stale DaemonSets when ztunnel is disabled.
- apiGroups:
  - apps
  resources:
  - daemonsets
  verbs:
  - create
  - delete
  - get
  - list
  - watch

---
# Source: cilium/templates/cilium-agent/rolebinding.yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: cilium-config-agent
  namespace: kube-system
  labels:
    app.kubernetes.io/part-of: cilium
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: cilium-config-agent
subjects:
  - kind: ServiceAccount
    name: "cilium"
    namespace: kube-system
---
# Source: cilium/templates/cilium-agent/rolebinding.yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: cilium-tlsinterception-secrets
  namespace: "cilium-secrets"
  labels:
    app.kubernetes.io/part-of: cilium
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: cilium-tlsinterception-secrets
subjects:
- kind: ServiceAccount
  name: "cilium"
  namespace: kube-system

---
# Source: cilium/templates/cilium-operator/rolebinding.yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: cilium-operator-tlsinterception-secrets
  namespace: "cilium-secrets"
  labels:
    app.kubernetes.io/part-of: cilium
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: cilium-operator-tlsinterception-secrets
subjects:
- kind: ServiceAccount
  name: "cilium-operator"
  namespace: kube-system
---
# Source: cilium/templates/cilium-operator/rolebinding.yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: cilium-operator-ztunnel
  namespace: kube-system
  labels:
    app.kubernetes.io/part-of: cilium
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: cilium-operator-ztunnel
subjects:
- kind: ServiceAccount
  name: "cilium-operator"
  namespace: kube-system

---
# Source: cilium/templates/cilium-agent/daemonset.yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: cilium
  namespace: kube-system
  labels:
    k8s-app: cilium
    app.kubernetes.io/part-of: cilium
    app.kubernetes.io/name: cilium-agent
spec:
  selector:
    matchLabels:
      k8s-app: cilium
  updateStrategy:
    rollingUpdate:
      maxUnavailable: 2
    type: RollingUpdate
  template:
    metadata:
      annotations:
        kubectl.kubernetes.io/default-container: cilium-agent
      labels:
        k8s-app: cilium
        app.kubernetes.io/name: cilium-agent
        app.kubernetes.io/part-of: cilium
    spec:
      securityContext:
        appArmorProfile:
          type: Unconfined
        seccompProfile:
          type: Unconfined
      containers:
      - name: cilium-agent
        image: "${IMAGE_CILIUM}"
        imagePullPolicy: IfNotPresent
        command:
        - cilium-agent
        args:
        - --config-dir=/tmp/cilium/config-map
        startupProbe:
          httpGet:
            host: "127.0.0.1"
            path: /healthz
            port: health
            scheme: HTTP
            httpHeaders:
            - name: "brief"
              value: "true"
          failureThreshold: 300
          periodSeconds: 2
          successThreshold: 1
          initialDelaySeconds: 5
        livenessProbe:
          httpGet:
            host: "127.0.0.1"
            path: /healthz
            port: health
            scheme: HTTP
            httpHeaders:
            - name: "brief"
              value: "true"
            - name: "require-k8s-connectivity"
              value: "false"
          periodSeconds: 30
          successThreshold: 1
          failureThreshold: 10
          timeoutSeconds: 5
        readinessProbe:
          httpGet:
            host: "127.0.0.1"
            path: /healthz
            port: health
            scheme: HTTP
            httpHeaders:
            - name: "brief"
              value: "true"
          periodSeconds: 30
          successThreshold: 1
          failureThreshold: 3
          timeoutSeconds: 5
        env:
        - name: K8S_NODE_NAME
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: spec.nodeName
        - name: CILIUM_K8S_NAMESPACE
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: metadata.namespace
        - name: CILIUM_CLUSTERMESH_CONFIG
          value: /var/lib/cilium/clustermesh/
        - name: GOMEMLIMIT
          valueFrom:
            resourceFieldRef:
              resource: limits.memory
              divisor: '1'
        - name: KUBERNETES_SERVICE_HOST
          value: "${VCP_API_HOST}"
        - name: KUBERNETES_SERVICE_PORT
          value: "443"
        - name: KUBE_CLIENT_BACKOFF_BASE
          value: "1"
        - name: KUBE_CLIENT_BACKOFF_DURATION
          value: "120"
        lifecycle:
          postStart:
            exec:
              command:
              - "bash"
              - "-c"
              - |
                    set -o errexit
                    set -o pipefail
                    set -o nounset
                    
                    # When running in AWS ENI mode, it's likely that 'aws-node' has
                    # had a chance to install SNAT iptables rules. These can result
                    # in dropped traffic, so we should attempt to remove them.
                    # We do it using a 'postStart' hook since this may need to run
                    # for nodes which might have already been init'ed but may still
                    # have dangling rules. This is safe because there are no
                    # dependencies on anything that is part of the startup script
                    # itself, and can be safely run multiple times per node (e.g. in
                    # case of a restart).
                    if [[ "$(iptables-save | grep -E -c 'AWS-SNAT-CHAIN|AWS-CONNMARK-CHAIN')" != "0" ]];
                    then
                        echo 'Deleting iptables rules created by the AWS CNI VPC plugin'
                        iptables-save | grep -E -v 'AWS-SNAT-CHAIN|AWS-CONNMARK-CHAIN' | iptables-restore
                    fi
                    echo 'Done!'
                    
          preStop:
            exec:
              command:
              - /cni-uninstall.sh
        ports:
        - name: health
          containerPort: 9879
          hostPort: 9879
          protocol: TCP
        securityContext:
          seLinuxOptions:
            level: s0
            type: spc_t
          capabilities:
            add:
              - CHOWN
              - KILL
              - NET_ADMIN
              - NET_RAW
              - IPC_LOCK
              - SYS_MODULE
              - SYS_ADMIN
              - SYS_RESOURCE
              - DAC_OVERRIDE
              - FOWNER
              - SETGID
              - SETUID
              - SYSLOG
            drop:
              - ALL
        terminationMessagePolicy: FallbackToLogsOnError
        volumeMounts:
        # Unprivileged containers need to mount /proc/sys/net from the host
        # to have write access
        - mountPath: /host/proc/sys/net
          name: host-proc-sys-net
        # Unprivileged containers need to mount /proc/sys/kernel from the host
        # to have write access
        - mountPath: /host/proc/sys/kernel
          name: host-proc-sys-kernel
        - name: bpf-maps
          mountPath: /sys/fs/bpf
          # Unprivileged containers can't set mount propagation to bidirectional
          # in this case we will mount the bpf fs from an init container that
          # is privileged and set the mount propagation from host to container
          # in Cilium.
          mountPropagation: HostToContainer
        - name: cilium-run
          mountPath: /var/run/cilium
        - name: cilium-netns
          mountPath: /var/run/cilium/netns
          mountPropagation: HostToContainer
        - name: etc-cni-netd
          mountPath: /host/etc/cni/net.d
        - name: clustermesh-secrets
          mountPath: /var/lib/cilium/clustermesh
          readOnly: true
          # Needed to be able to load kernel modules
        - name: lib-modules
          mountPath: /lib/modules
          readOnly: true
        - name: xtables-lock
          mountPath: /run/xtables.lock
        - name: tmp
          mountPath: /tmp
        
      initContainers:
      - name: config
        image: "${IMAGE_CILIUM}"
        imagePullPolicy: IfNotPresent
        command:
        - cilium-dbg
        - build-config
        env:
        - name: K8S_NODE_NAME
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: spec.nodeName
        - name: CILIUM_K8S_NAMESPACE
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: metadata.namespace
        - name: KUBERNETES_SERVICE_HOST
          value: "${VCP_API_HOST}"
        - name: KUBERNETES_SERVICE_PORT
          value: "443"
        volumeMounts:
        - name: tmp
          mountPath: /tmp
        terminationMessagePolicy: FallbackToLogsOnError
        securityContext:
          capabilities:
            add:
              - NET_ADMIN
            drop:
              - ALL
      # Required to mount cgroup2 filesystem on the underlying Kubernetes node.
      # We use nsenter command with host's cgroup and mount namespaces enabled.
      - name: mount-cgroup
        image: "${IMAGE_CILIUM}"
        imagePullPolicy: IfNotPresent
        env:
        - name: CGROUP_ROOT
          value: /run/cilium/cgroupv2
        - name: BIN_PATH
          value: /opt/cni/bin
        command:
        - bash
        - -ec
        # The statically linked Go program binary is invoked to avoid any
        # dependency on utilities like sh and mount that can be missing on certain
        # distros installed on the underlying host. Copy the binary to the
        # same directory where we install cilium cni plugin so that exec permissions
        # are available.
        - |
          cp /usr/bin/cilium-mount /hostbin/cilium-mount;
          nsenter --cgroup=/hostproc/1/ns/cgroup --mount=/hostproc/1/ns/mnt "${BIN_PATH}/cilium-mount" $CGROUP_ROOT;
          rm /hostbin/cilium-mount
        volumeMounts:
        - name: hostproc
          mountPath: /hostproc
        - name: cni-path
          mountPath: /hostbin
        terminationMessagePolicy: FallbackToLogsOnError
        securityContext:
          seLinuxOptions:
            level: s0
            type: spc_t
          capabilities:
            add:
              - SYS_ADMIN
              - SYS_CHROOT
              - SYS_PTRACE
            drop:
              - ALL
      - name: apply-sysctl-overwrites
        image: "${IMAGE_CILIUM}"
        imagePullPolicy: IfNotPresent
        env:
        - name: BIN_PATH
          value: /opt/cni/bin
        command:
        - bash
        - -ec
        # The statically linked Go program binary is invoked to avoid any
        # dependency on utilities like sh that can be missing on certain
        # distros installed on the underlying host. Copy the binary to the
        # same directory where we install cilium cni plugin so that exec permissions
        # are available.
        - |
          cp /usr/bin/cilium-sysctlfix /hostbin/cilium-sysctlfix;
          nsenter --mount=/hostproc/1/ns/mnt "${BIN_PATH}/cilium-sysctlfix";
          rm /hostbin/cilium-sysctlfix
        volumeMounts:
        - name: hostproc
          mountPath: /hostproc
        - name: cni-path
          mountPath: /hostbin
        terminationMessagePolicy: FallbackToLogsOnError
        securityContext:
          seLinuxOptions:
            level: s0
            type: spc_t
          capabilities:
            add:
              - SYS_ADMIN
              - SYS_CHROOT
              - SYS_PTRACE
            drop:
              - ALL
      # Mount the bpf fs if it is not mounted. We will perform this task
      # from a privileged container because the mount propagation bidirectional
      # only works from privileged containers.
      - name: mount-bpf-fs
        image: "${IMAGE_CILIUM}"
        imagePullPolicy: IfNotPresent
        args:
        - 'mount | grep "/sys/fs/bpf type bpf" || mount -t bpf bpf /sys/fs/bpf'
        command:
        - /bin/bash
        - -c
        - --
        terminationMessagePolicy: FallbackToLogsOnError
        securityContext:
          privileged: true
        volumeMounts:
        - name: bpf-maps
          mountPath: /sys/fs/bpf
          mountPropagation: Bidirectional
      - name: clean-cilium-state
        image: "${IMAGE_CILIUM}"
        imagePullPolicy: IfNotPresent
        command:
        - /init-container.sh
        env:
        - name: CILIUM_ALL_STATE
          valueFrom:
            configMapKeyRef:
              name: cilium-config
              key: clean-cilium-state
              optional: true
        - name: CILIUM_BPF_STATE
          valueFrom:
            configMapKeyRef:
              name: cilium-config
              key: clean-cilium-bpf-state
              optional: true
        - name: WRITE_CNI_CONF_WHEN_READY
          valueFrom:
            configMapKeyRef:
              name: cilium-config
              key: write-cni-conf-when-ready
              optional: true
        - name: KUBERNETES_SERVICE_HOST
          value: "${VCP_API_HOST}"
        - name: KUBERNETES_SERVICE_PORT
          value: "443"
        terminationMessagePolicy: FallbackToLogsOnError
        securityContext:
          seLinuxOptions:
            level: s0
            type: spc_t
          capabilities:
            add:
              - NET_ADMIN
              - SYS_MODULE
              - SYS_ADMIN
              - SYS_RESOURCE
            drop:
              - ALL
        volumeMounts:
        - name: bpf-maps
          mountPath: /sys/fs/bpf
          # Required to mount cgroup filesystem from the host to cilium agent pod
        - name: cilium-cgroup
          mountPath: /run/cilium/cgroupv2
          mountPropagation: HostToContainer
        - name: cilium-run
          mountPath: /var/run/cilium # wait-for-kube-proxy
      # Install the CNI binaries in an InitContainer so we don't have a writable host mount in the agent
      - name: install-cni-binaries
        image: "${IMAGE_CILIUM}"
        imagePullPolicy: IfNotPresent
        command:
          - "/install-plugin.sh"
        resources:
          limits:
            cpu: 1
            memory: 1Gi
          requests:
            cpu: 100m
            memory: 10Mi
        securityContext:
          seLinuxOptions:
            level: s0
            type: spc_t
          capabilities:
            drop:
              - ALL
        terminationMessagePolicy: FallbackToLogsOnError
        volumeMounts:
          - name: cni-path
            mountPath: /host/opt/cni/bin # .Values.cni.install
      restartPolicy: Always
      priorityClassName: system-node-critical
      serviceAccountName: "cilium"
      automountServiceAccountToken: true
      terminationGracePeriodSeconds: 1
      hostNetwork: true
      
      affinity:
        podAntiAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
          - labelSelector:
              matchLabels:
                k8s-app: cilium
            topologyKey: kubernetes.io/hostname
      nodeSelector:
        kubernetes.io/os: linux
      tolerations:
        - operator: Exists
      volumes:
      # For sharing configuration between the "config" initContainer and the agent
      - name: tmp
        emptyDir: {}
        # To keep state between restarts / upgrades
      - name: cilium-run
        hostPath:
          path: /var/run/cilium
          type: DirectoryOrCreate
        # To exec into pod network namespaces
      - name: cilium-netns
        hostPath:
          path: /var/run/netns
          type: DirectoryOrCreate
        # To keep state between restarts / upgrades for bpf maps
      - name: bpf-maps
        hostPath:
          path: /sys/fs/bpf
          type: DirectoryOrCreate
      # To mount cgroup2 filesystem on the host or apply sysctlfix
      - name: hostproc
        hostPath:
          path: /proc
          type: Directory
      # To keep state between restarts / upgrades for cgroup2 filesystem
      - name: cilium-cgroup
        hostPath:
          path: /run/cilium/cgroupv2
          type: DirectoryOrCreate
      # To install cilium cni plugin in the host
      - name: cni-path
        hostPath:
          path:  /opt/cni/bin
          type: DirectoryOrCreate
        # To install cilium cni configuration in the host
      - name: etc-cni-netd
        hostPath:
          path: /etc/cni/net.d
          type: DirectoryOrCreate
        # To be able to load kernel modules
      - name: lib-modules
        hostPath:
          path: /lib/modules
        # To access iptables concurrently with other processes (e.g. kube-proxy)
      - name: xtables-lock
        hostPath:
          path: /run/xtables.lock
          type: FileOrCreate
        # To read the clustermesh configuration
      - name: clustermesh-secrets
        projected:
          # note: the leading zero means this number is in octal representation: do not remove it
          defaultMode: 0400
          sources:
          - secret:
              name: cilium-clustermesh
              optional: true
              # note: items are not explicitly listed here, since the entries of this secret
              # depend on the peers configured, and that would cause a restart of all agents
              # at every addition/removal. Leaving the field empty makes each secret entry
              # to be automatically projected into the volume as a file whose name is the key.
          - secret:
              name: clustermesh-apiserver-remote-cert
              optional: true
              items:
              - key: tls.key
                path: common-etcd-client.key
              - key: tls.crt
                path: common-etcd-client.crt
              - key: ca.crt
                path: common-etcd-client-ca.crt
          # note: we configure the volume for the kvstoremesh-specific certificate
          # regardless of whether KVStoreMesh is enabled or not, so that it can be
          # automatically mounted in case KVStoreMesh gets subsequently enabled,
          # without requiring an agent restart.
          - secret:
              name: clustermesh-apiserver-local-cert
              optional: true
              items:
              - key: tls.key
                path: local-etcd-client.key
              - key: tls.crt
                path: local-etcd-client.crt
              - key: ca.crt
                path: local-etcd-client-ca.crt
      - name: host-proc-sys-net
        hostPath:
          path: /proc/sys/net
          type: Directory
      - name: host-proc-sys-kernel
        hostPath:
          path: /proc/sys/kernel
          type: Directory
