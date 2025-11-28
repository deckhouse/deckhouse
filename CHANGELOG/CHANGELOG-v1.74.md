# Changelog v1.74

## [MALFORMED]


 - #16209 missing section, missing summary, missing type, unknown section ""
 - #16354 missing section, missing summary, missing type, unknown section ""

## Know before update


 - The minimum supported version of Kubernetes is now 1.30. All control plane components will restart.

## Features


 - **[candi]** Add capability version check to check_gpu step. [#16665](https://github.com/deckhouse/deckhouse/pull/16665)
 - **[candi]** Add dynamic default for maxPods based on podSubnetNodeCIDRPrefix. [#16564](https://github.com/deckhouse/deckhouse/pull/16564)
    Kubelet will restart. The maximum default number of pods per node now automatically depends on the node CIDR size.
 - **[candi]** Enable unconditional CDI support [#16313](https://github.com/deckhouse/deckhouse/pull/16313)
    Containerd v1 will be restarted
 - **[candi]** Enable feature gates for Dynamic Resource Allocation [#16311](https://github.com/deckhouse/deckhouse/pull/16311)
    kubelet, api-server, scheduler will be restarted
 - **[candi]** Add k8s featureGates management via the control-plane-manager module. [#16185](https://github.com/deckhouse/deckhouse/pull/16185)
 - **[candi]** added support for Kubernetes 1.34 and discontinued support for Kubernetes 1.29. [#15518](https://github.com/deckhouse/deckhouse/pull/15518)
    The minimum supported version of Kubernetes is now 1.30. All control plane components will restart.
 - **[cloud-provider-huaweicloud]** allow users to overwrite default NIC in both CloudPermanent and CloudEphemeral [#15810](https://github.com/deckhouse/deckhouse/pull/15810)
 - **[cloud-provider-huaweicloud]** add Virtual IP support [#15600](https://github.com/deckhouse/deckhouse/pull/15600)
 - **[cni-cilium]** Add the ability to configure the mapDynamicSizeRatio parameter for specific nodes using CiliumNodeConfig. [#16326](https://github.com/deckhouse/deckhouse/pull/16326)
 - **[cni-cilium]** Add SCTP support [#16297](https://github.com/deckhouse/deckhouse/pull/16297)
 - **[cni-cilium]** add prometheus metric `bpf_progs_complexity_max_verified_insts ` for maximum bpf instruction complexity, available with kernel >= 5.16. [#14723](https://github.com/deckhouse/deckhouse/pull/14723)
 - **[control-plane-manager]** Dynamic setting of terminated-pod-gc-threshold depends on number of nodes in cluster [#16266](https://github.com/deckhouse/deckhouse/pull/16266)
    After upgrading Deckhouse with this feature, the kube-controller-manager will be restarted, and the default value of terminated-pod-gc-threshold will be reconfigured
 - **[control-plane-manager]** Add k8s featureGates management via the control-plane-manager module. [#16185](https://github.com/deckhouse/deckhouse/pull/16185)
 - **[deckhouse]** Integrity control for modules - use read only file system model. [#15019](https://github.com/deckhouse/deckhouse/pull/15019)
 - **[deckhouse-controller]** add package status service [#16465](https://github.com/deckhouse/deckhouse/pull/16465)
 - **[deckhouse-controller]** switch on nelm in controller logic [#16142](https://github.com/deckhouse/deckhouse/pull/16142)
 - **[deckhouse-controller]** Add foundational API structures and controllers for Package System. [#16016](https://github.com/deckhouse/deckhouse/pull/16016)
 - **[deckhouse-controller]** collect-debug-info command has been moved to the d8 utility. [#15767](https://github.com/deckhouse/deckhouse/pull/15767)
 - **[deckhouse-controller]** Restrict using of `d8ms-*` prefix for all objects. [#15147](https://github.com/deckhouse/deckhouse/pull/15147)
    Objects with prefix `d8ms-` could NOT be created by users in their's D8 clusters.
 - **[dhctl]** Isolate temp dir for singleshot RPC and dhctl to avoid cleanup race. [#15794](https://github.com/deckhouse/deckhouse/pull/15794)
 - **[ingress-nginx]** Nginx versions of Ingress-Nginx controllers of 1.10 and 1.12 versions are updated to 1.26.1. [#16476](https://github.com/deckhouse/deckhouse/pull/16476)
    Ingress-nginx controllers' pods of 1.10 and 1.12 will be restarted.
 - **[ingress-nginx]** GeoIP version metric is reported by the controllers and this information is available on the Ingress Nginx Controllers dashboard. [#16449](https://github.com/deckhouse/deckhouse/pull/16449)
    All ingress-nginx controllers' pods will be restarted.
 - **[istio]** Added fqdn support in `alliance.ingressGateway.advertise` section. [#16488](https://github.com/deckhouse/deckhouse/pull/16488)
 - **[metallb]** Updated MetalLB version from 0.14.8 to 0.15.2. [#16210](https://github.com/deckhouse/deckhouse/pull/16210)
 - **[node-local-dns]** The cache has been optimized for the dev mode of the cluster. [#16535](https://github.com/deckhouse/deckhouse/pull/16535)
 - **[node-manager]** validate .spec.kubelet.maxPods against the number of available pod IPs per node [#16695](https://github.com/deckhouse/deckhouse/pull/16695)
 - **[node-manager]** Add k8s featureGates management via the control-plane-manager module. [#16185](https://github.com/deckhouse/deckhouse/pull/16185)
 - **[node-manager]** deny use CAPS StaticInstance if address similar any node in DKP [#15991](https://github.com/deckhouse/deckhouse/pull/15991)
 - **[node-manager]** Prevent users workload deploy on nodes during first bashible running steps. [#14828](https://github.com/deckhouse/deckhouse/pull/14828)
 - **[user-authn]** Add `spec.resources` to `DexAuthenticator`; disable VPA for the instance when it’s set [#16226](https://github.com/deckhouse/deckhouse/pull/16226)

## Fixes


 - **[candi]** Add `registry.d8-system.svc` to `no_proxy` list to bypass proxy for internal registry requests. [#16595](https://github.com/deckhouse/deckhouse/pull/16595)
 - **[candi]** Improve node-user retry logic to skip failing API servers. [#16493](https://github.com/deckhouse/deckhouse/pull/16493)
 - **[candi]** Refactor bashible OS detection to use the new version map structure and shared package-manager helpers [#16459](https://github.com/deckhouse/deckhouse/pull/16459)
 - **[candi]** bb-event-error-create function fix [#16411](https://github.com/deckhouse/deckhouse/pull/16411)
 - **[candi]** Bashible deckhouse path special case for AltLinux. [#16407](https://github.com/deckhouse/deckhouse/pull/16407)
 - **[candi]** Exclude I/O loopback from node ip discovery. [#16179](https://github.com/deckhouse/deckhouse/pull/16179)
 - **[cloud-provider-dvp]** Stop preferring fqdn to hostname in cloud-init configurations. [#16124](https://github.com/deckhouse/deckhouse/pull/16124)
 - **[cloud-provider-huaweicloud]** fix Provider ID [#16679](https://github.com/deckhouse/deckhouse/pull/16679)
 - **[cloud-provider-huaweicloud]** add EnterpriseProjectID support for cinder-based (<10Gi) volumes [#16618](https://github.com/deckhouse/deckhouse/pull/16618)
 - **[cloud-provider-openstack]** fix discovery data merging for hybrid cases [#16067](https://github.com/deckhouse/deckhouse/pull/16067)
 - **[cloud-provider-vcd]** Implemented a hack to migrate etcd disk to VCD independent disk to prevent deletion of etcd data. [#16302](https://github.com/deckhouse/deckhouse/pull/16302)
    To migrate, you must perform a `converge`, which causes the master server to be recreated. If you are using only one master server with the manual address assignment via the `mainNetworkIPAddresses` parameter, add two more IP addresses for the migration process.
 - **[common]** Add `registry.d8-system.svc` to `no_proxy` list in `helm_lib` `_envs_for_proxy.tpl`. [#16595](https://github.com/deckhouse/deckhouse/pull/16595)
 - **[control-plane-manager]** Add explicit “etcd join” phase for control-plane scaling in 1.33 [#16609](https://github.com/deckhouse/deckhouse/pull/16609)
    Allows scaling control-plane from 1→3 in clusters where ControlPlaneKubeletLocalMode=true.
 - **[deckhouse-controller]** Fixed verifying migrated modules [#16673](https://github.com/deckhouse/deckhouse/pull/16673)
 - **[deckhouse-controller]** Make compaction metrics provide actionable observability [#16659](https://github.com/deckhouse/deckhouse/pull/16659)
 - **[deckhouse-controller]** fix conversion applying for external modules [#16546](https://github.com/deckhouse/deckhouse/pull/16546)
 - **[deckhouse-controller]** Fixed module documentation collection from EROFS mounted modules [#16495](https://github.com/deckhouse/deckhouse/pull/16495)
 - **[deckhouse-controller]** handle metrics if hook are failed [#16319](https://github.com/deckhouse/deckhouse/pull/16319)
 - **[deckhouse-controller]** Fix incorrect time value in minor release notification messages [#16271](https://github.com/deckhouse/deckhouse/pull/16271)
 - **[dhctl]** make AllowTcpForwarding preflight check interrupt bootstrap proccess [#16250](https://github.com/deckhouse/deckhouse/pull/16250)
 - **[dhctl]** Fix endless converge loop for clusters with NAT instances [#16230](https://github.com/deckhouse/deckhouse/pull/16230)
 - **[dhctl]** Check all dhctl dependencies from single SSH connection. [#16120](https://github.com/deckhouse/deckhouse/pull/16120)
 - **[dhctl]** Fix and refactor cleaning temp dir function for better cleaning. [#15794](https://github.com/deckhouse/deckhouse/pull/15794)
 - **[istio]** Fixied false positive alert D8IstioRemoteClusterNotSynced and improved description it. [#15826](https://github.com/deckhouse/deckhouse/pull/15826)
 - **[loki]** fix for Discarded Samples alert [#16374](https://github.com/deckhouse/deckhouse/pull/16374)
 - **[multitenancy-manager]** Fix indent for non-vpa resources block [#16471](https://github.com/deckhouse/deckhouse/pull/16471)
 - **[node-manager]** Fix mig-manager reconfigure script waiting for disruptive approve behavior. [#16655](https://github.com/deckhouse/deckhouse/pull/16655)
 - **[node-manager]** Fix bashible-apiserver checksum update [#16621](https://github.com/deckhouse/deckhouse/pull/16621)
 - **[node-manager]** earlyoom ds fix after switching to ssa [#16562](https://github.com/deckhouse/deckhouse/pull/16562)
 - **[node-manager]** Fix staticMachine != StaticInstance race in caps. [#16315](https://github.com/deckhouse/deckhouse/pull/16315)
 - **[node-manager]** move bb-label-node-bashible-first-run-finished to bashible template [#16307](https://github.com/deckhouse/deckhouse/pull/16307)
 - **[prometheus]** Add ingressClassName to grafana/prometheus redirect ingress [#16116](https://github.com/deckhouse/deckhouse/pull/16116)
 - **[registrypackages]** Which was added to RPP. [#16563](https://github.com/deckhouse/deckhouse/pull/16563)
 - **[user-authn]** Fix login error 500 with password policy enabled. [#16703](https://github.com/deckhouse/deckhouse/pull/16703)

## Chore


 - **[candi]** Bump patch versions of Kubernetes images and cve fixes [#16455](https://github.com/deckhouse/deckhouse/pull/16455)
    Kubernetes control-plane components will restart, kubelet will restart
 - **[deckhouse]** Ignore absent chart file. [#15949](https://github.com/deckhouse/deckhouse/pull/15949)
 - **[dhctl]** Fix gossh client reconnections. [#16709](https://github.com/deckhouse/deckhouse/pull/16709)
 - **[dhctl]** Debug logs are disabled if bashible is launched via commander. 10 bashible global retry count and 5 for each step. [#15738](https://github.com/deckhouse/deckhouse/pull/15738)
 - **[ingress-nginx]** Improved documentation for the ModSecurity (WAF). [#16268](https://github.com/deckhouse/deckhouse/pull/16268)
 - **[loki]** Add alerts and graphs for Discarded Samples [#16137](https://github.com/deckhouse/deckhouse/pull/16137)
 - **[node-local-dns]** Stale-dns-connections-cleaner was removed as the issue was fixed in cni-cilium upstream [#16447](https://github.com/deckhouse/deckhouse/pull/16447)
 - **[node-manager]** Migration of node-inhibitor to k8s.io library. [#16237](https://github.com/deckhouse/deckhouse/pull/16237)

