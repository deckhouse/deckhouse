# Changelog v1.75

## Know before update


 - Breaking changes:
    - The default value of `Certificate.Spec.PrivateKey.RotationPolicy` is now `Always`.
    - The default value for the `Certificate` resource's `revisionHistoryLimit` field is now set to 1.
    - Metrics changes. A high cardinality label, called `path`, was removed from the `certmanager_acme_client_request_count `and `certmanager_acme_client_request_duration_seconds` metrics.
    Feature:
    - Added the ability to configure requests and limits for pods used for ACME HTTP-01 challenges. Configurable in the `Issuer` and `ClusterIssuer` objects. For configuring the built-in  DKP CluserIssers (`letsencrypt` and `letsencrypt-staging`) added settings in moduleConfig.
 - Control-plane components and kubelets will restart to pick up the new feature gates
    on clusters where they were not enabled before. VPA may start applying in-place
    updates instead of only eviction-based updates.
 - If you have controllers running 1.9 the automatic upgrade will be blocked until the version is updated, upgrading to 1.10+ will cause the corresponding controllers to restart.
 - If you have used certain features of `operator-trivy` before, a new alert named `VulnerableImagesDenialConfigNotMigrated` might start firing after update. In that case, you must manually move `denyVulnerableImages` section of settings from `admission-policy-engine` to `operator-trivy` module config. Alert message will provide necessary instructions on how to do so.
 - Istio version 1.19.7 has been removed because it is considered outdated. In this regard, errors may occur when updating the Deckhouse version. It is recommended to upgrade Istio from version 1.19.7 to version 1.21.6 before upgrading Deckhouse release.
 - Mode `Auto` is deprecated and will be removed in a future API version. Use explicit modes like `Recreate`, `Initial`, or `InPlaceOrRecreate` instead.
 - The default VPA mode for Loki components is changed from Auto to InPlaceOrRecreate.
    Loki pods will now prefer in-place resource updates when supported by the cluster,
    falling back to pod recreation only when required.
 - The minimum supported version of Kubernetes is now 1.31. All control plane components will restart.
 - Will restart all d8 pods on dkp release with this changes.

## Features


 - **[admission-policy-engine]** Added policy to deny exec/attach to pods with heritage deckhouse label. [#16749](https://github.com/deckhouse/deckhouse/pull/16749)
 - **[candi]** Added support for multiple Kubernetes v1.35 feature gates. [#18044](https://github.com/deckhouse/deckhouse/pull/18044)
 - **[candi]** Enable DRA alpha feature gates for multi allocations [#17993](https://github.com/deckhouse/deckhouse/pull/17993)
    Kubelet, api-server, controller-manager and scheduler will be restarted.
 - **[candi]** Added parsing oss.yaml file in werf. [#17567](https://github.com/deckhouse/deckhouse/pull/17567)
 - **[candi]** Added support for Kubernetes 1.35 and discontinued support for Kubernetes 1.30. Default Kubernetes version was changed 1.32->1.33. [#17504](https://github.com/deckhouse/deckhouse/pull/17504)
    The minimum supported version of Kubernetes is now 1.31. All control plane components will restart.
 - **[candi]** Implementing SecurityPolicyExceptions in modules cert-manager, user-authz, user-authn, multitenancy-manager, admission-policy-engine, basic-auth. [#16738](https://github.com/deckhouse/deckhouse/pull/16738)
 - **[candi]** Added annotation for node by creating converger user. [#16734](https://github.com/deckhouse/deckhouse/pull/16734)
 - **[cert-manager]** Bumped version up to v1.19.2. [#17486](https://github.com/deckhouse/deckhouse/pull/17486)
    Breaking changes:
    - The default value of `Certificate.Spec.PrivateKey.RotationPolicy` is now `Always`.
    - The default value for the `Certificate` resource's `revisionHistoryLimit` field is now set to 1.
    - Metrics changes. A high cardinality label, called `path`, was removed from the `certmanager_acme_client_request_count `and `certmanager_acme_client_request_duration_seconds` metrics.
    Feature:
    - Added the ability to configure requests and limits for pods used for ACME HTTP-01 challenges. Configurable in the `Issuer` and `ClusterIssuer` objects. For configuring the built-in  DKP CluserIssers (`letsencrypt` and `letsencrypt-staging`) added settings in moduleConfig.
 - **[cloud-provider-dvp]** Created of NP automatic. [#17286](https://github.com/deckhouse/deckhouse/pull/17286)
 - **[cloud-provider-dvp]** Added managed-by, cluster-uuid, vm_name labels to all cluster's infra objects. [#17267](https://github.com/deckhouse/deckhouse/pull/17267)
 - **[cloud-provider-dvp]** Clarified CSI errors. [#16434](https://github.com/deckhouse/deckhouse/pull/16434)
 - **[cloud-provider-zvirt]** add customNetworkConfig [#17879](https://github.com/deckhouse/deckhouse/pull/17879)
 - **[cni-cilium]** Allowed configuring the InPlaceOrRecreate VPA updated mode for Cilium components. [#17252](https://github.com/deckhouse/deckhouse/pull/17252)
    Users can explicitly select the InPlaceOrRecreate VPA mode for Cilium pods via ModuleConfig.
    Default behavior remains unchanged.
 - **[cni-cilium]** Added Hubble metrics and logs settings. [#16669](https://github.com/deckhouse/deckhouse/pull/16669)
    Cilium agent's will be restarted.
 - **[common]** add support accesiblenamespaces in k8s v1.35 [#18069](https://github.com/deckhouse/deckhouse/pull/18069)
 - **[control-plane-manager]** Anonymous access to kube-apiserver health endpoints is now enabled via AuthenticationConfiguration and proxy sidecar is removed. [#17968](https://github.com/deckhouse/deckhouse/pull/17968)
    Kube-apiserver will be restarted due to changes in manifest.
 - **[control-plane-manager]** Added update-observer component for real-time Kubernetes version update monitoring. [#17457](https://github.com/deckhouse/deckhouse/pull/17457)
    Cluster administrators now have detailed visibility into Kubernetes version updates through the new `d8-cluster-kubernetes` ConfigMap in the `kube-system` namespace.
 - **[control-plane-manager]** Added support for enabling/disabling specific scheduler extensions and setting custom values. [#16892](https://github.com/deckhouse/deckhouse/pull/16892)
 - **[control-plane-manager]** Implement etcd-arbiter mode for HA capability with less resources. This will allow to bootstrap only etcd node without control-plane components. [#16716](https://github.com/deckhouse/deckhouse/pull/16716)
    Will restart all d8 pods on dkp release with this changes.
 - **[deckhouse]** Added version checking of module dependencies to scheduler. [#17646](https://github.com/deckhouse/deckhouse/pull/17646)
 - **[deckhouse]** Added configurable scan interval for a ModuleSource discovery. [#17622](https://github.com/deckhouse/deckhouse/pull/17622)
 - **[deckhouse-controller]** Rewrited d8-cluster-configuration webhook from bash to Go. [#17073](https://github.com/deckhouse/deckhouse/pull/17073)
 - **[deckhouse-controller]** Added Application statistics logic. [#16809](https://github.com/deckhouse/deckhouse/pull/16809)
 - **[descheduler]** Add RemovePodsViolatingTopologySpreadConstraint strategy to v1alpha2 API for rebalancing pods across topology domains. [#18107](https://github.com/deckhouse/deckhouse/pull/18107)
    It evicts pods that violate TopologySpreadConstraints, enabling automatic rebalancing across availability zones after zone recovery.
 - **[descheduler]** Updated descheduler to the 0.34 version. 
    Descheduler evicts pods with a larger restart count first it should make workload balancing in the cluster more stable.
    Descheduler respects DRA resources. [#16846](https://github.com/deckhouse/deckhouse/pull/16846)
 - **[dhctl]** Improved UX related to bootstrap resources phase. [#17742](https://github.com/deckhouse/deckhouse/pull/17742)
 - **[dhctl]** Allowed updating master images. [#17295](https://github.com/deckhouse/deckhouse/pull/17295)
 - **[dhctl]** Added a check in dhctl bootstrap to ensure the current user’s shell is bash. [#16980](https://github.com/deckhouse/deckhouse/pull/16980)
 - **[dhctl]** Added a wait for converger user creation on all master nodes. [#16734](https://github.com/deckhouse/deckhouse/pull/16734)
 - **[dhctl]** Added bootstrap support with the registry module for Direct and Unmanaged modes. [#16103](https://github.com/deckhouse/deckhouse/pull/16103)
 - **[extended-monitoring]** Added new options for customizing IAE . [#16902](https://github.com/deckhouse/deckhouse/pull/16902)
 - **[istio]** Removing depricated version of Istio 1.19.7 [#17916](https://github.com/deckhouse/deckhouse/pull/17916)
    Istio version 1.19.7 has been removed because it is considered outdated. In this regard, errors may occur when updating the Deckhouse version. It is recommended to upgrade Istio from version 1.19.7 to version 1.21.6 before upgrading Deckhouse release.
 - **[istio]** Changed name and type of istio-cni ConfigMap. [#17297](https://github.com/deckhouse/deckhouse/pull/17297)
 - **[istio]** Added the InPlaceOrRecreate VPA update mode for Istio components. [#17255](https://github.com/deckhouse/deckhouse/pull/17255)
    Users can explicitly configure the InPlaceOrRecreate VPA mode for Istio workloads in ModuleConfig.
    Default VPA mode for Istio has been updated to InPlaceOrRecreate.
 - **[istio]** Improved federation discovery observability by logging published services count. [#17146](https://github.com/deckhouse/deckhouse/pull/17146)
 - **[log-shipper]** Added metric and alert for not valid logshipper config. [#17010](https://github.com/deckhouse/deckhouse/pull/17010)
 - **[loki]** Changed the default VPA update mode for Loki from Auto to InPlaceOrRecreate. [#17254](https://github.com/deckhouse/deckhouse/pull/17254)
    The default VPA mode for Loki components is changed from Auto to InPlaceOrRecreate.
    Loki pods will now prefer in-place resource updates when supported by the cluster,
    falling back to pod recreation only when required.
 - **[multitenancy-manager]** Added `unmanaged` and `skip-heritage` functions for objects in ProjectTemplate. [#17462](https://github.com/deckhouse/deckhouse/pull/17462)
 - **[node-manager]** Added new standalone node-controller for Node/NodeGroup hooks logic. [#17836](https://github.com/deckhouse/deckhouse/pull/17836)
 - **[node-manager]** Added alerts about missing cgroup v2 and/or containerd v2 support on nodes. [#17658](https://github.com/deckhouse/deckhouse/pull/17658)
 - **[node-manager]** Added configurable swap mechanism for Kubernetes pods using new memorySwap NodeGroup field. [#16747](https://github.com/deckhouse/deckhouse/pull/16747)
    Enabling swap for a NodeGroup will cause a kubelet restart on all nodes of that group.
 - **[node-manager]** Allowed per-GPU custom MIG configurations via `customConfigs` with automatic config/label naming. [#16678](https://github.com/deckhouse/deckhouse/pull/16678)
 - **[prometheus]** Added new alert to monitor remote write endpoint availability. [#17677](https://github.com/deckhouse/deckhouse/pull/17677)
    low
 - **[registry]** Added `Proxy` and `Local` registry operation modes. [#17405](https://github.com/deckhouse/deckhouse/pull/17405)
 - **[registry]** Added bootstrap support for Direct and Unmanaged modes. [#16103](https://github.com/deckhouse/deckhouse/pull/16103)
 - **[user-authn]** Added Prometheus alert for Dex AuthRequest ResourceQuota monitoring. [#17263](https://github.com/deckhouse/deckhouse/pull/17263)
 - **[user-authn]** Added refreshTokenAbsoluteLifetime parameter to limit maximum lifetime of refresh tokens. [#17114](https://github.com/deckhouse/deckhouse/pull/17114)
 - **[user-authn]** Added `enableBasicAuth` support for LDAP provider. [#17022](https://github.com/deckhouse/deckhouse/pull/17022)
    LDAP provider can now enable Basic Auth for the published Kubernetes API endpoint.
 - **[user-authn]** Added optional Kerberos (SPNEGO) SSO to the LDAP Dex provider with keytab-based validation. [#16196](https://github.com/deckhouse/deckhouse/pull/16196)
 - **[user-authn]** Improved support for custom CA in the GitLab Dex provider (refined version of PR [#15825](https://github.com/deckhouse/deckhouse/pull/15825)
 - **[user-authn]** Introduced `UserOperation` hook for local Dex user operations (reset password, reset 2FA, lock/unlock) with status reporting. [#15561](https://github.com/deckhouse/deckhouse/pull/15561)
 - **[user-authz]** Restrict user roles from listing namespaces; use AccessibleNamespaces in non-CE editions [#17651](https://github.com/deckhouse/deckhouse/pull/17651)
 - **[user-authz]** Added AccessibleNamespaces API to list namespaces accessible to the requesting user. [#17436](https://github.com/deckhouse/deckhouse/pull/17436)
 - **[user-authz]** Allowed project Admins access to Roles, RoleBindings and AuthorizationRules on project namespaces. [#17090](https://github.com/deckhouse/deckhouse/pull/17090)
 - **[user-authz]** Added BulkSubjectAccessReview API for checking multiple permissions in a single request. [#17080](https://github.com/deckhouse/deckhouse/pull/17080)
 - **[vertical-pod-autoscaler]** Updated vpa module to 1.5.1. [#16814](https://github.com/deckhouse/deckhouse/pull/16814)
    Mode `Auto` is deprecated and will be removed in a future API version. Use explicit modes like `Recreate`, `Initial`, or `InPlaceOrRecreate` instead.

## Fixes


 - **[admission-policy-engine]** Prevent unintended Gatekeeper constraints from being rendered for SecurityPolicy when boolean fields are omitted. [#18007](https://github.com/deckhouse/deckhouse/pull/18007)
    Workload Pods are no longer denied by unrelated SecurityPolicy checks (e.g. hostNetwork/hostPort) when corresponding policy fields are not explicitly set.
 - **[admission-policy-engine]** Fixed a bootstrap deadlock by excluding Gatekeeper webhook pods from constraints. [#17791](https://github.com/deckhouse/deckhouse/pull/17791)
 - **[admission-policy-engine]** Fixed multiple CVEs in admission-policy-engine module images (ratify, gatekeeper) by updating. dependencies. [#17667](https://github.com/deckhouse/deckhouse/pull/17667)
 - **[admission-policy-engine]** Fixed tri-state semantics for empty arrays and avoided empty objects in OperationPolicy/SecurityPolicy values. [#17343](https://github.com/deckhouse/deckhouse/pull/17343)
 - **[admission-policy-engine]** Added and extend unit tests to cover tri-state behavior (omitted / empty / non-empty) and nested empty-array cases for both hooks. [#17308](https://github.com/deckhouse/deckhouse/pull/17308)
 - **[candi]** Server bootstrap logs are no longer transmitted via nc; Python is used instead. [#17451](https://github.com/deckhouse/deckhouse/pull/17451)
 - **[candi]** Improved static node cleanup script. [#17418](https://github.com/deckhouse/deckhouse/pull/17418)
 - **[candi]** Disabled kernel.panic parameter check in kubelet. [#17296](https://github.com/deckhouse/deckhouse/pull/17296)
 - **[candi]** Made modify_user in add_node_user bashible step idempotent. [#17111](https://github.com/deckhouse/deckhouse/pull/17111)
 - **[candi]** Added fallback to dnf package manager from yum install and remove bashbooster func's. [#17012](https://github.com/deckhouse/deckhouse/pull/17012)
 - **[candi]** Added bashible 064 step criDir fallback. [#16934](https://github.com/deckhouse/deckhouse/pull/16934)
 - **[candi]** Added bashible events generateName. [#16768](https://github.com/deckhouse/deckhouse/pull/16768)
 - **[candi]** Moved the default values for registry in initConfiguration to dhctl. [#16103](https://github.com/deckhouse/deckhouse/pull/16103)
 - **[chrony]** Mitigated CVE-2025-58181. [#17959](https://github.com/deckhouse/deckhouse/pull/17959)
 - **[cloud-provider-dvp]** Prevents orphaned VMBDA objects. [#17682](https://github.com/deckhouse/deckhouse/pull/17682)
 - **[cloud-provider-dvp]** Prevented the CCM from recreating external LoadBalancers during Service deletion. [#17446](https://github.com/deckhouse/deckhouse/pull/17446)
 - **[cloud-provider-dynamix]** Fixed a queue hang caused by the module components failing to start. [#16796](https://github.com/deckhouse/deckhouse/pull/16796)
 - **[cloud-provider-huaweicloud]** Fixed a queue hang caused by the module components failing to start. [#16796](https://github.com/deckhouse/deckhouse/pull/16796)
 - **[cloud-provider-vcd]** Fixed a queue hang caused by the module components failing to start. [#16796](https://github.com/deckhouse/deckhouse/pull/16796)
 - **[cloud-provider-yandex]** Added fallback to `nat_instance_internal_address_calculated`. [#17341](https://github.com/deckhouse/deckhouse/pull/17341)
 - **[cloud-provider-zvirt]** Fixed a queue hang caused by the module components failing to start. [#16796](https://github.com/deckhouse/deckhouse/pull/16796)
 - **[common]** Restricted kubelet static pod manifest processing to .yaml and .yml files. [#17842](https://github.com/deckhouse/deckhouse/pull/17842)
 - **[common]** Disabled kernel.panic parameter check in kubelet. [#17296](https://github.com/deckhouse/deckhouse/pull/17296)
 - **[control-plane-manager]** Upgrade etcd to 3.6.8. [#18038](https://github.com/deckhouse/deckhouse/pull/18038)
    etcd will restart.
 - **[control-plane-manager]** Removed liveness and readiness probes from update-observer container. [#17789](https://github.com/deckhouse/deckhouse/pull/17789)
 - **[control-plane-manager]** Extended authz webhook matchConditions to bypass critical control-plane identities and avoid deadlocks. [#17644](https://github.com/deckhouse/deckhouse/pull/17644)
    Prevents control-plane components (including CAPI controllers) and Deckhouse service accounts
    from being blocked by the authorization webhook.
    Reduces the risk of cluster deadlocks and improves recoverability when fail-closed authorization is enabled.
 - **[control-plane-manager]** Upgraded etcd to 3.6.7. [#17492](https://github.com/deckhouse/deckhouse/pull/17492)
    etcd will restart.
 - **[control-plane-manager]** Switched kube-apiserver to structured authorization config with fail-closed webhook. [#17183](https://github.com/deckhouse/deckhouse/pull/17183)
    Authorization webhook now works in fail-closed mode. If the webhook is unavailable, authorization requests are denied instead of falling back to RBAC.
 - **[deckhouse]** Added exception to system-ns.deckhouse.io policy. [#17754](https://github.com/deckhouse/deckhouse/pull/17754)
 - **[deckhouse]** Fixed missing module stage in the Module CR, restoring experimental module warnings. [#17244](https://github.com/deckhouse/deckhouse/pull/17244)
 - **[deckhouse]** Fixed deckhouse-registry secret validation. [#17122](https://github.com/deckhouse/deckhouse/pull/17122)
 - **[deckhouse]** Added validation for deckhouse-registry Secret fields to reject spaces and newlines. [#16101](https://github.com/deckhouse/deckhouse/pull/16101)
 - **[deckhouse-controller]** Fixed release notification time for deckhouse and module releases. [#17583](https://github.com/deckhouse/deckhouse/pull/17583)
 - **[deckhouse-controller]** Fixed `--insecure` flag being ignored in registry client operations. [#17554](https://github.com/deckhouse/deckhouse/pull/17554)
 - **[deckhouse-controller]** Fixed patch releases being skipped on minor updates. [#17548](https://github.com/deckhouse/deckhouse/pull/17548)
 - **[deckhouse-controller]** Fixed D8ModuleOutdatedByMajorVersion alert persist after update. [#17468](https://github.com/deckhouse/deckhouse/pull/17468)
 - **[deckhouse-controller]** Fixed incorrect MUP fallback for module releases. [#17434](https://github.com/deckhouse/deckhouse/pull/17434)
 - **[deckhouse-controller]** Fixed corner cases in d8-cluster-configuration webhook. [#17342](https://github.com/deckhouse/deckhouse/pull/17342)
 - **[deckhouse-controller]** Fixed stale ModuleConfigurationError metrics not being reset when ModuleRelease is deleted or module is disabled. [#16940](https://github.com/deckhouse/deckhouse/pull/16940)
 - **[deckhouse-controller]** Rollback nelm version. [#16770](https://github.com/deckhouse/deckhouse/pull/16770)
 - **[deckhouse-controller]** Removed track-termination-mode notation. [#16612](https://github.com/deckhouse/deckhouse/pull/16612)
 - **[descheduler]** Fixed module queue hang when a v1alpha1 Descheduler CR with deprecated-only strategies is applied. [#17986](https://github.com/deckhouse/deckhouse/pull/17986)
 - **[descheduler]** Removed implicit default thresholds from Descheduler CRD and align behavior with upstream. [#17488](https://github.com/deckhouse/deckhouse/pull/17488)
    Thresholds and targetThresholds are no longer implicitly defaulted.
    If a resource is not specified in the Descheduler CR, it is treated as 100% and does not participate in eviction logic.
 - **[dhctl]** Fixed to allow skip dhctl preflight check-staticinstance-by-ssh-credentials. [#18077](https://github.com/deckhouse/deckhouse/pull/18077)
 - **[dhctl]** Made control-plane node SSH IP lookup non-strict in converge infrastructure hooks. [#18063](https://github.com/deckhouse/deckhouse/pull/18063)
 - **[dhctl]** Fixed dhctl server startup order and interrupt child process on backend connection failure. [#17966](https://github.com/deckhouse/deckhouse/pull/17966)
 - **[dhctl]** Added state saver to cluster for bootstrap additional control-plane and static nodes. [#17943](https://github.com/deckhouse/deckhouse/pull/17943)
 - **[dhctl]** Stopped cleaning the temporary directory when `converge` or `converge-migration` fails. [#17943](https://github.com/deckhouse/deckhouse/pull/17943)
 - **[dhctl]** Fixed node template diff output during converge when templates are empty but objects differ. [#17943](https://github.com/deckhouse/deckhouse/pull/17943)
 - **[dhctl]** Added infrastructure states and NodeUser to sanitize in klog. [#17943](https://github.com/deckhouse/deckhouse/pull/17943)
 - **[dhctl]** Logged Kubernetes requests and responses in JSON format instead of protobuf bytes in debug logs. [#17943](https://github.com/deckhouse/deckhouse/pull/17943)
 - **[dhctl]** Fixed dhctl clissh scp command. [#17896](https://github.com/deckhouse/deckhouse/pull/17896)
 - **[dhctl]** Fixed dhctl in SSH tunnel preflight check. [#17805](https://github.com/deckhouse/deckhouse/pull/17805)
 - **[dhctl]** Removed unnecessary artifact from dhctl. [#17797](https://github.com/deckhouse/deckhouse/pull/17797)
 - **[dhctl]** Fixed data race and panic in lease tryRenew. [#17735](https://github.com/deckhouse/deckhouse/pull/17735)
 - **[dhctl]** Fixed dhctl panic on destructive chages if master ip node is nill in update pipeline. [#17351](https://github.com/deckhouse/deckhouse/pull/17351)
 - **[dhctl]** Removed some internal phases from progress bar. [#17340](https://github.com/deckhouse/deckhouse/pull/17340)
 - **[dhctl]** Updated tests. [#17310](https://github.com/deckhouse/deckhouse/pull/17310)
 - **[dhctl]** Fixed initconfiguration generation logic. [#17285](https://github.com/deckhouse/deckhouse/pull/17285)
 - **[dhctl]** Removed dhctl object node from cluster in converge. [#17163](https://github.com/deckhouse/deckhouse/pull/17163)
 - **[dhctl]** Changed SSH logging. [#17143](https://github.com/deckhouse/deckhouse/pull/17143)
 - **[dhctl]** Added `dvp provider.kubeconfigDataBase64` preflight check. [#16945](https://github.com/deckhouse/deckhouse/pull/16945)
 - **[dhctl]** Fixed --skip-resources flag behaviour in destroy command. [#16904](https://github.com/deckhouse/deckhouse/pull/16904)
 - **[dhctl]** Added many fixes in destroy command and restart destroy command. [#16904](https://github.com/deckhouse/deckhouse/pull/16904)
 - **[dhctl]** Fixed dhctl bootstrap-phase abort running after dhctl bootstrap-phasebase-infra. [#16829](https://github.com/deckhouse/deckhouse/pull/16829)
 - **[dhctl]** Fixed kube token handling. [#16735](https://github.com/deckhouse/deckhouse/pull/16735)
 - **[docs]** Added docs about how NGC execution works. [#17870](https://github.com/deckhouse/deckhouse/pull/17870)
 - **[docs]** Fixed registry-modules-watcher deleting all documentation when registry returns an error. [#16771](https://github.com/deckhouse/deckhouse/pull/16771)
 - **[ingress-nginx]** The annotation validation is fixed in 1.12. [#18078](https://github.com/deckhouse/deckhouse/pull/18078)
    All ingress-nginx controller pods of the 1.12 version will be restarted.
 - **[ingress-nginx]** An http to https redirect to a wrong host is fixed. [#17931](https://github.com/deckhouse/deckhouse/pull/17931)
    All ingress-nginx controller pods will be restarted.
 - **[ingress-nginx]** Restored the expected behavior of the Ingress resource annotation validation toggle in controller v1.12. [#17809](https://github.com/deckhouse/deckhouse/pull/17809)
    All ingress controller pods will restart.
 - **[ingress-nginx]** The real-ip-cidr patches are updated to use correct nginx variables. [#17402](https://github.com/deckhouse/deckhouse/pull/17402)
    All ingress-nginx controllers' pods will be restarted.
 - **[ingress-nginx]** Added OWASP modesecurity core rule set support. [#17348](https://github.com/deckhouse/deckhouse/pull/17348)
    Pods of all ingress-nginx controller will be restarted.
 - **[ingress-nginx]** Improved configuration validation and documentation. [#17307](https://github.com/deckhouse/deckhouse/pull/17307)
 - **[ingress-nginx]** Added panel GeoIP DB status per controller in VHosts Grafana dashboard. [#17219](https://github.com/deckhouse/deckhouse/pull/17219)
 - **[ingress-nginx]** Fixed accepting X-Forwareded/ProxyProtocol headers from untrusted networks. [#17060](https://github.com/deckhouse/deckhouse/pull/17060)
    All ingress nginx controller pods will be restarted.
 - **[ingress-nginx]** Fixed correct controller termination. [#17041](https://github.com/deckhouse/deckhouse/pull/17041)
    restart controllers
 - **[ingress-nginx]** Added architecture-bashed node affinity settings. [#16939](https://github.com/deckhouse/deckhouse/pull/16939)
 - **[ingress-nginx]** Fixed the display of IP addresses in the status of Ingress resources with the LoadBalancer type. [#15892](https://github.com/deckhouse/deckhouse/pull/15892)
 - **[keepalived]** Updated manual switch instructions in FAQ to use debug container. [#17982](https://github.com/deckhouse/deckhouse/pull/17982)
 - **[log-shipper]** Fixed source-specific log label enrichment and simplified transform processing. [#16989](https://github.com/deckhouse/deckhouse/pull/16989)
    Changing the order of transformations only affects the operation of the log-shipper.
 - **[monitoring-kubernetes]** Added unsupported ValidatingAdmissionPolicy API versions on Kubernetes 1.34. [#17007](https://github.com/deckhouse/deckhouse/pull/17007)
 - **[multitenancy-manager]** Fixed multiple CVEs in multitenancy-manager module images by updating dependencies. [#17534](https://github.com/deckhouse/deckhouse/pull/17534)
 - **[network-policy-engine]** Fixed a bug that led to CrashLoopBackOff kube-router's pods. [#17737](https://github.com/deckhouse/deckhouse/pull/17737)
 - **[node-manager]** Fixed logging errors during ssh connections in caps. [#17802](https://github.com/deckhouse/deckhouse/pull/17802)
 - **[node-manager]** Added deletion of the kubelet checkpoint file `/var/lib/kubelet/pod_status_manager_state` immediately before kubelet restart. [#17403](https://github.com/deckhouse/deckhouse/pull/17403)
    Prevents kubelet startup panic caused by incompatible or corrupted.
 - **[node-manager]** Added patch to fix memory manager error after reboot. [#17331](https://github.com/deckhouse/deckhouse/pull/17331)
 - **[node-manager]** Added adjust StaticMachineTemplate webhook to allow first change of labelSelector. [#17276](https://github.com/deckhouse/deckhouse/pull/17276)
 - **[node-manager]** Added bashible-apiserver retry on start failed. [#17249](https://github.com/deckhouse/deckhouse/pull/17249)
 - **[node-manager]** Fixed capi_crds_cabundle_injection. [#17193](https://github.com/deckhouse/deckhouse/pull/17193)
 - **[node-manager]** Added annotate draining node when deleting. [#17189](https://github.com/deckhouse/deckhouse/pull/17189)
 - **[node-manager]** Enabledoptional prom-rule. [#17112](https://github.com/deckhouse/deckhouse/pull/17112)
 - **[node-manager]** Enabled use node IP to get ApiServer for CAPI on bootstrap. [#17076](https://github.com/deckhouse/deckhouse/pull/17076)
 - **[node-manager]** Added middleware to bashible-apiserver to log bashible resource requests and responses. [#17019](https://github.com/deckhouse/deckhouse/pull/17019)
 - **[node-manager]** Adjusted the regexp used for NodeGroup priority generation in the cluster-autoscaler priority expander fallback. [#16998](https://github.com/deckhouse/deckhouse/pull/16998)
 - **[node-manager]** Fixed conditions calc for static NodeGroup. [#16811](https://github.com/deckhouse/deckhouse/pull/16811)
 - **[node-manager]** Reduced CAPS log noise and duplicate messages. [#16805](https://github.com/deckhouse/deckhouse/pull/16805)
 - **[node-manager]** Updated go dependencies in the bashible-api-server. [#16103](https://github.com/deckhouse/deckhouse/pull/16103)
 - **[prometheus]** Fixed rebuild of trickster. [#17115](https://github.com/deckhouse/deckhouse/pull/17115)
 - **[registry]** Fixed validation of input image list changes in the registry checker. [#17472](https://github.com/deckhouse/deckhouse/pull/17472)
 - **[registry]** Omitted the auth field in DockerConfig when credentials (username and password) are empty. [#17310](https://github.com/deckhouse/deckhouse/pull/17310)
 - **[registrypackages]** Upgraded containerd to 1.7.30 and 2.1.6. [#17510](https://github.com/deckhouse/deckhouse/pull/17510)
    Containerd will restart.
 - **[terraform-manager]** Fixed terraform CVE. [#17862](https://github.com/deckhouse/deckhouse/pull/17862)
 - **[user-authn]** Fixed LDAP authentication failure when filter field contains trailing newline from YAML literal block scalar. [#17950](https://github.com/deckhouse/deckhouse/pull/17950)
 - **[user-authn]** Ships Dex Kubernetes storage CRDs with the module to prevent missing-CRD bootstrap failures. [#17885](https://github.com/deckhouse/deckhouse/pull/17885)
    On fresh clusters, Dex storage CRDs (e.g. OfflineSessions/RefreshToken) are now installed by the module,
    preventing hook/informer startup failures due to absent `dex.coreos.com` CRDs.
 - **[user-authn]** Improved Dex LDAP Kerberos (SPNEGO) logs and error handling. [#17543](https://github.com/deckhouse/deckhouse/pull/17543)
 - **[user-authn]** Fixed multiple CVEs in user-authn module images by updating dependencies. [#17518](https://github.com/deckhouse/deckhouse/pull/17518)
 - **[user-authn]** Forbided IP addresses in DexAuthenticator domain fields; only DNS names are allowed. [#17305](https://github.com/deckhouse/deckhouse/pull/17305)
    DexAuthenticator resources with IP addresses in domain fields will now be rejected at creation/update time with a clear error message. Previously, such resources were accepted but failed silently during Ingress creation.
 - **[user-authn]** Enabled hide internal error details from users in Dex to prevent information disclosure. [#17177](https://github.com/deckhouse/deckhouse/pull/17177)
 - **[user-authz]** Fixed SecurityPolicyException usage, added CR presence check. [#17660](https://github.com/deckhouse/deckhouse/pull/17660)
 - **[user-authz]** Allowed node-local `user-authz-webhook` listener port (40443/TCP) for hostNetwork pods. [#17656](https://github.com/deckhouse/deckhouse/pull/17656)
    The `user-authz-webhook` DaemonSet now explicitly declares its listener port and has a matching
    SecurityPolicyException. This prevents Admission Policy Engine validation failures in-cluster.
    A targeted `dmt lint` exception is added for the `host-network-ports` rule because it enforces
    the 4200–4299 range and does not take SecurityPolicyException into account.
 - **[user-authz]** Made user-authz webhook use node-local kube-apiserver endpoint to avoid ClusterIP connectivity issues. [#17580](https://github.com/deckhouse/deckhouse/pull/17580)
    Improves stability of the user-authz authorization webhook in environments where hostNetwork pods cannot reach ClusterIP services.
    Prevents intermittent Kubernetes API errors when kube-apiserver authorization is configured in fail-closed mode.

## Chore


 - **[candi]** Bump patch versions of Kubernetes images. [#18175](https://github.com/deckhouse/deckhouse/pull/18175)
    Kubernetes control-plane components will restart, kubelet will restart
 - **[candi]** Removed patches for kubernetes 1.30, which is not supported since Deckhouse v1.75.0. [#17998](https://github.com/deckhouse/deckhouse/pull/17998)
 - **[candi]** Bump patch versions of Kubernetes images. [#17930](https://github.com/deckhouse/deckhouse/pull/17930)
    Kubernetes control-plane components will restart, kubelet will restart
 - **[candi]** Removed insecure kube-apiserver cipher suites `TLS_RSA_WITH_AES_256_GCM_SHA384`, `TLS_RSA_WITH_AES_128_GCM_SHA256`, added fixed names for `TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256`, `TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256`. [#17777](https://github.com/deckhouse/deckhouse/pull/17777)
 - **[candi]** Removed overrides for journald configuration. [#17769](https://github.com/deckhouse/deckhouse/pull/17769)
 - **[candi]** Added module directory localization. [#17544](https://github.com/deckhouse/deckhouse/pull/17544)
 - **[candi]** Bumped patch versions of Kubernetes images. [#16955](https://github.com/deckhouse/deckhouse/pull/16955)
    Kubernetes control-plane components will restart, kubelet will restart.
 - **[candi]** Updated documentation in candi. [#16933](https://github.com/deckhouse/deckhouse/pull/16933)
 - **[candi]** Updated static nodes with topology labels via /var/lib/node_labels. [#16816](https://github.com/deckhouse/deckhouse/pull/16816)
 - **[candi]** Bumped autoscaler version to 1.32.2. [#16610](https://github.com/deckhouse/deckhouse/pull/16610)
 - **[cloud-provider-aws]** Added module directory localization. [#17544](https://github.com/deckhouse/deckhouse/pull/17544)
 - **[cloud-provider-aws]** Updated module internals for Nelm compatibility. [#17150](https://github.com/deckhouse/deckhouse/pull/17150)
 - **[cloud-provider-azure]** Fixed cloud providers linter warnings. [#17992](https://github.com/deckhouse/deckhouse/pull/17992)
 - **[cloud-provider-azure]** Added module directory localization. [#17544](https://github.com/deckhouse/deckhouse/pull/17544)
 - **[cloud-provider-azure]** Updated module internals for Nelm compatibility. [#17150](https://github.com/deckhouse/deckhouse/pull/17150)
 - **[cloud-provider-dvp]** Fixed cloud providers linter warnings. [#17992](https://github.com/deckhouse/deckhouse/pull/17992)
 - **[cloud-provider-dvp]** Added module directory localization. [#17544](https://github.com/deckhouse/deckhouse/pull/17544)
 - **[cloud-provider-dvp]** Added ownerReferences to VM-related objects (managed by CAPDVP and CSI). [#17268](https://github.com/deckhouse/deckhouse/pull/17268)
 - **[cloud-provider-dvp]** Updated module internals for Nelm compatibility. [#17150](https://github.com/deckhouse/deckhouse/pull/17150)
 - **[cloud-provider-dynamix]** Added module directory localization. [#17544](https://github.com/deckhouse/deckhouse/pull/17544)
 - **[cloud-provider-gcp]** Added module directory localization. [#17544](https://github.com/deckhouse/deckhouse/pull/17544)
 - **[cloud-provider-gcp]** Updated module internals for Nelm compatibility. [#17150](https://github.com/deckhouse/deckhouse/pull/17150)
 - **[cloud-provider-huaweicloud]** Added module directory localization. [#17544](https://github.com/deckhouse/deckhouse/pull/17544)
 - **[cloud-provider-openstack]** Fixed cloud providers linter warnings. [#17992](https://github.com/deckhouse/deckhouse/pull/17992)
 - **[cloud-provider-openstack]** Added module directory localization. [#17544](https://github.com/deckhouse/deckhouse/pull/17544)
 - **[cloud-provider-openstack]** Updated module internals for Nelm compatibility. [#17150](https://github.com/deckhouse/deckhouse/pull/17150)
 - **[cloud-provider-vcd]** Fixed cloud providers linter warnings. [#17992](https://github.com/deckhouse/deckhouse/pull/17992)
 - **[cloud-provider-vcd]** Added module directory localization. [#17544](https://github.com/deckhouse/deckhouse/pull/17544)
 - **[cloud-provider-vsphere]** Fixed cloud providers linter warnings. [#17992](https://github.com/deckhouse/deckhouse/pull/17992)
 - **[cloud-provider-vsphere]** Added module directory localization. [#17544](https://github.com/deckhouse/deckhouse/pull/17544)
 - **[cloud-provider-vsphere]** Updated module internals for Nelm compatibility. [#17150](https://github.com/deckhouse/deckhouse/pull/17150)
 - **[cloud-provider-yandex]** Fixed cloud providers linter warnings. [#17992](https://github.com/deckhouse/deckhouse/pull/17992)
 - **[cloud-provider-yandex]** Added module directory localization. [#17544](https://github.com/deckhouse/deckhouse/pull/17544)
 - **[cloud-provider-yandex]** Updated module internals for Nelm compatibility. [#17150](https://github.com/deckhouse/deckhouse/pull/17150)
 - **[cloud-provider-zvirt]** Fixed cloud providers linter warnings. [#17992](https://github.com/deckhouse/deckhouse/pull/17992)
 - **[cloud-provider-zvirt]** Added module directory localization. [#17544](https://github.com/deckhouse/deckhouse/pull/17544)
 - **[cni-cilium]** Changed GO target version to 1.25. [#17981](https://github.com/deckhouse/deckhouse/pull/17981)
 - **[cni-cilium]** Fixed code in egress-gateway-agent (se-plus), check-wg-kernel-compat and safe-agent-updater images with linter recommendations. [#17763](https://github.com/deckhouse/deckhouse/pull/17763)
 - **[cni-cilium]** Added SVACE analyze for modules. [#17514](https://github.com/deckhouse/deckhouse/pull/17514)
 - **[cni-flannel]** Changed GO target version to 1.25. [#17981](https://github.com/deckhouse/deckhouse/pull/17981)
 - **[cni-flannel]** Fixed code in flanneld image with linter recommendations. [#17763](https://github.com/deckhouse/deckhouse/pull/17763)
 - **[cni-flannel]** Restarted cni-flannel agents. [#17532](https://github.com/deckhouse/deckhouse/pull/17532)
 - **[cni-simple-bridge]** Restarted cni-simple-bridge agents. [#17532](https://github.com/deckhouse/deckhouse/pull/17532)
 - **[common]** Refactored debug-container image to support distroless environments and include essential tools. [#17982](https://github.com/deckhouse/deckhouse/pull/17982)
 - **[common]** Changed GO target version to 1.25 in vxlan-offloading-fixer image. [#17981](https://github.com/deckhouse/deckhouse/pull/17981)
 - **[control-plane-manager]** Improved the control-plane-manager etcd re-join logic for losing a member by destructive changes. converge [#17347](https://github.com/deckhouse/deckhouse/pull/17347)
 - **[csi-vsphere]** Updated module internals for Nelm compatibility. [#17150](https://github.com/deckhouse/deckhouse/pull/17150)
 - **[deckhouse]** Module moved from core distribtion into separate external module. [#16328](https://github.com/deckhouse/deckhouse/pull/16328)
    If you have used certain features of `operator-trivy` before, a new alert named `VulnerableImagesDenialConfigNotMigrated` might start firing after update. In that case, you must manually move `denyVulnerableImages` section of settings from `admission-policy-engine` to `operator-trivy` module config. Alert message will provide necessary instructions on how to do so.
 - **[dhctl]** Add output of remained resources for creation resources bootstrap phase in dhctl. [#18046](https://github.com/deckhouse/deckhouse/pull/18046)
 - **[dhctl]** Added preflight tests. [#17261](https://github.com/deckhouse/deckhouse/pull/17261)
 - **[dhctl]** Added preflight check to get access staticInstance with sshcredentials. [#16974](https://github.com/deckhouse/deckhouse/pull/16974)
 - **[docs]** Network ports for hostNetwork virtualization components actualization [#18139](https://github.com/deckhouse/deckhouse/pull/18139)
 - **[extended-monitoring]** Moved events exporter to our repo. [#17091](https://github.com/deckhouse/deckhouse/pull/17091)
 - **[ingress-nginx]** Changed GO target version to 1.25. [#17981](https://github.com/deckhouse/deckhouse/pull/17981)
 - **[ingress-nginx]** Nginx is updated to v1.29.5 in all controllers, golang dependencies are also updated. [#17914](https://github.com/deckhouse/deckhouse/pull/17914)
    All ingres-nginx controller pods will be restated.
 - **[ingress-nginx]** Added ingress-nginx controller of version v1.14.3. [#17864](https://github.com/deckhouse/deckhouse/pull/17864)
    The change does not affect existing clusters unless controllerVersion `1.14` is selected.
 - **[ingress-nginx]** Removed 1.9 controller version. [#17832](https://github.com/deckhouse/deckhouse/pull/17832)
    If you have controllers running 1.9 the automatic upgrade will be blocked until the version is updated, upgrading to 1.10+ will cause the corresponding controllers to restart.
 - **[ingress-nginx]** Fixed code in failover-cleaner, protobuf-exporter, proxy-failover-iptables and proxy-failover images with linter recommendations. [#17763](https://github.com/deckhouse/deckhouse/pull/17763)
 - **[ingress-nginx]** Restarted ingress-nginx controllers. [#17532](https://github.com/deckhouse/deckhouse/pull/17532)
 - **[ingress-nginx]** Added the ability to customize ports for the load balancer Service. [#17433](https://github.com/deckhouse/deckhouse/pull/17433)
 - **[istio]** Changed GO target version to 1.25. [#17981](https://github.com/deckhouse/deckhouse/pull/17981)
 - **[istio]** Fixed code in api-proxy and metadata-exporter images with linter recommendations. [#17763](https://github.com/deckhouse/deckhouse/pull/17763)
 - **[kube-dns]** Changed GO target version to 1.25. [#17981](https://github.com/deckhouse/deckhouse/pull/17981)
 - **[kube-dns]** Fixed code in sts-pods-hosts-appender-webhook image with linter recommendations. [#17763](https://github.com/deckhouse/deckhouse/pull/17763)
 - **[kube-proxy]** API endpoints discovery hook now uses EndpointSlice [#18083](https://github.com/deckhouse/deckhouse/pull/18083)
 - **[kube-proxy]** Changed GO target version to 1.25. [#17981](https://github.com/deckhouse/deckhouse/pull/17981)
 - **[kube-proxy]** Fixed code in init-container image with linter recommendations. [#17763](https://github.com/deckhouse/deckhouse/pull/17763)
 - **[kube-proxy]** Restarted kube-proxy agents. [#17532](https://github.com/deckhouse/deckhouse/pull/17532)
 - **[log-shipper]** Migrate Loki endpoint discovery from v1.Endpoints to discovery.k8s.io/v1.EndpointSlice for Kubernetes 1.35 compatibility. [#18036](https://github.com/deckhouse/deckhouse/pull/18036)
 - **[metallb]** Dashboard templates will be imported only if prometheus, prometheus-operator modules are enabled. [#17840](https://github.com/deckhouse/deckhouse/pull/17840)
 - **[monitoring-ping]** Changed GO target version to 1.25. [#17981](https://github.com/deckhouse/deckhouse/pull/17981)
 - **[monitoring-ping]** Fixed code in monitoring-ping image with linter recommendations. [#17763](https://github.com/deckhouse/deckhouse/pull/17763)
 - **[network-policy-engine]** Restarted network-policy-engine agents. [#17532](https://github.com/deckhouse/deckhouse/pull/17532)
 - **[node-local-dns]** Changed GO target version to 1.25. [#17981](https://github.com/deckhouse/deckhouse/pull/17981)
 - **[node-local-dns]** Fixed code in iptables-loop and coredns images with linter recommendations. [#17763](https://github.com/deckhouse/deckhouse/pull/17763)
 - **[node-local-dns]** Restarted non-cilium setups of node-local-dns. [#17532](https://github.com/deckhouse/deckhouse/pull/17532)
 - **[node-manager]** Added patches for cluster-autoscaler. [#18004](https://github.com/deckhouse/deckhouse/pull/18004)
 - **[node-manager]** Bumped autoscaler version to 1.34. [#16610](https://github.com/deckhouse/deckhouse/pull/16610)
 - **[node-manager]** Bumped capi version 1.10.6 > 1.11.3. [#16153](https://github.com/deckhouse/deckhouse/pull/16153)
 - **[openvpn]** Changed GO target version to 1.25. [#17981](https://github.com/deckhouse/deckhouse/pull/17981)
 - **[openvpn]** Fixed code in easyrsa-migrator and openvpn images with linter recommendations. [#17763](https://github.com/deckhouse/deckhouse/pull/17763)
 - **[openvpn]** Restarted openvpn instances, all vpn connections disrupted. [#17532](https://github.com/deckhouse/deckhouse/pull/17532)
 - **[prometheus]** Added new rules for grafana dashboards. [#15865](https://github.com/deckhouse/deckhouse/pull/15865)
 - **[service-with-healthchecks]** Changed GO target version to 1.25. [#17981](https://github.com/deckhouse/deckhouse/pull/17981)
 - **[service-with-healthchecks]** Fixed code in artifact image with linter recommendations. [#17763](https://github.com/deckhouse/deckhouse/pull/17763)
 - **[service-with-healthchecks]** Added SVACE analyze for modules. [#17514](https://github.com/deckhouse/deckhouse/pull/17514)
 - **[terraform-manager]** Updated terraform-manager images build. [#16941](https://github.com/deckhouse/deckhouse/pull/16941)
 - **[user-authz]** Fixed golangci-lint findings in multiple modules (user-authn, user-authz, admission-policy-engine, multitenancy-manager). [#17672](https://github.com/deckhouse/deckhouse/pull/17672)
 - **[vertical-pod-autoscaler]** Enabled using InPlaceOrRecreate update mode instead of Auto for Deckhouse-managed VPAs. [#17011](https://github.com/deckhouse/deckhouse/pull/17011)
    Control-plane components and kubelets will restart to pick up the new feature gates
    on clusters where they were not enabled before. VPA may start applying in-place
    updates instead of only eviction-based updates.

