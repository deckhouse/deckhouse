# Changelog v1.67

## Know before update


 - All modules with distroless image will be restarted.
 - Dashboard module works only via HTTPS and no longer supports the "Basic" HTTP authentication scheme.
 - Ingress-nginx pods will restart. The validation of the generated NGINX configuration during the validation of `Ingress` resources is disabled until finding a way of running validations safely.
 - Please note the following important points for metallb module:
    - Cluster readiness: before migration, you may need to ensure the cluster is in a specific state (refer to cluster alerts).
    - Backup configurations: it is highly recommended to backup custom resource configurations like L2Advertisement and IPAddressPool which were created manually bypassing the ModuleConfig before migration.
    - Additional resources: after migrating the L2 balancer, additional MetalLoadBalancerClass resources will be created in the cluster. These resources are used to configure the module instead of ModuleConfig.
 - Prometheus will be restarted.
 - The `releaseChannel`, `bundle` and `logLevel` parameters of the InitConfiguration resource are no longer available. Use the corresponding parameters in the ModuleConfig `deckhouse` instead.
 - v1alpha1 API version for ModulePullOverride is deprecated. A module source is now selected according to the moduleConfig.
 - v1alpha1 API version for ModuleUpdatePolicy is deprecated, the  source for the module and the module update policy is now selected according to the moduleConfig.

## Features


 - **[admission-policy-engine]** Update trivy-provider to support insecure/customCA registries. [#10749](https://github.com/deckhouse/deckhouse/pull/10749)
 - **[candi]** Added support for the new cloud provider — HuaweiCloud. [#10142](https://github.com/deckhouse/deckhouse/pull/10142)
 - **[candi]** Preparatory phase for bashible without bundles. [#9761](https://github.com/deckhouse/deckhouse/pull/9761)
 - **[cni-cilium]** Added eBPF-powered DHCP server for Pods. [#10651](https://github.com/deckhouse/deckhouse/pull/10651)
 - **[control-plane-manager]** Added two kube-scheduler profiles: high-node-utilization and no-scoring. [#10954](https://github.com/deckhouse/deckhouse/pull/10954)
 - **[deckhouse]** Update ModulePullOverride to work with the new module system. Deprecate v1alpha1 API version for ModulePullOverride. [#10903](https://github.com/deckhouse/deckhouse/pull/10903)
    v1alpha1 API version for ModulePullOverride is deprecated. A module source is now selected according to the moduleConfig.
 - **[deckhouse]** Add Deckhouse Kubernetes Platform Standard Edition Plus (SE+). [#10852](https://github.com/deckhouse/deckhouse/pull/10852)
 - **[deckhouse]** Fire an alert when a module config has an obsolete version. [#10796](https://github.com/deckhouse/deckhouse/pull/10796)
 - **[deckhouse]** Modules from sources are not installed by default anymore. All modules from sources are become visible by default. CRD `Module` spec observability improved. [#10336](https://github.com/deckhouse/deckhouse/pull/10336)
    v1alpha1 API version for ModuleUpdatePolicy is deprecated, the  source for the module and the module update policy is now selected according to the moduleConfig.
 - **[deckhouse-controller]** Installation of a module done without waiting `Manual` update approval or update windows. [#10684](https://github.com/deckhouse/deckhouse/pull/10684)
 - **[deckhouse-controller]** Improved deckhouse-controller logger. [#10479](https://github.com/deckhouse/deckhouse/pull/10479)
 - **[dhctl]** Add PostCloud step to verify cloud API access from master host during bootstrap. [#10311](https://github.com/deckhouse/deckhouse/pull/10311)
 - **[dhctl]** Preparatory phase for bashible without bundles. [#9761](https://github.com/deckhouse/deckhouse/pull/9761)
 - **[istio]** Bump Istio version to `1.21.6`. Bump Kiali version to `1.81`. [#9930](https://github.com/deckhouse/deckhouse/pull/9930)
    In environments where legacy versions of istio are used, the D8 update will be blocked, and the `D8IstioDeprecatedIstioVersionInstalled` alert will be fired.
 - **[metallb]** Implemented the module with advanced features. For more details, refer to the documentation. [#9658](https://github.com/deckhouse/deckhouse/pull/9658)
    Please note the following important points for metallb module:
    - Cluster readiness: before migration, you may need to ensure the cluster is in a specific state (refer to cluster alerts).
    - Backup configurations: it is highly recommended to backup custom resource configurations like L2Advertisement and IPAddressPool which were created manually bypassing the ModuleConfig before migration.
    - Additional resources: after migrating the L2 balancer, additional MetalLoadBalancerClass resources will be created in the cluster. These resources are used to configure the module instead of ModuleConfig.
 - **[node-manager]** Preparatory phase for bashible without bundles. [#9761](https://github.com/deckhouse/deckhouse/pull/9761)
 - **[operator-trivy]** Add option for configure custom CAs for docker registries. [#10768](https://github.com/deckhouse/deckhouse/pull/10768)
 - **[operator-trivy]** An option for disabling SBOM generation. [#10701](https://github.com/deckhouse/deckhouse/pull/10701)
    Once `disableSBOMGeneration` set to true, ALL current SBOM reports are deleted (one-time operation).
 - **[registrypackages]** Update containerd to v1.7.24. [#10966](https://github.com/deckhouse/deckhouse/pull/10966)
 - **[service-with-healthchecks]** A new module has been added that performs additional checks. Based on the results of these checks, traffic can be directed to different internal processes internally independently and only if they are ready. [#9371](https://github.com/deckhouse/deckhouse/pull/9371)
 - **[user-authn]** Add dexclient with the specific annotation to kubernetes client trustedPeers. [#11193](https://github.com/deckhouse/deckhouse/pull/11193)
 - **[user-authz]** New use subroles for DKP, and DVP aggregation. [#10936](https://github.com/deckhouse/deckhouse/pull/10936)
 - **[user-authz]** Replace use admin roles with use user roles for manage manager roles. [#10681](https://github.com/deckhouse/deckhouse/pull/10681)
    Reducing rights for manage manager roles.

## Fixes


 - **[candi]** Bump runc version to 1.2.3 and add patch to revert tun/tap to default device rules. [#11280](https://github.com/deckhouse/deckhouse/pull/11280)
 - **[candi]** Fix converge of Azure cluster without nameservers in config. [#10931](https://github.com/deckhouse/deckhouse/pull/10931)
 - **[candi]** Fixed double default-unreachable-toleration-seconds in kubeadm ClusterConfiguration. [#10828](https://github.com/deckhouse/deckhouse/pull/10828)
 - **[candi]** no_proxy/http_proxy tuning, `bb-set-proxy/bb-unset-proxy` functions. [#10676](https://github.com/deckhouse/deckhouse/pull/10676)
 - **[cert-manager]** Bump cert-manager version. [#10525](https://github.com/deckhouse/deckhouse/pull/10525)
 - **[cloud-provider-aws]** AWS network_border_group fix. [#11744](https://github.com/deckhouse/deckhouse/pull/11744)
 - **[cni-cilium]** Fixed tunnelMode setting for Static clusters. [#11233](https://github.com/deckhouse/deckhouse/pull/11233)
 - **[cni-cilium]** Fixed package dropping issues with VXLAN and VMWare-hosted nodes. [#10087](https://github.com/deckhouse/deckhouse/pull/10087)
 - **[cni-flannel]** Fixed a bug in the `check_cni_configuration` hook that could cause the main queue in the `deckhouse` to stop. [#11831](https://github.com/deckhouse/deckhouse/pull/11831)
 - **[cni-flannel]** Fixed package dropping issues with VXLAN and VMWare-hosted nodes. [#10087](https://github.com/deckhouse/deckhouse/pull/10087)
 - **[dashboard]** Specified the correct certificates for ingress to access via kube-rbac-proxy. [#11444](https://github.com/deckhouse/deckhouse/pull/11444)
 - **[deckhouse]** Fix checking automatic kubernetes version in managed clusters. [#12684](https://github.com/deckhouse/deckhouse/pull/12684)
 - **[deckhouse]** Fix module requirements parsing. [#11733](https://github.com/deckhouse/deckhouse/pull/11733)
 - **[deckhouse]** Fix deleting module config for embedded modules. [#11347](https://github.com/deckhouse/deckhouse/pull/11347)
 - **[deckhouse]** Fix several deployed module releases. [#11245](https://github.com/deckhouse/deckhouse/pull/11245)
 - **[deckhouse]** Fix source deletion error. [#10750](https://github.com/deckhouse/deckhouse/pull/10750)
 - **[deckhouse-controller]** Skip DeckhouseRelease requirement if it's not registered. [#11902](https://github.com/deckhouse/deckhouse/pull/11902)
 - **[deckhouse-controller]** Update golang.org/x/net to `0.33.0`. [#11316](https://github.com/deckhouse/deckhouse/pull/11316)
 - **[deckhouse-controller]** check spec object in validations [#11216](https://github.com/deckhouse/deckhouse/pull/11216)
 - **[deckhouse-controller]** Rollout restart for Deckhouse workloads (heritage=deckhouse) is forbidden. [#10844](https://github.com/deckhouse/deckhouse/pull/10844)
 - **[descheduler]** Fix label conversion. [#11303](https://github.com/deckhouse/deckhouse/pull/11303)
 - **[descheduler]** Fix conversation from v1alpha1 to v1alpha2 (add nodeAffinityType default for removePodsViolatingNodeAffinity strategy). [#11223](https://github.com/deckhouse/deckhouse/pull/11223)
 - **[descheduler]** Fix and update descheduler. [#10361](https://github.com/deckhouse/deckhouse/pull/10361)
    descheduler pod will be restarted.
 - **[dhctl]** Add validations settings for Huawei Cloud & Dynamix. [#11337](https://github.com/deckhouse/deckhouse/pull/11337)
 - **[dhctl]** Resource readiness fix. [#11018](https://github.com/deckhouse/deckhouse/pull/11018)
 - **[dhctl]** Fix the `master_ip_address_for_ssh` terraform output variable handling in converge. [#11039](https://github.com/deckhouse/deckhouse/pull/11039)
 - **[dhctl]** Fixed bootstrap on systems with umask `027`. [#10967](https://github.com/deckhouse/deckhouse/pull/10967)
 - **[dhctl]** Add tasks for moduleconfigs routines for post bootstrap and creating with resources phases. [#10688](https://github.com/deckhouse/deckhouse/pull/10688)
 - **[dhctl]** Fixed work with drain for nodes with kruise.io DaemonSet. [#10578](https://github.com/deckhouse/deckhouse/pull/10578)
 - **[dhctl]** Delete docker CRI type support. [#10114](https://github.com/deckhouse/deckhouse/pull/10114)
 - **[docs]** Adding info about Log Retention Settings Discrepancy in kube-bench test. [#11352](https://github.com/deckhouse/deckhouse/pull/11352)
    Users will be warned about inconsistencies in log storage settings when checking with kube-bench.
 - **[docs]** Added information about preflight skips flags. [#10916](https://github.com/deckhouse/deckhouse/pull/10916)
 - **[docs]** Fix priority-class module documentation. [#10897](https://github.com/deckhouse/deckhouse/pull/10897)
 - **[docs]** Add required NetworkInterface AWS policies. [#10842](https://github.com/deckhouse/deckhouse/pull/10842)
 - **[helm_lib]** Updated helm_lib to 1.37.1, which should fix issue [#10950](https://github.com/deckhouse/deckhouse/pull/10950)
 - **[ingress-nginx]** latest CVE fixes backported. [#12757](https://github.com/deckhouse/deckhouse/pull/12757)
    Ingress-nginx pods will restart. The validation of the generated NGINX configuration during the validation of `Ingress` resources is disabled until finding a way of running validations safely.
 - **[istio]** CRDs for 1.21 version moved to correct path [#11738](https://github.com/deckhouse/deckhouse/pull/11738)
 - **[istio]** Fixes for Federation/Multicluster metadata-exporter and for istio-operator. [#11150](https://github.com/deckhouse/deckhouse/pull/11150)
 - **[istio]** Fixed `IngressIstioController` CRD docs rendering. [#10581](https://github.com/deckhouse/deckhouse/pull/10581)
 - **[metallb]** Fixed race condition in migration hook. [#12176](https://github.com/deckhouse/deckhouse/pull/12176)
 - **[metallb]** Fix overlapping networks in IPAddressPools. [#11579](https://github.com/deckhouse/deckhouse/pull/11579)
 - **[metallb]** Fix metallb RBAC rules. [#11298](https://github.com/deckhouse/deckhouse/pull/11298)
 - **[metallb]** Fixed MetalLB migration issues and improved stability. [#11224](https://github.com/deckhouse/deckhouse/pull/11224)
 - **[multitenancy-manager]** Fix the 'secure' and 'secure_with_dedicated_nodes' project templates and add supporting default values. [#11096](https://github.com/deckhouse/deckhouse/pull/11096)
 - **[node-manager]** Tune CAPS sync period. [#11384](https://github.com/deckhouse/deckhouse/pull/11384)
 - **[node-manager]** Fix the key usage with cert-authority. [#10718](https://github.com/deckhouse/deckhouse/pull/10718)
 - **[node-manager]** no_proxy/http_proxy tuning, `bb-set-proxy/bb-unset-proxy` functions. [#10676](https://github.com/deckhouse/deckhouse/pull/10676)
 - **[prometheus]** Excluded unwanted labels from `D8CNIMisconfigured` alert. [#11301](https://github.com/deckhouse/deckhouse/pull/11301)
 - **[prometheus]** Fixes for D8CNIMisconfigured alerts — renamed group and typos fixed. [#11259](https://github.com/deckhouse/deckhouse/pull/11259)
 - **[prometheus]** Rename targets for alerts and docs fixes. [#11243](https://github.com/deckhouse/deckhouse/pull/11243)
 - **[runtime-audit-engine]** Fix k8s labels collection from containers in syscall event source. [#10639](https://github.com/deckhouse/deckhouse/pull/10639)
 - **[user-authn]** Fix error in dexClient internal openapi spec. [#11285](https://github.com/deckhouse/deckhouse/pull/11285)
 - **[user-authn]** Add profile scope to OIDC auth requests. [#11083](https://github.com/deckhouse/deckhouse/pull/11083)
 - **[user-authn]** Extend list annotations helm.sh/ to delete from secret. [#10918](https://github.com/deckhouse/deckhouse/pull/10918)

## Chore


 - **[candi]** Bump patch versions of Kubernetes images: `v1.29.12`, `v1.30.8`, `v1.31.4` [#11046](https://github.com/deckhouse/deckhouse/pull/11046)
    Kubernetes control-plane components will restart, kubelet will restart.
 - **[candi]** Update scratch image. [#10921](https://github.com/deckhouse/deckhouse/pull/10921)
    All modules with distroless image will be restarted.
 - **[candi]** Reduced usage of apt and yum. [#10867](https://github.com/deckhouse/deckhouse/pull/10867)
 - **[cloud-provider-aws]** Removed legacy "098_remove_cbr0.sh.tpl" step. [#10888](https://github.com/deckhouse/deckhouse/pull/10888)
 - **[cloud-provider-dynamix]** Adopt cloudProviderDynamix to new default StorageClass logic. [#10736](https://github.com/deckhouse/deckhouse/pull/10736)
 - **[cloud-provider-gcp]** Removed legacy "098_remove_cbr0.sh.tpl" step. [#10888](https://github.com/deckhouse/deckhouse/pull/10888)
 - **[cloud-provider-yandex]** Removed legacy "098_remove_cbr0.sh.tpl" step. [#10888](https://github.com/deckhouse/deckhouse/pull/10888)
 - **[cni-cilium]** Adding pre-migration compatibility checks for CNI configurations. [#10640](https://github.com/deckhouse/deckhouse/pull/10640)
    You will now receive alerts if not all parameters are clearly specified in the CNI `moduleConfigs`.
 - **[cni-flannel]** Adding pre-migration compatibility checks for CNI configurations. [#10640](https://github.com/deckhouse/deckhouse/pull/10640)
    You will now receive alerts if not all parameters are clearly specified in the CNI `moduleConfigs`.
 - **[cni-simple-bridge]** Adding pre-migration compatibility checks for CNI configurations. [#10640](https://github.com/deckhouse/deckhouse/pull/10640)
    You will now receive alerts if not all parameters are clearly specified in the CNI `moduleConfigs`.
 - **[control-plane-manager]** Update D8KubernetesVersionIsDeprecated alert to set k8s v1.27 deprecated. [#11687](https://github.com/deckhouse/deckhouse/pull/11687)
 - **[dashboard]** Updated to 7.10.0 [#10301](https://github.com/deckhouse/deckhouse/pull/10301)
    Dashboard module works only via HTTPS and no longer supports the "Basic" HTTP authentication scheme.
 - **[deckhouse-controller]** Update the drain mechanism in the copied code. [#10578](https://github.com/deckhouse/deckhouse/pull/10578)
 - **[deckhouse-controller]** Refactor release processing. [#10268](https://github.com/deckhouse/deckhouse/pull/10268)
 - **[dhctl]** Forbid to use logLevel bundle and releaseChannel from deckhouse init configuration. [#10882](https://github.com/deckhouse/deckhouse/pull/10882)
    The `releaseChannel`, `bundle` and `logLevel` parameters of the InitConfiguration resource are no longer available. Use the corresponding parameters in the ModuleConfig `deckhouse` instead.
 - **[docs]** Get rid of numeric prefixes in module URL. [#10561](https://github.com/deckhouse/deckhouse/pull/10561)
 - **[docs]** Add Deckhouse Virtualization Platform documentation. [#10223](https://github.com/deckhouse/deckhouse/pull/10223)
 - **[global-hooks]** Move `global.storageClass` to `global.modules.storageClass`. [#9859](https://github.com/deckhouse/deckhouse/pull/9859)
 - **[ingress-nginx]** Minor nginx dashboard improvements [#10800](https://github.com/deckhouse/deckhouse/pull/10800)
 - **[ingress-nginx]** Minor VHost dashboard improvements. [#10370](https://github.com/deckhouse/deckhouse/pull/10370)
 - **[istio]** Got rid of legacy RBACs created by istio-operator. [#10829](https://github.com/deckhouse/deckhouse/pull/10829)
 - **[istio]** Enabling the use of self hosted certificates for metadata decrypt and excluding certificate verification in Istio Multicluster and Federation. [#10740](https://github.com/deckhouse/deckhouse/pull/10740)
 - **[node-manager]** Rewrite NodeGroup convesion webhook on Python. [#10777](https://github.com/deckhouse/deckhouse/pull/10777)
 - **[operator-trivy]** Use local policies. [#10799](https://github.com/deckhouse/deckhouse/pull/10799)
 - **[prometheus]** Add init container and switcher for prom longterm. [#11658](https://github.com/deckhouse/deckhouse/pull/11658)
 - **[prometheus]** Add init container and switcher for prom image. [#11085](https://github.com/deckhouse/deckhouse/pull/11085)
    Prometheus will be restarted.
 - **[user-authz]** Replace manage capabilities and scopes. [#10810](https://github.com/deckhouse/deckhouse/pull/10810)

