# Changelog v1.74

## [MALFORMED]


 - #16795 unknown section "metrics-storage"
 - #17702 unknown section "registry-packages"

## Know before update


 - The minimum supported version of Kubernetes is now 1.30. All control plane components will restart.

## Features


 - **[candi]** Display lists of third-party (OSS) components in the modules documentation. [#17530](https://github.com/deckhouse/deckhouse/pull/17530)
 - **[candi]** Added GPU compute capability check to `check_gpu` bootstrap step. [#16665](https://github.com/deckhouse/deckhouse/pull/16665)
 - **[candi]** Added dynamic default for `maxPods` based on `podSubnetNodeCIDRPrefix`. [#16564](https://github.com/deckhouse/deckhouse/pull/16564)
    Kubelet will restart. The maximum default number of pods per node now automatically depends on the node CIDR size.
 - **[candi]** Enabled unconditional CDI support for containerd v1. [#16313](https://github.com/deckhouse/deckhouse/pull/16313)
    Containerd v1 will be restarted.
 - **[candi]** Enabled feature gates for Dynamic Resource Allocation. [#16311](https://github.com/deckhouse/deckhouse/pull/16311)
    Kubelet, api-server, and scheduler will be restarted.
 - **[candi]** Added Kubernetes feature gate management via the `control-plane-manager` module. [#16185](https://github.com/deckhouse/deckhouse/pull/16185)
 - **[candi]** Added support for Kubernetes 1.34 and discontinued support for Kubernetes 1.29. [#15518](https://github.com/deckhouse/deckhouse/pull/15518)
    The minimum supported version of Kubernetes is now 1.30. All control plane components will restart.
 - **[cloud-provider-aws]** Adds support for PublicNetworkAllowList to restrict incoming traffic [#16854](https://github.com/deckhouse/deckhouse/pull/16854)
 - **[cloud-provider-dvp]** Add validation for VirtualMachineClass and boot images before VM creation [#17755](https://github.com/deckhouse/deckhouse/pull/17755)
 - **[cloud-provider-huaweicloud]** Allowed users to overwrite default NIC in both CloudPermanent and CloudEphemeral nodes. [#15810](https://github.com/deckhouse/deckhouse/pull/15810)
 - **[cloud-provider-huaweicloud]** Added Virtual IP support. [#15600](https://github.com/deckhouse/deckhouse/pull/15600)
 - **[cni-cilium]** Added support for configuring the `mapDynamicSizeRatio` parameter for specific nodes using CiliumNodeConfig. [#16326](https://github.com/deckhouse/deckhouse/pull/16326)
 - **[cni-cilium]** Added SCTP protocol support. [#16297](https://github.com/deckhouse/deckhouse/pull/16297)
 - **[cni-cilium]** Added Prometheus metric `bpf_progs_complexity_max_verified_insts ` for maximum BPF instruction complexity (available with kernel >= 5.16). [#14723](https://github.com/deckhouse/deckhouse/pull/14723)
 - **[common]** Resource quota ignore mechanism for pvc and pods [#17068](https://github.com/deckhouse/deckhouse/pull/17068)
 - **[control-plane-manager]** Made the `terminated-pod-gc-threshold` setting dynamic based on the number of nodes in a cluster. [#16266](https://github.com/deckhouse/deckhouse/pull/16266)
    Kube-controller-manager will be restarted, and the default value of `terminated-pod-gc-threshold` will be reconfigured.
 - **[control-plane-manager]** Added Kubernetes feature gate management via the `control-plane-manager` module. [#16185](https://github.com/deckhouse/deckhouse/pull/16185)
 - **[deckhouse]** Replaced file-based module loading with a read-only EROFS installation system. [#15019](https://github.com/deckhouse/deckhouse/pull/15019)
 - **[deckhouse-controller]** Add ObjectKeeper controller. [#16773](https://github.com/deckhouse/deckhouse/pull/16773)
 - **[deckhouse-controller]** Added the `PackageStatusService` service to process events from `PackageOperator` and update the application status. [#16465](https://github.com/deckhouse/deckhouse/pull/16465)
 - **[deckhouse-controller]** Enabled Nelm environment flag in controller logic. [#16142](https://github.com/deckhouse/deckhouse/pull/16142)
 - **[deckhouse-controller]** Added scaffolding for a new Package System (8 CRDs and 6 placeholder controllers). [#16016](https://github.com/deckhouse/deckhouse/pull/16016)
 - **[deckhouse-controller]** Moved the `collect-debug-info` command from `deckhouse-controller` to the `d8` tool. [#15767](https://github.com/deckhouse/deckhouse/pull/15767)
 - **[deckhouse-controller]** Restricted the `d8ms-*` prefix for internal Deckhouse objects. [#15147](https://github.com/deckhouse/deckhouse/pull/15147)
    Users won't be able to create objects with the `d8ms-` prefix in their Deckhouse clusters.
 - **[dhctl]** Skipped application edition validation for standalone builds. [#17154](https://github.com/deckhouse/deckhouse/pull/17154)
 - **[dhctl]** Isolated temporary directory for singleshot RPC and dhctl to avoid cleanup race. [#15794](https://github.com/deckhouse/deckhouse/pull/15794)
 - **[dhctl]** Skipped application edition validation for standalone builds. [#15493](https://github.com/deckhouse/deckhouse/pull/15493)
 - **[ingress-nginx]** Updated Nginx versions of NGINX Ingress Controller 1.10 and 1.12 to version 1.26.1. [#16476](https://github.com/deckhouse/deckhouse/pull/16476)
    NGINX Ingress Controller pods of versions 1.10 and 1.12 will be restarted.
 - **[ingress-nginx]** Added the `geoip_version` metric to NGINX Ingress Controller dashboards to indicate issues with GeoIP DB in the cluster. [#16449](https://github.com/deckhouse/deckhouse/pull/16449)
    All NGINX Ingress Controller pods will be restarted.
 - **[ingress-nginx]** Increased fault tolerance when the MaxMind service is unavailable or download limits are exceeded. [#15276](https://github.com/deckhouse/deckhouse/pull/15276)
    the instances will be restarted.
 - **[istio]** Added FQDN support to the `alliance.ingressGateway.advertise` section. [#16488](https://github.com/deckhouse/deckhouse/pull/16488)
 - **[metallb]** Updated MetalLB version from 0.14.8 to 0.15.2. [#16210](https://github.com/deckhouse/deckhouse/pull/16210)
 - **[node-local-dns]** Optimized the cache plugin for clusters in dev mode. [#16535](https://github.com/deckhouse/deckhouse/pull/16535)
 - **[node-local-dns]** node-local-dns daemonset updating process is synced with cilium agents. [#16295](https://github.com/deckhouse/deckhouse/pull/16295)
 - **[node-manager]** Added validation to ensure `.spec.kubelet.maxPods` doesn't exceed the number of available Pod IP capacity per node. [#16695](https://github.com/deckhouse/deckhouse/pull/16695)
 - **[node-manager]** Added Kubernetes feature gate management via the `control-plane-manager` module. [#16185](https://github.com/deckhouse/deckhouse/pull/16185)
 - **[node-manager]** Denied applying of CAPS StaticInstance resources whose address is similar to any node in the Deckhouse cluster. [#15991](https://github.com/deckhouse/deckhouse/pull/15991)
 - **[node-manager]** Prevented user workload deployment during the node's first Bashible run. [#14828](https://github.com/deckhouse/deckhouse/pull/14828)
 - **[prometheus]** improve redirects from Grafana to the Deckhouse UI when Grafana is disabled [#16988](https://github.com/deckhouse/deckhouse/pull/16988)
    no impact
 - **[prometheus]** Add prometheus configuration reload failed alert [#17128](https://github.com/deckhouse/deckhouse/pull/17128)
 - **[prometheus]** Replace PrometheusRules with ClusterObservabilityMetricsRulesGroups or ClusterObservabilityPropagatedMetricsRulesGroups when deployed using helm_lib_prometheus_rules helper and the observability module is enabled [#17329](https://github.com/deckhouse/deckhouse/pull/17329)
 - **[prometheus]** Replace PrometheusRules with ClusterObservabilityMetricsRulesGroups or ClusterObservabilityPropagatedMetricsRulesGroups when deployed using helm_lib_prometheus_rules helper and the observability module is enabled [#16405](https://github.com/deckhouse/deckhouse/pull/16405)
 - **[user-authn]** Added `spec.resources` (CPU, memory requests, limits) to DexAuthenticator and disabled VPA creation when it’s set. [#16226](https://github.com/deckhouse/deckhouse/pull/16226)

## Fixes


 - **[admission-policy-engine]** Refactor constraint templates [#17882](https://github.com/deckhouse/deckhouse/pull/17882)
 - **[admission-policy-engine]** Allow DELETE operations, add containerPorts check in case of hostNetwork [#17084](https://github.com/deckhouse/deckhouse/pull/17084)
 - **[candi]** Added a Netplan override to force the secondary NIC to use the main routing table, fixing cloud-init PBR conflicts. [#16625](https://github.com/deckhouse/deckhouse/pull/16625)
 - **[candi]** Remove duplicate `additional_disks_hashes` definition in static-node Terraform module. [#17441](https://github.com/deckhouse/deckhouse/pull/17441)
 - **[candi]** Updated the bashible step to include Linux kernel versions that address CVE-2025-37999 [#17300](https://github.com/deckhouse/deckhouse/pull/17300)
 - **[candi]** Allow manually stopping DVP node VirtualMachines in nested clusters by using runPolicy AlwaysOnUnlessStoppedManually. [#17110](https://github.com/deckhouse/deckhouse/pull/17110)
 - **[candi]** remove excessive netcat calls from d8-shutdown-inhibitor [#17153](https://github.com/deckhouse/deckhouse/pull/17153)
 - **[candi]** Add pause and kubernetes-api-proxy registry packages to bashible `bb-package-fetch` to prevent node failures during containerd major upgrades. [#17047](https://github.com/deckhouse/deckhouse/pull/17047)
 - **[candi]** Added `registry.d8-system.svc` to `no_proxy` list to bypass proxy for internal registry requests. [#16595](https://github.com/deckhouse/deckhouse/pull/16595)
 - **[candi]** Improved node-user retry logic to skip failing API servers. [#16493](https://github.com/deckhouse/deckhouse/pull/16493)
 - **[candi]** Refactored Bashible OS detection to use the new version map structure and shared package-manager helpers. [#16459](https://github.com/deckhouse/deckhouse/pull/16459)
 - **[candi]** Fixed an issue in `bb-event-error-create` that prevented some logs from sending. [#16411](https://github.com/deckhouse/deckhouse/pull/16411)
 - **[candi]** Applied a `PROMPT_COMMAND`-based PATH guard to restore expected PATH behavior when `~/.bashrc` overwrites PATH. [#16407](https://github.com/deckhouse/deckhouse/pull/16407)
 - **[candi]** Excluded I/O loopback from node IP discovery. [#16179](https://github.com/deckhouse/deckhouse/pull/16179)
 - **[cilium-hubble]** Fix affinity in HA mode [#16862](https://github.com/deckhouse/deckhouse/pull/16862)
    In HA cluster mode hubble-ui and hubble-relay will be restarted
 - **[cloud-provider-aws]** fix cve [#17470](https://github.com/deckhouse/deckhouse/pull/17470)
 - **[cloud-provider-aws]** fix cve [#16843](https://github.com/deckhouse/deckhouse/pull/16843)
 - **[cloud-provider-azure]** fixed patch in azure [#17696](https://github.com/deckhouse/deckhouse/pull/17696)
 - **[cloud-provider-azure]** fixed cve [#16839](https://github.com/deckhouse/deckhouse/pull/16839)
 - **[cloud-provider-dvp]** Fixed VM deletion timeout and improved memory validation error reporting [#17587](https://github.com/deckhouse/deckhouse/pull/17587)
 - **[cloud-provider-dvp]** Cleanup orphaned resources when VM creation fails [#17533](https://github.com/deckhouse/deckhouse/pull/17533)
 - **[cloud-provider-dvp]** this PR gives a lot more informational errors and messages to user [#17165](https://github.com/deckhouse/deckhouse/pull/17165)
 - **[cloud-provider-dvp]** Fix healthCheckNodePort collisions [#16996](https://github.com/deckhouse/deckhouse/pull/16996)
 - **[cloud-provider-dvp]** fixed CVE [#16810](https://github.com/deckhouse/deckhouse/pull/16810)
 - **[cloud-provider-dvp]** this changes fix some of cases when pods stuck in Completed/Error [#16741](https://github.com/deckhouse/deckhouse/pull/16741)
 - **[cloud-provider-dvp]** Stopped preferring FQDN to hostname in cloud-init configurations. [#16124](https://github.com/deckhouse/deckhouse/pull/16124)
 - **[cloud-provider-huaweicloud]** fix cve [#17171](https://github.com/deckhouse/deckhouse/pull/17171)
 - **[cloud-provider-huaweicloud]** Updated the `caphc-controller-manager` component for the Huawei Cloud provider. [#16679](https://github.com/deckhouse/deckhouse/pull/16679)
 - **[cloud-provider-huaweicloud]** Added `enterpriseProjectID` support for Cinder-based (<10Gi) volumes. [#16618](https://github.com/deckhouse/deckhouse/pull/16618)
 - **[cloud-provider-openstack]** fix cve [#17082](https://github.com/deckhouse/deckhouse/pull/17082)
 - **[cloud-provider-openstack]** Fixed discovery data merging for hybrid cases. [#16067](https://github.com/deckhouse/deckhouse/pull/16067)
 - **[cloud-provider-vcd]** fix cve [#17136](https://github.com/deckhouse/deckhouse/pull/17136)
 - **[cloud-provider-vcd]** Implemented a hack to migrate etcd disk to VCD independent disk to prevent deletion of etcd data. [#16302](https://github.com/deckhouse/deckhouse/pull/16302)
    To migrate, you must perform a `converge`, which causes the master server to be recreated. If you are using only one master server with the manual address assignment via the `mainNetworkIPAddresses` parameter, add two more IP addresses for the migration process.
 - **[cloud-provider-vsphere]** fix cve [#17106](https://github.com/deckhouse/deckhouse/pull/17106)
 - **[cloud-provider-yandex]** fix cve [#17469](https://github.com/deckhouse/deckhouse/pull/17469)
 - **[cloud-provider-zvirt]** fix cve [#17093](https://github.com/deckhouse/deckhouse/pull/17093)
 - **[cni-cilium]** Fix issue in generating CiliumEgressGatewayPolicy CR. [#17949](https://github.com/deckhouse/deckhouse/pull/17949)
    All current connections powered by EgressGateways will be terminated.
 - **[cni-cilium]** Fix hook discovery_cni_exclusive.go [#17719](https://github.com/deckhouse/deckhouse/pull/17719)
    If the SDN module is used in the cluster, the Cilium agent pods will be restarted.
 - **[cni-cilium]** Fixed egress-gateway-agent controller logic for deleted resources and disable dev logging. [#17378](https://github.com/deckhouse/deckhouse/pull/17378)
 - **[common]** Latest CVEs are fixed. [#17222](https://github.com/deckhouse/deckhouse/pull/17222)
    All pods running kube-rbac-proxy will be restarted.
 - **[common]** Added `registry.d8-system.svc` to `no_proxy` list in `helm_lib` `_envs_for_proxy.tpl`. [#16595](https://github.com/deckhouse/deckhouse/pull/16595)
 - **[control-plane-manager]** Added explicit `etcd join` phase for control-plane scaling in 1.33. [#16609](https://github.com/deckhouse/deckhouse/pull/16609)
    Allows scaling control-plane from 1 to 3 in clusters where `ControlPlaneKubeletLocalMode=true`.
 - **[deckhouse]** Fix module rerun. [#17478](https://github.com/deckhouse/deckhouse/pull/17478)
 - **[deckhouse]** Fix module installer cleanup. [#17301](https://github.com/deckhouse/deckhouse/pull/17301)
 - **[deckhouse]** Fix module docs rendering. [#17245](https://github.com/deckhouse/deckhouse/pull/17245)
 - **[deckhouse]** Fix module enabling. [#17057](https://github.com/deckhouse/deckhouse/pull/17057)
 - **[deckhouse]** Fix module enabling. [#17009](https://github.com/deckhouse/deckhouse/pull/17009)
 - **[deckhouse-controller]** Exclude all service accounts from `d8-` namespaces in `d8ms-prefix` ValidatingAdmissionPolicy. [#17440](https://github.com/deckhouse/deckhouse/pull/17440)
 - **[deckhouse-controller]** Fixed "multiple readiness hooks found" error on hook registration retry after a failure. [#16778](https://github.com/deckhouse/deckhouse/pull/16778)
 - **[deckhouse-controller]** Fix conversions for external modules [#16772](https://github.com/deckhouse/deckhouse/pull/16772)
 - **[deckhouse-controller]** Fixed false-positive "multiple readiness hooks found" error on module hook registration retry. [#16694](https://github.com/deckhouse/deckhouse/pull/16694)
 - **[deckhouse-controller]** Fixed an issue where modules enabled through ModuleManager after migration bypassed ModuleConfig release validation. [#16673](https://github.com/deckhouse/deckhouse/pull/16673)
 - **[deckhouse-controller]** Replaced old binary compaction metrics with new informative metrics that show actual compaction frequency and queue load per hook. [#16659](https://github.com/deckhouse/deckhouse/pull/16659)
 - **[deckhouse-controller]** Fixed a crash during external module updates with conversions that caused ModuleRelease to fail validation due to a forbidden property error. [#16546](https://github.com/deckhouse/deckhouse/pull/16546)
 - **[deckhouse-controller]** Fixed module documentation collection from EROFS mounted modules. [#16495](https://github.com/deckhouse/deckhouse/pull/16495)
 - **[deckhouse-controller]** Now whenever hooks fail, Deckhouse handles and returns corresponding metrics along with an error. [#16319](https://github.com/deckhouse/deckhouse/pull/16319)
 - **[deckhouse-controller]** Fixed incorrect time value in minor release notification messages. [#16271](https://github.com/deckhouse/deckhouse/pull/16271)
 - **[dhctl]** Fix panic in dhctl config render kubeadm-config command. [#17934](https://github.com/deckhouse/deckhouse/pull/17934)
 - **[dhctl]** Improved reliability when connecting to dhctl servers by adding retry logic and better error handling during startup [#17698](https://github.com/deckhouse/deckhouse/pull/17698)
 - **[dhctl]** Eliminated double root-password prompts during Terraform validation. [#16591](https://github.com/deckhouse/deckhouse/pull/16591)
 - **[dhctl]** Now the `AllowTcpForwarding` preflight check can interrupt a bootstrap process. [#16250](https://github.com/deckhouse/deckhouse/pull/16250)
 - **[dhctl]** Fixed endless converge loop for clusters with NAT instances. [#16230](https://github.com/deckhouse/deckhouse/pull/16230)
 - **[dhctl]** Now the dhctl dependency validation can run within a single SSH connection. [#16120](https://github.com/deckhouse/deckhouse/pull/16120)
 - **[dhctl]** Isolated temporary directory for singleshot RPC and dhctl to avoid cleanup race. [#15794](https://github.com/deckhouse/deckhouse/pull/15794)
 - **[dhctl]** Fixed a memory leak in Terraform exporter. [#15350](https://github.com/deckhouse/deckhouse/pull/15350)
 - **[extended-monitoring]** Cleanup exporter metrics when the monitored resource has been deleted [#17988](https://github.com/deckhouse/deckhouse/pull/17988)
 - **[extended-monitoring]** Add namespace-scoped overrides [#17213](https://github.com/deckhouse/deckhouse/pull/17213)
 - **[ingress-nginx]** A false-positive trigger of alert GeoIPDownloadErrorDetectedFromMaxMind is fixed. [#17741](https://github.com/deckhouse/deckhouse/pull/17741)
 - **[ingress-nginx]** The CVE-2026-1580, CVE-2026-24512, CVE-2026-24513, CVE-2026-24514 CVEs fixes are backported. [#17823](https://github.com/deckhouse/deckhouse/pull/17823)
    The ingress nginx controllers' pods will be restated.
 - **[ingress-nginx]** The CVE-2026-1580, CVE-2026-24512, CVE-2026-24513, CVE-2026-24514 CVEs are fixed. [#17795](https://github.com/deckhouse/deckhouse/pull/17795)
    The ingress nginx controllers' pods will be restated.
 - **[ingress-nginx]** Latest CVEs are fixed. [#17222](https://github.com/deckhouse/deckhouse/pull/17222)
    All pods running kube-rbac-proxy will be restarted.
 - **[ingress-nginx]** Improved stability of geoproxy service startup. [#17140](https://github.com/deckhouse/deckhouse/pull/17140)
 - **[istio]** Correction of an useless error in the Istio CNI workflow [#17787](https://github.com/deckhouse/deckhouse/pull/17787)
 - **[istio]** Correction  in Kiali of an insignificant error [#16880](https://github.com/deckhouse/deckhouse/pull/16880)
 - **[istio]** Fixed false-positive alert `D8IstioRemoteClusterNotSynced` and improved its description. [#15826](https://github.com/deckhouse/deckhouse/pull/15826)
 - **[loki]** Fixed the `LokiDiscardedSamplesWarning` alert. [#16374](https://github.com/deckhouse/deckhouse/pull/16374)
 - **[metallb]** Fixed D8MetallbBGPSessionDown alert false positives caused by duplicate metrics without port information. [#17563](https://github.com/deckhouse/deckhouse/pull/17563)
 - **[monitoring-kubernetes]** fix kube_persistentvolume_is_local recording rule when there are more than one kube-state-metrics exporter in cluster [#17877](https://github.com/deckhouse/deckhouse/pull/17877)
 - **[monitoring-kubernetes]** fix kube_persistentvolume_is_local recording rule for not-bound PVCs [#17638](https://github.com/deckhouse/deckhouse/pull/17638)
 - **[multitenancy-manager]** Add validation to restrict Project name length to 53 characters. [#16926](https://github.com/deckhouse/deckhouse/pull/16926)
    Prevents creation of Projects with too-long names that would lead to invalid generated Kubernetes resource names.
 - **[multitenancy-manager]** Fixed indentation in the manifest of `multitenancy-manager`. [#16471](https://github.com/deckhouse/deckhouse/pull/16471)
 - **[node-manager]** Fix panic in cluster-autoscaler caused by nil pointer dereference during node removal simulation. [#17924](https://github.com/deckhouse/deckhouse/pull/17924)
 - **[node-manager]** remove excessive netcat calls from d8-shutdown-inhibitor [#17153](https://github.com/deckhouse/deckhouse/pull/17153)
 - **[node-manager]** It fixes issues in the DaemonSet manifest for fencing module. [#17087](https://github.com/deckhouse/deckhouse/pull/17087)
 - **[node-manager]** Fixed `mig-manager` reconfigure script to correctly handle auto-approved disruptive node group changes. [#16655](https://github.com/deckhouse/deckhouse/pull/16655)
 - **[node-manager]** Set to rescan power-button input devices and refreshes stale descriptors, ensuring the shutdown inhibitor continues receiving button-press events. [#16651](https://github.com/deckhouse/deckhouse/pull/16651)
 - **[node-manager]** Fixed `bashible-apiserver` checksum update. [#16621](https://github.com/deckhouse/deckhouse/pull/16621)
 - **[node-manager]** Updated helm-lib to ensure `privileged` is set to `false` whenever `allowPrivilegeEscalation` is `false`, preventing invalid configurations after the switch to SSA. [#16562](https://github.com/deckhouse/deckhouse/pull/16562)
 - **[node-manager]** Added early StaticInstance reservation with automatic rollback on failure. [#16315](https://github.com/deckhouse/deckhouse/pull/16315)
 - **[node-manager]** Moved `bb-label-node-bashible-first-run-finished` to a Bashible template. [#16307](https://github.com/deckhouse/deckhouse/pull/16307)
 - **[prometheus]** Make Grafana redirect ingress pass the annotations validation. [#17816](https://github.com/deckhouse/deckhouse/pull/17816)
 - **[prometheus]** Added `ingressClassName` to the `grafana/prometheus` redirect Ingress. [#16116](https://github.com/deckhouse/deckhouse/pull/16116)
 - **[registry]** Fixed validation of input image list changes in the registry checker. [#17553](https://github.com/deckhouse/deckhouse/pull/17553)
 - **[registry]** Omitted the auth field in DockerConfig when credentials (username and password) are empty. [#17333](https://github.com/deckhouse/deckhouse/pull/17333)
 - **[registrypackages]** Added `which` to RPP. [#16563](https://github.com/deckhouse/deckhouse/pull/16563)
 - **[terraform-manager]** yandex terraform version was updated [#16779](https://github.com/deckhouse/deckhouse/pull/16779)
 - **[user-authn]** Quote service names to prevent digit-only names from breaking yaml parser [#17020](https://github.com/deckhouse/deckhouse/pull/17020)
 - **[user-authn]** Added a warning that DexAuthenticator only works over HTTPS. [#16721](https://github.com/deckhouse/deckhouse/pull/16721)
 - **[user-authz]** Allow project-scoped roles to access Cluster-wide objects [#16896](https://github.com/deckhouse/deckhouse/pull/16896)

## Chore


 - **[candi]** Bumped patch versions of Kubernetes images and CVE fixes. [#16455](https://github.com/deckhouse/deckhouse/pull/16455)
    Kubernetes control-plane components and kubelet will restart.
 - **[deckhouse]** Bumped `addon-operator` dependency to ignore absent chart file. [#15949](https://github.com/deckhouse/deckhouse/pull/15949)
 - **[dhctl]** Expand SSH output logs on errors for debug, verbose purposes. [#16915](https://github.com/deckhouse/deckhouse/pull/16915)
 - **[dhctl]** Set default ssh port to 22, to backward compatibility with cli ssh behavior in dhctl. [#16947](https://github.com/deckhouse/deckhouse/pull/16947)
 - **[dhctl]** Fixed `gossh` client reconnections. [#16709](https://github.com/deckhouse/deckhouse/pull/16709)
 - **[dhctl]** Disabled Bashible debug console when launched via `commander` and capped retries to 10 restarts and 5 attempts per step. [#15738](https://github.com/deckhouse/deckhouse/pull/15738)
 - **[ingress-nginx]** removed deprecated alert GeoIPDownloadErrorDetected. [#17711](https://github.com/deckhouse/deckhouse/pull/17711)
    All instances will be restarted.
 - **[ingress-nginx]** Improved documentation for the ModSecurity (WAF). [#16268](https://github.com/deckhouse/deckhouse/pull/16268)
 - **[loki]** Added alerts and graphs for discarded log samples. [#16137](https://github.com/deckhouse/deckhouse/pull/16137)
 - **[node-local-dns]** Added logging of slow upstream queries and a new coredns_kubeforward_slow_requests_total metric for tracking them. [#16808](https://github.com/deckhouse/deckhouse/pull/16808)
 - **[node-local-dns]** Removed `stale-dns-connections-cleaner`, since the related issue was fixed in `cni-cilium` upstream. [#16447](https://github.com/deckhouse/deckhouse/pull/16447)
 - **[node-manager]** Node inhibitor was migrated to the kubernetes.io library. [#16237](https://github.com/deckhouse/deckhouse/pull/16237)

