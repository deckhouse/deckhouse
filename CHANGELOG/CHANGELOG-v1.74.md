# Changelog v1.74

## [MALFORMED]


 - #16906 invalid type "сhore"
 - #17408 unknown section "metrics-storage"
 - #17872 unknown section "cloud-instance-manager"

## Know before update


 - Cilium agents will be restarted during the update.
 - The minimum supported version of Kubernetes is now 1.30. All control plane components will restart.
 - This update triggers a rolling update of the flannel pods.
 - This update triggers a rolling update of the kube-proxy pods.
 - This update triggers a rolling update of the network-policy-engine pods.

## Features


 - **[candi]** Added GPU compute capability check to `check_gpu` bootstrap step. [#16665](https://github.com/deckhouse/deckhouse/pull/16665)
 - **[candi]** Added Kubernetes feature gate management via the `control-plane-manager` module. [#16185](https://github.com/deckhouse/deckhouse/pull/16185)
 - **[candi]** Added dynamic default for `maxPods` based on `podSubnetNodeCIDRPrefix`. [#16564](https://github.com/deckhouse/deckhouse/pull/16564)
    Kubelet will restart. The maximum default number of pods per node now automatically depends on the node CIDR size.
 - **[candi]** Added support for Kubernetes 1.34 and discontinued support for Kubernetes 1.29. [#15518](https://github.com/deckhouse/deckhouse/pull/15518)
    The minimum supported version of Kubernetes is now 1.30. All control plane components will restart.
 - **[candi]** Display lists of third-party (OSS) components in the modules documentation. [#17530](https://github.com/deckhouse/deckhouse/pull/17530)
 - **[candi]** Enabled feature gates for Dynamic Resource Allocation. [#16311](https://github.com/deckhouse/deckhouse/pull/16311)
    Kubelet, api-server, and scheduler will be restarted.
 - **[candi]** Enabled unconditional CDI support for containerd v1. [#16313](https://github.com/deckhouse/deckhouse/pull/16313)
    Containerd v1 will be restarted.
 - **[cloud-provider-aws]** Adds support for PublicNetworkAllowList to restrict incoming traffic [#16854](https://github.com/deckhouse/deckhouse/pull/16854)
 - **[cloud-provider-dvp]** Add validation for VirtualMachineClass and boot images before VM creation [#17755](https://github.com/deckhouse/deckhouse/pull/17755)
 - **[cloud-provider-dvp]** Clarified CSI errors. [#16434](https://github.com/deckhouse/deckhouse/pull/16434)
 - **[cloud-provider-huaweicloud]** Added Virtual IP support. [#15600](https://github.com/deckhouse/deckhouse/pull/15600)
 - **[cloud-provider-huaweicloud]** Allowed users to overwrite default NIC in both CloudPermanent and CloudEphemeral nodes. [#15810](https://github.com/deckhouse/deckhouse/pull/15810)
 - **[cni-cilium]** Added Prometheus metric `bpf_progs_complexity_max_verified_insts ` for maximum BPF instruction complexity (available with kernel >= 5.16). [#14723](https://github.com/deckhouse/deckhouse/pull/14723)
 - **[cni-cilium]** Added SCTP protocol support. [#16297](https://github.com/deckhouse/deckhouse/pull/16297)
 - **[cni-cilium]** Added support for configuring the `mapDynamicSizeRatio` parameter for specific nodes using CiliumNodeConfig. [#16326](https://github.com/deckhouse/deckhouse/pull/16326)
 - **[common]** Resource quota ignore mechanism for pvc and pods [#17217](https://github.com/deckhouse/deckhouse/pull/17217)
 - **[control-plane-manager]** Added Kubernetes feature gate management via the `control-plane-manager` module. [#16185](https://github.com/deckhouse/deckhouse/pull/16185)
 - **[control-plane-manager]** Made the `terminated-pod-gc-threshold` setting dynamic based on the number of nodes in a cluster. [#16266](https://github.com/deckhouse/deckhouse/pull/16266)
    Kube-controller-manager will be restarted, and the default value of `terminated-pod-gc-threshold` will be reconfigured.
 - **[deckhouse-controller]** Add ObjectKeeper controller. [#16773](https://github.com/deckhouse/deckhouse/pull/16773)
 - **[deckhouse-controller]** Added scaffolding for a new Package System (8 CRDs and 6 placeholder controllers). [#16016](https://github.com/deckhouse/deckhouse/pull/16016)
 - **[deckhouse-controller]** Added the `PackageStatusService` service to process events from `PackageOperator` and update the application status. [#16465](https://github.com/deckhouse/deckhouse/pull/16465)
 - **[deckhouse-controller]** Enabled Nelm environment flag in controller logic. [#16142](https://github.com/deckhouse/deckhouse/pull/16142)
 - **[deckhouse-controller]** Moved the `collect-debug-info` command from `deckhouse-controller` to the `d8` tool. [#15767](https://github.com/deckhouse/deckhouse/pull/15767)
 - **[deckhouse-controller]** Restricted the `d8ms-*` prefix for internal Deckhouse objects. [#15147](https://github.com/deckhouse/deckhouse/pull/15147)
    Users won't be able to create objects with the `d8ms-` prefix in their Deckhouse clusters.
 - **[deckhouse]** Replaced file-based module loading with a read-only EROFS installation system. [#15019](https://github.com/deckhouse/deckhouse/pull/15019)
 - **[dhctl]** Isolated temporary directory for singleshot RPC and dhctl to avoid cleanup race. [#15794](https://github.com/deckhouse/deckhouse/pull/15794)
 - **[dhctl]** Skipped application edition validation for standalone builds. [#17154](https://github.com/deckhouse/deckhouse/pull/17154)
 - **[docs]** Added manifest for internal LB VK Cloud. [#16057](https://github.com/deckhouse/deckhouse/pull/16057)
 - **[ingress-nginx]** Added the `geoip_version` metric to NGINX Ingress Controller dashboards to indicate issues with GeoIP DB in the cluster. [#16449](https://github.com/deckhouse/deckhouse/pull/16449)
    All NGINX Ingress Controller pods will be restarted.
 - **[ingress-nginx]** Increased fault tolerance when the MaxMind service is unavailable or download limits are exceeded. [#15276](https://github.com/deckhouse/deckhouse/pull/15276)
    the instances will be restarted.
 - **[ingress-nginx]** Updated Nginx versions of NGINX Ingress Controller 1.10 and 1.12 to version 1.26.1. [#16476](https://github.com/deckhouse/deckhouse/pull/16476)
    NGINX Ingress Controller pods of versions 1.10 and 1.12 will be restarted.
 - **[istio]** Added FQDN support to the `alliance.ingressGateway.advertise` section. [#16488](https://github.com/deckhouse/deckhouse/pull/16488)
 - **[istio]** Added access log format setting for proxy sidecars. [#16129](https://github.com/deckhouse/deckhouse/pull/16129)
 - **[istio]** Allow custom ports in metadataEndpoint URLs for IstioFederation and IstioMulticluster CRDs. [#19322](https://github.com/deckhouse/deckhouse/pull/19322)
 - **[istio]** fixing the CVE in Kiali [#17097](https://github.com/deckhouse/deckhouse/pull/17097)
 - **[metallb]** Updated MetalLB version from 0.14.8 to 0.15.2. [#16210](https://github.com/deckhouse/deckhouse/pull/16210)
 - **[node-local-dns]** Optimized the cache plugin for clusters in dev mode. [#16535](https://github.com/deckhouse/deckhouse/pull/16535)
 - **[node-local-dns]** node-local-dns daemonset updating process is synced with cilium agents. [#16295](https://github.com/deckhouse/deckhouse/pull/16295)
 - **[node-manager]** Added Kubernetes feature gate management via the `control-plane-manager` module. [#16185](https://github.com/deckhouse/deckhouse/pull/16185)
 - **[node-manager]** Added validation to ensure `.spec.kubelet.maxPods` doesn't exceed the number of available Pod IP capacity per node. [#16695](https://github.com/deckhouse/deckhouse/pull/16695)
 - **[node-manager]** Denied applying of CAPS StaticInstance resources whose address is similar to any node in the Deckhouse cluster. [#15991](https://github.com/deckhouse/deckhouse/pull/15991)
 - **[node-manager]** Prevented user workload deployment during the node's first Bashible run. [#14828](https://github.com/deckhouse/deckhouse/pull/14828)
 - **[node-manager]** validate .spec.kubelet.maxPods against the number of available pod IPs per node [#16737](https://github.com/deckhouse/deckhouse/pull/16737)
 - **[prometheus]** Add prometheus configuration reload failed alert [#17128](https://github.com/deckhouse/deckhouse/pull/17128)
 - **[prometheus]** Replace PrometheusRules with ClusterObservabilityMetricsRulesGroups or ClusterObservabilityPropagatedMetricsRulesGroups when deployed using helm_lib_prometheus_rules helper and the observability module is enabled [#16405](https://github.com/deckhouse/deckhouse/pull/16405)
 - **[prometheus]** improve redirects from Grafana to the Deckhouse UI when Grafana is disabled [#16988](https://github.com/deckhouse/deckhouse/pull/16988)
    no impact
 - **[user-authn]** Added `spec.resources` (CPU, memory requests, limits) to DexAuthenticator and disabled VPA creation when it’s set. [#16226](https://github.com/deckhouse/deckhouse/pull/16226)

## Fixes


 - **[admission-policy-engine]** Allow DELETE operations, add containerPorts check in case of hostNetwork [#17084](https://github.com/deckhouse/deckhouse/pull/17084)
 - **[admission-policy-engine]** Fixed proxy support for trivy-provider [#16113](https://github.com/deckhouse/deckhouse/pull/16113)
 - **[admission-policy-engine]** Prevent unintended Gatekeeper constraints from being rendered for SecurityPolicy when boolean fields are omitted. [#18197](https://github.com/deckhouse/deckhouse/pull/18197)
    Workload Pods are no longer denied by unrelated SecurityPolicy checks (e.g. hostNetwork/hostPort) when corresponding policy fields are not explicitly set.
 - **[admission-policy-engine]** Prohibit only creation or modification for objects with vulnerable images [#16134](https://github.com/deckhouse/deckhouse/pull/16134)
 - **[admission-policy-engine]** Refactor constraint templates [#17882](https://github.com/deckhouse/deckhouse/pull/17882)
 - **[candi]** Add pause and kubernetes-api-proxy registry packages to bashible `bb-package-fetch` to prevent node failures during containerd major upgrades. [#17047](https://github.com/deckhouse/deckhouse/pull/17047)
 - **[candi]** Added `registry.d8-system.svc` to `no_proxy` list to bypass proxy for internal registry requests. [#16595](https://github.com/deckhouse/deckhouse/pull/16595)
 - **[candi]** Added a Netplan override to force the secondary NIC to use the main routing table, fixing cloud-init PBR conflicts. [#16625](https://github.com/deckhouse/deckhouse/pull/16625)
 - **[candi]** Allow manually stopping DVP node VirtualMachines in nested clusters by using runPolicy AlwaysOnUnlessStoppedManually. [#17110](https://github.com/deckhouse/deckhouse/pull/17110)
 - **[candi]** Applied a `PROMPT_COMMAND`-based PATH guard to restore expected PATH behavior when `~/.bashrc` overwrites PATH. [#16407](https://github.com/deckhouse/deckhouse/pull/16407)
 - **[candi]** Bashible script fix to handle multiple GPUs [#18158](https://github.com/deckhouse/deckhouse/pull/18158)
 - **[candi]** Excluded I/O loopback from node IP discovery. [#16179](https://github.com/deckhouse/deckhouse/pull/16179)
 - **[candi]** Fixed an issue in `bb-event-error-create` that prevented some logs from sending. [#16411](https://github.com/deckhouse/deckhouse/pull/16411)
 - **[candi]** Improved node-user retry logic to skip failing API servers. [#16493](https://github.com/deckhouse/deckhouse/pull/16493)
 - **[candi]** Refactored Bashible OS detection to use the new version map structure and shared package-manager helpers. [#16459](https://github.com/deckhouse/deckhouse/pull/16459)
 - **[candi]** Remove duplicate `additional_disks_hashes` definition in static-node Terraform module. [#17441](https://github.com/deckhouse/deckhouse/pull/17441)
 - **[candi]** Updated the bashible step to include Linux kernel versions that address CVE-2025-37999 [#17300](https://github.com/deckhouse/deckhouse/pull/17300)
 - **[candi]** fix if node has bashible-uninitialized taint in race condition. [#18133](https://github.com/deckhouse/deckhouse/pull/18133)
 - **[candi]** remove excessive netcat calls from d8-shutdown-inhibitor [#17240](https://github.com/deckhouse/deckhouse/pull/17240)
 - **[cilium-hubble]** Fix affinity in HA mode [#16862](https://github.com/deckhouse/deckhouse/pull/16862)
    In HA cluster mode hubble-ui and hubble-relay will be restarted
 - **[cilium-hubble]** Fixed CVE-2026-33186 in the hubble-ui image. [#18720](https://github.com/deckhouse/deckhouse/pull/18720)
 - **[cloud-provider-aws]** add information about AWS security group rules limits [#18853](https://github.com/deckhouse/deckhouse/pull/18853)
 - **[cloud-provider-aws]** fix cve [#16843](https://github.com/deckhouse/deckhouse/pull/16843)
 - **[cloud-provider-aws]** fix getInstancesByIDs to comply with the describeInstanceBatcher. [#18267](https://github.com/deckhouse/deckhouse/pull/18267)
 - **[cloud-provider-aws]** fix ssh access sg creation with disableDefaultSecurityGroup passed [#16081](https://github.com/deckhouse/deckhouse/pull/16081)
 - **[cloud-provider-azure]** fix build image for azure ccm [#16560](https://github.com/deckhouse/deckhouse/pull/16560)
 - **[cloud-provider-azure]** fix disk discovery for Ubuntu 22.04 Gen2 VMs with NVMe controllers [#19330](https://github.com/deckhouse/deckhouse/pull/19330)
 - **[cloud-provider-azure]** fixed cve [#16839](https://github.com/deckhouse/deckhouse/pull/16839)
 - **[cloud-provider-azure]** fixed patch in azure [#17696](https://github.com/deckhouse/deckhouse/pull/17696)
 - **[cloud-provider-dvp]** Added fixes an infinite deletion state of DeckhouseMachine. [#17585](https://github.com/deckhouse/deckhouse/pull/17585)
 - **[cloud-provider-dvp]** Added functionality to wait for a disk to be attached to a VM [#16965](https://github.com/deckhouse/deckhouse/pull/16965)
 - **[cloud-provider-dvp]** Cleanup orphaned resources when VM creation fails [#17533](https://github.com/deckhouse/deckhouse/pull/17533)
 - **[cloud-provider-dvp]** Correct the calculation of the path to the device [#16212](https://github.com/deckhouse/deckhouse/pull/16212)
 - **[cloud-provider-dvp]** Fix healthCheckNodePort collisions [#16996](https://github.com/deckhouse/deckhouse/pull/16996)
 - **[cloud-provider-dvp]** Fixed VM deletion timeout and improved memory validation error reporting [#17587](https://github.com/deckhouse/deckhouse/pull/17587)
 - **[cloud-provider-dvp]** Stopped preferring FQDN to hostname in cloud-init configurations. [#16124](https://github.com/deckhouse/deckhouse/pull/16124)
 - **[cloud-provider-dvp]** Validate VirtualMachineClass and boot images before VM creation to prevent Pending state [#17898](https://github.com/deckhouse/deckhouse/pull/17898)
 - **[cloud-provider-dvp]** added fixes an infinite deletion state of DeckhouseMachine. [#18163](https://github.com/deckhouse/deckhouse/pull/18163)
 - **[cloud-provider-dvp]** fixed CVE [#16810](https://github.com/deckhouse/deckhouse/pull/16810)
 - **[cloud-provider-dvp]** this PR gives a lot more informational errors and messages to user [#17165](https://github.com/deckhouse/deckhouse/pull/17165)
 - **[cloud-provider-dvp]** this changes fix some of cases when pods stuck in Completed/Error [#16741](https://github.com/deckhouse/deckhouse/pull/16741)
 - **[cloud-provider-huaweicloud]** Added `enterpriseProjectID` support for Cinder-based (<10Gi) volumes. [#16618](https://github.com/deckhouse/deckhouse/pull/16618)
 - **[cloud-provider-huaweicloud]** Updated the `caphc-controller-manager` component for the Huawei Cloud provider. [#16679](https://github.com/deckhouse/deckhouse/pull/16679)
 - **[cloud-provider-huaweicloud]** fix CSI unpublishValidation for non exist ECS instance [#16916](https://github.com/deckhouse/deckhouse/pull/16916)
 - **[cloud-provider-huaweicloud]** fix cve [#17171](https://github.com/deckhouse/deckhouse/pull/17171)
 - **[cloud-provider-openstack]** Add loadBalancer.enabled flag to prevent CCM crashes on k8s 1.32 without Octavia service [#18228](https://github.com/deckhouse/deckhouse/pull/18228)
 - **[cloud-provider-openstack]** Fixed discovery data merging for hybrid cases. [#16067](https://github.com/deckhouse/deckhouse/pull/16067)
 - **[cloud-provider-openstack]** Increase interval and timeout for health monitor [#19349](https://github.com/deckhouse/deckhouse/pull/19349)
 - **[cloud-provider-openstack]** fix LB.enabled flag [#18402](https://github.com/deckhouse/deckhouse/pull/18402)
 - **[cloud-provider-openstack]** fix cve [#17082](https://github.com/deckhouse/deckhouse/pull/17082)
 - **[cloud-provider-vcd]** Implemented a hack to migrate etcd disk to VCD independent disk to prevent deletion of etcd data. [#16302](https://github.com/deckhouse/deckhouse/pull/16302)
    To migrate, you must perform a `converge`, which causes the master server to be recreated. If you are using only one master server with the manual address assignment via the `mainNetworkIPAddresses` parameter, add two more IP addresses for the migration process.
 - **[cloud-provider-vcd]** fix cve [#17136](https://github.com/deckhouse/deckhouse/pull/17136)
 - **[cloud-provider-vsphere]** Fix maping for kubernetes data disk [#17859](https://github.com/deckhouse/deckhouse/pull/17859)
 - **[cloud-provider-vsphere]** Fix vSphere privilege matrix and describe instructions for setting up environment via vSphere Client [#18849](https://github.com/deckhouse/deckhouse/pull/18849)
 - **[cloud-provider-vsphere]** fix cloud-data-discoverer (SPBM) [#16589](https://github.com/deckhouse/deckhouse/pull/16589)
 - **[cloud-provider-vsphere]** fix cve [#17106](https://github.com/deckhouse/deckhouse/pull/17106)
 - **[cloud-provider-vsphere]** fix stale session for cloud-data-discoverer [#17089](https://github.com/deckhouse/deckhouse/pull/17089)
 - **[cloud-provider-vsphere]** fix vSphere storageClass template [#16275](https://github.com/deckhouse/deckhouse/pull/16275)
 - **[cloud-provider-yandex]** Terraform auto converger was failed for WithNATInstance layout. [#16427](https://github.com/deckhouse/deckhouse/pull/16427)
 - **[cloud-provider-yandex]** cloud-provider-yandex CVE's was fixed [#16611](https://github.com/deckhouse/deckhouse/pull/16611)
 - **[cloud-provider-yandex]** fix cve [#17469](https://github.com/deckhouse/deckhouse/pull/17469)
 - **[cloud-provider-zvirt]** fix CSI token refresh patch apply [#18449](https://github.com/deckhouse/deckhouse/pull/18449)
 - **[cloud-provider-zvirt]** fix cve [#17093](https://github.com/deckhouse/deckhouse/pull/17093)
 - **[cni-cilium]** Fix hook discovery_cni_exclusive.go [#17719](https://github.com/deckhouse/deckhouse/pull/17719)
    If the SDN module is used in the cluster, the Cilium agent pods will be restarted.
 - **[cni-cilium]** Fixed CVE-2026-33186, CVE-2026-27142, and CVE-2026-27139 by updating grpc dependency and Go version, and resolved build compatibility issues. [#18646](https://github.com/deckhouse/deckhouse/pull/18646)
 - **[cni-cilium]** Fixed egress-gateway-agent controller logic for deleted resources and disable dev logging. [#17378](https://github.com/deckhouse/deckhouse/pull/17378)
 - **[cni-cilium]** Some issues have been fixed in the EgressGateway. [#16479](https://github.com/deckhouse/deckhouse/pull/16479)
 - **[cni-cilium]** The MTU configuration has been updated. [#16751](https://github.com/deckhouse/deckhouse/pull/16751)
    The MTU will be updated on all interfaces of all pods.
 - **[cni-cilium]** Updated go-jose dependency to v4.1.4 to fix CVE-2026-34986. [#19011](https://github.com/deckhouse/deckhouse/pull/19011)
    Cilium agents will be restarted during the update.
 - **[cni-flannel]** Fixed CVE-2026-33186 by updating google.golang.org/grpc in flanneld. [#19106](https://github.com/deckhouse/deckhouse/pull/19106)
    This update triggers a rolling update of the flannel pods.
 - **[cni-simple-bridge]** Refactored python image source and pip exclusion. [#19151](https://github.com/deckhouse/deckhouse/pull/19151)
 - **[common]** Added `registry.d8-system.svc` to `no_proxy` list in `helm_lib` `_envs_for_proxy.tpl`. [#16595](https://github.com/deckhouse/deckhouse/pull/16595)
 - **[common]** Fixed CVE-2026-24051 in the CoreDNS image. [#18613](https://github.com/deckhouse/deckhouse/pull/18613)
 - **[common]** Fixed CVE-2026-33186 in the CoreDNS image. [#18723](https://github.com/deckhouse/deckhouse/pull/18723)
    CoreDNS pods will undergo a rolling restart.
 - **[common]** Latest CVEs are fixed. [#17222](https://github.com/deckhouse/deckhouse/pull/17222)
    All pods running kube-rbac-proxy will be restarted.
 - **[common]** Removed Python completely from the debug-container image as it is no longer needed, resolving corresponding CVEs, and silenced false positives for etcd binaries via VEX. [#18843](https://github.com/deckhouse/deckhouse/pull/18843)
 - **[control-plane-manager]** Add vex for CVE-2025-31133, CVE-2025-52881 . [#16337](https://github.com/deckhouse/deckhouse/pull/16337)
 - **[control-plane-manager]** Added explicit `etcd join` phase for control-plane scaling in 1.33. [#16609](https://github.com/deckhouse/deckhouse/pull/16609)
    Allows scaling control-plane from 1 to 3 in clusters where `ControlPlaneKubeletLocalMode=true`.
 - **[control-plane-manager]** Upgraded etcd to 3.6.7. [#17492](https://github.com/deckhouse/deckhouse/pull/17492)
    etcd will restart.
 - **[control-plane-manager]** upgrade etcd to 3.6.7. [#17537](https://github.com/deckhouse/deckhouse/pull/17537)
    etcd will restart.
 - **[dashboard]** Fixed CVE-2025-22868, CVE-2025-22870, CVE-2025-22872, CVE-2025-47914, CVE-2025-58181 [#17243](https://github.com/deckhouse/deckhouse/pull/17243)
 - **[dashboard]** Fixed CVE-2025-30204 by updating dashboard components [#16927](https://github.com/deckhouse/deckhouse/pull/16927)
 - **[deckhouse-controller]** A module that conditionally depends on another is no longer disabled when an incompatible version of that dependency is enabled; the enable is rejected instead. [#21190](https://github.com/deckhouse/deckhouse/pull/21190)
 - **[deckhouse-controller]** Don't create an external module release for a module that is still shipped embedded, so it can't replace or duplicate the embedded copy. [#21131](https://github.com/deckhouse/deckhouse/pull/21131)
 - **[deckhouse-controller]** Exclude all service accounts from `d8-` namespaces in `d8ms-prefix` ValidatingAdmissionPolicy. [#17440](https://github.com/deckhouse/deckhouse/pull/17440)
 - **[deckhouse-controller]** Fix conversions for external modules [#16772](https://github.com/deckhouse/deckhouse/pull/16772)
 - **[deckhouse-controller]** Fixed "multiple readiness hooks found" error on hook registration retry after a failure. [#16778](https://github.com/deckhouse/deckhouse/pull/16778)
 - **[deckhouse-controller]** Fixed a crash during external module updates with conversions that caused ModuleRelease to fail validation due to a forbidden property error. [#16546](https://github.com/deckhouse/deckhouse/pull/16546)
 - **[deckhouse-controller]** Fixed an issue where modules enabled through ModuleManager after migration bypassed ModuleConfig release validation. [#16673](https://github.com/deckhouse/deckhouse/pull/16673)
 - **[deckhouse-controller]** Fixed incorrect time value in minor release notification messages. [#16271](https://github.com/deckhouse/deckhouse/pull/16271)
 - **[deckhouse-controller]** Fixed module documentation collection from EROFS mounted modules. [#16495](https://github.com/deckhouse/deckhouse/pull/16495)
 - **[deckhouse-controller]** Now whenever hooks fail, Deckhouse handles and returns corresponding metrics along with an error. [#16319](https://github.com/deckhouse/deckhouse/pull/16319)
 - **[deckhouse-controller]** Replaced old binary compaction metrics with new informative metrics that show actual compaction frequency and queue load per hook. [#16659](https://github.com/deckhouse/deckhouse/pull/16659)
 - **[deckhouse]** Fix module docs rendering. [#17245](https://github.com/deckhouse/deckhouse/pull/17245)
 - **[deckhouse]** Fix module enabling. [#17057](https://github.com/deckhouse/deckhouse/pull/17057)
 - **[deckhouse]** Fix module installer cleanup. [#17301](https://github.com/deckhouse/deckhouse/pull/17301)
 - **[deckhouse]** Fix module rerun. [#17478](https://github.com/deckhouse/deckhouse/pull/17478)
 - **[deckhouse]** Fix validation logic for a disabled module [#16385](https://github.com/deckhouse/deckhouse/pull/16385)
 - **[deckhouse]** Fix webhook-handler CVEs. [#19078](https://github.com/deckhouse/deckhouse/pull/19078)
 - **[deckhouse]** Remove notified=false annotation reset from runReleaseDeploy in the module release controller. [#19176](https://github.com/deckhouse/deckhouse/pull/19176)
 - **[dhctl]** Added nil check to dhctl during converge in migrator [#16289](https://github.com/deckhouse/deckhouse/pull/16289)
 - **[dhctl]** Added validation of the command execution status code [#19292](https://github.com/deckhouse/deckhouse/pull/19292)
 - **[dhctl]** Fix StaticInstance readiness check and refactoring readiness check for resources. [#16616](https://github.com/deckhouse/deckhouse/pull/16616)
 - **[dhctl]** Fix converge manifests for static cluster in commander. [#16504](https://github.com/deckhouse/deckhouse/pull/16504)
 - **[dhctl]** Fix getting passphrase for key from connection config for cli. [#16100](https://github.com/deckhouse/deckhouse/pull/16100)
 - **[dhctl]** Fix panic during destroy. Change opentofu log level to INFO. [#16726](https://github.com/deckhouse/deckhouse/pull/16726)
 - **[dhctl]** Fix panic in dhctl config render kubeadm-config command. [#17934](https://github.com/deckhouse/deckhouse/pull/17934)
 - **[dhctl]** Fix parallel bootstrap cloud permanent nodes [#16886](https://github.com/deckhouse/deckhouse/pull/16886)
 - **[dhctl]** Fixed a memory leak in Terraform exporter. [#15350](https://github.com/deckhouse/deckhouse/pull/15350)
 - **[dhctl]** Fixed dhctl in SSH tunnel preflight check. [#17805](https://github.com/deckhouse/deckhouse/pull/17805)
 - **[dhctl]** Fixed endless converge loop for clusters with NAT instances. [#16230](https://github.com/deckhouse/deckhouse/pull/16230)
 - **[dhctl]** Improved reliability when connecting to dhctl servers by adding retry logic and better error handling during startup [#17698](https://github.com/deckhouse/deckhouse/pull/17698)
 - **[dhctl]** Isolated temporary directory for singleshot RPC and dhctl to avoid cleanup race. [#15794](https://github.com/deckhouse/deckhouse/pull/15794)
 - **[dhctl]** Move yandex withNATInstance layout settings from preflights to preparator. [#16100](https://github.com/deckhouse/deckhouse/pull/16100)
 - **[dhctl]** Now the `AllowTcpForwarding` preflight check can interrupt a bootstrap process. [#16250](https://github.com/deckhouse/deckhouse/pull/16250)
 - **[dhctl]** Now the dhctl dependency validation can run within a single SSH connection. [#16120](https://github.com/deckhouse/deckhouse/pull/16120)
 - **[dhctl]** Validate WithNATInstance Yandex layout params only in bootstrap. [#16427](https://github.com/deckhouse/deckhouse/pull/16427)
 - **[dhctl]** fix dhctl in SSH tunnel preflight check [#17881](https://github.com/deckhouse/deckhouse/pull/17881)
 - **[dhctl]** mitigate CVE-2026-33186 [#18620](https://github.com/deckhouse/deckhouse/pull/18620)
 - **[docs]** Fix vSphere privilege matrix and describe instructions for setting up environment via vSphere Client [#18849](https://github.com/deckhouse/deckhouse/pull/18849)
 - **[extended-monitoring]** Add namespace-scoped overrides [#17213](https://github.com/deckhouse/deckhouse/pull/17213)
 - **[extended-monitoring]** Cleanup exporter metrics when the monitored resource has been deleted [#17988](https://github.com/deckhouse/deckhouse/pull/17988)
 - **[extended-monitoring]** Fix extended-monitoring.deckhouse.io/enabled label handling [#16372](https://github.com/deckhouse/deckhouse/pull/16372)
    the extended monitoring will only be enabled when the label is explicitly set on a namespace
 - **[extended-monitoring]** drop metrics when extended monitoring is disabled for node(s) [#16446](https://github.com/deckhouse/deckhouse/pull/16446)
    erroneous alerts for node disk usage are fixed
 - **[ingress-nginx]** A false-positive trigger of alert GeoIPDownloadErrorDetectedFromMaxMind is fixed. [#17741](https://github.com/deckhouse/deckhouse/pull/17741)
 - **[ingress-nginx]** A symlink to the new opentelemetry config path is added. [#16433](https://github.com/deckhouse/deckhouse/pull/16433)
    Ingress-Nginx controller's pods of 1.9 version will be restarted.
 - **[ingress-nginx]** CVE-2025-15566 is backported. [#19208](https://github.com/deckhouse/deckhouse/pull/19208)
    All pods of Ingress-NGINX controller will be restarted.
 - **[ingress-nginx]** CVE-2026-3288 fix is backported in all Ingress-Nginx controllers. [#18410](https://github.com/deckhouse/deckhouse/pull/18410)
    All Ingress-Nginx controller pods will be restarted.
 - **[ingress-nginx]** CVE-2026-4342 fix is backported to Dechkouse 1.74. [#18946](https://github.com/deckhouse/deckhouse/pull/18946)
    All Ingress-NGINX controller pods will be restarted.
 - **[ingress-nginx]** CVEs fixed [#16340](https://github.com/deckhouse/deckhouse/pull/16340)
 - **[ingress-nginx]** Fixed CVEs [#16432](https://github.com/deckhouse/deckhouse/pull/16432)
 - **[ingress-nginx]** Improved stability of geoproxy service startup. [#17140](https://github.com/deckhouse/deckhouse/pull/17140)
 - **[ingress-nginx]** Latest CVEs are fixed. [#17222](https://github.com/deckhouse/deckhouse/pull/17222)
    All pods running kube-rbac-proxy will be restarted.
 - **[ingress-nginx]** Nginx and module's dependencies are updated. [#18156](https://github.com/deckhouse/deckhouse/pull/18156)
    All ingress-nginx controller pods will be restared.
 - **[ingress-nginx]** The CVE-2026-1580, CVE-2026-24512, CVE-2026-24513, CVE-2026-24514 CVEs fixes are backported. [#17823](https://github.com/deckhouse/deckhouse/pull/17823)
    The ingress nginx controllers' pods will be restated.
 - **[istio]** Correction  in Kiali of an insignificant error [#16880](https://github.com/deckhouse/deckhouse/pull/16880)
 - **[istio]** Correction of an useless error in the Istio CNI workflow [#17787](https://github.com/deckhouse/deckhouse/pull/17787)
 - **[istio]** Erroneous option in 1.25 control-plane helm template fixed. [#16412](https://github.com/deckhouse/deckhouse/pull/16412)
 - **[istio]** Fix CVE for Istio version 1.21 and 1.25 [#17298](https://github.com/deckhouse/deckhouse/pull/17298)
 - **[istio]** Fixed AuthorizationPolicy CRD insufficiency for Istio 1.25. [#16605](https://github.com/deckhouse/deckhouse/pull/16605)
 - **[istio]** Fixed false-positive alert `D8IstioRemoteClusterNotSynced` and improved its description. [#15826](https://github.com/deckhouse/deckhouse/pull/15826)
 - **[istio]** Fixing the list of requests from istiod to gateway API [#18056](https://github.com/deckhouse/deckhouse/pull/18056)
 - **[istio]** Implement graceful metadata secret renewal for multiclusters. [#20192](https://github.com/deckhouse/deckhouse/pull/20192)
 - **[istio]** Reduce CPU and RAM for regenerate multicluster JWT token and sort ingressGateway [#18569](https://github.com/deckhouse/deckhouse/pull/18569)
 - **[istio]** The same owner is specified for the files that are used to run in the operator container. [#16154](https://github.com/deckhouse/deckhouse/pull/16154)
 - **[istio]** added iptables wrapper in cni-v1x21x6 [#18954](https://github.com/deckhouse/deckhouse/pull/18954)
    istio-cni-nodes will be restarted
 - **[istio]** fixed CVE-2026-34986 [#18973](https://github.com/deckhouse/deckhouse/pull/18973)
    istio module pods will be restarted
 - **[istio]** fixed CVE-2026-39882, CVE-2026-39883 and CVE-2026-35206 [#19091](https://github.com/deckhouse/deckhouse/pull/19091)
    istio module pods will be restarted
 - **[istio]** fixed CVEs in module images [#19363](https://github.com/deckhouse/deckhouse/pull/19363)
    module pods will be restarted
 - **[istio]** fixed cve in module images [#18576](https://github.com/deckhouse/deckhouse/pull/18576)
    pods in d8-istio namespace will be restarted
 - **[istio]** fixing the CVE in Kiali [#17045](https://github.com/deckhouse/deckhouse/pull/17045)
 - **[keepalived]** Excluded vulnerable pip-25.3 from keepalived final image to fix CVE-2026-1703 [#19150](https://github.com/deckhouse/deckhouse/pull/19150)
 - **[kube-dns]** Improved /etc/hosts renderer compatibility with admission-policy-engine Restricted mode. [#16599](https://github.com/deckhouse/deckhouse/pull/16599)
 - **[kube-proxy]** Fixed CVE-2026-33186 and CVE-2026-24051 in kube-proxy dependencies. [#19119](https://github.com/deckhouse/deckhouse/pull/19119)
    This update triggers a rolling update of the kube-proxy pods.
 - **[loki]** Fixed the `LokiDiscardedSamplesWarning` alert. [#16374](https://github.com/deckhouse/deckhouse/pull/16374)
 - **[loki]** disable send analytics report to stats.grafana.org [#17109](https://github.com/deckhouse/deckhouse/pull/17109)
    config module loki ↓
 - **[monitoring-kubernetes]** Rollout changes for resources metrics kubelet [#16408](https://github.com/deckhouse/deckhouse/pull/16408)
 - **[monitoring-kubernetes]** fix CVE-2025-52881 for node-exporter [#16376](https://github.com/deckhouse/deckhouse/pull/16376)
 - **[monitoring-kubernetes]** fix kube_persistentvolume_is_local recording rule for not-bound PVCs [#17638](https://github.com/deckhouse/deckhouse/pull/17638)
 - **[monitoring-kubernetes]** fix kube_persistentvolume_is_local recording rule when there are more than one kube-state-metrics exporter in cluster [#17877](https://github.com/deckhouse/deckhouse/pull/17877)
 - **[monitoring-kubernetes]** remove the Docker traces from the module code [#16542](https://github.com/deckhouse/deckhouse/pull/16542)
    node-exporter pods will be rollout restarted during upgrade
 - **[multitenancy-manager]** Fixed indentation in the manifest of `multitenancy-manager`. [#16471](https://github.com/deckhouse/deckhouse/pull/16471)
 - **[multitenancy-manager]** fix CVE-2024-25621  CVE-2025-64329 [#16360](https://github.com/deckhouse/deckhouse/pull/16360)
 - **[network-gateway]** Fixed werf import syntax for compatibility with older werf versions. [#19276](https://github.com/deckhouse/deckhouse/pull/19276)
 - **[network-gateway]** Updated dnsmasq to v2.92-alt2 to address multiple security vulnerabilities (CVE-2026-*) [#19935](https://github.com/deckhouse/deckhouse/pull/19935)
 - **[network-gateway]** Updated python image source and mitigated pip CVE-2026-1703 [#19147](https://github.com/deckhouse/deckhouse/pull/19147)
 - **[network-policy-engine]** Fixed CVE-2026-34040, CVE-2026-33997, and CVE-2026-33186 in network-policy-engine dependencies. [#19108](https://github.com/deckhouse/deckhouse/pull/19108)
    This update triggers a rolling update of the network-policy-engine pods.
 - **[node-local-dns]** Fix name of registry secret in safe-updater deployment [#19889](https://github.com/deckhouse/deckhouse/pull/19889)
 - **[node-local-dns]** Return stale-dns-connections-cleaner [#18740](https://github.com/deckhouse/deckhouse/pull/18740)
    An additional service daemonset will be added.
 - **[node-manager]** Added cleanup for oversized MCM MachineSet revision history annotation [#19657](https://github.com/deckhouse/deckhouse/pull/19657)
 - **[node-manager]** Added early StaticInstance reservation with automatic rollback on failure. [#16315](https://github.com/deckhouse/deckhouse/pull/16315)
 - **[node-manager]** Fix panic in cluster-autoscaler caused by nil pointer dereference during node removal simulation. [#17924](https://github.com/deckhouse/deckhouse/pull/17924)
 - **[node-manager]** Fix panic in registry packages proxy if image not found. [#16425](https://github.com/deckhouse/deckhouse/pull/16425)
 - **[node-manager]** Fixed `bashible-apiserver` checksum update. [#16621](https://github.com/deckhouse/deckhouse/pull/16621)
 - **[node-manager]** Fixed `mig-manager` reconfigure script to correctly handle auto-approved disruptive node group changes. [#16655](https://github.com/deckhouse/deckhouse/pull/16655)
 - **[node-manager]** It fixes issues in the DaemonSet manifest for fencing module. [#17087](https://github.com/deckhouse/deckhouse/pull/17087)
 - **[node-manager]** Moved `bb-label-node-bashible-first-run-finished` to a Bashible template. [#16307](https://github.com/deckhouse/deckhouse/pull/16307)
 - **[node-manager]** Set to rescan power-button input devices and refreshes stale descriptors, ensuring the shutdown inhibitor continues receiving button-press events. [#16651](https://github.com/deckhouse/deckhouse/pull/16651)
 - **[node-manager]** Updated helm-lib to ensure `privileged` is set to `false` whenever `allowPrivilegeEscalation` is `false`, preventing invalid configurations after the switch to SSA. [#16562](https://github.com/deckhouse/deckhouse/pull/16562)
 - **[node-manager]** remove excessive netcat calls from d8-shutdown-inhibitor [#17240](https://github.com/deckhouse/deckhouse/pull/17240)
 - **[operator-trivy]** Add grep to node-collector and improve error reporting [#16277](https://github.com/deckhouse/deckhouse/pull/16277)
 - **[operator-trivy]** Fix CIS Benchmark report template [#16489](https://github.com/deckhouse/deckhouse/pull/16489)
 - **[prometheus]** Added `ingressClassName` to the `grafana/prometheus` redirect Ingress. [#16116](https://github.com/deckhouse/deckhouse/pull/16116)
 - **[prometheus]** Fix description for not usable CVE [#16377](https://github.com/deckhouse/deckhouse/pull/16377)
 - **[prometheus]** Fix namespace label value in the Ingress Nginx controller and several other metrics [#16720](https://github.com/deckhouse/deckhouse/pull/16720)
    Ingress Nginx controller dashboards are fixed
 - **[prometheus]** Make Grafana redirect ingress pass the annotations validation. [#17816](https://github.com/deckhouse/deckhouse/pull/17816)
 - **[registry]** Fixed validation of input image list changes in the registry checker. [#17472](https://github.com/deckhouse/deckhouse/pull/17472)
 - **[registry]** Omitted the auth field in DockerConfig when credentials (username and password) are empty. [#17333](https://github.com/deckhouse/deckhouse/pull/17333)
 - **[registry]** Updated auth image Go dependencies to fix Go CVEs. [#18232](https://github.com/deckhouse/deckhouse/pull/18232)
    Registry pods will be restarted.
 - **[registrypackages]** Added `which` to RPP. [#16563](https://github.com/deckhouse/deckhouse/pull/16563)
 - **[registrypackages]** Added vex with CVE-2026-33186. [#18686](https://github.com/deckhouse/deckhouse/pull/18686)
 - **[registrypackages]** Fixed permissions for directory with cni-plugins in PCI-DSS clusters [#16409](https://github.com/deckhouse/deckhouse/pull/16409)
 - **[registrypackages]** Fixes CVE in kubernetes-cni [#16343](https://github.com/deckhouse/deckhouse/pull/16343)
 - **[registrypackages]** Update containerd to 1.7.29 / 2.1.5 and runc to 1.3.3 [#16335](https://github.com/deckhouse/deckhouse/pull/16335)
 - **[registrypackages]** Update integrity patch for containerd (cse only). [#17000](https://github.com/deckhouse/deckhouse/pull/17000)
 - **[registrypackages]** Update runc to 1.3.1. [#16263](https://github.com/deckhouse/deckhouse/pull/16263)
 - **[registrypackages]** Upgrade containerd to 1.7.30 and 2.1.6. [#17639](https://github.com/deckhouse/deckhouse/pull/17639)
    Containerd will restart.
 - **[registrypackages]** Upgraded containerd to 1.7.30 and 2.1.6. [#17510](https://github.com/deckhouse/deckhouse/pull/17510)
    Containerd will restart.
 - **[service-with-healthchecks]** Fixed CVEs [#16950](https://github.com/deckhouse/deckhouse/pull/16950)
 - **[terraform-manager]** Update Yandex Terraform version. [#17142](https://github.com/deckhouse/deckhouse/pull/17142)
 - **[terraform-manager]** yandex terraform version was updated [#16779](https://github.com/deckhouse/deckhouse/pull/16779)
 - **[upmeter]** fix securityxontext for statefulset [#16534](https://github.com/deckhouse/deckhouse/pull/16534)
    upmeter check
 - **[user-authn]** Added a warning that DexAuthenticator only works over HTTPS. [#16721](https://github.com/deckhouse/deckhouse/pull/16721)
 - **[user-authn]** Fix BadRequest after the change password redirect when password policy is enabled [#16744](https://github.com/deckhouse/deckhouse/pull/16744)
 - **[user-authn]** Fix login error 500 with password policy enabled. [#16703](https://github.com/deckhouse/deckhouse/pull/16703)
 - **[user-authn]** Quote service names to prevent digit-only names from breaking yaml parser [#17020](https://github.com/deckhouse/deckhouse/pull/17020)
 - **[user-authn]** Rollback patch for handling insecureSkipEmailVerified condition [#16347](https://github.com/deckhouse/deckhouse/pull/16347)
 - **[user-authn]** skipApproval no longer bypasses TOTP. When 2FA is enabled, users are sent to /totp before approval, so “auth request does not have an identity for approval” no longer occurs [#16946](https://github.com/deckhouse/deckhouse/pull/16946)
 - **[user-authz]** Allow project-scoped roles to access Cluster-wide objects [#16896](https://github.com/deckhouse/deckhouse/pull/16896)
 - **[user-authz]** cache namespace label checks in the user-authz webhook via informer to avoid per-request apiserver GETs [#16920](https://github.com/deckhouse/deckhouse/pull/16920)

## Chore


 - **[candi]** Bumped patch versions of Kubernetes images and CVE fixes. [#16455](https://github.com/deckhouse/deckhouse/pull/16455)
    Kubernetes control-plane components and kubelet will restart.
 - **[cilium-hubble]** Added vex with CVE-2026-33726 for hubble [#18921](https://github.com/deckhouse/deckhouse/pull/18921)
 - **[cilium-hubble]** Fixed vex file [#19015](https://github.com/deckhouse/deckhouse/pull/19015)
 - **[cloud-provider-openstack]** Fix linter warning for cloud-provider-openstack [#18959](https://github.com/deckhouse/deckhouse/pull/18959)
 - **[cni-cilium]** Added vex with CVE-2026-33726 for hubble [#18921](https://github.com/deckhouse/deckhouse/pull/18921)
 - **[cni-cilium]** Fixed vex file [#19015](https://github.com/deckhouse/deckhouse/pull/19015)
 - **[cni-cilium]** Refactor build to use pre-packaged dependencies from envoyproxy_deps repository instead of downloading from GitHub at build time [#18939](https://github.com/deckhouse/deckhouse/pull/18939)
    Cilium agents will be restarted.
 - **[common]** update pip version [#16228](https://github.com/deckhouse/deckhouse/pull/16228)
 - **[deckhouse-controller]** update pip version [#16228](https://github.com/deckhouse/deckhouse/pull/16228)
 - **[deckhouse-tools]** update pip version [#16228](https://github.com/deckhouse/deckhouse/pull/16228)
 - **[deckhouse]** Bumped `addon-operator` dependency to ignore absent chart file. [#15949](https://github.com/deckhouse/deckhouse/pull/15949)
 - **[deckhouse]** update pip version [#16228](https://github.com/deckhouse/deckhouse/pull/16228)
 - **[descheduler]** Grant RBAC for PersistentVolumeClaims so the descheduler can list and watch PVCs [#18787](https://github.com/deckhouse/deckhouse/pull/18787)
 - **[dhctl]** Disabled Bashible debug console when launched via `commander` and capped retries to 10 restarts and 5 attempts per step. [#15738](https://github.com/deckhouse/deckhouse/pull/15738)
 - **[dhctl]** Expand SSH output logs on errors for debug, verbose purposes. [#16915](https://github.com/deckhouse/deckhouse/pull/16915)
 - **[dhctl]** Fixed `gossh` client reconnections. [#16709](https://github.com/deckhouse/deckhouse/pull/16709)
 - **[dhctl]** Set default ssh port to 22, to backward compatibility with cli ssh behavior in dhctl. [#16947](https://github.com/deckhouse/deckhouse/pull/16947)
 - **[docs]** Add NGC examples for automatically installation of NVIDIA drivers. [#16864](https://github.com/deckhouse/deckhouse/pull/16864)
 - **[ingress-nginx]** Fix CVEs in sources [#16190](https://github.com/deckhouse/deckhouse/pull/16190)
 - **[ingress-nginx]** Improved documentation for the ModSecurity (WAF). [#16268](https://github.com/deckhouse/deckhouse/pull/16268)
 - **[ingress-nginx]** removed deprecated alert GeoIPDownloadErrorDetected. [#17751](https://github.com/deckhouse/deckhouse/pull/17751)
    All instances will be restarted.
 - **[istio]** Changing the multi-network Istio documentation [#18591](https://github.com/deckhouse/deckhouse/pull/18591)
 - **[istio]** Fix CVEs in sources [#16191](https://github.com/deckhouse/deckhouse/pull/16191)
 - **[istio]** Vex justified CVE-2026-42151, CVE-2026-42154 and CVE-2026-44903 in pilot and operator images. Fixed CVE-2026-46680 in operator 1.25 [#20034](https://github.com/deckhouse/deckhouse/pull/20034)
 - **[istio]** Vex mitigation implementation [#20564](https://github.com/deckhouse/deckhouse/pull/20564)
 - **[istio]** Warning about the inability to use user 1337 for user applications [#18592](https://github.com/deckhouse/deckhouse/pull/18592)
 - **[loki]** Added alerts and graphs for discarded log samples. [#16137](https://github.com/deckhouse/deckhouse/pull/16137)
 - **[monitoring-custom]** Add clarification to D8ReservedNodeLabelOrTaintFound alert description. [#19295](https://github.com/deckhouse/deckhouse/pull/19295)
 - **[monitoring-kubernetes]** Added missing severity_level label to the PodStatusIsIncorrect alert. [#16549](https://github.com/deckhouse/deckhouse/pull/16549)
 - **[node-local-dns]** Added logging of slow upstream queries and a new coredns_kubeforward_slow_requests_total metric for tracking them. [#16808](https://github.com/deckhouse/deckhouse/pull/16808)
 - **[node-local-dns]** Removed `stale-dns-connections-cleaner`, since the related issue was fixed in `cni-cilium` upstream. [#16447](https://github.com/deckhouse/deckhouse/pull/16447)
 - **[node-manager]** Node inhibitor was migrated to the kubernetes.io library. [#16237](https://github.com/deckhouse/deckhouse/pull/16237)
 - **[prometheus]** update pip version [#16228](https://github.com/deckhouse/deckhouse/pull/16228)
 - **[registry]** Update dependencies to fix CVEs [#16635](https://github.com/deckhouse/deckhouse/pull/16635)
 - **[registrypackages]** update pip version [#16228](https://github.com/deckhouse/deckhouse/pull/16228)
