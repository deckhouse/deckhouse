# Changelog v1.77

## Know before update


 - Affects clusters where `publishAPI` is enabled together with `enableBasicAuth` on a
    Crowd, OIDC or LDAP `DexProvider`. Both are disabled by default, so a default
    installation is not affected.
    
    Previously the proxy copied directory group names into the `X-Remote-Group` header
    verbatim. Because kube-apiserver trusts that header from this proxy, a user in a
    directory group literally named `system:masters` was granted cluster-admin. Group
    names are chosen by the directory administrator, not by Kubernetes.
    
    After updating, any group whose name begins with `system:` is dropped before the
    request reaches kube-apiserver, and each drop is logged by the basic-auth-proxy pod
    with the user and the group name. A user authenticating with a `system:`-prefixed
    username is refused with `403 Forbidden`. This matches the `userValidationRules`
    already enforced on the OIDC/JWT path.
    
    Before updating, audit your directory for groups whose name begins with `system:`
    and check whether anyone currently relies on one to reach the Kubernetes API through
    basic auth. Such users will lose those privileges on update and must be granted
    access through a normal group bound by RBAC instead. Check the basic-auth-proxy logs
    after updating for `dropping group` messages.
 - All control plane components will be restarted.
 - All the `ingress-nginx` module Pods will be restarted.
 - Applying this change restarts control-plane components.
 - Cloud provider configuration for DKP clusters deployed in DVP needs to be migrated from ProviderClusterConfiguration to ModuleConfig settings. For migration instructions, refer to "Cluster and Infrastructure" in the DKP FAQ.
 - Custom roles of the legacy experimental RBACv2 scheme must be migrated to the new `d8:custom:*` scheme before upgrading to DKP 1.78. The `D8UserAuthzLegacyRBACv2CustomRoleFound` alert identifies affected roles. Refer to the `user-authz` module FAQ for migration instructions.
 - If your cluster was previously upgraded to DKP 1.76.0 from a version earlier than 1.76 and was not patched, `kubectl logs` and `kubectl exec` may fail cluster-wide for all users. Apply the manual workaround from the [PR description](https://github.com/deckhouse/deckhouse/pull/20877) before upgrading.
 - In the `metallb` module, BGP configuration was migrated from ModuleConfig to dedicated custom resources. Existing configuration is migrated automatically during the update. Before editing the generated resources, disable further migration by updating the ModuleConfig version. Afterwards, remove the obsolete BGP settings from the ModuleConfig. See the module documentation for details.
 - Nodes in maintenance mode could previously cause the fencing-agent to crash while attempting to arm the watchdog. The agent now skips watchdog arming during maintenance and resumes it automatically afterwards.
 - The CoreDNS and kube-dns components will be restarted after the update.
 - The `cni-cilium` components (cilium-agent, operator) will be restarted. A change has been made to the Network Policy that affects the behavior of the `ipBlock: 0.0.0.0/0` rule.
 - The `dexclient.deckhouse.io/allow-access-to-kubernetes` and
    `dexauthenticator.deckhouse.io/allow-access-to-kubernetes` annotations were previously
    evaluated by key presence, so any value — including "false" — granted the client the
    right to obtain tokens with `aud=kubernetes` and placed it into `trustedPeers` of the
    `kubernetes` OAuth2Client, whose tokens kube-apiserver accepts.
    
    The value is now parsed as a boolean. Only 1, t, T, TRUE, true and True grant the
    capability. Any other value, including "false", "0", "yes", "enabled" and an empty
    string, disables it; unparseable values are rejected with a warning in the
    deckhouse Pod log naming the object and the offending value.
    
    Affected: clusters with a DexClient or DexAuthenticator that carries the annotation
    with a value other than a boolean true. Those objects lose access to the Kubernetes
    API on update, which for most of them is the behaviour their author intended.
    
    Before updating, run
    `kubectl get dexclient,dexauthenticator -A -o json | jq -r '.items[] | select(.metadata.annotations | to_entries[]? | select(.key | endswith("deckhouse.io/allow-access-to-kubernetes")) | .value | ascii_downcase | IN("1","t","true") | not) | "\(.kind)/\(.metadata.namespace)/\(.metadata.name)"'`
    and, for every object listed, either set the annotation to "true" if it genuinely needs
    Kubernetes API access, or remove the annotation to confirm it does not.
 - The `early-oom` DaemonSet in the `d8-cloud-instance-manager` namespace will be removed.
 - The `egress-gateway-agent` component will be restarted.
 - The `local-path-provisioner` Pod is restarted during the update. PV provisioning and teardown briefly pauses while the new Pod becomes `Ready`; existing volumes are not affected. If you use a custom `helperPod.yaml` template in the `local-path-config` ConfigMap, remove any unsupported security-sensitive settings before upgrading. Otherwise, the `local-path-provisioner` will reject the template after the update.
 - The `metallb` components (controller, speaker, l2lb) will be restarted.
 - The `node-local-dns` components will be restarted.
 - The `service-with-healthchecks` components (controller, agent) will be restarted.
 - The `service-with-healthchecks` status logic was heavily refactored to reduce API and etcd load. If you rely on `lastProbeTime` observability on every probe, explicitly enable `verboseStatus` in the module configuration.
 - The cilium-hubble components (hubble-ui, hubble-relay) will restart after the update.
 - The cni-cilium components (cilium agent, operator) will restart after the update.
 - The minimum supported version of Kubernetes is now 1.32. All control plane components will be restarted.
 - The node-local-dns DaemonSet pods will restart after the update.
 - This release fixes an issue where transient cluster DNS failures could cause the user-authz-webhook to restart, temporarily denying all Kubernetes API requests until DNS recovered.
 - Unsafe custom HelperPod settings in the `local-path-config` ConfigMap are no longer accepted. Default DKP installations are unaffected.
 - Values taken from `Project.spec.parameters` are now quoted where they are substituted into the
    shipped project templates, and a parameter that changes the structure of the rendered manifests
    rather than only their values is refused for any template, including custom ones. An administrator
    name may no longer contain control characters, and `clusterLogDestinationName` must be a Kubernetes
    object name or empty. Both are checked from the reconcile loop as well as at admission, so a
    project already carrying such a value goes into an error state on its next reconcile rather than
    at its next edit; the same holds for a custom template that turns a parameter into several objects
    or into YAML, which is refused when the project is created, edited, or reconciled with a change to
    apply. The module's admission policy now also matches CREATE, so objects labelled
    `heritage: multitenancy-manager` can only be created by the module itself.
 - Vertical Pod Autoscaler components will be restarted.
 - When using containerdV2, the performance of istio-cni breaks when mounting internal paths.
 - With `enableMultiTenancy` enabled in the `user-authz` settings, effective permissions now include both ClusterAuthorizationRule and Kubernetes RBAC permissions. Existing RoleBinding and ClusterRoleBinding objects that were previously ignored outside the ClusterAuthorizationRule scope may become effective after the upgrade. Review existing RBAC bindings for affected users and groups.

## Features


 - **[admission-policy-engine]** Added a ValidatingAdmissionPolicy that forbids removing Deckhouse finalizers containing `deckhouse.io`. [#21217](https://github.com/deckhouse/deckhouse/pull/21217)
 - **[admission-policy-engine]** Added a complex nodeAffinity selector for the Gatekeeper controller-manager. [#20656](https://github.com/deckhouse/deckhouse/pull/20656)
 - **[admission-policy-engine]** Grant kubeadm:cluster-admins group granular full access to module CRDs via dedicated ClusterRole d8:admission-policy-engine:admin-kubeconfig. [#19420](https://github.com/deckhouse/deckhouse/pull/19420)
 - **[admission-policy-engine]** Refactored constraints and tests and added support for container-level SecurityPolicyException. [#18668](https://github.com/deckhouse/deckhouse/pull/18668)
 - **[candi]** Added oss.yaml files for cloud provider modules. [#18989](https://github.com/deckhouse/deckhouse/pull/18989)
 - **[candi]** Added support for Kubernetes 1.36 and discontinued support for Kubernetes 1.31. The default Kubernetes version was changed from 1.33 to 1.34. [#19623](https://github.com/deckhouse/deckhouse/pull/19623)
    The minimum supported version of Kubernetes is now 1.32. All control plane components will be restarted.
 - **[candi]** Added x-kubernetes-sensitive-data masking for sensitive fields. [#19634](https://github.com/deckhouse/deckhouse/pull/19634)
 - **[candi]** Migrated node-manager (candi) builds to the unified builder with package manager and Go toolchain. [#19882](https://github.com/deckhouse/deckhouse/pull/19882)
 - **[cert-manager]** Auto-enable cainjector when cluster resources require cert-manager CA injection annotations. [#19850](https://github.com/deckhouse/deckhouse/pull/19850)
 - **[cilium-hubble]** Added authorization to the Cilium Hubble UI. [#18895](https://github.com/deckhouse/deckhouse/pull/18895)
 - **[cilium-hubble]** Fixed securityContext and added a SecurityPolicyException. [#21545](https://github.com/deckhouse/deckhouse/pull/21545)
 - **[cloud-provider-aws]** Added oss.yaml files for cloud provider modules. [#18989](https://github.com/deckhouse/deckhouse/pull/18989)
 - **[cloud-provider-aws]** Migrated AWS to Cilium CNI by default for new clusters. [#19774](https://github.com/deckhouse/deckhouse/pull/19774)
 - **[cloud-provider-aws]** Migrated cloud-provider RBAC v1 `user-authz` ClusterRoles to a shared `helm-lib` template. [#19964](https://github.com/deckhouse/deckhouse/pull/19964)
 - **[cloud-provider-aws]** Migrated cloud-provider-aws builds to the unified builder with package manager and Go toolchain. [#19616](https://github.com/deckhouse/deckhouse/pull/19616)
 - **[cloud-provider-azure]** Added oss.yaml files for cloud provider modules. [#18989](https://github.com/deckhouse/deckhouse/pull/18989)
 - **[cloud-provider-azure]** Migrated Azure CCM and CDD deployments to shared lib-helm templates. [#19264](https://github.com/deckhouse/deckhouse/pull/19264)
 - **[cloud-provider-azure]** Migrated cloud-provider RBAC v1 `user-authz` ClusterRoles to a shared `helm-lib` template. [#19964](https://github.com/deckhouse/deckhouse/pull/19964)
 - **[cloud-provider-azure]** Migrated remaining cloud-provider Go artifact builds to the unified `builder/golang` image. [#20084](https://github.com/deckhouse/deckhouse/pull/20084)
 - **[cloud-provider-azure]** Migrated to Cilium CNI by default for new clusters. [#19807](https://github.com/deckhouse/deckhouse/pull/19807)
 - **[cloud-provider-dvp]** Add an option to disable the CCM LoadBalancer functionality via ConfigMap. [#21296](https://github.com/deckhouse/deckhouse/pull/21296)
 - **[cloud-provider-dvp]** Added live StorageClass migration for VM disks via DeckhouseMachine spec changes. [#19919](https://github.com/deckhouse/deckhouse/pull/19919)
 - **[cloud-provider-dvp]** Added oss.yaml files for cloud provider modules. [#18989](https://github.com/deckhouse/deckhouse/pull/18989)
 - **[cloud-provider-dvp]** Cleaned up LoadBalancer and ServiceWithHealthchecks resources during DVP cluster deletion. [#20078](https://github.com/deckhouse/deckhouse/pull/20078)
 - **[cloud-provider-dvp]** Enabled the security policy check and added SecurityPolicyException for DVP CSI and CCM. [#18873](https://github.com/deckhouse/deckhouse/pull/18873)
 - **[cloud-provider-dvp]** Migrated cloud-provider RBAC v1 `user-authz` ClusterRoles to a shared `helm-lib` template. [#19964](https://github.com/deckhouse/deckhouse/pull/19964)
 - **[cloud-provider-dvp]** Migrated module configuration from ProviderClusterConfiguration to ModuleConfig. [#20236](https://github.com/deckhouse/deckhouse/pull/20236)
    Cloud provider configuration for DKP clusters deployed in DVP needs to be migrated from ProviderClusterConfiguration to ModuleConfig settings. For migration instructions, refer to "Cluster and Infrastructure" in the DKP FAQ.
 - **[cloud-provider-dvp]** Migrated remaining cloud-provider Go artifact builds to the unified `builder/golang` image. [#20084](https://github.com/deckhouse/deckhouse/pull/20084)
 - **[cloud-provider-dynamix]** Added oss.yaml files for cloud provider modules. [#18989](https://github.com/deckhouse/deckhouse/pull/18989)
 - **[cloud-provider-dynamix]** Migrated Dynamix, GCP, and OpenStack templates to define from helm-lib. [#19267](https://github.com/deckhouse/deckhouse/pull/19267)
 - **[cloud-provider-dynamix]** Migrated cloud-provider RBAC v1 `user-authz` ClusterRoles to a shared `helm-lib` template. [#19964](https://github.com/deckhouse/deckhouse/pull/19964)
 - **[cloud-provider-dynamix]** Migrated remaining cloud-provider Go artifact builds to the unified `builder/golang` image. [#20084](https://github.com/deckhouse/deckhouse/pull/20084)
 - **[cloud-provider-gcp]** Added oss.yaml files for cloud provider modules. [#18989](https://github.com/deckhouse/deckhouse/pull/18989)
 - **[cloud-provider-gcp]** Migrated Dynamix, GCP, and OpenStack templates to define from helm-lib. [#19267](https://github.com/deckhouse/deckhouse/pull/19267)
 - **[cloud-provider-gcp]** Migrated cloud-provider RBAC v1 `user-authz` ClusterRoles to a shared `helm-lib` template. [#19964](https://github.com/deckhouse/deckhouse/pull/19964)
 - **[cloud-provider-gcp]** Migrated remaining cloud-provider Go artifact builds to the unified `builder/golang` image. [#20084](https://github.com/deckhouse/deckhouse/pull/20084)
 - **[cloud-provider-gcp]** Migrated to Cilium CNI by default for new clusters. [#19807](https://github.com/deckhouse/deckhouse/pull/19807)
 - **[cloud-provider-huaweicloud]** Added oss.yaml files for cloud provider modules. [#18989](https://github.com/deckhouse/deckhouse/pull/18989)
 - **[cloud-provider-huaweicloud]** Bumped the CAPI provider to cluster-api v1.12 and fixed the Machine initialization contract. [#20723](https://github.com/deckhouse/deckhouse/pull/20723)
 - **[cloud-provider-huaweicloud]** Migrated cloud-provider RBAC v1 `user-authz` ClusterRoles to a shared `helm-lib` template. [#19964](https://github.com/deckhouse/deckhouse/pull/19964)
 - **[cloud-provider-huaweicloud]** Migrated remaining cloud-provider Go artifact builds to the unified `builder/golang` image. [#20084](https://github.com/deckhouse/deckhouse/pull/20084)
 - **[cloud-provider-huaweicloud]** Switched Huawei Cloud to OpenTofu mode with terraform-state migration in terraform-manager. [#19647](https://github.com/deckhouse/deckhouse/pull/19647)
 - **[cloud-provider-openstack]** Added CAPI support for the OpenStack cloud provider. [#20515](https://github.com/deckhouse/deckhouse/pull/20515)
 - **[cloud-provider-openstack]** Added oss.yaml files for cloud provider modules. [#18989](https://github.com/deckhouse/deckhouse/pull/18989)
 - **[cloud-provider-openstack]** Migrated Dynamix, GCP, and OpenStack templates to define from helm-lib. [#19267](https://github.com/deckhouse/deckhouse/pull/19267)
 - **[cloud-provider-openstack]** Migrated cloud-provider RBAC v1 `user-authz` ClusterRoles to a shared `helm-lib` template. [#19964](https://github.com/deckhouse/deckhouse/pull/19964)
 - **[cloud-provider-openstack]** Migrated remaining cloud-provider Go artifact builds to the unified `builder/golang` image. [#20084](https://github.com/deckhouse/deckhouse/pull/20084)
 - **[cloud-provider-openstack]** Switched OpenStack to OpenTofu mode with terraform-state migration in terraform-manager. [#19381](https://github.com/deckhouse/deckhouse/pull/19381)
 - **[cloud-provider-vcd]** Added oss.yaml files for cloud provider modules. [#18989](https://github.com/deckhouse/deckhouse/pull/18989)
 - **[cloud-provider-vcd]** Added support for setting a predefined VCD LoadBalancer IP via Service annotation. [#19953](https://github.com/deckhouse/deckhouse/pull/19953)
 - **[cloud-provider-vcd]** Migrated Dynamix, GCP, and OpenStack templates to define from helm-lib. [#19267](https://github.com/deckhouse/deckhouse/pull/19267)
 - **[cloud-provider-vcd]** Migrated cloud-provider RBAC v1 `user-authz` ClusterRoles to a shared `helm-lib` template. [#19964](https://github.com/deckhouse/deckhouse/pull/19964)
 - **[cloud-provider-vcd]** Migrated from Terraform to OpenTofu. [#19594](https://github.com/deckhouse/deckhouse/pull/19594)
 - **[cloud-provider-vcd]** Migrated remaining cloud-provider Go artifact builds to the unified `builder/golang` image. [#20084](https://github.com/deckhouse/deckhouse/pull/20084)
 - **[cloud-provider-vcd]** Migrated the VCD CAPI provider to cluster-api v1.12 with the v1beta2 contract. [#18518](https://github.com/deckhouse/deckhouse/pull/18518)
 - **[cloud-provider-vsphere]** Added oss.yaml files for cloud provider modules. [#18989](https://github.com/deckhouse/deckhouse/pull/18989)
 - **[cloud-provider-vsphere]** Enabled the security policy check and added SecurityPolicyException rendering for vSphere CCM and CSI components. [#18905](https://github.com/deckhouse/deckhouse/pull/18905)
 - **[cloud-provider-vsphere]** Migrated cloud-provider RBAC v1 `user-authz` ClusterRoles to a shared `helm-lib` template. [#19964](https://github.com/deckhouse/deckhouse/pull/19964)
 - **[cloud-provider-vsphere]** Migrated from Terraform to OpenTofu. [#19645](https://github.com/deckhouse/deckhouse/pull/19645)
 - **[cloud-provider-vsphere]** Migrated remaining cloud-provider Go artifact builds to the unified `builder/golang` image. [#20084](https://github.com/deckhouse/deckhouse/pull/20084)
 - **[cloud-provider-yandex]** Added CAPI support for the Yandex provider. [#19432](https://github.com/deckhouse/deckhouse/pull/19432)
 - **[cloud-provider-yandex]** Added oss.yaml files for cloud provider modules. [#18989](https://github.com/deckhouse/deckhouse/pull/18989)
 - **[cloud-provider-yandex]** Enabled the security policy check and added SecurityPolicyException rendering for Yandex CCM and CSI components. [#18899](https://github.com/deckhouse/deckhouse/pull/18899)
 - **[cloud-provider-yandex]** Migrated cloud-provider RBAC v1 `user-authz` ClusterRoles to a shared `helm-lib` template. [#19964](https://github.com/deckhouse/deckhouse/pull/19964)
 - **[cloud-provider-yandex]** Migrated remaining cloud-provider Go artifact builds to the unified `builder/golang` image. [#20084](https://github.com/deckhouse/deckhouse/pull/20084)
 - **[cloud-provider-zvirt]** Added oss.yaml files for cloud provider modules. [#18989](https://github.com/deckhouse/deckhouse/pull/18989)
 - **[cloud-provider-zvirt]** Enabled the security policy check and added SecurityPolicyException rendering for zVirt CCM and CSI components. [#18902](https://github.com/deckhouse/deckhouse/pull/18902)
 - **[cloud-provider-zvirt]** Migrated cloud-provider RBAC v1 `user-authz` ClusterRoles to a shared `helm-lib` template. [#19964](https://github.com/deckhouse/deckhouse/pull/19964)
 - **[cloud-provider-zvirt]** Migrated remaining cloud-provider Go artifact builds to the unified `builder/golang` image. [#20084](https://github.com/deckhouse/deckhouse/pull/20084)
 - **[cni-cilium]** Added RBACv1 roles (User and ClusterEditor) for HubbleMonitoringConfig resource. [#19629](https://github.com/deckhouse/deckhouse/pull/19629)
 - **[cni-cilium]** Disabled BPF trace events by default. [#19997](https://github.com/deckhouse/deckhouse/pull/19997)
 - **[cni-cilium]** Fixed securityContext and added a SecurityPolicyException. [#21545](https://github.com/deckhouse/deckhouse/pull/21545)
 - **[cni-cilium]** Reduced the CPU load in cilium-agent with hubble enabled. [#19669](https://github.com/deckhouse/deckhouse/pull/19669)
 - **[cni-flannel]** Fixed securityContext and added SecurityPolicyException [#21709](https://github.com/deckhouse/deckhouse/pull/21709)
 - **[common]** Add token_namespace and token_name labels to serviceaccount_stale_tokens_total kube-apiserver metric [#19506](https://github.com/deckhouse/deckhouse/pull/19506)
 - **[common]** Added a Kubernetes scheduler patch that prevents scheduling pods onto nodes during graceful shutdown. [#20264](https://github.com/deckhouse/deckhouse/pull/20264)
 - **[common]** Added opt-in ACL-filtered cross-namespace listing for `d8 k get -A --scope=...` so users without cluster-wide RBAC get filtered results instead of 403. [#21206](https://github.com/deckhouse/deckhouse/pull/21206)
    kube-apiserver image is rebuilt; control-plane components restart during the update.
 - **[common]** Added oss.yaml files for cloud provider modules. [#18989](https://github.com/deckhouse/deckhouse/pull/18989)
 - **[common]** Added real-time namespace watching via `d8 k get ns -w`. [#21101](https://github.com/deckhouse/deckhouse/pull/21101)
 - **[common]** Added real-time support for `d8 k get ns -w`. [#19676](https://github.com/deckhouse/deckhouse/pull/19676)
 - **[common]** Updated the Ingress NGINX Controller file system layout and added the `validationIsolationMode` setting. [#18145](https://github.com/deckhouse/deckhouse/pull/18145)
    All Ingress NGINX Controller pods will be restarted.
 - **[control-plane-manager]** Added periodic etcd defragmentation with a configurable schedule via ModuleConfig. [#20575](https://github.com/deckhouse/deckhouse/pull/20575)
 - **[control-plane-manager]** Added the ControlPlaneNode and ControlPlaneOperation resources for monitoring and managing control plane updates. [#17798](https://github.com/deckhouse/deckhouse/pull/17798)
    All control plane components will be restarted.
 - **[control-plane-manager]** Control-plane resource requests can now be configured in ModuleConfig control-plane-manager. [#20127](https://github.com/deckhouse/deckhouse/pull/20127)
 - **[control-plane-manager]** Extend d8:control-plane-manager:admin-kubeconfig-supplement with granular permissions for standard Kubernetes resources not covered by user-authz:cluster-admin. [#19420](https://github.com/deckhouse/deckhouse/pull/19420)
 - **[control-plane-manager]** Moved the encryptionAlgorithm parameter to the control-plane-manager ModuleConfig. [#20497](https://github.com/deckhouse/deckhouse/pull/20497)
 - **[control-plane-manager]** Publish API ingress settings are moved to control-plane-manager module, ingress resource is moved to kube-system namespace. [#18628](https://github.com/deckhouse/deckhouse/pull/18628)
 - **[deckhouse-controller]** Added mechanism for blocking deckhouse release if cluster have alerts with high severity. [#20616](https://github.com/deckhouse/deckhouse/pull/20616)
 - **[deckhouse-controller]** Added resource grant resolving. [#20841](https://github.com/deckhouse/deckhouse/pull/20841)
 - **[deckhouse-controller]** harden migrated modules check [#20949](https://github.com/deckhouse/deckhouse/pull/20949)
 - **[deckhouse]** Add package health monitor. [#19711](https://github.com/deckhouse/deckhouse/pull/19711)
 - **[deckhouse]** Added a beforeDeleteHelm hook binding for packages. [#21344](https://github.com/deckhouse/deckhouse/pull/21344)
 - **[deckhouse]** Added a conditions summary to applications. [#20273](https://github.com/deckhouse/deckhouse/pull/20273)
 - **[deckhouse]** Added a newly-found versions counter to PackageRepositoryOperation. [#19929](https://github.com/deckhouse/deckhouse/pull/19929)
 - **[deckhouse]** Added localized disable messages to modules and packages. [#20791](https://github.com/deckhouse/deckhouse/pull/20791)
 - **[deckhouse]** Added maintenance mode for applications. [#21173](https://github.com/deckhouse/deckhouse/pull/21173)
 - **[deckhouse]** Added publicDomainTemplate for the application platform. [#21052](https://github.com/deckhouse/deckhouse/pull/21052)
 - **[deckhouse]** Added the x-deckhouse-ui-order extension to application package settings schemas. [#21465](https://github.com/deckhouse/deckhouse/pull/21465)
 - **[deckhouse]** Added the x-deckhouse-ui-validation-description extension to application package settings schemas. [#21495](https://github.com/deckhouse/deckhouse/pull/21495)
 - **[deckhouse]** Allowed specific experimental modules via allowedExperimentalModules. [#20582](https://github.com/deckhouse/deckhouse/pull/20582)
 - **[deckhouse]** Application status now exposes endpoints (`status.urls` with `url` and `description`) collected from chart Ingresses annotated with `packages.deckhouse.io/application-endpoint`. [#21104](https://github.com/deckhouse/deckhouse/pull/21104)
 - **[deckhouse]** Check registry pagination to enable incremental scan. [#18898](https://github.com/deckhouse/deckhouse/pull/18898)
 - **[deckhouse]** Export module versions to DOP via mm_module_enabled. [#19290](https://github.com/deckhouse/deckhouse/pull/19290)
 - **[deckhouse]** Made the `settings.bundle` parameter immutable after the cluster is deployed. [#20557](https://github.com/deckhouse/deckhouse/pull/20557)
 - **[deckhouse]** Make packages nelm operation timeout configurable [#21621](https://github.com/deckhouse/deckhouse/pull/21621)
 - **[deckhouse]** Packages cleanup orphan nelm releases. [#19773](https://github.com/deckhouse/deckhouse/pull/19773)
 - **[deckhouse]** Reserved the d8a- prefix for Deckhouse application objects. [#21431](https://github.com/deckhouse/deckhouse/pull/21431)
 - **[deckhouse]** Rework D8DeckhouseQueueIsHung alert with queue head info and severity based on module criticality. [#20476](https://github.com/deckhouse/deckhouse/pull/20476)
 - **[deckhouse]** Surface scan results on PackageRepository status. [#20279](https://github.com/deckhouse/deckhouse/pull/20279)
 - **[deckhouse]** The PackageRepositoryOperation type field now uses an enum. [#19951](https://github.com/deckhouse/deckhouse/pull/19951)
 - **[deckhouse]** Webhook-handler will reload exited shell-operator now. [#19592](https://github.com/deckhouse/deckhouse/pull/19592)
 - **[descheduler]** Added a DRS-Lens Grafana dashboard showing the workload happiness score. [#21143](https://github.com/deckhouse/deckhouse/pull/21143)
 - **[dhctl]** Add global flag dhctl, improve interactive logging. [#19879](https://github.com/deckhouse/deckhouse/pull/19879)
 - **[dhctl]** Added a log area to the interactive mode of dhctl. [#20351](https://github.com/deckhouse/deckhouse/pull/20351)
 - **[dhctl]** Added a preflight check for cloud disk name length. [#20029](https://github.com/deckhouse/deckhouse/pull/20029)
 - **[dhctl]** Added basic OpenTelemetry support to dhctl. [#19738](https://github.com/deckhouse/deckhouse/pull/19738)
 - **[dhctl]** Added bootstrap support with the `Local` registry mode. [#18262](https://github.com/deckhouse/deckhouse/pull/18262)
 - **[dhctl]** Added bundle-specific system requirements to preflight validation. [#21408](https://github.com/deckhouse/deckhouse/pull/21408)
 - **[dhctl]** Added deprecation warnings in dhctl for deprecated fields in resources. [#21273](https://github.com/deckhouse/deckhouse/pull/21273)
 - **[dhctl]** Added support in `dhctl` for working with a dependency image and invoking a validator shipped with the cloud provider module. [#20236](https://github.com/deckhouse/deckhouse/pull/20236)
    Cloud provider configuration for DKP clusters deployed in DVP needs to be migrated from ProviderClusterConfiguration to ModuleConfig settings. For migration instructions, refer to "Cluster and Infrastructure" in the DKP FAQ.
 - **[dhctl]** Generate cni-* ModuleConfig from each cloud provider's candi/cni-bootstrap.yml at bootstrap, with user-override confirmation and gRPC inspection (Validation.ConfigExtender). [#20143](https://github.com/deckhouse/deckhouse/pull/20143)
 - **[dhctl]** Improved the preflight check for the deckhouse user. [#19431](https://github.com/deckhouse/deckhouse/pull/19431)
 - **[dhctl]** Logging refactoring for dhctl. [#19422](https://github.com/deckhouse/deckhouse/pull/19422)
 - **[dhctl]** Switched the default SSH implementation in dhctl to gossh. [#21256](https://github.com/deckhouse/deckhouse/pull/21256)
 - **[docs]** Added a new security events documentation page. [#19706](https://github.com/deckhouse/deckhouse/pull/19706)
 - **[docs]** Added oss.yaml files for cloud provider modules. [#18989](https://github.com/deckhouse/deckhouse/pull/18989)
 - **[istio]** Add ambient waypoint support with RBAC fixes for Gateway API resources. [#19841](https://github.com/deckhouse/deckhouse/pull/19841)
 - **[istio]** Add explicit configuration option `ambient.enabled` to manage Istio ambient mesh. [#19318](https://github.com/deckhouse/deckhouse/pull/19318)
 - **[istio]** Added Istio v1.27 OpenAPI support and operator-free compatibility hooks. [#20757](https://github.com/deckhouse/deckhouse/pull/20757)
 - **[istio]** Added a threat model for CE and EE editions. [#20435](https://github.com/deckhouse/deckhouse/pull/20435)
 - **[istio]** Added deploy dependencies for Istio webhook configurations. [#20992](https://github.com/deckhouse/deckhouse/pull/20992)
 - **[istio]** Added support for an extra root CA for JWKS fetching in Istio. [#20567](https://github.com/deckhouse/deckhouse/pull/20567)
 - **[istio]** Added support for custom annotations on the Istio Ingress Gateway controller. [#20486](https://github.com/deckhouse/deckhouse/pull/20486)
 - **[istio]** Added support for custom extension providers. [#20685](https://github.com/deckhouse/deckhouse/pull/20685)
 - **[istio]** Added the WaypointInstance CRD and waypoint-controller for Istio waypoint proxy management. [#20188](https://github.com/deckhouse/deckhouse/pull/20188)
 - **[istio]** Added the alliance-healthcheck component to monitor inter-cluster connectivity. [#19006](https://github.com/deckhouse/deckhouse/pull/19006)
 - **[istio]** Changed the API-proxy VPA update mode to avoid work interruption. [#19602](https://github.com/deckhouse/deckhouse/pull/19602)
 - **[istio]** Changed the preferred Istio default subdomain for metadata-exporter. [#19299](https://github.com/deckhouse/deckhouse/pull/19299)
    matadata-exporter pod will be restarted
 - **[istio]** Enabled HBONE support on sidecar proxies when ambient mode is enabled. [#19715](https://github.com/deckhouse/deckhouse/pull/19715)
 - **[istio]** Implemented graceful metadata secret renewal for multiclusters. [#19278](https://github.com/deckhouse/deckhouse/pull/19278)
 - **[istio]** Implemented the Istio Telemetry API. [#19636](https://github.com/deckhouse/deckhouse/pull/19636)
 - **[istio]** Modernized IngressIstioController gateway manifests and added client network topology configuration. [#21304](https://github.com/deckhouse/deckhouse/pull/21304)
    Existing IngressIstioController gateway DaemonSets are rolled out after upgrade. Default static resource requests change from 350m CPU/500Mi memory to 100m CPU/128Mi memory; default VPA bounds change to 100m–1000m CPU and 128Mi–2000Mi memory. Set explicit resourcesRequests values before upgrading if the previous sizing must be retained.
 - **[istio]** Updated the Istio module documentation for automatic data plane upgrades. [#20817](https://github.com/deckhouse/deckhouse/pull/20817)
 - **[keepalived]** Fixed securityContext and added SecurityPolicyException [#21713](https://github.com/deckhouse/deckhouse/pull/21713)
 - **[kube-proxy]** Fixed securityContext and added SecurityPolicyException [#21710](https://github.com/deckhouse/deckhouse/pull/21710)
 - **[metallb]** Added RBACv1 roles (User and ClusterAdmin) for MetalLoadBalancerClass resource. [#19627](https://github.com/deckhouse/deckhouse/pull/19627)
 - **[metallb]** Refactored BGP configuration to use dedicated CRDs (MetalLoadBalancerPool, MetalLoadBalancerBGPPeer, MetalLoadBalancerConfiguration) instead of ModuleConfig. [#18660](https://github.com/deckhouse/deckhouse/pull/18660)
    In the `metallb` module, BGP configuration was migrated from ModuleConfig to dedicated custom resources. Existing configuration is migrated automatically during the update. Before editing the generated resources, disable further migration by updating the ModuleConfig version. Afterwards, remove the obsolete BGP settings from the ModuleConfig. See the module documentation for details.
 - **[multitenancy-manager]** Cluster resource grants — control which cluster-scoped resources a project may use, set per-project defaults, and cap usage with object quotas. [#20520](https://github.com/deckhouse/deckhouse/pull/20520)
 - **[multitenancy-manager]** Cluster resource grants — split governed-resource definitions from validation/defaulting reference paths so any module can register a path; object quota delegated to Kubernetes ResourceQuota. [#20611](https://github.com/deckhouse/deckhouse/pull/20611)
 - **[multitenancy-manager]** Grant kubeadm:cluster-admins group granular full access to module CRDs via dedicated ClusterRole d8:multitenancy-manager:admin-kubeconfig. [#19420](https://github.com/deckhouse/deckhouse/pull/19420)
 - **[network-gateway]** Fixed securityContext and added SecurityPolicyException [#21712](https://github.com/deckhouse/deckhouse/pull/21712)
 - **[network-policy-engine]** Fixed securityContext and added SecurityPolicyException [#21711](https://github.com/deckhouse/deckhouse/pull/21711)
 - **[node-local-dns]** Implemented a new `nodeLocalDns` option to disable IPv6 DNS resolving. [#19282](https://github.com/deckhouse/deckhouse/pull/19282)
 - **[node-manager]** Add cluster-autoscaler 1.35 support with ported Deckhouse patches and CVE dependency bumps. [#21821](https://github.com/deckhouse/deckhouse/pull/21821)
 - **[node-manager]** Added Instance API v1alpha2 with a unified machine and bashible status model. [#18795](https://github.com/deckhouse/deckhouse/pull/18795)
 - **[node-manager]** Added a release requirement that can block updates while nodes are still running containerd v1.x. [#22015](https://github.com/deckhouse/deckhouse/pull/22015)
 - **[node-manager]** Added an e2e test for cluster-autoscaler. [#18817](https://github.com/deckhouse/deckhouse/pull/18817)
 - **[node-manager]** Moved CAPI v1beta2 management into node-controller. [#20682](https://github.com/deckhouse/deckhouse/pull/20682)
 - **[node-manager]** Removed `early-oom` component from the `node-manager` module. [#20806](https://github.com/deckhouse/deckhouse/pull/20806)
    The `early-oom` DaemonSet in the `d8-cloud-instance-manager` namespace will be removed.
 - **[registry-packages-proxy]** Added handling of deckhouse-cli artefacts to registry-packages-proxy. [#20025](https://github.com/deckhouse/deckhouse/pull/20025)
 - **[registry-packages-proxy]** Made CLI image pulls in registry-packages-proxy platform-aware. [#20795](https://github.com/deckhouse/deckhouse/pull/20795)
 - **[registry-packages-proxy]** Served the plugin contract from the image manifest in registry-packages-proxy. [#20800](https://github.com/deckhouse/deckhouse/pull/20800)
 - **[registry]** Added bootstrap support with the `Local` registry mode. [#18262](https://github.com/deckhouse/deckhouse/pull/18262)
 - **[registrypackages]** Added oss.yaml files for cloud provider modules. [#18989](https://github.com/deckhouse/deckhouse/pull/18989)
 - **[registrypackages]** Added patches for containerd 2.2.3 with integrity logic. [#19076](https://github.com/deckhouse/deckhouse/pull/19076)
 - **[registrypackages]** Added patches for containerd 2.2.4 with integrity logic. [#20298](https://github.com/deckhouse/deckhouse/pull/20298)
 - **[registrypackages]** Fixed containerd patches for the integrity feature when using a custom CA. [#20845](https://github.com/deckhouse/deckhouse/pull/20845)
 - **[service-with-healthchecks]** Added RBACv1 roles (User and Editor) for ServiceWithHealthchecks resource. [#19625](https://github.com/deckhouse/deckhouse/pull/19625)
 - **[user-authn]** Added the DexProviderCheck resource for on-demand connectivity and credential diagnostics of Dex authentication providers. [#20319](https://github.com/deckhouse/deckhouse/pull/20319)
 - **[user-authn]** Brute-force protection for Dex — per-IP rate limit on password endpoints and account lockout for LDAP/Crowd connectors. [#19542](https://github.com/deckhouse/deckhouse/pull/19542)
 - **[user-authn]** Grant kubeadm:cluster-admins group granular full access to module CRDs via dedicated ClusterRole d8:user-authn:admin-kubeconfig. [#19420](https://github.com/deckhouse/deckhouse/pull/19420)
 - **[user-authn]** UserOperation audit logs record the initiator admin's email from the deckhouse.io/initiator annotation, along with the operation type and the target user. [#20281](https://github.com/deckhouse/deckhouse/pull/20281)
 - **[user-authn]** UserOperation supports permanent user locks via spec.lock.permanent. [#20281](https://github.com/deckhouse/deckhouse/pull/20281)
 - **[user-authz]** Added alert and DKP 1.78 release requirement for custom roles of the legacy experimental RBACv2 scheme. [#21381](https://github.com/deckhouse/deckhouse/pull/21381)
    Custom roles of the legacy experimental RBACv2 scheme must be migrated to the new `d8:custom:*` scheme before upgrading to DKP 1.78. The `D8UserAuthzLegacyRBACv2CustomRoleFound` alert identifies affected roles. Refer to the `user-authz` module FAQ for migration instructions.
 - **[user-authz]** Grant kubeadm:cluster-admins group granular full access to module CRDs via dedicated ClusterRole d8:user-authz:admin-kubeconfig. [#19420](https://github.com/deckhouse/deckhouse/pull/19420)
 - **[vertical-pod-autoscaler]** Upgrade Vertical Pod Autoscaler to v1.7.0. [#21387](https://github.com/deckhouse/deckhouse/pull/21387)
    Vertical Pod Autoscaler components will be restarted.

## Fixes


 - **[admission-policy-engine]** Bumped ratify to 1.4.1 and fixed DMT allowing multiple oss.yaml files. [#20797](https://github.com/deckhouse/deckhouse/pull/20797)
 - **[admission-policy-engine]** Changed the default PSS policy to Baseline for unrecognized Deckhouse versions. [#19663](https://github.com/deckhouse/deckhouse/pull/19663)
 - **[admission-policy-engine]** Changed the label for container SecurityPolicyExceptions. [#20678](https://github.com/deckhouse/deckhouse/pull/20678)
 - **[admission-policy-engine]** Fixed the constraint-template check for the AssignImage CRD. [#20782](https://github.com/deckhouse/deckhouse/pull/20782)
 - **[admission-policy-engine]** Made Gatekeeper pods tolerate the csi-not-bootstrapped taint to prevent webhook deadlock during worker node replacement. [#19383](https://github.com/deckhouse/deckhouse/pull/19383)
 - **[admission-policy-engine]** Updated Gatekeeper to 3.22.2 to fix CVEs. [#20454](https://github.com/deckhouse/deckhouse/pull/20454)
 - **[admission-policy-engine]** fix cve [#21986](https://github.com/deckhouse/deckhouse/pull/21986)
 - **[candi]** Changed the automatic Kubernetes version from 1.33 to 1.34. [#20572](https://github.com/deckhouse/deckhouse/pull/20572)
    Automatic k8s version is changed to 1.34 from 1.33, automatic setting will result in upgrade.
 - **[candi]** Enable the DRADeviceTaints feature gate by default for Kubernetes 1.33–1.35. [#21745](https://github.com/deckhouse/deckhouse/pull/21745)
 - **[candi]** Fixed 01-bootstrap-prerequisites.sh. [#20367](https://github.com/deckhouse/deckhouse/pull/20367)
 - **[candi]** Fixed containerd credential escaping by rendering username and password into the auth string. [#20589](https://github.com/deckhouse/deckhouse/pull/20589)
 - **[candi]** Fixed static node cleanup to wipe data on externally mounted volumes before unmounting, preventing stale data from causing re-bootstrap failures. [#20066](https://github.com/deckhouse/deckhouse/pull/20066)
 - **[candi]** Install kubernetes-cni 1.9.1 on nodes. [#21963](https://github.com/deckhouse/deckhouse/pull/21963)
 - **[candi]** Made the `005_integrate_kubernetes_data_device.sh.tpl` step idempotent. [#21168](https://github.com/deckhouse/deckhouse/pull/21168)
 - **[candi]** Removed the Python requirement from bashible bootstrap and switched Registry Packages Proxy package installation to static binaries. [#18626](https://github.com/deckhouse/deckhouse/pull/18626)
 - **[candi]** Retried kube API errors in rpp-get during registry packages discovery. [#19673](https://github.com/deckhouse/deckhouse/pull/19673)
 - **[candi]** Revert feature gate DRADeviceTaints [#21726](https://github.com/deckhouse/deckhouse/pull/21726)
 - **[candi]** Used a short timeout for deleting MirrorPods. [#20565](https://github.com/deckhouse/deckhouse/pull/20565)
 - **[candi]** kube-apiserver no longer caches watches for `ManifestCheckpointContentChunk` resources from `state-snapshotter`. [#21208](https://github.com/deckhouse/deckhouse/pull/21208)
    kube-apiserver static pod is reconfigured and restarts on the next control-plane sync.
 - **[cert-manager]** fix cve [#21986](https://github.com/deckhouse/deckhouse/pull/21986)
 - **[cilium-hubble]** Bump the cel-go dependency in the hubble-ui backend to fix known CVEs. [#21872](https://github.com/deckhouse/deckhouse/pull/21872)
    The cilium-hubble components (hubble-ui, hubble-relay) will restart after the update.
 - **[cilium-hubble]** Fixed CVE-2026-29181 in hubble-ui-backend by bumping OpenTelemetry Go to v1.41.0. [#20087](https://github.com/deckhouse/deckhouse/pull/20087)
 - **[cilium-hubble]** Fixed CVE-2026-41520 in hubble-ui-backend. [#20359](https://github.com/deckhouse/deckhouse/pull/20359)
 - **[cloud-provider-aws]** Added a new Bashible step to install `linux-modules-extra` on Ubuntu nodes. [#19415](https://github.com/deckhouse/deckhouse/pull/19415)
 - **[cloud-provider-aws]** Adds SecurityPolicyException for CCM and CSI components, fixes VPA and PodMonitor module guards, migrates hand-written templates to helm_lib helpers. [#21397](https://github.com/deckhouse/deckhouse/pull/21397)
 - **[cloud-provider-aws]** Bumped Go module dependencies to fix known CVEs in terraform-manager, cloud-data-discoverer; documented GO-2026-5932 vulnerability via VEX. [#20780](https://github.com/deckhouse/deckhouse/pull/20780)
 - **[cloud-provider-aws]** Bumped helm_lib with liveness probe parameters for the CSI controller. [#19694](https://github.com/deckhouse/deckhouse/pull/19694)
 - **[cloud-provider-aws]** Do not fail the node bootstrap or update when `linux-modules-extra` is unavailable for the running kernel. [#21955](https://github.com/deckhouse/deckhouse/pull/21955)
 - **[cloud-provider-azure]** Bumped Go module dependencies to fix known CVEs in azuredisk-csi, cloud-data-discoverer. [#20780](https://github.com/deckhouse/deckhouse/pull/20780)
 - **[cloud-provider-azure]** Bumped helm_lib with liveness probe parameters for the CSI controller. [#19694](https://github.com/deckhouse/deckhouse/pull/19694)
 - **[cloud-provider-dvp]** Add skip storage class annotation handling to skip discovery of some storage classes from parent clusters, e.g., local disks. [#19696](https://github.com/deckhouse/deckhouse/pull/19696)
 - **[cloud-provider-dvp]** Added mount points for the validation-webhook image so serving certificates persist correctly. [#21526](https://github.com/deckhouse/deckhouse/pull/21526)
 - **[cloud-provider-dvp]** Added the d8_cloud_provider_dvp_migration_pending metric for the D8CloudProviderDVPMigrationPending alert. [#21409](https://github.com/deckhouse/deckhouse/pull/21409)
 - **[cloud-provider-dvp]** Always use WaitForFirstConsumer for child-cluster DVP StorageClasses and recreate incompatible managed classes during upgrade. [#20116](https://github.com/deckhouse/deckhouse/pull/20116)
 - **[cloud-provider-dvp]** Bumped Go module dependencies to fix known CVEs in cloud-controller-manager, dvp-csi, capdvp, cloud-data-discoverer. [#20780](https://github.com/deckhouse/deckhouse/pull/20780)
 - **[cloud-provider-dvp]** Bumped helm_lib with liveness probe parameters for the CSI controller. [#19694](https://github.com/deckhouse/deckhouse/pull/19694)
 - **[cloud-provider-dvp]** Fixed CVEs in the cloud-provider-dvp module. [#20796](https://github.com/deckhouse/deckhouse/pull/20796)
 - **[cloud-provider-dvp]** Fixed LoadBalancer stuck in pending by retrying on conflict when updating ServiceWithHealthchecks and propagating IP to the child cluster service status. [#19590](https://github.com/deckhouse/deckhouse/pull/19590)
 - **[cloud-provider-dvp]** Fixed ModuleConfig access for users with the d8:manage:infrastructure:viewer and d8:manage:infrastructure:manager roles. [#21487](https://github.com/deckhouse/deckhouse/pull/21487)
 - **[cloud-provider-dvp]** Fixed duplicate volume mounts in the DVP CSI driver on kubelet retries. [#20714](https://github.com/deckhouse/deckhouse/pull/20714)
 - **[cloud-provider-dvp]** Made cloud-init secrets immutable to prevent post-bootstrap data mutation. [#20930](https://github.com/deckhouse/deckhouse/pull/20930)
 - **[cloud-provider-dvp]** Made the DVP CSI driver wait for VirtualDisk readiness in CreateVolume so provisioning errors are propagated instead of marking the PV as Bound. [#20566](https://github.com/deckhouse/deckhouse/pull/20566)
 - **[cloud-provider-dvp]** Restored correct service-lb-controller enablement in cloud-provider-dvp. [#20177](https://github.com/deckhouse/deckhouse/pull/20177)
 - **[cloud-provider-dvp]** Stopped setting empty `dvp.deckhouse.io/cluster-uuid` and `dvp.deckhouse.io/hostname` labels. [#21110](https://github.com/deckhouse/deckhouse/pull/21110)
 - **[cloud-provider-dvp]** Treated a missing VMBDA as success on disk detach. [#21266](https://github.com/deckhouse/deckhouse/pull/21266)
 - **[cloud-provider-dvp]** add labels to cloudinit secrets in the terraform [#20321](https://github.com/deckhouse/deckhouse/pull/20321)
 - **[cloud-provider-dvp]** fix CSI vmBDA retry loop that caused x1580 retries and blocked new PVC mounts on static→dynamic node migration [#20145](https://github.com/deckhouse/deckhouse/pull/20145)
 - **[cloud-provider-dvp]** fix dvp kubernetes dependency mismatch [#21367](https://github.com/deckhouse/deckhouse/pull/21367)
 - **[cloud-provider-dvp]** skip foreign nodes in cloud controller manager [#21780](https://github.com/deckhouse/deckhouse/pull/21780)
 - **[cloud-provider-dynamix]** Bumped helm_lib with liveness probe parameters for the CSI controller. [#19694](https://github.com/deckhouse/deckhouse/pull/19694)
 - **[cloud-provider-gcp]** Bumped Go module dependencies to fix known CVEs in terraform-manager, cloud-controller-manager, cloud-data-discoverer; documented GO-2026-5932 vulnerability via VEX. [#20780](https://github.com/deckhouse/deckhouse/pull/20780)
 - **[cloud-provider-gcp]** Bumped helm_lib with liveness probe parameters for the CSI controller. [#19694](https://github.com/deckhouse/deckhouse/pull/19694)
 - **[cloud-provider-huaweicloud]** Adds patches to the upstream version to make it ignore static nodes [#21351](https://github.com/deckhouse/deckhouse/pull/21351)
 - **[cloud-provider-huaweicloud]** Bumped helm_lib with liveness probe parameters for the CSI controller. [#19694](https://github.com/deckhouse/deckhouse/pull/19694)
 - **[cloud-provider-huaweicloud]** Recreated the etcd disk when a master VM is recreated. [#20801](https://github.com/deckhouse/deckhouse/pull/20801)
 - **[cloud-provider-huaweicloud]** Separated HuaweiCloudMachine VM lookup logic for create/delete and marked machines stuck in Deleting for more than 24h. [#19375](https://github.com/deckhouse/deckhouse/pull/19375)
 - **[cloud-provider-openstack]** Add the missing with-uninitialized toleration strategy for capo-controller-manager. [#21706](https://github.com/deckhouse/deckhouse/pull/21706)
 - **[cloud-provider-openstack]** Bumped Go module dependencies to fix known CVEs in terraform-manager, cloud-controller-manager, cinder-csi-plugin, cloud-data-discoverer; documented GO-2026-5932, CVE-2026-41579 vulnerabilities via VEX. [#20780](https://github.com/deckhouse/deckhouse/pull/20780)
 - **[cloud-provider-openstack]** Bumped helm_lib with liveness probe parameters for the CSI controller. [#19694](https://github.com/deckhouse/deckhouse/pull/19694)
 - **[cloud-provider-openstack]** Fixed a permanent cinder-csi-plugin failure after re-authentication in long-running pods. [#20394](https://github.com/deckhouse/deckhouse/pull/20394)
 - **[cloud-provider-openstack]** Prevented the OpenStack CCM from deleting Kubernetes nodes when OpenStack temporarily reports an instance as not found. [#20743](https://github.com/deckhouse/deckhouse/pull/20743)
 - **[cloud-provider-vcd]** Added werf deploy-dependency annotations to capcd webhook configurations to enforce correct deploy ordering. [#20987](https://github.com/deckhouse/deckhouse/pull/20987)
 - **[cloud-provider-vcd]** Bumped Go module dependencies to fix known CVEs in infra-controller-manager, cloud-data-discoverer. [#20780](https://github.com/deckhouse/deckhouse/pull/20780)
 - **[cloud-provider-vcd]** Bumped helm_lib with liveness probe parameters for the CSI controller. [#19694](https://github.com/deckhouse/deckhouse/pull/19694)
 - **[cloud-provider-vcd]** Fix LogrAdapter panic in VCD infra-controller-manager [#20107](https://github.com/deckhouse/deckhouse/pull/20107)
 - **[cloud-provider-vcd]** Fixed a malformed CCM --controllers argument and restored legacy controller names for VCD Legacy. [#21261](https://github.com/deckhouse/deckhouse/pull/21261)
 - **[cloud-provider-vcd]** Fixed the SecurityPolicyException in CAPCD. [#19539](https://github.com/deckhouse/deckhouse/pull/19539)
 - **[cloud-provider-vcd]** Fixes infra-controller-manager pods scheduling on the same node in HA mode. [#20752](https://github.com/deckhouse/deckhouse/pull/20752)
 - **[cloud-provider-vsphere]** Bumped Go module dependencies to fix known CVEs in terraform-manager, cloud-data-discoverer. [#20780](https://github.com/deckhouse/deckhouse/pull/20780)
 - **[cloud-provider-vsphere]** Bumped helm_lib with liveness probe parameters for the CSI controller. [#19694](https://github.com/deckhouse/deckhouse/pull/19694)
 - **[cloud-provider-vsphere]** Upgraded CSI plugin versions and fixed CVEs in cloud-provider-vsphere. [#18159](https://github.com/deckhouse/deckhouse/pull/18159)
 - **[cloud-provider-vsphere]** fix missing default StorageClass annotation when a DatastoreCluster entry sorts before all Datastore entries [#21111](https://github.com/deckhouse/deckhouse/pull/21111)
 - **[cloud-provider-vsphere]** normalizes new paths and makes bashible resolve existing paths case-insensitively [#19653](https://github.com/deckhouse/deckhouse/pull/19653)
 - **[cloud-provider-yandex]** Bumped Go module dependencies to fix known CVEs in cloud-metrics-exporter, cloud-migrator, cloud-data-discoverer. [#20780](https://github.com/deckhouse/deckhouse/pull/20780)
 - **[cloud-provider-yandex]** Bumped helm_lib with liveness probe parameters for the CSI controller. [#19694](https://github.com/deckhouse/deckhouse/pull/19694)
 - **[cloud-provider-yandex]** Fixed the volume-expansion-mode annotation so Yandex CSI supports online PVC resize. [#20578](https://github.com/deckhouse/deckhouse/pull/20578)
 - **[cloud-provider-zvirt]** Bumped Go module dependencies to fix known CVEs in cloud-controller-manager, capz, cloud-data-discoverer. [#20780](https://github.com/deckhouse/deckhouse/pull/20780)
 - **[cloud-provider-zvirt]** Bumped helm_lib with liveness probe parameters for the CSI controller. [#19694](https://github.com/deckhouse/deckhouse/pull/19694)
 - **[cni-cilium]** Bump cel-go and gopacket dependencies to fix known CVEs. [#21872](https://github.com/deckhouse/deckhouse/pull/21872)
    The cni-cilium components (cilium agent, operator) will restart after the update.
 - **[cni-cilium]** Bumped Go dependencies in the egress-gateway-agent image to fix known CVEs. [#21558](https://github.com/deckhouse/deckhouse/pull/21558)
    The `egress-gateway-agent` component will be restarted.
 - **[cni-cilium]** Changed default cilium-agent health check port from 9876 to 9879 to avoid conflict with Istio's ControlZ port. [#20348](https://github.com/deckhouse/deckhouse/pull/20348)
 - **[cni-cilium]** Fixed CVE-2026-41520 for the cilium-bugtool util. [#20067](https://github.com/deckhouse/deckhouse/pull/20067)
 - **[cni-cilium]** Fixed Cilium agent CPU overload on nodes with many CPUs. [#19903](https://github.com/deckhouse/deckhouse/pull/19903)
 - **[cni-cilium]** Fixed infinite reconciliation of EgressGateway objects and improved status reporting. [#19219](https://github.com/deckhouse/deckhouse/pull/19219)
 - **[cni-cilium]** Updated Cilium from 1.17.4 to 1.17.17. [#20720](https://github.com/deckhouse/deckhouse/pull/20720)
    The `cni-cilium` components (cilium-agent, operator) will be restarted. A change has been made to the Network Policy that affects the behavior of the `ipBlock: 0.0.0.0/0` rule.
 - **[cni-flannel]** Reverted module stage from Deprecated back to General Availability to stop false deprecation alerts. [#20294](https://github.com/deckhouse/deckhouse/pull/20294)
 - **[cni-simple-bridge]** Fix simple bridge script to add ip rule for two NICs nodes. [#20428](https://github.com/deckhouse/deckhouse/pull/20428)
 - **[common]** Added checks for the `observability` module in `upmeter`. [#18111](https://github.com/deckhouse/deckhouse/pull/18111)
 - **[common]** Added support for `op_*` PromQL functions to the `operator-prometheus` parser. [#20254](https://github.com/deckhouse/deckhouse/pull/20254)
 - **[common]** Enforced permanent use of the default CA bundle in `image-availability-exporter` in the `extended-monitoring` module. [#20175](https://github.com/deckhouse/deckhouse/pull/20175)
 - **[common]** Fixed CVE-2026-29181 in CoreDNS. [#20038](https://github.com/deckhouse/deckhouse/pull/20038)
 - **[common]** Fixed CVE-2026-40898 in CoreDNS by updating the quic-go dependency. [#20736](https://github.com/deckhouse/deckhouse/pull/20736)
 - **[common]** Fixed CVEs in the `events-exporter`, `extended-monitoring-exporter`, `image-availability-exporter`, and `x509-certificate-exporter` images. [#20395](https://github.com/deckhouse/deckhouse/pull/20395)
    The corresponding exporter Pods will be restarted.
 - **[common]** Fixed CVEs in the `k8s-prometheus-adapter` and `prometheus-reverse-proxy` images by bumping grpc to v1.79.3 and otel/sdk to v1.43.0. [#20395](https://github.com/deckhouse/deckhouse/pull/20395)
    The `prometheus-metrics-adapter` Pods will be restarted.
 - **[common]** Fixed CVEs in the `kube-state-metrics`, `node-exporter`, `oom-kills-exporter`, and `kubelet-eviction-thresholds-exporter` images. [#20395](https://github.com/deckhouse/deckhouse/pull/20395)
    The corresponding monitoring Pods will be restarted.
 - **[common]** Fixed CVEs in the `loki` image by updating Go dependencies. [#20395](https://github.com/deckhouse/deckhouse/pull/20395)
    The `loki` Pod will be restarted.
 - **[common]** Fixed CVEs in the `monitoring-ping` image by updating Go dependencies. [#20395](https://github.com/deckhouse/deckhouse/pull/20395)
    The `monitoring-ping` Pods will be restarted.
 - **[common]** Fixed CVEs in the `prometheus-operator` image by rebuilding on the golang builder and updating Go dependencies. [#20395](https://github.com/deckhouse/deckhouse/pull/20395)
    The `prometheus-operator` Pod will be restarted.
 - **[common]** Fixed CVEs in the `pushgateway` image by bumping it from 1.11.1 to 1.11.3. [#20395](https://github.com/deckhouse/deckhouse/pull/20395)
    The `pushgateway` Pod will be restarted.
 - **[common]** Fixed CVEs in the `upmeter` and `smoke-mini` images by updating Go dependencies. [#20395](https://github.com/deckhouse/deckhouse/pull/20395)
    The `upmeter` and `smoke-mini` Pods will be restarted.
 - **[common]** Fixed CVEs in the vector reloader image by updating Go dependencies. [#20395](https://github.com/deckhouse/deckhouse/pull/20395)
    The `log-shipper` Pods will be restarted.
 - **[common]** Fixed IngressNginxController HPA CPU metric calculation to use per-pod CPU and smoother averaging in `ingress-nginx`. [#20426](https://github.com/deckhouse/deckhouse/pull/20426)
    HPA for LoadBalancer-based IngressNginxController may scale more accurately under real load and react less noisily due to the metric window change from 1m to 3m.
 - **[common]** Fixed a nil pointer panic in `upmeter` on shutdown when server initialization is incomplete. [#19554](https://github.com/deckhouse/deckhouse/pull/19554)
 - **[common]** Fixed activating modsecurity CRS rules when the Ingress NGINX Controller is running with IsolatedProcess validation. [#20155](https://github.com/deckhouse/deckhouse/pull/20155)
    All Ingress NGINX Controller pods will be restarted.
 - **[common]** Fixed invalid PromQL expression in the `D8UpmeterProbeGarbagePodsFromDeployments` alert of `upmeter`. [#19571](https://github.com/deckhouse/deckhouse/pull/19571)
 - **[common]** Fixed mount points in a `geoproxy` image in `ingress-nginx`. [#20099](https://github.com/deckhouse/deckhouse/pull/20099)
    Geoproxy image will be restarted.
 - **[common]** Fixed the StaticInstance preflight check failing when SSHCredentials has no private key. [#19527](https://github.com/deckhouse/deckhouse/pull/19527)
 - **[common]** Fixed the `D8UpmeterProbeGarbagePodsFromDeployments` alert to eliminate false positives. [#19382](https://github.com/deckhouse/deckhouse/pull/19382)
 - **[common]** Hide inaccessible Projects. [#22077](https://github.com/deckhouse/deckhouse/pull/22077)
    Applying this change restarts control-plane components.
 - **[common]** Improved isolated validation for Ingress NGINX Controller 1.14 and 1.15. [#19543](https://github.com/deckhouse/deckhouse/pull/19543)
    controller pods with version 1.14 and 1.15 will be restarted.
 - **[common]** Normalized the kernel version from uname to semver for non-standard Linux release names. [#19329](https://github.com/deckhouse/deckhouse/pull/19329)
    Nodes with Debian 13 kernels (e.g. 6.12.74+deb13+1-cloud-amd64) previously failed the kernel version check and could not join the cluster.
 - **[common]** Quoted all variable interpolations in YAML manifests rendered by the `upmeter` observability-rules-group-recording and observability-rules-group-alert probes. [#20217](https://github.com/deckhouse/deckhouse/pull/20217)
 - **[common]** Restricted the system scope so `d8 k get` with `--scope=system` returns only `default`, `d8-*`, and `kube-*` namespaces. [#21532](https://github.com/deckhouse/deckhouse/pull/21532)
    kube-apiserver image is rebuilt; control-plane components restart during the update.
 - **[common]** Switched smoke-mini checks to full service FQDN to reduce unnecessary requests and added request/session timeouts to prevent hanging probe calls. [#19310](https://github.com/deckhouse/deckhouse/pull/19310)
    upmeter probes
 - **[common]** Updated NGINX to 1.30.1 in the `ingress-nginx` module. [#19846](https://github.com/deckhouse/deckhouse/pull/19846)
    All Ingress NGINX Controller pods will be restarted.
 - **[common]** Updated NGINX to 1.30.2 in `ingress-nginx`. [#20173](https://github.com/deckhouse/deckhouse/pull/20173)
    All Ingress NGINX Controller pods will be restarted.
 - **[common]** fix cve [#21986](https://github.com/deckhouse/deckhouse/pull/21986)
 - **[control-plane-manager]** Added an audit policy rule to log kubectl get logs requests (pods/log). [#20274](https://github.com/deckhouse/deckhouse/pull/20274)
 - **[control-plane-manager]** Allow change labels and annotations for secret d8-secret-encryption-key [#20103](https://github.com/deckhouse/deckhouse/pull/20103)
 - **[control-plane-manager]** Allowed admins to manage ModuleReleases, ModuleDocumentations, and ModuleSettingsDefinitions. [#19931](https://github.com/deckhouse/deckhouse/pull/19931)
 - **[control-plane-manager]** Bumped vulnerable Go dependencies in control-plane components to close known CVEs. [#21417](https://github.com/deckhouse/deckhouse/pull/21417)
 - **[control-plane-manager]** Fix dhctl bootstrap failing on CSE clusters with 'apiserver.signature Enforce' on control-plane-manager moduleConfig [#21962](https://github.com/deckhouse/deckhouse/pull/21962)
 - **[control-plane-manager]** Fixed Helm adoption of `kubeadm:cluster-admins` ClusterRoleBinding on upgrade from a DKP version prior to 1.76. [#20877](https://github.com/deckhouse/deckhouse/pull/20877)
    If your cluster was previously upgraded to DKP 1.76.0 from a version earlier than 1.76 and was not patched, `kubectl logs` and `kubectl exec` may fail cluster-wide for all users. Apply the manual workaround from the [PR description](https://github.com/deckhouse/deckhouse/pull/20877) before upgrading.
 - **[control-plane-manager]** Fixed ModuleConfig deletion in control-plane-manager. [#21094](https://github.com/deckhouse/deckhouse/pull/21094)
 - **[control-plane-manager]** Fixed a deadlock where a periodic etcd defragmentation operation could hold the etcd operation slot forever. [#21353](https://github.com/deckhouse/deckhouse/pull/21353)
 - **[control-plane-manager]** Fixed an error in the root certificate validation function. [#20507](https://github.com/deckhouse/deckhouse/pull/20507)
 - **[control-plane-manager]** Fixed the publishAPI migration for clusters with legacy user-authn v1 settings. [#19921](https://github.com/deckhouse/deckhouse/pull/19921)
 - **[control-plane-manager]** Improved recovery and convergence of interrupted etcd member joins. [#21467](https://github.com/deckhouse/deckhouse/pull/21467)
    New and recreated control-plane nodes now retry partially completed etcd joins and learner promotion automatically. Existing healthy etcd members are unaffected.
 - **[control-plane-manager]** Removed upmeter from the AUDIT-RULES file. [#21169](https://github.com/deckhouse/deckhouse/pull/21169)
    go generation tests
 - **[control-plane-manager]** Skip rebind of ClusterRoleBinding/kubeadm:cluster-admins until the cluster is fully bootstrapped; harden the reconciliation hook. Fixes "cannot change roleRef" on fresh clusters. [#19667](https://github.com/deckhouse/deckhouse/pull/19667)
 - **[control-plane-manager]** Update Go dependencies of `control-plane-manager` and `etcd` images and record VEX statements for `GO-2026-5932`. [#22152](https://github.com/deckhouse/deckhouse/pull/22152)
 - **[csi-vsphere]** Migrated storage policy support  from the `cloud-provider-vsphere` module. [#19965](https://github.com/deckhouse/deckhouse/pull/19965)
 - **[deckhouse-controller]** A module that conditionally depends on another is no longer disabled when an incompatible version of that dependency is enabled; the enable is rejected instead. [#20334](https://github.com/deckhouse/deckhouse/pull/20334)
 - **[deckhouse-controller]** Fixed ModuleConfig validation. [#21257](https://github.com/deckhouse/deckhouse/pull/21257)
 - **[deckhouse-controller]** Fixed a SIGSEGV on a nil HookController when module hook registration is retried after a transient failure. [#21201](https://github.com/deckhouse/deckhouse/pull/21201)
 - **[deckhouse-controller]** Fixed an applications charts rendering issue. [#20260](https://github.com/deckhouse/deckhouse/pull/20260)
 - **[deckhouse-controller]** Fixed showing warnings while errors during kubectl edit. [#21264](https://github.com/deckhouse/deckhouse/pull/21264)
 - **[deckhouse-controller]** Fixed validation for switching ClusterConfiguration kubernetesVersion from an explicit version to Automatic. [#20323](https://github.com/deckhouse/deckhouse/pull/20323)
 - **[deckhouse-controller]** Module releases rendered with nelm no longer raise false absent-resource alerts. [#21834](https://github.com/deckhouse/deckhouse/pull/21834)
 - **[deckhouse-controller]** ModuleDocumentation will not be created for embedded modules. [#21682](https://github.com/deckhouse/deckhouse/pull/21682)
 - **[deckhouse-controller]** add werf dependency to webhook [#20968](https://github.com/deckhouse/deckhouse/pull/20968)
 - **[deckhouse-controller]** nelm now takes over fields left by the old Helm 3 engine, so stale fields get removed on module upgrade. [#21834](https://github.com/deckhouse/deckhouse/pull/21834)
 - **[deckhouse]** Add verbosity to veritysetup format and delete damaged hash. [#22128](https://github.com/deckhouse/deckhouse/pull/22128)
 - **[deckhouse]** Added admission policy that denies non-system users from setting or changing the `heritage` label. [#19668](https://github.com/deckhouse/deckhouse/pull/19668)
    Non-system users can no longer add or modify the `heritage` label; this prevents accidental creation of user-managed resources that become protected by Deckhouse heritage restrictions.
 - **[deckhouse]** An unset `settings.update.mode` now defaults to `AutoPatch` instead of silently running as `Auto`. [#21926](https://github.com/deckhouse/deckhouse/pull/21926)
 - **[deckhouse]** Bumped addon-operator to fix module installation with conversion webhooks. [#21366](https://github.com/deckhouse/deckhouse/pull/21366)
 - **[deckhouse]** Cleaned availableRepositories on PackageRepository deletion. [#19973](https://github.com/deckhouse/deckhouse/pull/19973)
 - **[deckhouse]** Deleted module documentation when a module is disabled. [#20434](https://github.com/deckhouse/deckhouse/pull/20434)
 - **[deckhouse]** Drive the webhook-handler hook reload's kubernetes-binding enable to completion with retry so validating webhooks are not left denying requests due to empty snapshots. [#21247](https://github.com/deckhouse/deckhouse/pull/21247)
 - **[deckhouse]** Fix a minor Deckhouse release being auto-applied as a patch when no Deployed release object is present. [#21984](https://github.com/deckhouse/deckhouse/pull/21984)
 - **[deckhouse]** Fix exp modules auto enabling. [#19670](https://github.com/deckhouse/deckhouse/pull/19670)
 - **[deckhouse]** Fix package status deadlock via coalescing workqueue. [#20676](https://github.com/deckhouse/deckhouse/pull/20676)
 - **[deckhouse]** Fixed ModulePullOverride for bundle-enabled modules. [#20763](https://github.com/deckhouse/deckhouse/pull/20763)
 - **[deckhouse]** Fixed Scaled stuck in Unknown on controller startup. [#20169](https://github.com/deckhouse/deckhouse/pull/20169)
 - **[deckhouse]** Fixed image extraction to respect opaque whiteouts. [#20502](https://github.com/deckhouse/deckhouse/pull/20502)
 - **[deckhouse]** Made module installation atomic and re-download incomplete versions. [#21462](https://github.com/deckhouse/deckhouse/pull/21462)
 - **[deckhouse]** Made validation webhooks return the Invalid reason. [#19904](https://github.com/deckhouse/deckhouse/pull/19904)
 - **[deckhouse]** Prevent NELM resource monitor reflector retries and handle resource namespaces correctly. [#22099](https://github.com/deckhouse/deckhouse/pull/22099)
 - **[deckhouse]** Re-enable kubernetes bindings in webhook-handler reload so hooks using `includeSnapshotsFrom` get populated snapshots. [#20644](https://github.com/deckhouse/deckhouse/pull/20644)
 - **[deckhouse]** Recover from panics in nelm client. [#19808](https://github.com/deckhouse/deckhouse/pull/19808)
 - **[deckhouse]** Restore ModuleIsInMaintenanceMode alert by switching to d8_module_config_maintenance sourced from ModuleConfig. [#19352](https://github.com/deckhouse/deckhouse/pull/19352)
 - **[deckhouse]** Revoke permission to use moduleconfig to user. [#19672](https://github.com/deckhouse/deckhouse/pull/19672)
 - **[deckhouse]** Used non-controller ownerRefs for multi-source package CRs. [#20045](https://github.com/deckhouse/deckhouse/pull/20045)
 - **[dhctl]** Added an explicit error when the discovered node IP is empty. [#21356](https://github.com/deckhouse/deckhouse/pull/21356)
 - **[dhctl]** Added reverse tunnel reachability checks. [#20609](https://github.com/deckhouse/deckhouse/pull/20609)
 - **[dhctl]** Added support for CAPI v1beta2 when deleting clusters and machines on destroy. [#21134](https://github.com/deckhouse/deckhouse/pull/21134)
 - **[dhctl]** Added verbose logging to bootstrap-phase subcommands in dhctl. [#21301](https://github.com/deckhouse/deckhouse/pull/21301)
 - **[dhctl]** Apply all deckhouse prerequisites (registry pull secret, RBAC, cluster-configuration secrets, d8-cluster-uuid, ...) before the deckhouse Deployment during bootstrap, so the controller never starts ahead of resources it and its hooks read. [#21132](https://github.com/deckhouse/deckhouse/pull/21132)
 - **[dhctl]** Bootstrap creates the CloudPermanent nodes and the additional masters for providers configured through ModuleConfig. [#21951](https://github.com/deckhouse/deckhouse/pull/21951)
 - **[dhctl]** Bound the registry packages proxy to an OS-assigned local port to avoid collisions during parallel bootstrap. [#21042](https://github.com/deckhouse/deckhouse/pull/21042)
 - **[dhctl]** Fix dhctl converge-migration interactive run. [#19923](https://github.com/deckhouse/deckhouse/pull/19923)
 - **[dhctl]** Fix panic in in-cluster converge-migration run of dhctl. [#19823](https://github.com/deckhouse/deckhouse/pull/19823)
 - **[dhctl]** Fix static preflights for dhctl run outside an install container. [#19809](https://github.com/deckhouse/deckhouse/pull/19809)
 - **[dhctl]** Fix the static-instances-ssh-access preflight check failing with "read-only file system" when the installer runs in a pod. [#21996](https://github.com/deckhouse/deckhouse/pull/21996)
 - **[dhctl]** Fixed SSH cleanup in dhctl. [#20301](https://github.com/deckhouse/deckhouse/pull/20301)
 - **[dhctl]** Fixed StaticInstance SSH credential preflight checks in dhctl. [#21460](https://github.com/deckhouse/deckhouse/pull/21460)
 - **[dhctl]** Fixed a converge-migration failure when no nodes need deletion. [#19835](https://github.com/deckhouse/deckhouse/pull/19835)
 - **[dhctl]** Fixed a gRPC stream cancel deadlock in dhctl for Commander. [#20998](https://github.com/deckhouse/deckhouse/pull/20998)
 - **[dhctl]** Fixed a panic caused by nil dereference in the dhctl destroy command. [#19716](https://github.com/deckhouse/deckhouse/pull/19716)
 - **[dhctl]** Fixed a panic in Commander Attach options. [#20894](https://github.com/deckhouse/deckhouse/pull/20894)
 - **[dhctl]** Fixed a panic in static preflight. [#20392](https://github.com/deckhouse/deckhouse/pull/20392)
 - **[dhctl]** Fixed a panic in the dhctl converge command. [#19753](https://github.com/deckhouse/deckhouse/pull/19753)
 - **[dhctl]** Fixed bootstrap in the `Local` registry mode. [#20516](https://github.com/deckhouse/deckhouse/pull/20516)
 - **[dhctl]** Fixed confirmation in dhctl. [#19917](https://github.com/deckhouse/deckhouse/pull/19917)
 - **[dhctl]** Fixed progress bar behavior in dhctl. [#20219](https://github.com/deckhouse/deckhouse/pull/20219)
 - **[dhctl]** Fixed the `pkill d8 k proxy` command in dhctl. [#20466](https://github.com/deckhouse/deckhouse/pull/20466)
 - **[dhctl]** Fixed the lock release command failing in an interactive terminal. [#20097](https://github.com/deckhouse/deckhouse/pull/20097)
 - **[dhctl]** Fixed the operation of `SSHProviderInitializer.Cleanup` for bootstrap commands. [#20096](https://github.com/deckhouse/deckhouse/pull/20096)
 - **[dhctl]** Omitted an error return when the static config is missing. [#20405](https://github.com/deckhouse/deckhouse/pull/20405)
 - **[dhctl]** Preflight checks are no longer re-run on every dhctl-server restart when the cluster config is unchanged. [#21108](https://github.com/deckhouse/deckhouse/pull/21108)
 - **[dhctl]** Prepared the control-plane template config for migration from ClusterConfiguration to ModuleConfig. [#19826](https://github.com/deckhouse/deckhouse/pull/19826)
 - **[dhctl]** Pull the external provider bundle from the upstream registry when dhctl runs outside the cluster, so manual converge/destroy no longer fails on the unreachable in-cluster registry mirror. [#21509](https://github.com/deckhouse/deckhouse/pull/21509)
 - **[dhctl]** Read external provider settings from its own bundle, so check and converge no longer fail on a download directory reused after bootstrap. [#21396](https://github.com/deckhouse/deckhouse/pull/21396)
 - **[dhctl]** Refuse to create the legacy provider Secret when editing the config of a ModuleConfig-driven cluster. [#21396](https://github.com/deckhouse/deckhouse/pull/21396)
 - **[dhctl]** Replaced app package references with the options package in multiple files. [#19702](https://github.com/deckhouse/deckhouse/pull/19702)
 - **[dhctl]** Wait for stronghold cluster sync before node deletion [#19643](https://github.com/deckhouse/deckhouse/pull/19643)
 - **[docs]** Add info about kernel requirement for containerdv2 migration. [#19437](https://github.com/deckhouse/deckhouse/pull/19437)
 - **[docs]** Change security events docs [#20993](https://github.com/deckhouse/deckhouse/pull/20993)
 - **[documentation]** Fix a memory leak in the documentation builder so memory is released between builds; add CPU/memory limits and an opt-in profiling endpoint. [#21934](https://github.com/deckhouse/deckhouse/pull/21934)
 - **[istio]** Add missing tools to proxyv2 images so the application-aware proxy termination hook works correctly. [#22040](https://github.com/deckhouse/deckhouse/pull/22040)
 - **[istio]** Added CARGO_PROXY to ztunnel image build [#20042](https://github.com/deckhouse/deckhouse/pull/20042)
 - **[istio]** Aligned CNI templates with upstream and fixed Istio 1.25 compatibility. [#20332](https://github.com/deckhouse/deckhouse/pull/20332)
 - **[istio]** CNI-node readonly root filesystem enable fix [#19762](https://github.com/deckhouse/deckhouse/pull/19762)
    When using containerdV2, the performance of istio-cni breaks when mounting internal paths.
 - **[istio]** Changed the proxy UID to 1337. [#20074](https://github.com/deckhouse/deckhouse/pull/20074)
 - **[istio]** Excluded Istio v1.27 images from the CSE build. [#20296](https://github.com/deckhouse/deckhouse/pull/20296)
 - **[istio]** Fix graceful draining of established HTTP connections when application pods terminate. [#22067](https://github.com/deckhouse/deckhouse/pull/22067)
 - **[istio]** Fix operator 1.25 startup with containerd v2 and RO rootfs [#21795](https://github.com/deckhouse/deckhouse/pull/21795)
 - **[istio]** Fixed CVEs in Istio as part of the July security update. [#21548](https://github.com/deckhouse/deckhouse/pull/21548)
 - **[istio]** Fixed CVEs in istioctl. [#21496](https://github.com/deckhouse/deckhouse/pull/21496)
 - **[istio]** Fixed access to api-proxy via ALB. [#21142](https://github.com/deckhouse/deckhouse/pull/21142)
 - **[istio]** Fixed ingressGateway advertise FQDN failing to create a ServiceEntry. [#19395](https://github.com/deckhouse/deckhouse/pull/19395)
 - **[istio]** Fixed nelm rollout ordering for Certificate resources. [#20924](https://github.com/deckhouse/deckhouse/pull/20924)
 - **[istio]** Fixed the discovery_operator_versions_to_install hook to migrate from Istio 1.21 to 1.25. [#19434](https://github.com/deckhouse/deckhouse/pull/19434)
 - **[istio]** Fixed werf files for Istio v1.27. [#20285](https://github.com/deckhouse/deckhouse/pull/20285)
 - **[istio]** More stable switching between Istio revisions. [#21034](https://github.com/deckhouse/deckhouse/pull/21034)
 - **[istio]** Restore Istiod pod anti-affinity for Istio 1.25 installations managed by the Sail Operator. [#22167](https://github.com/deckhouse/deckhouse/pull/22167)
 - **[istio]** Reverted setting readOnlyRootFilesystem to true in istio-init. [#21601](https://github.com/deckhouse/deckhouse/pull/21601)
 - **[istio]** fixed CVEs in module images [#19364](https://github.com/deckhouse/deckhouse/pull/19364)
    module pods will be restarted
 - **[kube-dns]** Bumped Go dependencies in the sts-pods-hosts-appender-webhook and coredns images to fix known CVEs. [#21389](https://github.com/deckhouse/deckhouse/pull/21389)
    The CoreDNS and kube-dns components will be restarted after the update.
 - **[kube-dns]** Fixing of the subsystems list for modules. [#20925](https://github.com/deckhouse/deckhouse/pull/20925)
 - **[kube-proxy]** Fixing of the subsystems list for modules. [#20925](https://github.com/deckhouse/deckhouse/pull/20925)
 - **[local-path-provisioner]** Add wildcard tolerations to the helper pod template so PVC provisioning works on tainted nodes after local-path-provisioner v0.0.32+. [#19447](https://github.com/deckhouse/deckhouse/pull/19447)
 - **[local-path-provisioner]** Backport HelperPod template validation to `local-path-provisioner` v0.0.34 to fix CVE-2026-44543 (HelperPod Template Injection, GHSA-7fxv-8wr2-mfc4, CVSS 8.7 High). [#20237](https://github.com/deckhouse/deckhouse/pull/20237)
    The `local-path-provisioner` Pod is restarted during the update. PV provisioning and teardown briefly pauses while the new Pod becomes `Ready`; existing volumes are not affected. If you use a custom `helperPod.yaml` template in the `local-path-config` ConfigMap, remove any unsupported security-sensitive settings before upgrading. Otherwise, the `local-path-provisioner` will reject the template after the update.
 - **[local-path-provisioner]** Update local-path-provisioner to v0.0.36 to pick up the upstream fix for CVE-2026-44543 (HelperPod template injection, CVSS 8.7). [#20449](https://github.com/deckhouse/deckhouse/pull/20449)
    Unsafe custom HelperPod settings in the `local-path-config` ConfigMap are no longer accepted. Default DKP installations are unaffected.
 - **[metallb]** Bumped Go dependencies in the metallb and l2lb images to fix known CVEs. [#21391](https://github.com/deckhouse/deckhouse/pull/21391)
    The `metallb` components (controller, speaker, l2lb) will be restarted.
 - **[multitenancy-manager]** A project parameter can no longer inject objects into the rendered project manifests. [#22091](https://github.com/deckhouse/deckhouse/pull/22091)
    Values taken from `Project.spec.parameters` are now quoted where they are substituted into the
    shipped project templates, and a parameter that changes the structure of the rendered manifests
    rather than only their values is refused for any template, including custom ones. An administrator
    name may no longer contain control characters, and `clusterLogDestinationName` must be a Kubernetes
    object name or empty. Both are checked from the reconcile loop as well as at admission, so a
    project already carrying such a value goes into an error state on its next reconcile rather than
    at its next edit; the same holds for a custom template that turns a parameter into several objects
    or into YAML, which is refused when the project is created, edited, or reconciled with a change to
    apply. The module's admission policy now also matches CREATE, so objects labelled
    `heritage: multitenancy-manager` can only be created by the module itself.
 - **[multitenancy-manager]** Grant and project admission webhooks no longer deadlock the Deckhouse queue when the multitenancy-manager backend denies, is slow, or is unreachable. [#20700](https://github.com/deckhouse/deckhouse/pull/20700)
 - **[multitenancy-manager]** Harden authn/authz validations and quote digit-only Project administrator names. [#22083](https://github.com/deckhouse/deckhouse/pull/22083)
    Existing ModuleConfigs with idTokenTTL >= 6h raise D8UserAuthnIDTokenTTLTooLong until lowered.
    User.spec.password patches after create are rejected; reset via UserOperation or d8 iam user reset-password.
 - **[multitenancy-manager]** fix cve [#21986](https://github.com/deckhouse/deckhouse/pull/21986)
 - **[network-gateway]** Fix dnsmasq (DHCP) and snat containers crash-loop. [#22150](https://github.com/deckhouse/deckhouse/pull/22150)
 - **[network-gateway]** Updated dnsmasq to v2.92-alt2 to address multiple security vulnerabilities. [#19928](https://github.com/deckhouse/deckhouse/pull/19928)
 - **[network-policy-engine]** Reverted module stage from Deprecated back to General Availability to stop false deprecation alerts. [#20294](https://github.com/deckhouse/deckhouse/pull/20294)
 - **[node-local-dns]** Bump Go dependencies in the coredns helper image to fix known CVEs. [#21871](https://github.com/deckhouse/deckhouse/pull/21871)
    The node-local-dns DaemonSet pods will restart after the update.
 - **[node-local-dns]** Bumped Go dependencies in the safe-updater and stale-dns-connections-cleaner images to fix known CVEs. [#21389](https://github.com/deckhouse/deckhouse/pull/21389)
    The `node-local-dns` components will be restarted.
 - **[node-manager]** Added RBAC policies for PersistentVolumes managed by capi-controller-manager. [#19291](https://github.com/deckhouse/deckhouse/pull/19291)
 - **[node-manager]** Added RBAC rules for node-manager. [#19720](https://github.com/deckhouse/deckhouse/pull/19720)
 - **[node-manager]** Added cleanup for oversized MCM MachineSet revision history annotation [#19652](https://github.com/deckhouse/deckhouse/pull/19652)
 - **[node-manager]** Annotate CAPI MachineDeployments with CPU/memory capacity for scale-from-zero [#21824](https://github.com/deckhouse/deckhouse/pull/21824)
 - **[node-manager]** CAPI crd served version fix [#19665](https://github.com/deckhouse/deckhouse/pull/19665)
 - **[node-manager]** Creating or re-applying an already-existing StaticInstance no longer fails address validation. [#21108](https://github.com/deckhouse/deckhouse/pull/21108)
 - **[node-manager]** Deployed `node-group-exporter` only after the cluster is bootstrapped. [#21008](https://github.com/deckhouse/deckhouse/pull/21008)
 - **[node-manager]** Enabled hostNetwork for node-controller during bootstrap. [#19974](https://github.com/deckhouse/deckhouse/pull/19974)
 - **[node-manager]** Filled empty Instance.spec.nodeRef when the linked Machine reports a node name. [#20432](https://github.com/deckhouse/deckhouse/pull/20432)
    <what to expect for users, possibly MULTI-LINE>, required if impact_level is high ↓
 - **[node-manager]** Fix fencing-agent crash when starting on a node in maintenance mode. [#20527](https://github.com/deckhouse/deckhouse/pull/20527)
    Nodes in maintenance mode could previously cause the fencing-agent to crash while attempting to arm the watchdog. The agent now skips watchdog arming during maintenance and resumes it automatically afterwards.
 - **[node-manager]** Fixed TLS vulnerabilities for `capi-controller-manager`. [#20144](https://github.com/deckhouse/deckhouse/pull/20144)
 - **[node-manager]** Fixed `StaticInstance` webhook markers and corrected `SSHCredentials` conversion logs for `v1alpha1`/`v1alpha2`. [#19757](https://github.com/deckhouse/deckhouse/pull/19757)
 - **[node-manager]** Fixed a duplicate v1alpha2 version in the NodeGroup CRD. [#20844](https://github.com/deckhouse/deckhouse/pull/20844)
 - **[node-manager]** Fixed a node-controller rollout deadlock on single-master clusters. [#21181](https://github.com/deckhouse/deckhouse/pull/21181)
 - **[node-manager]** Fixed cluster-autoscaler alert rules for MCM mode. [#20866](https://github.com/deckhouse/deckhouse/pull/20866)
 - **[node-manager]** Fixed cluster-autoscaler scale-from-zero node label verification. [#20948](https://github.com/deckhouse/deckhouse/pull/20948)
    low
 - **[node-manager]** Fixed the convert static cluster configuration hook. [#21345](https://github.com/deckhouse/deckhouse/pull/21345)
 - **[node-manager]** Fixed the keep-policy hook failing when the CAPI conversion webhook is unavailable. [#20990](https://github.com/deckhouse/deckhouse/pull/20990)
 - **[node-manager]** Improve fencing-agent health monitor logging — warn on fallback feeding, error on watchdog starvation, add diagnostic context to all feeding log messages. [#19400](https://github.com/deckhouse/deckhouse/pull/19400)
    Operators can now detect degraded fencing states (quorum loss, API unreachability) through log levels and diagnostic fields without parsing log messages.
 - **[node-manager]** Include system labels in CAPI MachineDeployment capacity annotation for correct scale-from-zero behavior [#20174](https://github.com/deckhouse/deckhouse/pull/20174)
    On CAPI-based clusters (DVP, VCD, zVirt, Dynamix, HuaweiCloud), scale-from-zero now correctly handles pods with nodeSelector targeting system labels (node.deckhouse.io/group, node.deckhouse.io/type, node-role.kubernetes.io/<ng-name>). Previously such pods remained Pending indefinitely when NodeGroup had minPerZone=0. No user action required — the fix is applied automatically on upgrade.
 - **[node-manager]** Made shutdown-inhibitor set the GracefulShutdownPostpone condition only to True or False. [#20716](https://github.com/deckhouse/deckhouse/pull/20716)
 - **[node-manager]** Prevented a kube-api-proxy outage when upstreams.json contains null or empty data. [#20026](https://github.com/deckhouse/deckhouse/pull/20026)
 - **[node-manager]** Reduced MachineDeployment creationTimeout to 5m for AWS spot instances. [#19073](https://github.com/deckhouse/deckhouse/pull/19073)
    <what to expect for users, possibly MULTI-LINE>, required if impact_level is high ↓
 - **[node-manager]** Removed the conflicting `ms` short name from the CAPI MachineSet CRD. [#21189](https://github.com/deckhouse/deckhouse/pull/21189)
 - **[node-manager]** Require a non-empty cluster UUID when rendering the node bootstrap script, preventing static nodes from stalling ~20m on a registry-packages-proxy 404. [#21132](https://github.com/deckhouse/deckhouse/pull/21132)
 - **[node-manager]** fix webook validation in node-controller on cri changes in nodegroup. [#20050](https://github.com/deckhouse/deckhouse/pull/20050)
 - **[node-manager]** hook to restore apiVersion on CAPI resources. [#20330](https://github.com/deckhouse/deckhouse/pull/20330)
 - **[prometheus]** Fixed CVEs in the `alertmanager`, `alerts-receiver`, `grafana-v10`, `memcached`, `mimir`, `prometheus`, `promxy` and `trickster` images by bumping dependencies and rebuilding on the golang builder. [#20395](https://github.com/deckhouse/deckhouse/pull/20395)
    Prometheus, Grafana, Alertmanager, and other monitoring components will be restarted.
 - **[registry-packages-proxy]** Added a fast path for already installed packages and improved logging in rpp-get. [#20015](https://github.com/deckhouse/deckhouse/pull/20015)
 - **[registry-packages-proxy]** Grant registry-packages-proxy RBAC to read unmasked registry credentials (modulesources/sensitive, packagerepositories/sensitive), restoring authentication to private registries. [#21513](https://github.com/deckhouse/deckhouse/pull/21513)
 - **[registrypackages]** Bump `containerd` v2 to 2.2.7 and patch `crictl` 1.36.0 Go dependencies to fix known CVEs. [#22152](https://github.com/deckhouse/deckhouse/pull/22152)
    Nodes with `ContainerdV2` will restarts the `containerd` service .
 - **[registrypackages]** Bump kubernetes-cni to 1.9.1 and update vulnerable Go dependencies to fix CVEs. [#21963](https://github.com/deckhouse/deckhouse/pull/21963)
 - **[registrypackages]** Updated `registrypackages/docker-registry` image Go dependencies to fix Go CVEs. [#20377](https://github.com/deckhouse/deckhouse/pull/20377)
 - **[registrypackages]** Updated the Mozilla CA snapshot used by d8-ca-updater and made the build fail on trusted expired certificates. [#20939](https://github.com/deckhouse/deckhouse/pull/20939)
 - **[service-with-healthchecks]** Bumped Go dependencies in the `service-with-healthchecks` image to fix known CVEs. [#21392](https://github.com/deckhouse/deckhouse/pull/21392)
    The `service-with-healthchecks` components (controller, agent) will be restarted.
 - **[service-with-healthchecks]** Fixed an API server overload issue ("status storm"), resolved validation errors for ClusterIP services, corrected pod readiness evaluation logic, and improved code quality. [#19455](https://github.com/deckhouse/deckhouse/pull/19455)
    The `service-with-healthchecks` status logic was heavily refactored to reduce API and etcd load. If you rely on `lastProbeTime` observability on every probe, explicitly enable `verboseStatus` in the module configuration.
 - **[user-authn]** Added the missing `kubeconfigPublishAPIEncodedName` field to CSE OpenAPI values. [#20864](https://github.com/deckhouse/deckhouse/pull/20864)
 - **[user-authn]** DexClient and DexAuthenticator now honour the value of the allow-access-to-kubernetes annotation instead of merely its presence. [#21979](https://github.com/deckhouse/deckhouse/pull/21979)
    The `dexclient.deckhouse.io/allow-access-to-kubernetes` and
    `dexauthenticator.deckhouse.io/allow-access-to-kubernetes` annotations were previously
    evaluated by key presence, so any value — including "false" — granted the client the
    right to obtain tokens with `aud=kubernetes` and placed it into `trustedPeers` of the
    `kubernetes` OAuth2Client, whose tokens kube-apiserver accepts.
    
    The value is now parsed as a boolean. Only 1, t, T, TRUE, true and True grant the
    capability. Any other value, including "false", "0", "yes", "enabled" and an empty
    string, disables it; unparseable values are rejected with a warning in the
    deckhouse Pod log naming the object and the offending value.
    
    Affected: clusters with a DexClient or DexAuthenticator that carries the annotation
    with a value other than a boolean true. Those objects lose access to the Kubernetes
    API on update, which for most of them is the behaviour their author intended.
    
    Before updating, run
    `kubectl get dexclient,dexauthenticator -A -o json | jq -r '.items[] | select(.metadata.annotations | to_entries[]? | select(.key | endswith("deckhouse.io/allow-access-to-kubernetes")) | .value | ascii_downcase | IN("1","t","true") | not) | "\(.kind)/\(.metadata.namespace)/\(.metadata.name)"'`
    and, for every object listed, either set the annotation to "true" if it genuinely needs
    Kubernetes API access, or remove the annotation to confirm it does not.
 - **[user-authn]** Fix Dex token refresh with upstream providers that rotate refresh tokens (GitLab), which logged users out every `idTokenTTL`. [#21685](https://github.com/deckhouse/deckhouse/pull/21685)
 - **[user-authn]** Fixed DexAuthenticator pod creation under ResourceQuota by setting init container CPU/memory limits to the sum of main container limits. [#21313](https://github.com/deckhouse/deckhouse/pull/21313)
 - **[user-authn]** Harden authn/authz validations and quote digit-only Project administrator names. [#22083](https://github.com/deckhouse/deckhouse/pull/22083)
    Existing ModuleConfigs with idTokenTTL >= 6h raise D8UserAuthnIDTokenTTLTooLong until lowered.
    User.spec.password patches after create are rejected; reset via UserOperation or d8 iam user reset-password.
 - **[user-authn]** Improve basic-auth-proxy request handling, cache implementation, and shutdown behavior. [#20076](https://github.com/deckhouse/deckhouse/pull/20076)
 - **[user-authn]** Reject mutually exclusive LDAP TLS settings in DexProvider; alert on legacy objects. [#19844](https://github.com/deckhouse/deckhouse/pull/19844)
    spec.ldap.insecureNoSSL combined with startTLS, insecureSkipVerify, or a non-empty rootCAData is now rejected by the CRD. Pre-existing DexProvider objects with such combinations keep working but trigger
    D8DexProviderLDAPTLSConflict until updated. Audit with: kubectl get dexproviders.deckhouse.io -o yaml | yq '.items[] | select(.spec.type=="LDAP") | {name: .metadata.name, ldap: .spec.ldap | pick(["insecureNoSSL","startTLS","insecureSkipVerify","rootCAData"])}'
 - **[user-authn]** UserOperation no longer keeps the bcrypt password hash in spec.resetPassword.newPasswordHash after the operation completes. [#20281](https://github.com/deckhouse/deckhouse/pull/20281)
 - **[user-authn]** basic-auth-proxy no longer forwards reserved `system:` groups and usernames from the external directory to kube-apiserver. [#21978](https://github.com/deckhouse/deckhouse/pull/21978)
    Affects clusters where `publishAPI` is enabled together with `enableBasicAuth` on a
    Crowd, OIDC or LDAP `DexProvider`. Both are disabled by default, so a default
    installation is not affected.
    
    Previously the proxy copied directory group names into the `X-Remote-Group` header
    verbatim. Because kube-apiserver trusts that header from this proxy, a user in a
    directory group literally named `system:masters` was granted cluster-admin. Group
    names are chosen by the directory administrator, not by Kubernetes.
    
    After updating, any group whose name begins with `system:` is dropped before the
    request reaches kube-apiserver, and each drop is logged by the basic-auth-proxy pod
    with the user and the group name. A user authenticating with a `system:`-prefixed
    username is refused with `403 Forbidden`. This matches the `userValidationRules`
    already enforced on the OIDC/JWT path.
    
    Before updating, audit your directory for groups whose name begins with `system:`
    and check whether anyone currently relies on one to reach the Kubernetes API through
    basic auth. Such users will lose those privileges on update and must be granted
    access through a normal group bound by RBAC instead. Check the basic-auth-proxy logs
    after updating for `dropping group` messages.
 - **[user-authn]** fix cve [#21986](https://github.com/deckhouse/deckhouse/pull/21986)
 - **[user-authz]** Extend cluster-admin clusterrole  with kubelet-api-admin rights. [#19878](https://github.com/deckhouse/deckhouse/pull/19878)
 - **[user-authz]** Fix multitenancy check hooks [#22178](https://github.com/deckhouse/deckhouse/pull/22178)
 - **[user-authz]** Fix multitenancy validatiing webhook to read effective enableMultiTenancy from the Module `v1alpha2` CR [#21967](https://github.com/deckhouse/deckhouse/pull/21967)
 - **[user-authz]** Harden authn/authz validations and quote digit-only Project administrator names. [#22083](https://github.com/deckhouse/deckhouse/pull/22083)
    Existing ModuleConfigs with idTokenTTL >= 6h raise D8UserAuthnIDTokenTTLTooLong until lowered.
    User.spec.password patches after create are rejected; reset via UserOperation or d8 iam user reset-password.
 - **[user-authz]** Hide namespace existence in access denials. [#22077](https://github.com/deckhouse/deckhouse/pull/22077)
 - **[user-authz]** Honor CAR-independent RBAC in webhook and permission-browser [#21243](https://github.com/deckhouse/deckhouse/pull/21243)
    With `enableMultiTenancy` enabled in the `user-authz` settings, effective permissions now include both ClusterAuthorizationRule and Kubernetes RBAC permissions. Existing RoleBinding and ClusterRoleBinding objects that were previously ignored outside the ClusterAuthorizationRule scope may become effective after the upgrade. Review existing RBAC bindings for affected users and groups.
 - **[user-authz]** User-authz-webhook now uses the node-local kube-apiserver endpoint for its discovery cache and liveness check, instead of resolving the `kubernetes.default` DNS name. [#21066](https://github.com/deckhouse/deckhouse/pull/21066)
    This release fixes an issue where transient cluster DNS failures could cause the user-authz-webhook to restart, temporarily denying all Kubernetes API requests until DNS recovered.
 - **[user-authz]** fix cve [#21986](https://github.com/deckhouse/deckhouse/pull/21986)

## Chore


 - **[candi]** Bump patch versions of Kubernetes images. [#19778](https://github.com/deckhouse/deckhouse/pull/19778)
    Kubernetes control-plane components will restart, kubelet will restart
 - **[candi]** Enable the `DRADeviceTaints` feature gate by default for Kubernetes 1.34+. [#21708](https://github.com/deckhouse/deckhouse/pull/21708)
    kube-apiserver, kube-scheduler, and kube-controller-manager will restart on clusters running Kubernetes 1.34 or newer.
 - **[candi]** Migrated bashible bootstrap from kubectl to d8-curl for Kubernetes API calls. [#19023](https://github.com/deckhouse/deckhouse/pull/19023)
 - **[candi]** Removed the default for encryptionAlgorithm in the ClusterConfiguration OpenAPI spec. [#20975](https://github.com/deckhouse/deckhouse/pull/20975)
 - **[candi]** Set kubelet `serializeImagePulls` to `false` to allow parallel image pulls on nodes. [#20415](https://github.com/deckhouse/deckhouse/pull/20415)
    Kubelet configuration changes on nodes and is applied through the normal node reconfiguration flow.
 - **[candi]** Updated base images to v1.3.22 to address yq CVEs. [#21864](https://github.com/deckhouse/deckhouse/pull/21864)
 - **[candi]** update base images [#22177](https://github.com/deckhouse/deckhouse/pull/22177)
 - **[cloud-provider-aws]** Removed the legacy d8-cni-configuration hook and Helm template. [#20834](https://github.com/deckhouse/deckhouse/pull/20834)
 - **[cloud-provider-aws]** Reverted the removal of the legacy d8-cni-configuration hook and Helm template. [#21043](https://github.com/deckhouse/deckhouse/pull/21043)
 - **[cloud-provider-azure]** Removed the legacy d8-cni-configuration hook and Helm template. [#20834](https://github.com/deckhouse/deckhouse/pull/20834)
 - **[cloud-provider-azure]** Reverted the removal of the legacy d8-cni-configuration hook and Helm template. [#21043](https://github.com/deckhouse/deckhouse/pull/21043)
 - **[cloud-provider-dvp]** Bumped the capdvp-controller-manager cluster-api dependency to v1.12.3. [#20664](https://github.com/deckhouse/deckhouse/pull/20664)
 - **[cloud-provider-dvp]** Removed the legacy d8-cni-configuration hook and Helm template. [#20834](https://github.com/deckhouse/deckhouse/pull/20834)
 - **[cloud-provider-dvp]** Reverted the removal of the legacy d8-cni-configuration hook and Helm template. [#21043](https://github.com/deckhouse/deckhouse/pull/21043)
 - **[cloud-provider-dynamix]** Removed the legacy d8-cni-configuration hook and Helm template. [#20834](https://github.com/deckhouse/deckhouse/pull/20834)
 - **[cloud-provider-dynamix]** Reverted the removal of the legacy d8-cni-configuration hook and Helm template. [#21043](https://github.com/deckhouse/deckhouse/pull/21043)
 - **[cloud-provider-gcp]** Removed the legacy d8-cni-configuration hook and Helm template. [#20834](https://github.com/deckhouse/deckhouse/pull/20834)
 - **[cloud-provider-gcp]** Reverted the removal of the legacy d8-cni-configuration hook and Helm template. [#21043](https://github.com/deckhouse/deckhouse/pull/21043)
 - **[cloud-provider-huaweicloud]** Removed the legacy d8-cni-configuration hook and Helm template. [#20834](https://github.com/deckhouse/deckhouse/pull/20834)
 - **[cloud-provider-huaweicloud]** Reverted the removal of the legacy d8-cni-configuration hook and Helm template. [#21043](https://github.com/deckhouse/deckhouse/pull/21043)
 - **[cloud-provider-openstack]** Removed the legacy d8-cni-configuration hook and Helm template. [#20834](https://github.com/deckhouse/deckhouse/pull/20834)
 - **[cloud-provider-openstack]** Reverted the removal of the legacy d8-cni-configuration hook and Helm template. [#21043](https://github.com/deckhouse/deckhouse/pull/21043)
 - **[cloud-provider-vcd]** Removed the legacy d8-cni-configuration hook and Helm template. [#20834](https://github.com/deckhouse/deckhouse/pull/20834)
 - **[cloud-provider-vcd]** Reverted the removal of the legacy d8-cni-configuration hook and Helm template. [#21043](https://github.com/deckhouse/deckhouse/pull/21043)
 - **[cloud-provider-vsphere]** Added module hook and template rendering tests for hybrid clusters with the vSphere cloud provider. [#19209](https://github.com/deckhouse/deckhouse/pull/19209)
 - **[cloud-provider-vsphere]** Removed the legacy d8-cni-configuration hook and Helm template. [#20834](https://github.com/deckhouse/deckhouse/pull/20834)
 - **[cloud-provider-vsphere]** Reverted the removal of the legacy d8-cni-configuration hook and Helm template. [#21043](https://github.com/deckhouse/deckhouse/pull/21043)
 - **[cloud-provider-vsphere]** move csi-vsphere-syncer image from 000-common into the cloud-provider-vsphere module [#21839](https://github.com/deckhouse/deckhouse/pull/21839)
 - **[cloud-provider-yandex]** Removed the legacy d8-cni-configuration hook and Helm template. [#20834](https://github.com/deckhouse/deckhouse/pull/20834)
 - **[cloud-provider-yandex]** Reverted the removal of the legacy d8-cni-configuration hook and Helm template. [#21043](https://github.com/deckhouse/deckhouse/pull/21043)
 - **[cloud-provider-zvirt]** Removed the legacy d8-cni-configuration hook and Helm template. [#20834](https://github.com/deckhouse/deckhouse/pull/20834)
 - **[cloud-provider-zvirt]** Reverted the removal of the legacy d8-cni-configuration hook and Helm template. [#21043](https://github.com/deckhouse/deckhouse/pull/21043)
 - **[cni-cilium]** Enabled BPF trace events for the Hubble module by default. [#21550](https://github.com/deckhouse/deckhouse/pull/21550)
 - **[cni-simple-bridge]** Fixed securityContext and added SecurityPolicyException [#21765](https://github.com/deckhouse/deckhouse/pull/21765)
 - **[common]** Added Ingress Nginx controller 1.15 and removed controller 1.10 with its `nginxProfilingEnabled` flag. [#19509](https://github.com/deckhouse/deckhouse/pull/19509)
    Users must switch to controller version 1.12 or newer.
 - **[common]** Moved the `extended-monitoring` module from the embedded to the external source. [#20670](https://github.com/deckhouse/deckhouse/pull/20670)
    extended-monitoring module
 - **[common]** Moved the `ingress-nginx` module from the embedded to the external source. [#20528](https://github.com/deckhouse/deckhouse/pull/20528)
    All the `ingress-nginx` module Pods will be restarted.
 - **[common]** Moved the `log-shipper` module to an external source. [#19710](https://github.com/deckhouse/deckhouse/pull/19710)
    log-shipper module
 - **[common]** Moved the `monitoring-applications` module from the embedded to the external source. [#21464](https://github.com/deckhouse/deckhouse/pull/21464)
    monitoring-applications module
 - **[common]** Moved the `monitoring-custom` module from the embedded to the external source. [#20667](https://github.com/deckhouse/deckhouse/pull/20667)
    monitoring-custom module
 - **[common]** Moved the `monitoring-kubernetes` module from the embedded to the external source. The Helm deprecated-API check and `autoK8sVersion` release requirement were migrated to `control-plane-manager`. [#21006](https://github.com/deckhouse/deckhouse/pull/21006)
    monitoring-kubernetes module
 - **[common]** Moved the `monitoring-ping` module from the embedded to the external source. [#20642](https://github.com/deckhouse/deckhouse/pull/20642)
 - **[common]** Moved the `prometheus-metrics-adapter` module from the embedded to the external source. [#20634](https://github.com/deckhouse/deckhouse/pull/20634)
    prometheus-metrics-adapter module
 - **[common]** Moved the `prometheus-pushgateway` module from the embedded to the external source. [#20591](https://github.com/deckhouse/deckhouse/pull/20591)
 - **[common]** Moved the `upmeter` module from the embedded to the external source. [#20869](https://github.com/deckhouse/deckhouse/pull/20869)
    upmeter
 - **[common]** Reduced the final release size by removing unused common images. [#20590](https://github.com/deckhouse/deckhouse/pull/20590)
 - **[common]** Refactored RBACv1 roles (User and ClusterEditor) for the IngressNginxController resource in `ingress-nginx`. [#19911](https://github.com/deckhouse/deckhouse/pull/19911)
 - **[common]** Remove `etcdctl` and `etcdutl` from the debug container image. [#22006](https://github.com/deckhouse/deckhouse/pull/22006)
 - **[common]** Removed the legacy get_cni_secret hook and cniSecretData values field. [#20834](https://github.com/deckhouse/deckhouse/pull/20834)
 - **[common]** Reverted the removal of the legacy get_cni_secret hook and cniSecretData values field. [#21043](https://github.com/deckhouse/deckhouse/pull/21043)
 - **[common]** Switch leftover Go image builds to builder/golang and pm instead of Alpine/apk [#21686](https://github.com/deckhouse/deckhouse/pull/21686)
 - **[common]** Updated IngressNginxController API to support improved resource management. [#18461](https://github.com/deckhouse/deckhouse/pull/18461)
 - **[common]** Updated `RUSTFLAGS` in `log-shipper` to improve security. [#18642](https://github.com/deckhouse/deckhouse/pull/18642)
 - **[control-plane-manager]** Disabled the kube-api QPS rate limit for kube-controller-manager. [#20410](https://github.com/deckhouse/deckhouse/pull/20410)
 - **[control-plane-manager]** Update Kubernetes patch versions to 1.34.10, 1.35.7 and 1.36.3 [#21932](https://github.com/deckhouse/deckhouse/pull/21932)
 - **[deckhouse-controller]** Bumped Kubernetes libraries to support Kubernetes 1.36. [#20488](https://github.com/deckhouse/deckhouse/pull/20488)
 - **[deckhouse-controller]** Updated addon-operator to v1.23.3. [#21170](https://github.com/deckhouse/deckhouse/pull/21170)
 - **[deckhouse]** Allow ClusterAdmin manage ModuleSettingsDefinitions with RBAC. [#21749](https://github.com/deckhouse/deckhouse/pull/21749)
 - **[deckhouse]** Built base-for-go on the distroless builder/golang image. [#20531](https://github.com/deckhouse/deckhouse/pull/20531)
 - **[deckhouse]** Bump dmt v0.1.91. [#21236](https://github.com/deckhouse/deckhouse/pull/21236)
 - **[deckhouse]** Created the monitoring-security module in CSE. [#20063](https://github.com/deckhouse/deckhouse/pull/20063)
 - **[deckhouse]** Decouple from dhctl/cmd/commands. [#19624](https://github.com/deckhouse/deckhouse/pull/19624)
 - **[deckhouse]** Finish failed operations. [#19236](https://github.com/deckhouse/deckhouse/pull/19236)
 - **[deckhouse]** Merge the monitoring-deckhouse module (dashboard, alert rules, PodMonitor) into the deckhouse module. [#20727](https://github.com/deckhouse/deckhouse/pull/20727)
 - **[deckhouse]** Removed the `app` short name from the `Application` resource. [#22138](https://github.com/deckhouse/deckhouse/pull/22138)
 - **[deckhouse]** Removed the in-tree operator-prometheus, prometheus, and loki modules; they are now sourced externally. [#20884](https://github.com/deckhouse/deckhouse/pull/20884)
 - **[dhctl]** Increased attempts for creating Deckhouse manifests in dhctl. [#20494](https://github.com/deckhouse/deckhouse/pull/20494)
 - **[docs]** Added Search API and OpenSearch admin API documentation for Deckhouse Code. [#21234](https://github.com/deckhouse/deckhouse/pull/21234)
 - **[docs]** Added ability to to specify localized title and description in OpenAPI example to render them in the documentation. [#21688](https://github.com/deckhouse/deckhouse/pull/21688)
 - **[docs]** Info about editions for egressgateway has been edited. [#19545](https://github.com/deckhouse/deckhouse/pull/19545)
 - **[docs]** Local authentication article in product documentation updated via [#20187](https://github.com/deckhouse/deckhouse/pull/20187)
 - **[docs]** Upgrade Hugo to v0.161.1. [#20131](https://github.com/deckhouse/deckhouse/pull/20131)
 - **[documentation]** Fix CVEs. Upgrade Hugo to v0.161.1 in documentation builder. [#20131](https://github.com/deckhouse/deckhouse/pull/20131)
 - **[go_lib]** Removed the legacy get_cni_secret hook library. [#20834](https://github.com/deckhouse/deckhouse/pull/20834)
 - **[istio]** Added CRDs for Istio v1.27. [#20046](https://github.com/deckhouse/deckhouse/pull/20046)
 - **[istio]** Added ClusterRoles (RBAC v1) over Istio module CustomResources. [#19587](https://github.com/deckhouse/deckhouse/pull/19587)
 - **[istio]** Added HTTPRoute for multiclusters. [#19603](https://github.com/deckhouse/deckhouse/pull/19603)
    low
 - **[istio]** Added Istio images for version 1.27.9. [#19810](https://github.com/deckhouse/deckhouse/pull/19810)
 - **[istio]** Added a logout button to the Kiali console. [#20922](https://github.com/deckhouse/deckhouse/pull/20922)
 - **[istio]** Added custom templates for the non-operator Istio mode. [#20284](https://github.com/deckhouse/deckhouse/pull/20284)
 - **[istio]** Added the supportsOperator flag in istio.internal.versionMap. [#19996](https://github.com/deckhouse/deckhouse/pull/19996)
 - **[istio]** Alliance-healthcheck now auto-restarts when globalVersion has been changed [#21783](https://github.com/deckhouse/deckhouse/pull/21783)
 - **[istio]** Built Clang binaries using a prepared LLVM cache. [#20816](https://github.com/deckhouse/deckhouse/pull/20816)
 - **[istio]** Changed UID and GID across all pods in the Istio module. [#18851](https://github.com/deckhouse/deckhouse/pull/18851)
    operator will be restarted
 - **[istio]** Fixed DMT lint errors in the istio module. [#21574](https://github.com/deckhouse/deckhouse/pull/21574)
 - **[istio]** Fixed Kiali image builds for CSE. [#21047](https://github.com/deckhouse/deckhouse/pull/21047)
 - **[istio]** Fixed Prometheus CVEs in Istio v1.27. [#20807](https://github.com/deckhouse/deckhouse/pull/20807)
 - **[istio]** Fixed a CVE in Istio images by bumping cel-go and cleaning up image configuration. [#21586](https://github.com/deckhouse/deckhouse/pull/21586)
 - **[istio]** Fixed prometheus, oras-go and go CVEs. [#21893](https://github.com/deckhouse/deckhouse/pull/21893)
 - **[istio]** Justified CVE-2026-42151 and CVE-2026-42154 with VEX in pilot and operator images. [#19949](https://github.com/deckhouse/deckhouse/pull/19949)
 - **[istio]** Justified CVE-2026-44903 with VEX and fixed CVE-2026-46680 in operator 1.25. [#20212](https://github.com/deckhouse/deckhouse/pull/20212)
 - **[istio]** Refactored Istio hooks to prepare for v1.27. [#20258](https://github.com/deckhouse/deckhouse/pull/20258)
 - **[istio]** Removed the d8-istio namespace from the exclude_namespaces list in cni-config. [#20630](https://github.com/deckhouse/deckhouse/pull/20630)
 - **[istio]** Set readOnlyRootFilesystem to true in the istio-init container. [#21096](https://github.com/deckhouse/deckhouse/pull/21096)
 - **[istio]** Skipped operator/IOP/Sail install paths when supportsOperator is false. [#20117](https://github.com/deckhouse/deckhouse/pull/20117)
 - **[istio]** changed vex CVE justifications in pilots images [#19572](https://github.com/deckhouse/deckhouse/pull/19572)
 - **[kube-dns]** Fixed securityContext and added a SecurityPolicyException. [#21501](https://github.com/deckhouse/deckhouse/pull/21501)
 - **[kube-proxy]** Fixed securityContext and added SecurityPolicyException [#21759](https://github.com/deckhouse/deckhouse/pull/21759)
 - **[metallb]** Fixed securityContext and added a SecurityPolicyException. [#21520](https://github.com/deckhouse/deckhouse/pull/21520)
 - **[network-gateway]** Fixed securityContext and added SecurityPolicyException [#21760](https://github.com/deckhouse/deckhouse/pull/21760)
 - **[network-policy-engine]** Fixed securityContext and added SecurityPolicyException [#21758](https://github.com/deckhouse/deckhouse/pull/21758)
 - **[node-local-dns]** Fixed securityContext and added a SecurityPolicyException. [#21501](https://github.com/deckhouse/deckhouse/pull/21501)
 - **[node-manager]** Containerd v1 is removed from the CSE edition [#22197](https://github.com/deckhouse/deckhouse/pull/22197)
 - **[node-manager]** Migrated node and NodeGroup reconciliation hooks to node-controller. [#18481](https://github.com/deckhouse/deckhouse/pull/18481)
 - **[node-manager]** Refactored the CAPS provider. [#18746](https://github.com/deckhouse/deckhouse/pull/18746)
 - **[node-manager]** Updated Cluster API from v1.11.3 to v1.12.9. [#21112](https://github.com/deckhouse/deckhouse/pull/21112)
 - **[openvpn]** Fixed securityContext and added a SecurityPolicyException. [#21538](https://github.com/deckhouse/deckhouse/pull/21538)
 - **[registrypackages]** Added containerd, kubelet, and kubernetes-cni sysext packages to registrypackages. [#20765](https://github.com/deckhouse/deckhouse/pull/20765)
 - **[registrypackages]** Updated containerd to 1.7.34 and 2.2.6, runc to 1.3.6, and related containerd and crictl dependencies. [#21347](https://github.com/deckhouse/deckhouse/pull/21347)
 - **[terraform-manager]** Upgraded OpenTofu to 1.12.0. [#20001](https://github.com/deckhouse/deckhouse/pull/20001)
 - **[user-authz]** Documented migration from the deprecated RBACv2 role names (d8:manage:*, d8:use:role:*) and old custom-role scheme to the new one (d8:system:*, d8:subsystem:*, d8:namespace:*, d8:custom:*). [#21290](https://github.com/deckhouse/deckhouse/pull/21290)
 - **[vertical-pod-autoscaler]** Added a cluster alert and Grafana dashboard that detect VerticalPodAutoscalers using the deprecated Auto update mode. [#20532](https://github.com/deckhouse/deckhouse/pull/20532)
