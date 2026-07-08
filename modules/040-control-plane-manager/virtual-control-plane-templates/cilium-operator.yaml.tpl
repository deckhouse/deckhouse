---
# Source: cilium/templates/cilium-configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: cilium-config
  namespace: ${NAMESPACE}
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
apiVersion: v1
kind: ServiceAccount
metadata:
  name: cilium-operator
  namespace: ${NAMESPACE}
automountServiceAccountToken: false
---
# Source: cilium/templates/cilium-operator/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: cilium-operator
  namespace: ${NAMESPACE}
  labels:
    io.cilium/app: operator
    name: cilium-operator
    app.kubernetes.io/part-of: cilium
    app.kubernetes.io/name: cilium-operator
spec:
  # See docs on ServerCapabilities.LeasesResourceLock in file pkg/k8s/version/version.go
  # for more details.
  replicas: 1
  selector:
    matchLabels:
      io.cilium/app: operator
      name: cilium-operator
  # ensure operator update on single node k8s clusters, by using rolling update with maxUnavailable=100% in case
  # of one replica and no user configured Recreate strategy.
  # otherwise an update might get stuck due to the default maxUnavailable=50% in combination with the
  # podAntiAffinity which prevents deployments of multiple operator replicas on the same node.
  strategy:
    rollingUpdate:
      maxSurge: 25%
      maxUnavailable: 100%
    type: RollingUpdate
  template:
    metadata:
      annotations:
        prometheus.io/port: "9963"
        prometheus.io/scrape: "true"
      labels:
        io.cilium/app: operator
        name: cilium-operator
        app.kubernetes.io/part-of: cilium
        app.kubernetes.io/name: cilium-operator
    spec:
      securityContext:
        seccompProfile:
          type: RuntimeDefault
      containers:
      - name: cilium-operator
        image: "${IMAGE_CILIUM_OPERATOR}"
        imagePullPolicy: IfNotPresent
        command:
        - cilium-operator-generic
        args:
        - --config-dir=/tmp/cilium/config-map
        - --debug=$(CILIUM_DEBUG)
        - --k8s-kubeconfig-path=/etc/cilium/kubeconfig/kubeconfig
        # Lease renewal timings relaxed for the kine/Postgres-backed tenant apiserver
        - --leader-election-lease-duration=60s
        - --leader-election-renew-deadline=40s
        - --leader-election-retry-period=10s
        env:
        - name: K8S_NODE_NAME
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: spec.nodeName
        - name: CILIUM_K8S_NAMESPACE
          value: kube-system
        - name: CILIUM_DEBUG
          valueFrom:
            configMapKeyRef:
              key: debug
              name: cilium-config
              optional: true
        ports:
        - name: health
          containerPort: 9234
        - name: prometheus
          containerPort: 9963
          protocol: TCP
        # Timings relaxed for the kine/Postgres-backed tenant apiserver
        livenessProbe:
          httpGet:
            host: "127.0.0.1"
            path: /healthz
            port: health
            scheme: HTTP
          initialDelaySeconds: 60
          periodSeconds: 15
          timeoutSeconds: 10
          failureThreshold: 6
        readinessProbe:
          httpGet:
            host: "127.0.0.1"
            path: /healthz
            port: health
            scheme: HTTP
          initialDelaySeconds: 0
          periodSeconds: 10
          timeoutSeconds: 10
          failureThreshold: 10
        volumeMounts:
        - name: cilium-config-path
          mountPath: /tmp/cilium/config-map
          readOnly: true
        - name: nested-kubeconfig
          mountPath: /etc/cilium/kubeconfig
          readOnly: true

        securityContext:
          allowPrivilegeEscalation: false
          capabilities:
            drop:
            - ALL
        terminationMessagePolicy: FallbackToLogsOnError
      restartPolicy: Always
      priorityClassName: system-cluster-critical
      serviceAccountName: "cilium-operator"
      automountServiceAccountToken: false
      # In HA mode, cilium-operator pods must not be scheduled on the same
      # node as they will clash with each other.
      affinity:
        podAntiAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
          - labelSelector:
              matchLabels:
                io.cilium/app: operator
            topologyKey: kubernetes.io/hostname
      nodeSelector:
        kubernetes.io/os: linux
      tolerations:
        - key: node-role.kubernetes.io/control-plane
          operator: Exists
        - key: node-role.kubernetes.io/master
          operator: Exists
        - key: node.kubernetes.io/not-ready
          operator: Exists
        - key: node.cloudprovider.kubernetes.io/uninitialized
          operator: Exists
        - key: node.cilium.io/agent-not-ready
          operator: Exists

      volumes:
        # To read the configuration from the config map
      - name: cilium-config-path
        configMap:
          name: cilium-config
      - name: nested-kubeconfig
        secret:
          secretName: d8-admin-kubeconfig-virtual
          items:
          - key: super-admin.conf
            path: kubeconfig
