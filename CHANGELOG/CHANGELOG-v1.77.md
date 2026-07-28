# Changelog v1.77

## [MALFORMED]


 - #18111 unknown section "upmeter"
 - #18145 unknown section "ingress-nginx"
 - #18262 unknown section "<kebab-case of a module name> | <1st level dir in the repo>"
 - #18461 unknown section "ingress-nginx"
 - #18642 unknown section "log-shipper"
 - #19310 unknown section "upmeter"
 - #19372 unknown section "extended-monitoring"
 - #19373 unknown section "loki"
 - #19374 unknown section "monitoring-kubernetes"
 - #19379 unknown section "prometheus-metrics-adapter"
 - #19380 unknown section "upmeter"
 - #19382 unknown section "upmeter"
 - #19399 unknown section "extended-monitoring"
 - #19473 unknown section "ingress-nginx"
 - #19509 unknown section "ingress-nginx"
 - #19512 unknown section "ingress-nginx"
 - #19543 unknown section "ingress-nginx"
 - #19554 unknown section "upmeter"
 - #19571 unknown section "upmeter"
 - #19619 unknown section "docs-builder"
 - #19710 unknown section "log-shipper"
 - #19723 unknown section "ingress-nginx"
 - #19749 unknown section "ingress-nginx"
 - #19799 unknown section "codeowners"
 - #19842 unknown section "extended-monitoring"
 - #19846 unknown section "ingress-nginx"
 - #19847 unknown section "prometheus-metrics-adapter"
 - #19849 unknown section "log-shipper"
 - #19859 unknown section "operator-prometheus"
 - #19864 unknown section "loki"
 - #19911 unknown section "ingress-nginx"
 - #19915 unknown section ""
 - #19948 unknown section "ingress-nginx"
 - #20036 unknown section "loki"
 - #20036 unknown section "operator-prometheus"
 - #20096 unknown section "<kebab-case of a module name> | <1st level dir in the repo>"
 - #20099 unknown section "ingress-nginx"
 - #20104 unknown section "ingress-nginx"
 - #20128 unknown section "ingress-nginx"
 - #20144 unknown section "<kebab-case of a module name> | <1st level dir in the repo>"
 - #20155 unknown section "ingress-nginx"
 - #20173 unknown section "ingress-nginx"
 - #20175 unknown section "extended-monitoring"
 - #20217 unknown section "upmeter"
 - #20248 unknown section "prometheus-metrics-adapter"
 - #20254 unknown section "operator-prometheus"
 - #20301 unknown section "<kebab-case of a module name> | <1st level dir in the repo>"
 - #20377 unknown section "<kebab-case of a module name> | <1st level dir in the repo>"
 - #20386 unknown section "registry-package-proxy"
 - #20395 unknown section "extended-monitoring"
 - #20395 unknown section "log-shipper"
 - #20395 unknown section "loki"
 - #20395 unknown section "monitoring-kubernetes"
 - #20395 unknown section "monitoring-ping"
 - #20395 unknown section "operator-prometheus"
 - #20395 unknown section "prometheus-metrics-adapter"
 - #20395 unknown section "prometheus-pushgateway"
 - #20395 unknown section "upmeter"
 - #20399 unknown section "<kebab-case of a module name> | <1st level dir in the repo>"
 - #20426 unknown section "ingress-nginx"
 - #20484 unknown section ""
 - #20494 unknown section ""
 - #20516 unknown section "<kebab-case of a module name> | <1st level dir in the repo>"
 - #20528 unknown section "ingress-nginx"
 - #20589 unknown section "<kebab-case of a module name> | <1st level dir in the repo>"
 - #20591 unknown section "prometheus-pushgateway"
 - #20600 unknown section "monitoring-kubernetes"
 - #20634 unknown section "prometheus-metrics-adapter"
 - #20642 unknown section "monitoring-ping"
 - #20667 unknown section "monitoring-custom"
 - #20670 unknown section "extended-monitoring"
 - #20780 unknown section ""
 - #20869 unknown section "upmeter"
 - #20929 unknown section "<kebab-case of a module name> | <1st level dir in the repo>"
 - #21006 unknown section "monitoring-kubernetes"
 - #21008 unknown section "<kebab-case of a module name> | <1st level dir in the repo>"
 - #21059 unknown section "<kebab-case of a module name> | <1st level dir in the repo>"
 - #21236 unknown section "<kebab-case of a module name> | <1st level dir in the repo>"
 - #21261 unknown section ""
 - #21464 unknown section "monitoring-applications"
 - #21601 unknown section ""

## Know before update


 - After upgrading to v1.76.0, kubectl logs and exec fail cluster-wide for all users. Manual workaround is available — see PR description.
 - BGP configurations are no longer configured via the module's `ModuleConfig`. The old BGP configurations will be automatically migrated to the new Custom Resources upon platform update.
    **Important:** Because the migration hook auto-generates the new resources from your existing legacy `ModuleConfig`, after the platform update you must switch the `version` field in the `metallb` `ModuleConfig` (to disable migrations) *before* manually editing the newly generated BGP CRDs. If you fail to do so, the migration hook will keep overwriting your manual edits.
    After a successful update, you must also manually delete the obsolete BGP settings (`bgpPeers`, `addressPools`, `bgpCommunities`) from the `metallb` `ModuleConfig`. Please refer to the module's documentation for detailed instructions.
 - Custom edits to the local-path-config ConfigMap that set unsafe HelperPod fields (privileged, capabilities, host namespaces, initContainers, custom volumes/volumeMounts, container probes/lifecycle, sysctls, etc.) will be rejected by the provisioner at startup. Default Deckhouse installations are unaffected.
 - Daemonset `early-oom` in namespace `d8-cloud-instance-manager` will be removed.
 - Deckhouse cluster have to be migrated from ProviderClusterConfiguration to ModuleConfig settings
 - If the cluster has custom roles of the legacy experimental RBACv2 scheme (`custom:*` names with the
    `rbac.deckhouse.io/kind: manage` or `use` labels or aggregation selectors), the
    `D8UserAuthzLegacyRBACv2CustomRoleFound` alert fires. Such roles will stop aggregating permissions
    in DKP 1.78 (the aggregation label scheme changes and custom roles get no compatibility aliases),
    and the upgrade to DKP 1.78 will be blocked until they are migrated to the new `d8:custom:*` scheme.
    See the user-authz module FAQ, section "How do I migrate custom roles to the new scheme in DKP 1.78?".
 - Previously, a transient cluster DNS failure could cause the user-authz-webhook liveness probe to fail and restart the pod, which combined with the fail-closed authorization webhook (failurePolicy: Deny) could deny all API requests, including cluster-admins, until DNS recovered.
 - Previously, the fencing-agent would crash with "permission denied" on /dev/watchdog
    when the node had a maintenance annotation (e.g. during Deckhouse updates).
    Now the agent skips watchdog arming during maintenance and arms it automatically
    when maintenance ends.
 - Refactor to controller-runtime and replace kubeadm with internal go_lib, all control-plane components will be restarted
 - The `local-path-provisioner` Pod is restarted during the update. PV provisioning/teardown briefly pauses while the new Pod becomes Ready; existing volumes are not affected. After the update the provisioner refuses to create a HelperPod whose template (loaded from the `local-path-config` ConfigMap) declares privileged containers, hostPath/custom volumes, host namespaces, added Linux capabilities or other security-sensitive fields, so any pre-existing manual override of `helperPod.yaml` that uses one of these fields must be removed before the upgrade.
 - The cni-cilium components (cilium agent, operator) will restart after the update. 
    A change has been made to the Network Policy that affects the behavior of the ipBlock: 0.0.0.0/0 rule.
 - The coredns and kube-dns components will restart after the update.
 - The egress-gateway-agent component will restart after the update.
 - The metallb components (controller, speaker, l2lb) will restart after the update.
 - The minimum supported version of Kubernetes is now 1.32. All control plane components will restart.
 - The node-local-dns components will restart after the update.
 - The service-with-healthchecks components (controller, agent) will restart after the update.
 - The status logic was heavily refactored to reduce API and etcd load. If you rely on `lastProbeTime` observability on every probe, you must explicitly enable `verboseStatus` in the module configuration.
 - Vertical Pod Autoscaler components will restart.
 - When using containerdV2, the performance of istio-cni breaks when mounting internal paths.
 - With enableMultiTenancy, effective access is now the union of CAR (within its limitNamespaces/namespaceSelector), AuthorizationRules, and plain RoleBindings/ClusterRoleBindings. Previously the webhook denied requests outside the CAR scope even when RBAC explicitly granted them: such existing bindings for subjects with a CAR silently become effective after the upgrade — review them. The CAR access level still does not apply outside its namespace limits. AccessibleNamespaces reflects the same union.

## Features


 - **[admission-policy-engine]** Add complex nodeAffinity selector for gatekeeper controller-manager [#20656](https://github.com/deckhouse/deckhouse/pull/20656)
 - **[admission-policy-engine]** Added a ValidatingAdmissionPolicy that forbids removing Deckhouse finalizers containing `deckhouse.io`. [#21217](https://github.com/deckhouse/deckhouse/pull/21217)
 - **[admission-policy-engine]** Global refactor of constraints and tests, add support for container-level SecurityPolicyException [#18668](https://github.com/deckhouse/deckhouse/pull/18668)
 - **[admission-policy-engine]** Grant kubeadm:cluster-admins group granular full access to module CRDs via dedicated ClusterRole d8:admission-policy-engine:admin-kubeconfig. [#19420](https://github.com/deckhouse/deckhouse/pull/19420)
 - **[candi]** Add oss.yaml files for cloud provider modules [#18989](https://github.com/deckhouse/deckhouse/pull/18989)
 - **[candi]** Added support for Kubernetes 1.36 and discontinued support for Kubernetes 1.31. Default Kubernetes version was changed 1.33->1.34. [#19623](https://github.com/deckhouse/deckhouse/pull/19623)
    The minimum supported version of Kubernetes is now 1.32. All control plane components will restart.
 - **[candi]** X-kubernetes-sensitive-data masking sensetive fields [#19634](https://github.com/deckhouse/deckhouse/pull/19634)
 - **[candi]** migrate to unified builder with package manager and Go toolchain [#19882](https://github.com/deckhouse/deckhouse/pull/19882)
 - **[cert-manager]** Auto-enable cainjector when cluster resources require cert-manager CA injection annotations. [#19850](https://github.com/deckhouse/deckhouse/pull/19850)
 - **[cilium-hubble]** Added authorization to UI [#18895](https://github.com/deckhouse/deckhouse/pull/18895)
 - **[cilium-hubble]** Fixed securityContext and added SecurityPolicyException [#21545](https://github.com/deckhouse/deckhouse/pull/21545)
 - **[cloud-provider-aws]** Add oss.yaml files for cloud provider modules [#18989](https://github.com/deckhouse/deckhouse/pull/18989)
 - **[cloud-provider-aws]** Migrate AWS to Cilium CNI [#19774](https://github.com/deckhouse/deckhouse/pull/19774)
 - **[cloud-provider-aws]** Migrated cloud-provider RBAC v1 `user-authz` ClusterRoles to a shared `helm-lib` template. [#19964](https://github.com/deckhouse/deckhouse/pull/19964)
 - **[cloud-provider-aws]** migrate to unified builder with package manager and Go toolchain [#19616](https://github.com/deckhouse/deckhouse/pull/19616)
 - **[cloud-provider-azure]** Add oss.yaml files for cloud provider modules [#18989](https://github.com/deckhouse/deckhouse/pull/18989)
 - **[cloud-provider-azure]** Migrate remaining cloud-provider Go artifact builds to the unified `builder/golang` image. [#20084](https://github.com/deckhouse/deckhouse/pull/20084)
 - **[cloud-provider-azure]** Migrate to Cilium CNI by default for new clusters [#19807](https://github.com/deckhouse/deckhouse/pull/19807)
 - **[cloud-provider-azure]** Migrated cloud-provider RBAC v1 `user-authz` ClusterRoles to a shared `helm-lib` template. [#19964](https://github.com/deckhouse/deckhouse/pull/19964)
 - **[cloud-provider-azure]** migrate to shared lib-helm templates for CCM and CDD deployments [#19264](https://github.com/deckhouse/deckhouse/pull/19264)
 - **[cloud-provider-dvp]** Add an option to disable the CCM LoadBalancer functionality via ConfigMap. [#21296](https://github.com/deckhouse/deckhouse/pull/21296)
 - **[cloud-provider-dvp]** Add oss.yaml files for cloud provider modules [#18989](https://github.com/deckhouse/deckhouse/pull/18989)
 - **[cloud-provider-dvp]** Migrate module to configuration from ProviderClusterConfiguration to ModuleConfig settings [#20236](https://github.com/deckhouse/deckhouse/pull/20236)
    Deckhouse cluster have to be migrated from ProviderClusterConfiguration to ModuleConfig settings
 - **[cloud-provider-dvp]** Migrate remaining cloud-provider Go artifact builds to the unified `builder/golang` image. [#20084](https://github.com/deckhouse/deckhouse/pull/20084)
 - **[cloud-provider-dvp]** Migrated cloud-provider RBAC v1 `user-authz` ClusterRoles to a shared `helm-lib` template. [#19964](https://github.com/deckhouse/deckhouse/pull/19964)
 - **[cloud-provider-dvp]** cleanup of LoadBalancer and ServiceWithHealthchecks resources during DVP cluster deletion [#20078](https://github.com/deckhouse/deckhouse/pull/20078)
 - **[cloud-provider-dvp]** enable security policy check and add SecurityPolicyException for DVP CSI and CCM [#18873](https://github.com/deckhouse/deckhouse/pull/18873)
 - **[cloud-provider-dvp]** live StorageClass migration for VM disks is now supported via DeckhouseMachine spec changes [#19919](https://github.com/deckhouse/deckhouse/pull/19919)
 - **[cloud-provider-dynamix]** Add oss.yaml files for cloud provider modules [#18989](https://github.com/deckhouse/deckhouse/pull/18989)
 - **[cloud-provider-dynamix]** Migrate Dynamix, GCP and OpenStack templates to define from helm-lib [#19267](https://github.com/deckhouse/deckhouse/pull/19267)
 - **[cloud-provider-dynamix]** Migrate remaining cloud-provider Go artifact builds to the unified `builder/golang` image. [#20084](https://github.com/deckhouse/deckhouse/pull/20084)
 - **[cloud-provider-dynamix]** Migrated cloud-provider RBAC v1 `user-authz` ClusterRoles to a shared `helm-lib` template. [#19964](https://github.com/deckhouse/deckhouse/pull/19964)
 - **[cloud-provider-gcp]** Add oss.yaml files for cloud provider modules [#18989](https://github.com/deckhouse/deckhouse/pull/18989)
 - **[cloud-provider-gcp]** Migrate Dynamix, GCP and OpenStack templates to define from helm-lib [#19267](https://github.com/deckhouse/deckhouse/pull/19267)
 - **[cloud-provider-gcp]** Migrate remaining cloud-provider Go artifact builds to the unified `builder/golang` image. [#20084](https://github.com/deckhouse/deckhouse/pull/20084)
 - **[cloud-provider-gcp]** Migrate to Cilium CNI by default for new clusters [#19807](https://github.com/deckhouse/deckhouse/pull/19807)
 - **[cloud-provider-gcp]** Migrated cloud-provider RBAC v1 `user-authz` ClusterRoles to a shared `helm-lib` template. [#19964](https://github.com/deckhouse/deckhouse/pull/19964)
 - **[cloud-provider-huaweicloud]** Add oss.yaml files for cloud provider modules [#18989](https://github.com/deckhouse/deckhouse/pull/18989)
 - **[cloud-provider-huaweicloud]** Migrate remaining cloud-provider Go artifact builds to the unified `builder/golang` image. [#20084](https://github.com/deckhouse/deckhouse/pull/20084)
 - **[cloud-provider-huaweicloud]** Migrated cloud-provider RBAC v1 `user-authz` ClusterRoles to a shared `helm-lib` template. [#19964](https://github.com/deckhouse/deckhouse/pull/19964)
 - **[cloud-provider-huaweicloud]** bump CAPI provider to cluster-api v1.12, fix Machine initialization contract [#20723](https://github.com/deckhouse/deckhouse/pull/20723)
 - **[cloud-provider-huaweicloud]** huaweicloud switched to OpenTofu mode with terraform-state migration integration in terraform-manager [#19647](https://github.com/deckhouse/deckhouse/pull/19647)
 - **[cloud-provider-openstack]** Add oss.yaml files for cloud provider modules [#18989](https://github.com/deckhouse/deckhouse/pull/18989)
 - **[cloud-provider-openstack]** Migrate Dynamix, GCP and OpenStack templates to define from helm-lib [#19267](https://github.com/deckhouse/deckhouse/pull/19267)
 - **[cloud-provider-openstack]** Migrate remaining cloud-provider Go artifact builds to the unified `builder/golang` image. [#20084](https://github.com/deckhouse/deckhouse/pull/20084)
 - **[cloud-provider-openstack]** Migrated cloud-provider RBAC v1 `user-authz` ClusterRoles to a shared `helm-lib` template. [#19964](https://github.com/deckhouse/deckhouse/pull/19964)
 - **[cloud-provider-openstack]** add capi support for openstack provider [#20515](https://github.com/deckhouse/deckhouse/pull/20515)
 - **[cloud-provider-openstack]** openStack switched to OpenTofu mode with terraform-state migration integration in terraform-manager [#19381](https://github.com/deckhouse/deckhouse/pull/19381)
 - **[cloud-provider-vcd]** Add oss.yaml files for cloud provider modules [#18989](https://github.com/deckhouse/deckhouse/pull/18989)
 - **[cloud-provider-vcd]** Added support for setting a predefined VCD LoadBalancer IP via Service annotation. [#19953](https://github.com/deckhouse/deckhouse/pull/19953)
 - **[cloud-provider-vcd]** Migrate Dynamix, GCP and OpenStack templates to define from helm-lib [#19267](https://github.com/deckhouse/deckhouse/pull/19267)
 - **[cloud-provider-vcd]** Migrate from Terraform to OpenTofu [#19594](https://github.com/deckhouse/deckhouse/pull/19594)
 - **[cloud-provider-vcd]** Migrate remaining cloud-provider Go artifact builds to the unified `builder/golang` image. [#20084](https://github.com/deckhouse/deckhouse/pull/20084)
 - **[cloud-provider-vcd]** Migrated cloud-provider RBAC v1 `user-authz` ClusterRoles to a shared `helm-lib` template. [#19964](https://github.com/deckhouse/deckhouse/pull/19964)
 - **[cloud-provider-vcd]** migrate VCD CAPI provider to cluster-api v1.12 with v1beta2 contract [#18518](https://github.com/deckhouse/deckhouse/pull/18518)
 - **[cloud-provider-vsphere]** Add oss.yaml files for cloud provider modules [#18989](https://github.com/deckhouse/deckhouse/pull/18989)
 - **[cloud-provider-vsphere]** Migrate from Terraform to OpenTofu [#19645](https://github.com/deckhouse/deckhouse/pull/19645)
 - **[cloud-provider-vsphere]** Migrate remaining cloud-provider Go artifact builds to the unified `builder/golang` image. [#20084](https://github.com/deckhouse/deckhouse/pull/20084)
 - **[cloud-provider-vsphere]** Migrated cloud-provider RBAC v1 `user-authz` ClusterRoles to a shared `helm-lib` template. [#19964](https://github.com/deckhouse/deckhouse/pull/19964)
 - **[cloud-provider-vsphere]** enable security policy check and add SecurityPolicyException rendering for vSphere CCM and CSI components [#18905](https://github.com/deckhouse/deckhouse/pull/18905)
 - **[cloud-provider-yandex]** Add oss.yaml files for cloud provider modules [#18989](https://github.com/deckhouse/deckhouse/pull/18989)
 - **[cloud-provider-yandex]** Migrate remaining cloud-provider Go artifact builds to the unified `builder/golang` image. [#20084](https://github.com/deckhouse/deckhouse/pull/20084)
 - **[cloud-provider-yandex]** Migrated cloud-provider RBAC v1 `user-authz` ClusterRoles to a shared `helm-lib` template. [#19964](https://github.com/deckhouse/deckhouse/pull/19964)
 - **[cloud-provider-yandex]** add capi support for yandex provider [#19432](https://github.com/deckhouse/deckhouse/pull/19432)
 - **[cloud-provider-yandex]** enable security policy check and add SecurityPolicyException rendering for Yandex CCM and CSI components [#18899](https://github.com/deckhouse/deckhouse/pull/18899)
 - **[cloud-provider-zvirt]** Add oss.yaml files for cloud provider modules [#18989](https://github.com/deckhouse/deckhouse/pull/18989)
 - **[cloud-provider-zvirt]** Migrate remaining cloud-provider Go artifact builds to the unified `builder/golang` image. [#20084](https://github.com/deckhouse/deckhouse/pull/20084)
 - **[cloud-provider-zvirt]** Migrated cloud-provider RBAC v1 `user-authz` ClusterRoles to a shared `helm-lib` template. [#19964](https://github.com/deckhouse/deckhouse/pull/19964)
 - **[cloud-provider-zvirt]** enable security policy check and add SecurityPolicyException rendering for zVirt CCM and CSI components [#18902](https://github.com/deckhouse/deckhouse/pull/18902)
 - **[cni-cilium]** Added RBACv1 roles (User and ClusterEditor) for HubbleMonitoringConfig resource. [#19629](https://github.com/deckhouse/deckhouse/pull/19629)
 - **[cni-cilium]** Disable bpf trace events by default [#19997](https://github.com/deckhouse/deckhouse/pull/19997)
 - **[cni-cilium]** Fixed securityContext and added SecurityPolicyException [#21545](https://github.com/deckhouse/deckhouse/pull/21545)
 - **[cni-cilium]** Reduced the CPU load in cilium-agent with hubble enabled. [#19669](https://github.com/deckhouse/deckhouse/pull/19669)
 - **[common]** Add oss.yaml files for cloud provider modules [#18989](https://github.com/deckhouse/deckhouse/pull/18989)
 - **[common]** Add real-time `d8 k get ns -w` [#19676](https://github.com/deckhouse/deckhouse/pull/19676)
 - **[common]** Add token_namespace and token_name labels to serviceaccount_stale_tokens_total kube-apiserver metric [#19506](https://github.com/deckhouse/deckhouse/pull/19506)
 - **[common]** Added a Kubernetes scheduler patch that prevents scheduling pods onto nodes during graceful shutdown [#20264](https://github.com/deckhouse/deckhouse/pull/20264)
 - **[common]** `d8 k get <resource> -A --scope=<accessible|projects|system|project:NAME>` returns an ACL-filtered listing instead of 403 for users without cluster-wide RBAC; opt-in via request headers, vanilla behavior unchanged without them. [#21206](https://github.com/deckhouse/deckhouse/pull/21206)
    kube-apiserver image is rebuilt; control-plane components restart during the update.
 - **[control-plane-manager]** Added periodic etcd defragmentation via a new DefragEtcd CPO step; configurable schedule and enable/disable flag via ModuleConfig [#20575](https://github.com/deckhouse/deckhouse/pull/20575)
 - **[control-plane-manager]** Control-plane resource requests can now be configured in ModuleConfig control-plane-manager. [#20127](https://github.com/deckhouse/deckhouse/pull/20127)
 - **[control-plane-manager]** Extend d8:control-plane-manager:admin-kubeconfig-supplement with granular permissions for standard Kubernetes resources not covered by user-authz:cluster-admin. [#19420](https://github.com/deckhouse/deckhouse/pull/19420)
 - **[control-plane-manager]** Moved the encryptionAlgorithm parameter to the control-plane-manager's module config [#20497](https://github.com/deckhouse/deckhouse/pull/20497)
 - **[control-plane-manager]** Publish API ingress settings are moved to control-plane-manager module, ingress resource is moved to kube-system namespace. [#18628](https://github.com/deckhouse/deckhouse/pull/18628)
 - **[control-plane-manager]** Refactor to controller-runtime and replace kubeadm with internal go_lib, all control-plane components will be restarted [#17798](https://github.com/deckhouse/deckhouse/pull/17798)
    Refactor to controller-runtime and replace kubeadm with internal go_lib, all control-plane components will be restarted
 - **[deckhouse-controller]** Added mechanism for blocking deckhouse release if cluster have alerts with high severity. [#20616](https://github.com/deckhouse/deckhouse/pull/20616)
 - **[deckhouse-controller]** add resource grant resolving [#20841](https://github.com/deckhouse/deckhouse/pull/20841)
 - **[deckhouse-controller]** harden migrated modules check [#20949](https://github.com/deckhouse/deckhouse/pull/20949)
 - **[deckhouse]** Add conditions summary to applications. [#20273](https://github.com/deckhouse/deckhouse/pull/20273)
 - **[deckhouse]** Add localized disable messages to modules and packages. [#20791](https://github.com/deckhouse/deckhouse/pull/20791)
 - **[deckhouse]** Add newly-found versions counter to PackageRepositoryOperation. [#19929](https://github.com/deckhouse/deckhouse/pull/19929)
 - **[deckhouse]** Add package health monitor. [#19711](https://github.com/deckhouse/deckhouse/pull/19711)
 - **[deckhouse]** Add publicDomainTemplate for application platform. [#21052](https://github.com/deckhouse/deckhouse/pull/21052)
 - **[deckhouse]** Allow specific experimental modules via allowedExperimentalModules. [#20582](https://github.com/deckhouse/deckhouse/pull/20582)
 - **[deckhouse]** Application status now exposes endpoints (`status.urls` with `url` and `description`) collected from chart Ingresses annotated with `packages.deckhouse.io/application-endpoint`. [#21104](https://github.com/deckhouse/deckhouse/pull/21104)
 - **[deckhouse]** Check registry pagination to enable incremental scan. [#18898](https://github.com/deckhouse/deckhouse/pull/18898)
 - **[deckhouse]** Export module versions to DOP via mm_module_enabled. [#19290](https://github.com/deckhouse/deckhouse/pull/19290)
 - **[deckhouse]** It is no longer possible to change the module set (settings.bundle) in a deployed cluster [#20557](https://github.com/deckhouse/deckhouse/pull/20557)
 - **[deckhouse]** Make packages nelm operation timeout configurable [#21621](https://github.com/deckhouse/deckhouse/pull/21621)
 - **[deckhouse]** Packages cleanup orphan nelm releases. [#19773](https://github.com/deckhouse/deckhouse/pull/19773)
 - **[deckhouse]** Reserve the d8a- prefix for Deckhouse application objects [#21431](https://github.com/deckhouse/deckhouse/pull/21431)
 - **[deckhouse]** Rework D8DeckhouseQueueIsHung alert with queue head info and severity based on module criticality. [#20476](https://github.com/deckhouse/deckhouse/pull/20476)
 - **[deckhouse]** Surface scan results on PackageRepository status. [#20279](https://github.com/deckhouse/deckhouse/pull/20279)
 - **[deckhouse]** The PackageRepositoryOperation type field now uses an enum. [#19951](https://github.com/deckhouse/deckhouse/pull/19951)
 - **[deckhouse]** Webhook-handler will reload exited shell-operator now. [#19592](https://github.com/deckhouse/deckhouse/pull/19592)
 - **[deckhouse]** add beforeDeleteHelm hook binding for packages [#21344](https://github.com/deckhouse/deckhouse/pull/21344)
 - **[deckhouse]** add maintenance mode for applications [#21173](https://github.com/deckhouse/deckhouse/pull/21173)
 - **[deckhouse]** add the x-deckhouse-ui-order extension to application package settings schemas [#21465](https://github.com/deckhouse/deckhouse/pull/21465)
 - **[deckhouse]** add the x-deckhouse-ui-validation-description extension to application package settings schemas [#21495](https://github.com/deckhouse/deckhouse/pull/21495)
 - **[descheduler]** Add DRS-Lens Grafana dashboard showing workload happiness score [#21143](https://github.com/deckhouse/deckhouse/pull/21143)
 - **[dhctl]** Add deprecation warnings to dhctl about all all deprecated fields in resources. [#21273](https://github.com/deckhouse/deckhouse/pull/21273)
 - **[dhctl]** Add global flag dhctl, improve interactive logging. [#19879](https://github.com/deckhouse/deckhouse/pull/19879)
 - **[dhctl]** Add log area to interactive mode of dhctl. [#20351](https://github.com/deckhouse/deckhouse/pull/20351)
 - **[dhctl]** Added bootstrap support with `Local` registry mode [#18262](https://github.com/deckhouse/deckhouse/pull/18262)
 - **[dhctl]** Added support in `dhctl` for working with a dependency image and invoking a validator shipped with the cloud provider module.. [#20236](https://github.com/deckhouse/deckhouse/pull/20236)
    Deckhouse cluster have to be migrated from ProviderClusterConfiguration to ModuleConfig settings
 - **[dhctl]** Generate cni-* ModuleConfig from each cloud provider's candi/cni-bootstrap.yml at bootstrap, with user-override confirmation and gRPC inspection (Validation.ConfigExtender). [#20143](https://github.com/deckhouse/deckhouse/pull/20143)
 - **[dhctl]** Logging refactoring for dhctl. [#19422](https://github.com/deckhouse/deckhouse/pull/19422)
 - **[dhctl]** Switch default for ssh in dhtcl to gossh [#21256](https://github.com/deckhouse/deckhouse/pull/21256)
 - **[dhctl]** Use bundle-specific system requirements during preflight validation. [#21408](https://github.com/deckhouse/deckhouse/pull/21408)
 - **[dhctl]** add basic OpenTelemetry support to dhctl [#19738](https://github.com/deckhouse/deckhouse/pull/19738)
 - **[dhctl]** add preflight check for cloud disk name length. [#20029](https://github.com/deckhouse/deckhouse/pull/20029)
 - **[dhctl]** improve preflight check for deckhouse user. [#19431](https://github.com/deckhouse/deckhouse/pull/19431)
 - **[docs]** Add new security events documentation page [#19706](https://github.com/deckhouse/deckhouse/pull/19706)
 - **[docs]** Add oss.yaml files for cloud provider modules [#18989](https://github.com/deckhouse/deckhouse/pull/18989)
 - **[istio]** Add WaypointInstance CRD and waypoint-controller for Istio waypoint proxy management. [#20188](https://github.com/deckhouse/deckhouse/pull/20188)
 - **[istio]** Add ambient waypoint support with RBAC fixes for Gateway API resources. [#19841](https://github.com/deckhouse/deckhouse/pull/19841)
 - **[istio]** Add deploy dependencies for Istio webhook configurations [#20992](https://github.com/deckhouse/deckhouse/pull/20992)
 - **[istio]** Add explicit configuration option `ambient.enabled` to manage Istio ambient mesh. [#19318](https://github.com/deckhouse/deckhouse/pull/19318)
 - **[istio]** Added new component alliance-healthcheck to monitor inter-cluster connectivity [#19006](https://github.com/deckhouse/deckhouse/pull/19006)
 - **[istio]** Change API-proxy VPA update mode to fix an interruption in work [#19602](https://github.com/deckhouse/deckhouse/pull/19602)
 - **[istio]** Changed preferred istio default subdomain for metadata-exporter [#19299](https://github.com/deckhouse/deckhouse/pull/19299)
    matadata-exporter pod will be restarted
 - **[istio]** Custom annotations for Ingress Controller [#20486](https://github.com/deckhouse/deckhouse/pull/20486)
 - **[istio]** Custom extension providers [#20685](https://github.com/deckhouse/deckhouse/pull/20685)
 - **[istio]** Enable HBONE support on sidecar proxies when ambient mode is enabled [#19715](https://github.com/deckhouse/deckhouse/pull/19715)
 - **[istio]** Implement graceful metadata secret renewal for multiclusters. [#19278](https://github.com/deckhouse/deckhouse/pull/19278)
 - **[istio]** Implementation of the Telemetry API [#19636](https://github.com/deckhouse/deckhouse/pull/19636)
 - **[istio]** JWKS extra root CA implementation [#20567](https://github.com/deckhouse/deckhouse/pull/20567)
 - **[istio]** Modernized IngressIstioController gateway manifests and added client network topology configuration. [#21304](https://github.com/deckhouse/deckhouse/pull/21304)
    Existing IngressIstioController gateway DaemonSets are rolled out after upgrade. Default static resource requests change from 350m CPU/500Mi memory to 100m CPU/128Mi memory; default VPA bounds change to 100m–1000m CPU and 128Mi–2000Mi memory. Set explicit resourcesRequests values before upgrading if the previous sizing must be retained.
 - **[istio]** Threat model for CE and EE [#20435](https://github.com/deckhouse/deckhouse/pull/20435)
 - **[istio]** Updated the Istio module documentation for automatic data plane upgrades. [#20817](https://github.com/deckhouse/deckhouse/pull/20817)
 - **[istio]** added istio v1.27↓ [#20757](https://github.com/deckhouse/deckhouse/pull/20757)
 - **[metallb]** Added RBACv1 roles (User and ClusterAdmin) for MetalLoadBalancerClass resource. [#19627](https://github.com/deckhouse/deckhouse/pull/19627)
 - **[metallb]** Refactored BGP configuration to use dedicated CRDs (`MetalLoadBalancerPool`, `MetalLoadBalancerBGPPeer`, `MetalLoadBalancerConfiguration`) instead of ModuleConfig. [#18660](https://github.com/deckhouse/deckhouse/pull/18660)
    BGP configurations are no longer configured via the module's `ModuleConfig`. The old BGP configurations will be automatically migrated to the new Custom Resources upon platform update.
    **Important:** Because the migration hook auto-generates the new resources from your existing legacy `ModuleConfig`, after the platform update you must switch the `version` field in the `metallb` `ModuleConfig` (to disable migrations) *before* manually editing the newly generated BGP CRDs. If you fail to do so, the migration hook will keep overwriting your manual edits.
    After a successful update, you must also manually delete the obsolete BGP settings (`bgpPeers`, `addressPools`, `bgpCommunities`) from the `metallb` `ModuleConfig`. Please refer to the module's documentation for detailed instructions.
 - **[multitenancy-manager]** Cluster resource grants — control which cluster-scoped resources a project may use, set per-project defaults, and cap usage with object quotas. [#20520](https://github.com/deckhouse/deckhouse/pull/20520)
 - **[multitenancy-manager]** Cluster resource grants — split governed-resource definitions from validation/defaulting reference paths so any module can register a path; object quota delegated to Kubernetes ResourceQuota. [#20611](https://github.com/deckhouse/deckhouse/pull/20611)
 - **[multitenancy-manager]** Grant kubeadm:cluster-admins group granular full access to module CRDs via dedicated ClusterRole d8:multitenancy-manager:admin-kubeconfig. [#19420](https://github.com/deckhouse/deckhouse/pull/19420)
 - **[node-local-dns]** Implemented a new `nodeLocalDns` option to disable IPv6 DNS resolving. [#19282](https://github.com/deckhouse/deckhouse/pull/19282)
 - **[node-manager]** Added Instance API v1alpha2 with unified machine and bashible status model [#18795](https://github.com/deckhouse/deckhouse/pull/18795)
 - **[node-manager]** Removed `early-oom` component from the `node-manager` module. [#20806](https://github.com/deckhouse/deckhouse/pull/20806)
    Daemonset `early-oom` in namespace `d8-cloud-instance-manager` will be removed.
 - **[node-manager]** e2e test for cluster-autoscaler [#18817](https://github.com/deckhouse/deckhouse/pull/18817)
 - **[node-manager]** move capi v1beta2 management into node-controller. [#20682](https://github.com/deckhouse/deckhouse/pull/20682)
 - **[registry-packages-proxy]** Make CLI images pull in registry-packages-proxy platform-aware [#20795](https://github.com/deckhouse/deckhouse/pull/20795)
 - **[registry-packages-proxy]** Serve plugin contract from manifest in registry-packages-proxy [#20800](https://github.com/deckhouse/deckhouse/pull/20800)
 - **[registry-packages-proxy]** add handling deckhouse-cli artefacts to registry-package-proxy module [#20025](https://github.com/deckhouse/deckhouse/pull/20025)
 - **[registry]** Added bootstrap support with `Local` registry mode [#18262](https://github.com/deckhouse/deckhouse/pull/18262)
 - **[registrypackages]** Add oss.yaml files for cloud provider modules [#18989](https://github.com/deckhouse/deckhouse/pull/18989)
 - **[registrypackages]** Add patches for containerd 2.2.3 with integrity logic [#19076](https://github.com/deckhouse/deckhouse/pull/19076)
 - **[registrypackages]** Add patches for containerd 2.2.4 with integrity logic [#20298](https://github.com/deckhouse/deckhouse/pull/20298)
 - **[registrypackages]** Fix containerd patches for new integrity feature with user's ca [#20845](https://github.com/deckhouse/deckhouse/pull/20845)
 - **[service-with-healthchecks]** Added RBACv1 roles (User and Editor) for ServiceWithHealthchecks resource. [#19625](https://github.com/deckhouse/deckhouse/pull/19625)
 - **[user-authn]** Add DexProviderCheck resource for on-demand connectivity and credential diagnostics of Dex authentication providers. [#20319](https://github.com/deckhouse/deckhouse/pull/20319)
 - **[user-authn]** Brute-force protection for Dex — per-IP rate limit on password endpoints and account lockout for LDAP/Crowd connectors. [#19542](https://github.com/deckhouse/deckhouse/pull/19542)
 - **[user-authn]** Grant kubeadm:cluster-admins group granular full access to module CRDs via dedicated ClusterRole d8:user-authn:admin-kubeconfig. [#19420](https://github.com/deckhouse/deckhouse/pull/19420)
 - **[user-authn]** UserOperation audit logs record the initiator admin's email from the deckhouse.io/initiator annotation, along with the operation type and the target user. [#20281](https://github.com/deckhouse/deckhouse/pull/20281)
 - **[user-authn]** UserOperation supports permanent user locks via spec.lock.permanent. [#20281](https://github.com/deckhouse/deckhouse/pull/20281)
 - **[user-authz]** Alert and DKP 1.78 release requirement for custom roles of the legacy experimental RBACv2 scheme. [#21381](https://github.com/deckhouse/deckhouse/pull/21381)
    If the cluster has custom roles of the legacy experimental RBACv2 scheme (`custom:*` names with the
    `rbac.deckhouse.io/kind: manage` or `use` labels or aggregation selectors), the
    `D8UserAuthzLegacyRBACv2CustomRoleFound` alert fires. Such roles will stop aggregating permissions
    in DKP 1.78 (the aggregation label scheme changes and custom roles get no compatibility aliases),
    and the upgrade to DKP 1.78 will be blocked until they are migrated to the new `d8:custom:*` scheme.
    See the user-authz module FAQ, section "How do I migrate custom roles to the new scheme in DKP 1.78?".
 - **[user-authz]** Grant kubeadm:cluster-admins group granular full access to module CRDs via dedicated ClusterRole d8:user-authz:admin-kubeconfig. [#19420](https://github.com/deckhouse/deckhouse/pull/19420)
 - **[vertical-pod-autoscaler]** Upgrade Vertical Pod Autoscaler to v1.7.0. [#21387](https://github.com/deckhouse/deckhouse/pull/21387)
    Vertical Pod Autoscaler components will restart.

## Fixes


 - **[admission-policy-engine]** Bump ratify up to 1.4.1, fix DMT allowing multiple oss.yaml files [#20797](https://github.com/deckhouse/deckhouse/pull/20797)
 - **[admission-policy-engine]** Change label for container security-policy-exceptions [#20678](https://github.com/deckhouse/deckhouse/pull/20678)
 - **[admission-policy-engine]** Changed default PSS policy to Baseline for unrecognized deckhouse versions [#19663](https://github.com/deckhouse/deckhouse/pull/19663)
 - **[admission-policy-engine]** Fix constraint-template check of AssignImage crd [#20782](https://github.com/deckhouse/deckhouse/pull/20782)
 - **[admission-policy-engine]** gatekeeper pods now tolerate csi-not-bootstrapped taint to prevent webhook deadlock during worker node replacement [#19383](https://github.com/deckhouse/deckhouse/pull/19383)
 - **[admission-policy-engine]** update Gatekeeper up to 3.22.2, fix CVEs [#20454](https://github.com/deckhouse/deckhouse/pull/20454)
 - **[candi]** Automatic k8s version is changed to 1.34 from 1.33 [#20572](https://github.com/deckhouse/deckhouse/pull/20572)
    Automatic k8s version is changed to 1.34 from 1.33, automatic setting will result in upgrade.
 - **[candi]** Fix 01-bootstrap-prerequisites.sh. [#20367](https://github.com/deckhouse/deckhouse/pull/20367)
 - **[candi]** Fix containerd credential escaping by rendering username/password into auth string. [#20589](https://github.com/deckhouse/deckhouse/pull/20589)
 - **[candi]** Make the `005_integrate_kubernetes_data_device.sh.tpl` step idempotent [#21168](https://github.com/deckhouse/deckhouse/pull/21168)
 - **[candi]** Use short timeout for deleting MirrorPod. [#20565](https://github.com/deckhouse/deckhouse/pull/20565)
 - **[candi]** fix static node cleanup to wipe data on externally mounted volumes before unmounting, preventing stale data from causing re-bootstrap failures [#20066](https://github.com/deckhouse/deckhouse/pull/20066)
 - **[candi]** kube-apiserver no longer caches watches for `ManifestCheckpointContentChunk` resources from `state-snapshotter`. [#21208](https://github.com/deckhouse/deckhouse/pull/21208)
    kube-apiserver static pod is reconfigured and restarts on the next control-plane sync.
 - **[candi]** remove Python requirement from bashible bootstrap and switch Registry Packages Proxy package installation to static binaries. [#18626](https://github.com/deckhouse/deckhouse/pull/18626)
 - **[candi]** retry kube API errors in rpp-get during registry packages discovery [#19673](https://github.com/deckhouse/deckhouse/pull/19673)
 - **[cilium-hubble]** Fixed CVE-2026-29181 in hubble-ui-backend  by bumping OpenTelemetry Go to v1.41.0 [#20087](https://github.com/deckhouse/deckhouse/pull/20087)
 - **[cilium-hubble]** Fixed CVE-2026-41520 in hubble-ui-backend [#20359](https://github.com/deckhouse/deckhouse/pull/20359)
 - **[cloud-provider-aws]** Added a new Bashible step to install `linux-modules-extra` on Ubuntu nodes. [#19415](https://github.com/deckhouse/deckhouse/pull/19415)
 - **[cloud-provider-aws]** Adds SecurityPolicyException for CCM and CSI components, fixes VPA and PodMonitor module guards, migrates hand-written templates to helm_lib helpers. [#21397](https://github.com/deckhouse/deckhouse/pull/21397)
 - **[cloud-provider-aws]** Bump helm_lib version with liveness probe parameters for CSI controller [#19694](https://github.com/deckhouse/deckhouse/pull/19694)
 - **[cloud-provider-azure]** Bump helm_lib version with liveness probe parameters for CSI controller [#19694](https://github.com/deckhouse/deckhouse/pull/19694)
 - **[cloud-provider-dvp]** Add d8_cloud_provider_dvp_migration_pending metric for D8CloudProviderDVPMigrationPending alert with logic that takes into account support for the new DVP configuration format in Commander [#21409](https://github.com/deckhouse/deckhouse/pull/21409)
 - **[cloud-provider-dvp]** Add mount points for the validation-webhook image to ensure serving certificates are properly persisted. [#21526](https://github.com/deckhouse/deckhouse/pull/21526)
 - **[cloud-provider-dvp]** Add skip storage class annotation handling to skip discovery of some storage classes from parent clusters, e.g., local disks. [#19696](https://github.com/deckhouse/deckhouse/pull/19696)
 - **[cloud-provider-dvp]** Always use WaitForFirstConsumer for child-cluster DVP StorageClasses and recreate incompatible managed classes during upgrade. [#20116](https://github.com/deckhouse/deckhouse/pull/20116)
 - **[cloud-provider-dvp]** Bump helm_lib version with liveness probe parameters for CSI controller [#19694](https://github.com/deckhouse/deckhouse/pull/19694)
 - **[cloud-provider-dvp]** CVE fixes [#20796](https://github.com/deckhouse/deckhouse/pull/20796)
 - **[cloud-provider-dvp]** DVP CSI driver now waits for VirtualDisk readiness in CreateVolume; provisioning errors (e.g. quota exceeded) are propagated to the nested cluster instead of incorrectly marking the PV as Bound [#20566](https://github.com/deckhouse/deckhouse/pull/20566)
 - **[cloud-provider-dvp]** Do not set labels dvp.deckhouse.io/cluster-uuid and dvp.deckhouse.io/hostname if their value is empty [#21110](https://github.com/deckhouse/deckhouse/pull/21110)
 - **[cloud-provider-dvp]** Fix ModuleConfig access for users with d8:manage:infrastructure:viewer and d8:manage:infrastructure:manager roles [#21487](https://github.com/deckhouse/deckhouse/pull/21487)
 - **[cloud-provider-dvp]** Fixed duplicate volume mounts in the DVP CSI driver on kubelet retries. [#20714](https://github.com/deckhouse/deckhouse/pull/20714)
 - **[cloud-provider-dvp]** add labels to cloudinit secrets in the terraform [#20321](https://github.com/deckhouse/deckhouse/pull/20321)
 - **[cloud-provider-dvp]** cloud-init secrets are now immutable to prevent post-bootstrap data mutation [#20930](https://github.com/deckhouse/deckhouse/pull/20930)
 - **[cloud-provider-dvp]** fix CSI vmBDA retry loop that caused x1580 retries and blocked new PVC mounts on static→dynamic node migration [#20145](https://github.com/deckhouse/deckhouse/pull/20145)
 - **[cloud-provider-dvp]** fix LoadBalancer stuck in pending state — retry on conflict when updating ServiceWithHealthchecks and propagate IP to child cluster service status [#19590](https://github.com/deckhouse/deckhouse/pull/19590)
 - **[cloud-provider-dvp]** fix dvp kubernetes dependency mismatch [#21367](https://github.com/deckhouse/deckhouse/pull/21367)
 - **[cloud-provider-dvp]** restores correct service-lb-controller enablement in cloud-provider-dvp [#20177](https://github.com/deckhouse/deckhouse/pull/20177)
 - **[cloud-provider-dvp]** treat missing VMBDA as success on disk detach [#21266](https://github.com/deckhouse/deckhouse/pull/21266)
 - **[cloud-provider-dynamix]** Bump helm_lib version with liveness probe parameters for CSI controller [#19694](https://github.com/deckhouse/deckhouse/pull/19694)
 - **[cloud-provider-gcp]** Bump helm_lib version with liveness probe parameters for CSI controller [#19694](https://github.com/deckhouse/deckhouse/pull/19694)
 - **[cloud-provider-huaweicloud]** Adds patches to the upstream version to make it ignore static nodes [#21351](https://github.com/deckhouse/deckhouse/pull/21351)
 - **[cloud-provider-huaweicloud]** Bump helm_lib version with liveness probe parameters for CSI controller [#19694](https://github.com/deckhouse/deckhouse/pull/19694)
 - **[cloud-provider-huaweicloud]** recreate etcd disk on master vm recreate [#20801](https://github.com/deckhouse/deckhouse/pull/20801)
 - **[cloud-provider-huaweicloud]** separate HuaweiCloudMachine VM lookup logic for create/delete and mark machines stuck in Deleting for more than 24h [#19375](https://github.com/deckhouse/deckhouse/pull/19375)
 - **[cloud-provider-openstack]** Bump helm_lib version with liveness probe parameters for CSI controller [#19694](https://github.com/deckhouse/deckhouse/pull/19694)
 - **[cloud-provider-openstack]** Prevent OpenStack CCM from deleting Kubernetes nodes when OpenStack temporarily reports an instance as not found [#20743](https://github.com/deckhouse/deckhouse/pull/20743)
 - **[cloud-provider-openstack]** fix cinder-csi-plugin permanent failure after re-authentication in long-running pods [#20394](https://github.com/deckhouse/deckhouse/pull/20394)
 - **[cloud-provider-vcd]** Bump helm_lib version with liveness probe parameters for CSI controller [#19694](https://github.com/deckhouse/deckhouse/pull/19694)
 - **[cloud-provider-vcd]** Fix LogrAdapter panic in VCD infra-controller-manager [#20107](https://github.com/deckhouse/deckhouse/pull/20107)
 - **[cloud-provider-vcd]** Fix SecurityPolicyException in CAPCD [#19539](https://github.com/deckhouse/deckhouse/pull/19539)
 - **[cloud-provider-vcd]** Fixes infra-controller-manager pods scheduling on the same node in HA mode. [#20752](https://github.com/deckhouse/deckhouse/pull/20752)
 - **[cloud-provider-vcd]** add werf deploy-dependency annotations to capcd webhook configurations to enforce correct deploy ordering. [#20987](https://github.com/deckhouse/deckhouse/pull/20987)
 - **[cloud-provider-vsphere]** Bump helm_lib version with liveness probe parameters for CSI controller [#19694](https://github.com/deckhouse/deckhouse/pull/19694)
 - **[cloud-provider-vsphere]** Upgrade CSI plugin versions and fix CVE for cloud-provider-vsphere [#18159](https://github.com/deckhouse/deckhouse/pull/18159)
 - **[cloud-provider-vsphere]** fix missing default StorageClass annotation when a DatastoreCluster entry sorts before all Datastore entries [#21111](https://github.com/deckhouse/deckhouse/pull/21111)
 - **[cloud-provider-vsphere]** normalizes new paths and makes bashible resolve existing paths case-insensitively [#19653](https://github.com/deckhouse/deckhouse/pull/19653)
 - **[cloud-provider-yandex]** Bump helm_lib version with liveness probe parameters for CSI controller [#19694](https://github.com/deckhouse/deckhouse/pull/19694)
 - **[cloud-provider-yandex]** fix volume-expansion-mode annotation - Yandex CSI supports online PVC resize without pod restart [#20578](https://github.com/deckhouse/deckhouse/pull/20578)
 - **[cloud-provider-zvirt]** Bump helm_lib version with liveness probe parameters for CSI controller [#19694](https://github.com/deckhouse/deckhouse/pull/19694)
 - **[cni-cilium]** Bump Go dependencies in the egress-gateway-agent image to fix known CVEs. [#21558](https://github.com/deckhouse/deckhouse/pull/21558)
    The egress-gateway-agent component will restart after the update.
 - **[cni-cilium]** Changed default cilium-agent health check port from 9876 to 9879 to avoid conflict with Istio's ControlZ port. [#20348](https://github.com/deckhouse/deckhouse/pull/20348)
 - **[cni-cilium]** Fixed CVE-2026-41520 for cilium-bugtool util [#20067](https://github.com/deckhouse/deckhouse/pull/20067)
 - **[cni-cilium]** Fixed infinite reconciliation of EgressGateway objects and improved status reporting. [#19219](https://github.com/deckhouse/deckhouse/pull/19219)
 - **[cni-cilium]** Update cilium version 1.17.4 -> 1.17.17. [#20720](https://github.com/deckhouse/deckhouse/pull/20720)
    The cni-cilium components (cilium agent, operator) will restart after the update. 
    A change has been made to the Network Policy that affects the behavior of the ipBlock: 0.0.0.0/0 rule.
 - **[cni-cilium]** fixed cilium agent cpu overload on nodes with many CPUs [#19903](https://github.com/deckhouse/deckhouse/pull/19903)
 - **[cni-flannel]** Reverted module stage from Deprecated back to General Availability to stop false deprecation alerts. [#20294](https://github.com/deckhouse/deckhouse/pull/20294)
 - **[cni-simple-bridge]** Fix simple bridge script to add ip rule for two NICs nodes. [#20428](https://github.com/deckhouse/deckhouse/pull/20428)
 - **[common]** Fixed CVE-2026-40898 in CoreDNS by updating the quic-go dependency. [#20736](https://github.com/deckhouse/deckhouse/pull/20736)
 - **[common]** Normalize kernel version from uname to semver [#19329](https://github.com/deckhouse/deckhouse/pull/19329)
    Nodes with Debian 13 kernels (e.g. 6.12.74+deb13+1-cloud-amd64) previously failed the kernel version check and could not join the cluster.
 - **[common]** `d8 k get <resource> -A --scope=system` now returns only `default`, `d8-*` and `kube-*` namespaces instead of every namespace outside projects. [#21532](https://github.com/deckhouse/deckhouse/pull/21532)
    kube-apiserver image is rebuilt; control-plane components restart during the update.
 - **[common]** fix StaticInstance preflight check for fails when SSHCredentials has no private key. [#19527](https://github.com/deckhouse/deckhouse/pull/19527)
 - **[common]** fixed CVE-2026-29181 in the CoreDNS [#20038](https://github.com/deckhouse/deckhouse/pull/20038)
 - **[control-plane-manager]** <ONE-LINE of what effectively changes for a user> [#21094](https://github.com/deckhouse/deckhouse/pull/21094)
 - **[control-plane-manager]** Add audit policy rule to log kubectl get logs requests (pods/log) [#20274](https://github.com/deckhouse/deckhouse/pull/20274)
 - **[control-plane-manager]** Admins now can manage modulereleases, moduledocumentations and modulesettingsdefinitions [#19931](https://github.com/deckhouse/deckhouse/pull/19931)
 - **[control-plane-manager]** Allow change labels and annotations for secret d8-secret-encryption-key [#20103](https://github.com/deckhouse/deckhouse/pull/20103)
 - **[control-plane-manager]** Bumped vulnerable Go dependencies in control-plane components to close known CVEs. [#21417](https://github.com/deckhouse/deckhouse/pull/21417)
 - **[control-plane-manager]** Fix `publishAPI` migration for clusters with legacy `user-authn` v1 settings. [#19921](https://github.com/deckhouse/deckhouse/pull/19921)
 - **[control-plane-manager]** Fixed a deadlock where a periodic etcd defragmentation operation could hold the etcd operation slot forever. [#21353](https://github.com/deckhouse/deckhouse/pull/21353)
 - **[control-plane-manager]** Fixed error from validating root certs function [#20507](https://github.com/deckhouse/deckhouse/pull/20507)
 - **[control-plane-manager]** Improve recovery and convergence of interrupted etcd member joins. [#21467](https://github.com/deckhouse/deckhouse/pull/21467)
    New and recreated control-plane nodes now retry partially completed etcd joins and learner promotion automatically. Existing healthy etcd members are unaffected.
 - **[control-plane-manager]** Skip rebind of ClusterRoleBinding/kubeadm:cluster-admins until the cluster is fully bootstrapped; harden the reconciliation hook. Fixes "cannot change roleRef" on fresh clusters. [#19667](https://github.com/deckhouse/deckhouse/pull/19667)
 - **[control-plane-manager]** fix Helm adoption of kubeadm:cluster-admins CRB on upgrade from pre-v1.76 [#20877](https://github.com/deckhouse/deckhouse/pull/20877)
    After upgrading to v1.76.0, kubectl logs and exec fail cluster-wide for all users. Manual workaround is available — see PR description.
 - **[control-plane-manager]** remove upmeter from audint-rules file [#21169](https://github.com/deckhouse/deckhouse/pull/21169)
    go generation tests
 - **[csi-vsphere]** Migrated storage policy support  from the `cloud-provider-vsphere` module. [#19965](https://github.com/deckhouse/deckhouse/pull/19965)
 - **[deckhouse-controller]** A module that conditionally depends on another is no longer disabled when an incompatible version of that dependency is enabled; the enable is rejected instead. [#20334](https://github.com/deckhouse/deckhouse/pull/20334)
 - **[deckhouse-controller]** Fix SIGSEGV on nil HookController when module hook registration is retried after a transient failure [#21201](https://github.com/deckhouse/deckhouse/pull/21201)
 - **[deckhouse-controller]** Fix applications charts rendering issue [#20260](https://github.com/deckhouse/deckhouse/pull/20260)
 - **[deckhouse-controller]** Fixed ModuleConfig validation. [#21257](https://github.com/deckhouse/deckhouse/pull/21257)
 - **[deckhouse-controller]** Fixed showing warnings while errors during kubectl edit. [#21264](https://github.com/deckhouse/deckhouse/pull/21264)
 - **[deckhouse-controller]** Fixed validation for switching ClusterConfiguration kubernetesVersion from an explicit version to Automatic. [#20323](https://github.com/deckhouse/deckhouse/pull/20323)
 - **[deckhouse-controller]** add werf dependency to webhook [#20968](https://github.com/deckhouse/deckhouse/pull/20968)
 - **[deckhouse]** Added admission policy that denies non-system users from setting or changing the `heritage` label. [#19668](https://github.com/deckhouse/deckhouse/pull/19668)
    Non-system users can no longer add or modify the `heritage` label; this prevents accidental creation of user-managed resources that become protected by Deckhouse heritage restrictions.
 - **[deckhouse]** Bump addon-operator to fix module install with conversion webhooks [#21366](https://github.com/deckhouse/deckhouse/pull/21366)
 - **[deckhouse]** Clean availableRepositories on PackageRepository deletion. [#19973](https://github.com/deckhouse/deckhouse/pull/19973)
 - **[deckhouse]** Delete module documentation when module is disabled [#20434](https://github.com/deckhouse/deckhouse/pull/20434)
 - **[deckhouse]** Drive the webhook-handler hook reload's kubernetes-binding enable to completion with retry so validating webhooks are not left denying requests due to empty snapshots. [#21247](https://github.com/deckhouse/deckhouse/pull/21247)
 - **[deckhouse]** Fix ModulePullOverride for bundle-enabled modules. [#20763](https://github.com/deckhouse/deckhouse/pull/20763)
 - **[deckhouse]** Fix Scaled stuck Unknown on controller startup [#20169](https://github.com/deckhouse/deckhouse/pull/20169)
 - **[deckhouse]** Fix exp modules auto enabling. [#19670](https://github.com/deckhouse/deckhouse/pull/19670)
 - **[deckhouse]** Fix image extraction to respect opaque whiteouts. [#20502](https://github.com/deckhouse/deckhouse/pull/20502)
 - **[deckhouse]** Fix package status deadlock via coalescing workqueue. [#20676](https://github.com/deckhouse/deckhouse/pull/20676)
 - **[deckhouse]** Make validation webhooks return Invalid reason [#19904](https://github.com/deckhouse/deckhouse/pull/19904)
 - **[deckhouse]** Re-enable kubernetes bindings in webhook-handler reload so hooks using `includeSnapshotsFrom` get populated snapshots. [#20644](https://github.com/deckhouse/deckhouse/pull/20644)
 - **[deckhouse]** Recover from panics in nelm client. [#19808](https://github.com/deckhouse/deckhouse/pull/19808)
 - **[deckhouse]** Restore ModuleIsInMaintenanceMode alert by switching to d8_module_config_maintenance sourced from ModuleConfig. [#19352](https://github.com/deckhouse/deckhouse/pull/19352)
 - **[deckhouse]** Revoke permission to use moduleconfig to user. [#19672](https://github.com/deckhouse/deckhouse/pull/19672)
 - **[deckhouse]** Use non-controller ownerRefs for multi-source package CRs. [#20045](https://github.com/deckhouse/deckhouse/pull/20045)
 - **[deckhouse]** atomically install modules and re-download incomplete versions [#21462](https://github.com/deckhouse/deckhouse/pull/21462)
 - **[dhctl]** Add explicit error if discovered node IP is empty. [#21356](https://github.com/deckhouse/deckhouse/pull/21356)
 - **[dhctl]** Add verbose logging to `bootstrap-phase` subcommands in dhctl. [#21301](https://github.com/deckhouse/deckhouse/pull/21301)
 - **[dhctl]** Apply all deckhouse prerequisites (registry pull secret, RBAC, cluster-configuration secrets, d8-cluster-uuid, ...) before the deckhouse Deployment during bootstrap, so the controller never starts ahead of resources it and its hooks read. [#21132](https://github.com/deckhouse/deckhouse/pull/21132)
 - **[dhctl]** Fix confirmation in dhctl. [#19917](https://github.com/deckhouse/deckhouse/pull/19917)
 - **[dhctl]** Fix dhctl converge-migration interactive run. [#19923](https://github.com/deckhouse/deckhouse/pull/19923)
 - **[dhctl]** Fix for the operation of `SSHProviderInitializer.Cleanup` for bootstrap commands. [#20096](https://github.com/deckhouse/deckhouse/pull/20096)
 - **[dhctl]** Fix panic in dhctl converge command [#19753](https://github.com/deckhouse/deckhouse/pull/19753)
 - **[dhctl]** Fix panic in in-cluster converge-migration run of dhctl. [#19823](https://github.com/deckhouse/deckhouse/pull/19823)
 - **[dhctl]** Fix progress bar behavior [#20219](https://github.com/deckhouse/deckhouse/pull/20219)
 - **[dhctl]** Fix static preflights for dhctl run outside an install container. [#19809](https://github.com/deckhouse/deckhouse/pull/19809)
 - **[dhctl]** Fix staticinstance ssh credential preflight in dhctl. [#21460](https://github.com/deckhouse/deckhouse/pull/21460)
 - **[dhctl]** Fixed panic in Commander Attach [#20894](https://github.com/deckhouse/deckhouse/pull/20894)
 - **[dhctl]** Fixed ssh cleanup in dhctl. [#20301](https://github.com/deckhouse/deckhouse/pull/20301)
 - **[dhctl]** Preflight checks are no longer re-run on every dhctl-server restart when the cluster config is unchanged. [#21108](https://github.com/deckhouse/deckhouse/pull/21108)
 - **[dhctl]** Pull the external provider bundle from the upstream registry when dhctl runs outside the cluster, so manual converge/destroy no longer fails on the unreachable in-cluster registry mirror. [#21509](https://github.com/deckhouse/deckhouse/pull/21509)
 - **[dhctl]** Read external provider settings from its own bundle, so check and converge no longer fail on a download directory reused after bootstrap. [#21396](https://github.com/deckhouse/deckhouse/pull/21396)
 - **[dhctl]** Refuse to create the legacy provider Secret when editing the config of a ModuleConfig-driven cluster. [#21396](https://github.com/deckhouse/deckhouse/pull/21396)
 - **[dhctl]** Replace app package references with options package in multiple files [#19702](https://github.com/deckhouse/deckhouse/pull/19702)
 - **[dhctl]** Wait for stronghold cluster sync before node deletion [#19643](https://github.com/deckhouse/deckhouse/pull/19643)
 - **[dhctl]** add reverse tunnel reachability checks [#20609](https://github.com/deckhouse/deckhouse/pull/20609)
 - **[dhctl]** bind registry packages proxy to an OS-assigned local port to avoid collisions during parallel bootstrap [#21042](https://github.com/deckhouse/deckhouse/pull/21042)
 - **[dhctl]** fix converge-migration failure when no nodes need deletion [#19835](https://github.com/deckhouse/deckhouse/pull/19835)
 - **[dhctl]** fix grpc stream cancel deadlock [#20998](https://github.com/deckhouse/deckhouse/pull/20998)
 - **[dhctl]** fix lock release command failing in interactive terminal. [#20097](https://github.com/deckhouse/deckhouse/pull/20097)
 - **[dhctl]** fix panic dereference in dhctl destroy command. [#19716](https://github.com/deckhouse/deckhouse/pull/19716)
 - **[dhctl]** fix panic in static preflight [#20392](https://github.com/deckhouse/deckhouse/pull/20392)
 - **[dhctl]** fixed the `pkill d8 k proxy` command in `dhctl` [#20466](https://github.com/deckhouse/deckhouse/pull/20466)
 - **[dhctl]** fixing the bootstrap in the local registry mode. [#20516](https://github.com/deckhouse/deckhouse/pull/20516)
 - **[dhctl]** omit error returning in cases, when static config is missed [#20405](https://github.com/deckhouse/deckhouse/pull/20405)
 - **[dhctl]** prepare control-plane template config for migration from ClusterConfiguration to ModuleConfig [#19826](https://github.com/deckhouse/deckhouse/pull/19826)
 - **[dhctl]** support capi v1beta2 when deleting clustrers and machines on destroy. [#21134](https://github.com/deckhouse/deckhouse/pull/21134)
 - **[docs]** Add info about kernel requirement for containerdv2 migration. [#19437](https://github.com/deckhouse/deckhouse/pull/19437)
 - **[docs]** Change security events docs [#20993](https://github.com/deckhouse/deckhouse/pull/20993)
 - **[istio]** Added CARGO_PROXY to ztunnel image build [#20042](https://github.com/deckhouse/deckhouse/pull/20042)
 - **[istio]** Align CNI templates with upstream and fix Istio 1.25 compatibility. [#20332](https://github.com/deckhouse/deckhouse/pull/20332)
 - **[istio]** CNI-node readonly root filesystem enable fix [#19762](https://github.com/deckhouse/deckhouse/pull/19762)
    When using containerdV2, the performance of istio-cni breaks when mounting internal paths.
 - **[istio]** Changed proxy UID to 1337 [#20074](https://github.com/deckhouse/deckhouse/pull/20074)
 - **[istio]** Fix CVE for July [#21548](https://github.com/deckhouse/deckhouse/pull/21548)
 - **[istio]** Fix CVE in istioctl [#21496](https://github.com/deckhouse/deckhouse/pull/21496)
 - **[istio]** Fix nelm rollout ordering for Certificate resources [#20924](https://github.com/deckhouse/deckhouse/pull/20924)
 - **[istio]** Fix werf files v1.27 [#20285](https://github.com/deckhouse/deckhouse/pull/20285)
 - **[istio]** Fixed access to api-proxy via ALB. [#21142](https://github.com/deckhouse/deckhouse/pull/21142)
 - **[istio]** Images v1.27 were excluded from CSE [#20296](https://github.com/deckhouse/deckhouse/pull/20296)
 - **[istio]** More stable switching between Istio revisions. [#21034](https://github.com/deckhouse/deckhouse/pull/21034)
 - **[istio]** fixed CVEs in module images [#19364](https://github.com/deckhouse/deckhouse/pull/19364)
    module pods will be restarted
 - **[istio]** fixed discovery_operator_versions_to_install.go hook to migrate from 1.21 to 1.25 [#19434](https://github.com/deckhouse/deckhouse/pull/19434)
 - **[istio]** ingressGateway advertise FQDN does not create a ServiceEntry due to an error [#19395](https://github.com/deckhouse/deckhouse/pull/19395)
 - **[kube-dns]** Bump Go dependencies in the sts-pods-hosts-appender-webhook and coredns images to fix known CVEs. [#21389](https://github.com/deckhouse/deckhouse/pull/21389)
    The coredns and kube-dns components will restart after the update.
 - **[kube-dns]** Fixing of the subsystems list for modules. [#20925](https://github.com/deckhouse/deckhouse/pull/20925)
 - **[kube-proxy]** Fixing of the subsystems list for modules. [#20925](https://github.com/deckhouse/deckhouse/pull/20925)
 - **[local-path-provisioner]** Add wildcard tolerations to the helper pod template so PVC provisioning works on tainted nodes after local-path-provisioner v0.0.32+. [#19447](https://github.com/deckhouse/deckhouse/pull/19447)
 - **[local-path-provisioner]** Backport HelperPod template validation to `local-path-provisioner` v0.0.34 to fix CVE-2026-44543 (HelperPod Template Injection, GHSA-7fxv-8wr2-mfc4, CVSS 8.7 High). [#20237](https://github.com/deckhouse/deckhouse/pull/20237)
    The `local-path-provisioner` Pod is restarted during the update. PV provisioning/teardown briefly pauses while the new Pod becomes Ready; existing volumes are not affected. After the update the provisioner refuses to create a HelperPod whose template (loaded from the `local-path-config` ConfigMap) declares privileged containers, hostPath/custom volumes, host namespaces, added Linux capabilities or other security-sensitive fields, so any pre-existing manual override of `helperPod.yaml` that uses one of these fields must be removed before the upgrade.
 - **[local-path-provisioner]** Update local-path-provisioner to v0.0.36 to pick up the upstream fix for CVE-2026-44543 (HelperPod template injection, CVSS 8.7). [#20449](https://github.com/deckhouse/deckhouse/pull/20449)
    Custom edits to the local-path-config ConfigMap that set unsafe HelperPod fields (privileged, capabilities, host namespaces, initContainers, custom volumes/volumeMounts, container probes/lifecycle, sysctls, etc.) will be rejected by the provisioner at startup. Default Deckhouse installations are unaffected.
 - **[metallb]** Bump Go dependencies in the metallb and l2lb images to fix known CVEs. [#21391](https://github.com/deckhouse/deckhouse/pull/21391)
    The metallb components (controller, speaker, l2lb) will restart after the update.
 - **[multitenancy-manager]** Grant and project admission webhooks no longer deadlock the Deckhouse queue when the multitenancy-manager backend denies, is slow, or is unreachable. [#20700](https://github.com/deckhouse/deckhouse/pull/20700)
 - **[network-gateway]** Updated dnsmasq to v2.92-alt2 to address multiple security vulnerabilities (CVE-2026-*) [#19928](https://github.com/deckhouse/deckhouse/pull/19928)
 - **[network-policy-engine]** Reverted module stage from Deprecated back to General Availability to stop false deprecation alerts. [#20294](https://github.com/deckhouse/deckhouse/pull/20294)
 - **[node-local-dns]** Bump Go dependencies in the safe-updater and stale-dns-connections-cleaner images to fix known CVEs. [#21389](https://github.com/deckhouse/deckhouse/pull/21389)
    The node-local-dns components will restart after the update.
 - **[node-manager]** Add RBAC rules for node-manager [#19720](https://github.com/deckhouse/deckhouse/pull/19720)
 - **[node-manager]** Added cleanup for oversized MCM MachineSet revision history annotation [#19652](https://github.com/deckhouse/deckhouse/pull/19652)
 - **[node-manager]** CAPI crd served version fix [#19665](https://github.com/deckhouse/deckhouse/pull/19665)
 - **[node-manager]** Creating or re-applying an already-existing StaticInstance no longer fails address validation. [#21108](https://github.com/deckhouse/deckhouse/pull/21108)
 - **[node-manager]** Deploy node-group-exporter if cluster is bootstrapped. [#21008](https://github.com/deckhouse/deckhouse/pull/21008)
 - **[node-manager]** Fill empty `Instance.spec.nodeRef` when the linked Machine reports a node name [#20432](https://github.com/deckhouse/deckhouse/pull/20432)
    <what to expect for users, possibly MULTI-LINE>, required if impact_level is high ↓
 - **[node-manager]** Fix cluster-autoscaler scale-from-zero node label verification. [#20948](https://github.com/deckhouse/deckhouse/pull/20948)
    low
 - **[node-manager]** Fix convert static cluster configuration hook. [#21345](https://github.com/deckhouse/deckhouse/pull/21345)
 - **[node-manager]** Fix fencing-agent crash when starting on a node in maintenance mode. [#20527](https://github.com/deckhouse/deckhouse/pull/20527)
    Previously, the fencing-agent would crash with "permission denied" on /dev/watchdog
    when the node had a maintenance annotation (e.g. during Deckhouse updates).
    Now the agent skips watchdog arming during maintenance and arms it automatically
    when maintenance ends.
 - **[node-manager]** Fixed TLS vulnerabilities for capi-controller-manager [#20144](https://github.com/deckhouse/deckhouse/pull/20144)
 - **[node-manager]** Fixed `StaticInstance` webhook markers and corrected `SSHCredentials` conversion logs for `v1alpha1`/`v1alpha2`. [#19757](https://github.com/deckhouse/deckhouse/pull/19757)
 - **[node-manager]** Improve fencing-agent health monitor logging — warn on fallback feeding, error on watchdog starvation, add diagnostic context to all feeding log messages. [#19400](https://github.com/deckhouse/deckhouse/pull/19400)
    Operators can now detect degraded fencing states (quorum loss, API unreachability) through log levels and diagnostic fields without parsing log messages.
 - **[node-manager]** Include system labels in CAPI MachineDeployment capacity annotation for correct scale-from-zero behavior [#20174](https://github.com/deckhouse/deckhouse/pull/20174)
    On CAPI-based clusters (DVP, VCD, zVirt, Dynamix, HuaweiCloud), scale-from-zero now correctly handles pods with nodeSelector targeting system labels (node.deckhouse.io/group, node.deckhouse.io/type, node-role.kubernetes.io/<ng-name>). Previously such pods remained Pending indefinitely when NodeGroup had minPerZone=0. No user action required — the fix is applied automatically on upgrade.
 - **[node-manager]** Reduce MachineDeployment creationTimeout to 5m for AWS spot instances [#19073](https://github.com/deckhouse/deckhouse/pull/19073)
    <what to expect for users, possibly MULTI-LINE>, required if impact_level is high ↓
 - **[node-manager]** Require a non-empty cluster UUID when rendering the node bootstrap script, preventing static nodes from stalling ~20m on a registry-packages-proxy 404. [#21132](https://github.com/deckhouse/deckhouse/pull/21132)
 - **[node-manager]** add rbac policies for persistantvolumes to manage from capi-controller-manager. [#19291](https://github.com/deckhouse/deckhouse/pull/19291)
 - **[node-manager]** enable hostNetwork for node-controller during bootstrap [#19974](https://github.com/deckhouse/deckhouse/pull/19974)
 - **[node-manager]** fix alert rules to handle split mode correctly when the MCM autoscaler job is used. [#20866](https://github.com/deckhouse/deckhouse/pull/20866)
 - **[node-manager]** fix duplicate v1alpha2 version in nodegroup crd. [#20844](https://github.com/deckhouse/deckhouse/pull/20844)
 - **[node-manager]** fix keep-policy hook failing when capi conversion webhook is unavailable. [#20990](https://github.com/deckhouse/deckhouse/pull/20990)
 - **[node-manager]** fix node-controller rollout deadlock on single-master clusters. [#21181](https://github.com/deckhouse/deckhouse/pull/21181)
 - **[node-manager]** fix webook validation in node-controller on cri changes in nodegroup. [#20050](https://github.com/deckhouse/deckhouse/pull/20050)
 - **[node-manager]** hook to restore apiVersion on CAPI resources. [#20330](https://github.com/deckhouse/deckhouse/pull/20330)
 - **[node-manager]** prevent kube-api-proxy outage when upstreams.json contains null or empty data [#20026](https://github.com/deckhouse/deckhouse/pull/20026)
 - **[node-manager]** remove conflicting ms short name from CAPI MachineSet. [#21189](https://github.com/deckhouse/deckhouse/pull/21189)
 - **[node-manager]** shutdown-inhibitor sets GracefulShutdownPostpone condition only to True or False, no more Unknown status [#20716](https://github.com/deckhouse/deckhouse/pull/20716)
 - **[prometheus]** Fix CVEs in the alertmanager, alerts-receiver, grafana-v10, memcached, mimir, prometheus, promxy and trickster images by bumping dependencies and rebuilding on the golang builder. [#20395](https://github.com/deckhouse/deckhouse/pull/20395)
    Prometheus, Grafana, Alertmanager and other monitoring components will be restarted after the update.
 - **[registry-packages-proxy]** Add fast-path for already installed packages and improve logging in rpp-get [#20015](https://github.com/deckhouse/deckhouse/pull/20015)
 - **[registry-packages-proxy]** Grant registry-packages-proxy RBAC to read unmasked registry credentials (modulesources/sensitive, packagerepositories/sensitive), restoring authentication to private registries. [#21513](https://github.com/deckhouse/deckhouse/pull/21513)
 - **[registrypackages]** Updated registrypackages/docker-registry image Go dependencies to fix Go CVEs. [#20377](https://github.com/deckhouse/deckhouse/pull/20377)
 - **[registrypackages]** Updated the Mozilla CA snapshot used by d8-ca-updater and made the build fail on trusted expired certificates. [#20939](https://github.com/deckhouse/deckhouse/pull/20939)
 - **[service-with-healthchecks]** Bump Go dependencies in the service-with-healthchecks image to fix known CVEs. [#21392](https://github.com/deckhouse/deckhouse/pull/21392)
    The service-with-healthchecks components (controller, agent) will restart after the update.
 - **[service-with-healthchecks]** Fixed an API server overload issue (status storm), resolved validation errors for ClusterIP services, corrected pod readiness evaluation logic, and improved code quality. [#19455](https://github.com/deckhouse/deckhouse/pull/19455)
    The status logic was heavily refactored to reduce API and etcd load. If you rely on `lastProbeTime` observability on every probe, you must explicitly enable `verboseStatus` in the module configuration.
 - **[user-authn]** Fixed DexAuthenticator pod creation under ResourceQuota by setting init container CPU/memory limits to the sum of main container limits. [#21313](https://github.com/deckhouse/deckhouse/pull/21313)
 - **[user-authn]** Improve basic-auth-proxy request handling, cache implementation, and shutdown behavior. [#20076](https://github.com/deckhouse/deckhouse/pull/20076)
 - **[user-authn]** Reject mutually exclusive LDAP TLS settings in DexProvider; alert on legacy objects. [#19844](https://github.com/deckhouse/deckhouse/pull/19844)
    spec.ldap.insecureNoSSL combined with startTLS, insecureSkipVerify, or a non-empty rootCAData is now rejected by the CRD. Pre-existing DexProvider objects with such combinations keep working but trigger
    D8DexProviderLDAPTLSConflict until updated. Audit with: kubectl get dexproviders.deckhouse.io -o yaml | yq '.items[] | select(.spec.type=="LDAP") | {name: .metadata.name, ldap: .spec.ldap | pick(["insecureNoSSL","startTLS","insecureSkipVerify","rootCAData"])}'
 - **[user-authn]** UserOperation no longer keeps the bcrypt password hash in spec.resetPassword.newPasswordHash after the operation completes. [#20281](https://github.com/deckhouse/deckhouse/pull/20281)
 - **[user-authn]** add forgotten field [#20864](https://github.com/deckhouse/deckhouse/pull/20864)
 - **[user-authz]** Extend cluster-admin clusterrole  with kubelet-api-admin rights. [#19878](https://github.com/deckhouse/deckhouse/pull/19878)
 - **[user-authz]** Honor CAR-independent RBAC in webhook and permission-browser [#21243](https://github.com/deckhouse/deckhouse/pull/21243)
    With enableMultiTenancy, effective access is now the union of CAR (within its limitNamespaces/namespaceSelector), AuthorizationRules, and plain RoleBindings/ClusterRoleBindings. Previously the webhook denied requests outside the CAR scope even when RBAC explicitly granted them: such existing bindings for subjects with a CAR silently become effective after the upgrade — review them. The CAR access level still does not apply outside its namespace limits. AccessibleNamespaces reflects the same union.
 - **[user-authz]** user-authz-webhook now uses the node-local kube-apiserver endpoint for its discovery cache and liveness check, instead of resolving the "kubernetes.default" DNS name. [#21066](https://github.com/deckhouse/deckhouse/pull/21066)
    Previously, a transient cluster DNS failure could cause the user-authz-webhook liveness probe to fail and restart the pod, which combined with the fail-closed authorization webhook (failurePolicy: Deny) could deny all API requests, including cluster-admins, until DNS recovered.

## Chore


 - **[candi]** Bump patch versions of Kubernetes images. [#19778](https://github.com/deckhouse/deckhouse/pull/19778)
    Kubernetes control-plane components will restart, kubelet will restart
 - **[candi]** Remove default for encryptionAlgorithm in ClusterConfiguration openAPI spec. [#20975](https://github.com/deckhouse/deckhouse/pull/20975)
 - **[candi]** Set kubelet `serializeImagePulls` to `false` to allow parallel image pulls on nodes. [#20415](https://github.com/deckhouse/deckhouse/pull/20415)
    Kubelet configuration changes on nodes and is applied through the normal node reconfiguration flow.
 - **[candi]** migrate bashible bootstrap from kubectl to d8-curl for kubernetes api calls [#19023](https://github.com/deckhouse/deckhouse/pull/19023)
 - **[cloud-provider-aws]** remove legacy d8-cni-configuration hook and helm template [#20834](https://github.com/deckhouse/deckhouse/pull/20834)
 - **[cloud-provider-aws]** revert legacy d8-cni-configuration hook and helm template removal [#21043](https://github.com/deckhouse/deckhouse/pull/21043)
 - **[cloud-provider-azure]** remove legacy d8-cni-configuration hook and helm template [#20834](https://github.com/deckhouse/deckhouse/pull/20834)
 - **[cloud-provider-azure]** revert legacy d8-cni-configuration hook and helm template removal [#21043](https://github.com/deckhouse/deckhouse/pull/21043)
 - **[cloud-provider-dvp]** bump capdvp-controller-manager cluster-api dependency to v1.12.3 [#20664](https://github.com/deckhouse/deckhouse/pull/20664)
 - **[cloud-provider-dvp]** remove legacy d8-cni-configuration hook and helm template [#20834](https://github.com/deckhouse/deckhouse/pull/20834)
 - **[cloud-provider-dvp]** revert legacy d8-cni-configuration hook and helm template removal [#21043](https://github.com/deckhouse/deckhouse/pull/21043)
 - **[cloud-provider-dynamix]** remove legacy d8-cni-configuration hook and helm template [#20834](https://github.com/deckhouse/deckhouse/pull/20834)
 - **[cloud-provider-dynamix]** revert legacy d8-cni-configuration hook and helm template removal [#21043](https://github.com/deckhouse/deckhouse/pull/21043)
 - **[cloud-provider-gcp]** remove legacy d8-cni-configuration hook and helm template [#20834](https://github.com/deckhouse/deckhouse/pull/20834)
 - **[cloud-provider-gcp]** revert legacy d8-cni-configuration hook and helm template removal [#21043](https://github.com/deckhouse/deckhouse/pull/21043)
 - **[cloud-provider-huaweicloud]** remove legacy d8-cni-configuration hook and helm template [#20834](https://github.com/deckhouse/deckhouse/pull/20834)
 - **[cloud-provider-huaweicloud]** revert legacy d8-cni-configuration hook and helm template removal [#21043](https://github.com/deckhouse/deckhouse/pull/21043)
 - **[cloud-provider-openstack]** remove legacy d8-cni-configuration hook and helm template [#20834](https://github.com/deckhouse/deckhouse/pull/20834)
 - **[cloud-provider-openstack]** revert legacy d8-cni-configuration hook and helm template removal [#21043](https://github.com/deckhouse/deckhouse/pull/21043)
 - **[cloud-provider-vcd]** remove legacy d8-cni-configuration hook and helm template [#20834](https://github.com/deckhouse/deckhouse/pull/20834)
 - **[cloud-provider-vcd]** revert legacy d8-cni-configuration hook and helm template removal [#21043](https://github.com/deckhouse/deckhouse/pull/21043)
 - **[cloud-provider-vsphere]** Add module hook and template rendering tests for hybrid cluster with vSphere cloud provider [#19209](https://github.com/deckhouse/deckhouse/pull/19209)
 - **[cloud-provider-vsphere]** remove legacy d8-cni-configuration hook and helm template [#20834](https://github.com/deckhouse/deckhouse/pull/20834)
 - **[cloud-provider-vsphere]** revert legacy d8-cni-configuration hook and helm template removal [#21043](https://github.com/deckhouse/deckhouse/pull/21043)
 - **[cloud-provider-yandex]** remove legacy d8-cni-configuration hook and helm template [#20834](https://github.com/deckhouse/deckhouse/pull/20834)
 - **[cloud-provider-yandex]** revert legacy d8-cni-configuration hook and helm template removal [#21043](https://github.com/deckhouse/deckhouse/pull/21043)
 - **[cloud-provider-zvirt]** remove legacy d8-cni-configuration hook and helm template [#20834](https://github.com/deckhouse/deckhouse/pull/20834)
 - **[cloud-provider-zvirt]** revert legacy d8-cni-configuration hook and helm template removal [#21043](https://github.com/deckhouse/deckhouse/pull/21043)
 - **[cni-cilium]** enable bpf trace events for hubble module by default [#21550](https://github.com/deckhouse/deckhouse/pull/21550)
 - **[common]** Reduce final release size by removing unneeded common images from final [#20590](https://github.com/deckhouse/deckhouse/pull/20590)
 - **[common]** remove legacy get_cni_secret hook and cniSecretData values field [#20834](https://github.com/deckhouse/deckhouse/pull/20834)
 - **[common]** revert legacy get_cni_secret hook and cniSecretData values field removal [#21043](https://github.com/deckhouse/deckhouse/pull/21043)
 - **[control-plane-manager]** Disable kube-api QPS rate limit for kube-controller-manager. [#20410](https://github.com/deckhouse/deckhouse/pull/20410)
 - **[deckhouse-controller]** Bump k8s libraries to support kubernetes 1.36. [#20488](https://github.com/deckhouse/deckhouse/pull/20488)
 - **[deckhouse-controller]** Updated addon-operator to v1.23.3. [#21170](https://github.com/deckhouse/deckhouse/pull/21170)
 - **[deckhouse]** Build base-for-go on distroless builder/golang. [#20531](https://github.com/deckhouse/deckhouse/pull/20531)
 - **[deckhouse]** Decouple from dhctl/cmd/commands. [#19624](https://github.com/deckhouse/deckhouse/pull/19624)
 - **[deckhouse]** Finish failed operations. [#19236](https://github.com/deckhouse/deckhouse/pull/19236)
 - **[deckhouse]** Merge the monitoring-deckhouse module (dashboard, alert rules, PodMonitor) into the deckhouse module. [#20727](https://github.com/deckhouse/deckhouse/pull/20727)
 - **[deckhouse]** Removed in-tree operator-prometheus, prometheus, and loki modules. These modules are now sourced externally [#20884](https://github.com/deckhouse/deckhouse/pull/20884)
 - **[deckhouse]** create module monitoring-security in cse [#20063](https://github.com/deckhouse/deckhouse/pull/20063)
 - **[docs]** Add Search API and OpenSearch admin API documentation for Deckhouse Code [#21234](https://github.com/deckhouse/deckhouse/pull/21234)
 - **[docs]** Info about editions for egressgateway has been edited. [#19545](https://github.com/deckhouse/deckhouse/pull/19545)
 - **[docs]** Local authentication article in product documentation updated via [#20187](https://github.com/deckhouse/deckhouse/pull/20187)
 - **[docs]** Upgrade Hugo to v0.161.1. [#20131](https://github.com/deckhouse/deckhouse/pull/20131)
 - **[documentation]** Fix CVEs. Upgrade Hugo to v0.161.1 in documentation builder. [#20131](https://github.com/deckhouse/deckhouse/pull/20131)
 - **[go_lib]** remove legacy get_cni_secret hook library [#20834](https://github.com/deckhouse/deckhouse/pull/20834)
 - **[istio]** Added ClusterRoles (v1) over module CustomResources [#19587](https://github.com/deckhouse/deckhouse/pull/19587)
 - **[istio]** Added HTTPRoute for multiclusters. [#19603](https://github.com/deckhouse/deckhouse/pull/19603)
    low
 - **[istio]** Added custom templates for non-operator case [#20284](https://github.com/deckhouse/deckhouse/pull/20284)
 - **[istio]** Fix CVE and image configuration [#21586](https://github.com/deckhouse/deckhouse/pull/21586)
 - **[istio]** Fixed prometheus CVEs in v1.27 [#20807](https://github.com/deckhouse/deckhouse/pull/20807)
 - **[istio]** Refactor hooks to prepare for 1.27 [#20258](https://github.com/deckhouse/deckhouse/pull/20258)
 - **[istio]** Vex justified CVE-2026-44903. Fixed CVE-2026-46680 in operator 1.25 [#20212](https://github.com/deckhouse/deckhouse/pull/20212)
 - **[istio]** added CRDs for v1.27 [#20046](https://github.com/deckhouse/deckhouse/pull/20046)
 - **[istio]** added istio images for version 1.27.9 [#19810](https://github.com/deckhouse/deckhouse/pull/19810)
 - **[istio]** added logout button in Kiali Console [#20922](https://github.com/deckhouse/deckhouse/pull/20922)
 - **[istio]** added supportsOperator flag in istio.internal.versionMap [#19996](https://github.com/deckhouse/deckhouse/pull/19996)
 - **[istio]** changed UID and GID over all pods in module [#18851](https://github.com/deckhouse/deckhouse/pull/18851)
    operator will be restarted
 - **[istio]** changed vex CVE justifications in pilots images [#19572](https://github.com/deckhouse/deckhouse/pull/19572)
 - **[istio]** clang binaries now builds using prepared Llvm cache [#20816](https://github.com/deckhouse/deckhouse/pull/20816)
 - **[istio]** d8-istio namespace was removed from exclude_namespaces list of cni-config [#20630](https://github.com/deckhouse/deckhouse/pull/20630)
 - **[istio]** fixed kiali images build for CSE [#21047](https://github.com/deckhouse/deckhouse/pull/21047)
 - **[istio]** fixes for dmt lint [#21574](https://github.com/deckhouse/deckhouse/pull/21574)
 - **[istio]** set readOnlyRootFilesystem to true in istio-init container [#21096](https://github.com/deckhouse/deckhouse/pull/21096)
 - **[istio]** skip operator/IOP/Sail install paths when supportsOperator is false [#20117](https://github.com/deckhouse/deckhouse/pull/20117)
 - **[istio]** vex justified CVE-2026-42151 and CVE-2026-42154 in pilot and operator images [#19949](https://github.com/deckhouse/deckhouse/pull/19949)
 - **[kube-dns]** Fixed securityContext and added SecurityPolicyException [#21501](https://github.com/deckhouse/deckhouse/pull/21501)
 - **[metallb]** Fixed securityContext and added SecurityPolicyException [#21520](https://github.com/deckhouse/deckhouse/pull/21520)
 - **[node-local-dns]** Fixed securityContext and added SecurityPolicyException [#21501](https://github.com/deckhouse/deckhouse/pull/21501)
 - **[node-manager]** CAPS provider refactor [#18746](https://github.com/deckhouse/deckhouse/pull/18746)
 - **[node-manager]** migrate node/nodegroup reconciliation hooks to node-controller. [#18481](https://github.com/deckhouse/deckhouse/pull/18481)
 - **[node-manager]** update CAPI from v1.11.3 to v1.12.9 [#21112](https://github.com/deckhouse/deckhouse/pull/21112)
 - **[openvpn]** Fixed securityContext and added SecurityPolicyException [#21538](https://github.com/deckhouse/deckhouse/pull/21538)
 - **[registrypackages]** Update containerd version to 1.7.34, 2.2.6, update runc to 1.3.6, update dependencies in containerd v1, v2, crictl. [#21347](https://github.com/deckhouse/deckhouse/pull/21347)
 - **[registrypackages]** add containerd, kubeletm kubernetes-cni sysext to registrypackages [#20765](https://github.com/deckhouse/deckhouse/pull/20765)
 - **[terraform-manager]** Upgrade opentofu to 1.12.0. [#20001](https://github.com/deckhouse/deckhouse/pull/20001)
 - **[user-authz]** Documented migration from the deprecated RBACv2 role names (d8:manage:*, d8:use:role:*) and old custom-role scheme to the new one (d8:system:*, d8:subsystem:*, d8:namespace:*, d8:custom:*). [#21290](https://github.com/deckhouse/deckhouse/pull/21290)
 - **[vertical-pod-autoscaler]** Add a cluster alert and Grafana dashboard that detect VerticalPodAutoscalers using the deprecated Auto update mode. [#20532](https://github.com/deckhouse/deckhouse/pull/20532)
