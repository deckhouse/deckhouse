# Changelog v1.75

## [MALFORMED]


 - #16328 unknown section "operator-trivy"
 - #17095 missing type
 - #17305 missing section, missing summary, missing type, unknown section ""

## Know before update


 - Breaking changes:
    - the default value of `Certificate.Spec.PrivateKey.RotationPolicy` is now `Always`
    - the default value for the `Certificate` resource's `revisionHistoryLimit` field is now set to 1
    - metrics changes. A high cardinality label, called `path`, was removed from the `certmanager_acme_client_request_count `and `certmanager_acme_client_request_duration_seconds` metrics.
    Feature:
    - Added the ability to configure requests and limits for pods used for ACME HTTP-01 challenges. Configurable in the `Issuer` and `ClusterIssuer` objects. For configuring the built-in  DKP CluserIssers (`letsencrypt` and `letsencrypt-staging`) added settings in moduleConfig.
 - Control-plane components and kubelets will restart to pick up the new feature gates
    on clusters where they were not enabled before. VPA may start applying in-place
    updates instead of only eviction-based updates.
 - The default VPA mode for Loki components is changed from Auto to InPlaceOrRecreate.
    Loki pods will now prefer in-place resource updates when supported by the cluster,
    falling back to pod recreation only when required.
 - Will restart all d8 pods on dkp release with this changes.
 - mode auto is deprecated and will be removed in a future API version. Use explicit modes like "Recreate", "Initial", or "InPlaceOrRecreate" instead.

## Features


 - **[admission-policy-engine]** add policy to deny exec/attach to pods with heritage deckhouse label [#16749](https://github.com/deckhouse/deckhouse/pull/16749)
 - **[candi]** Add parsing oss.yaml file in werf [#17567](https://github.com/deckhouse/deckhouse/pull/17567)
 - **[candi]** Implementing SecurityPolicyExceptions in modules 101-cert-manager, 140-user-authz, 150-user-authn, 160-multitenancy-manager, 015-admission-policy-engine, 500-basic-auth [#16738](https://github.com/deckhouse/deckhouse/pull/16738)
 - **[candi]** Add annotation for node by creating converger user. [#16734](https://github.com/deckhouse/deckhouse/pull/16734)
 - **[cert-manager]** bump version up to v1.19.2 [#17486](https://github.com/deckhouse/deckhouse/pull/17486)
    Breaking changes:
    - the default value of `Certificate.Spec.PrivateKey.RotationPolicy` is now `Always`
    - the default value for the `Certificate` resource's `revisionHistoryLimit` field is now set to 1
    - metrics changes. A high cardinality label, called `path`, was removed from the `certmanager_acme_client_request_count `and `certmanager_acme_client_request_duration_seconds` metrics.
    Feature:
    - Added the ability to configure requests and limits for pods used for ACME HTTP-01 challenges. Configurable in the `Issuer` and `ClusterIssuer` objects. For configuring the built-in  DKP CluserIssers (`letsencrypt` and `letsencrypt-staging`) added settings in moduleConfig.
 - **[cloud-provider-dvp]** add managed-by, cluster-uuid, vm_name labels to all cluster's infra objects [#17267](https://github.com/deckhouse/deckhouse/pull/17267)
 - **[cloud-provider-dvp]** clarify CSI errors [#16434](https://github.com/deckhouse/deckhouse/pull/16434)
 - **[cni-cilium]** Allow configuring the InPlaceOrRecreate VPA update mode for Cilium components. [#17252](https://github.com/deckhouse/deckhouse/pull/17252)
    Users can explicitly select the InPlaceOrRecreate VPA mode for Cilium pods via ModuleConfig.
    Default behavior remains unchanged.
 - **[cni-cilium]** Added Hubble metrics and logs settings. [#16669](https://github.com/deckhouse/deckhouse/pull/16669)
    Cilium agent's will be restarted.
 - **[control-plane-manager]** the ability to enable and disable some scheduler extensions and set up custom values. [#16892](https://github.com/deckhouse/deckhouse/pull/16892)
 - **[control-plane-manager]** Implement etcd-arbiter mode for HA capability with less resources. This will allow to bootstrap only etcd node without control-plane components. [#16716](https://github.com/deckhouse/deckhouse/pull/16716)
    Will restart all d8 pods on dkp release with this changes.
 - **[deckhouse]** Add configurable scan interval for a ModuleSource discovery. [#17622](https://github.com/deckhouse/deckhouse/pull/17622)
 - **[deckhouse-controller]** rewrite d8-cluster-configuration webhook from bash to Go [#17073](https://github.com/deckhouse/deckhouse/pull/17073)
 - **[deckhouse-controller]** add Application statistics logic [#16809](https://github.com/deckhouse/deckhouse/pull/16809)
 - **[descheduler]** update descheduler to the 1.34 version. 
    Descheduler evicts pods with a larger restart count first it should make workload balancing in the cluster more stable.
    Descheduler respects DRA resources. [#16846](https://github.com/deckhouse/deckhouse/pull/16846)
 - **[dhctl]** allow updating master images [#17295](https://github.com/deckhouse/deckhouse/pull/17295)
 - **[dhctl]** Check if user shell differs from bash. [#16980](https://github.com/deckhouse/deckhouse/pull/16980)
 - **[dhctl]** Wait for converger user will be presented on all master nodes. [#16734](https://github.com/deckhouse/deckhouse/pull/16734)
 - **[dhctl]** Added bootstrap support with the registry module [#16103](https://github.com/deckhouse/deckhouse/pull/16103)
 - **[extended-monitoring]** Add new options for customizing IAE [#16902](https://github.com/deckhouse/deckhouse/pull/16902)
 - **[istio]** Changed name and type of istio-cni ConfigMap [#17297](https://github.com/deckhouse/deckhouse/pull/17297)
 - **[istio]** Support InPlaceOrRecreate VPA update mode for Istio components. [#17255](https://github.com/deckhouse/deckhouse/pull/17255)
    Users can explicitly configure the InPlaceOrRecreate VPA mode for Istio workloads in ModuleConfig.
    Default VPA mode for Istio has been updated to InPlaceOrRecreate.
 - **[istio]** federation discovery hook logs verbosity improved [#17146](https://github.com/deckhouse/deckhouse/pull/17146)
 - **[log-shipper]** Add metric and alert for not valid logshipper config [#17010](https://github.com/deckhouse/deckhouse/pull/17010)
 - **[loki]** Change the default VPA update mode for Loki from Auto to InPlaceOrRecreate. [#17254](https://github.com/deckhouse/deckhouse/pull/17254)
    The default VPA mode for Loki components is changed from Auto to InPlaceOrRecreate.
    Loki pods will now prefer in-place resource updates when supported by the cluster,
    falling back to pod recreation only when required.
 - **[multitenancy-manager]** Add "unmanaged" and "skip-heritage" functions for objects in ProjectTemplate [#17462](https://github.com/deckhouse/deckhouse/pull/17462)
 - **[node-manager]** Add configurable swap mechanism for Kubernetes pods using new memorySwap NodeGroup field [#16747](https://github.com/deckhouse/deckhouse/pull/16747)
    Enabling swap for a NodeGroup will cause a kubelet restart on all nodes of that group
 - **[prometheus]** improve redirects from Grafana to the Deckhouse UI when Grafana is disabled [#16988](https://github.com/deckhouse/deckhouse/pull/16988)
    no impact
 - **[user-authn]** Added Prometheus alert for Dex AuthRequest ResourceQuota monitoring. [#17263](https://github.com/deckhouse/deckhouse/pull/17263)
 - **[user-authn]** Add refreshTokenAbsoluteLifetime parameter to limit maximum lifetime of refresh tokens [#17114](https://github.com/deckhouse/deckhouse/pull/17114)
 - **[user-authn]** Add `enableBasicAuth` support for LDAP provider. [#17022](https://github.com/deckhouse/deckhouse/pull/17022)
    LDAP provider can now enable Basic Auth for the published Kubernetes API endpoint.
 - **[user-authn]** Add optional Kerberos (SPNEGO) SSO to the LDAP Dex provider with keytab-based validation [#16196](https://github.com/deckhouse/deckhouse/pull/16196)
 - **[user-authz]** Add AccessibleNamespaces API to list namespaces accessible to the requesting user [#17436](https://github.com/deckhouse/deckhouse/pull/17436)
 - **[user-authz]** Allow project Admins access to Roles, RoleBindings and AuthorizationRules on project namespaces [#17090](https://github.com/deckhouse/deckhouse/pull/17090)
 - **[user-authz]** Add BulkSubjectAccessReview API for checking multiple permissions in a single request [#17080](https://github.com/deckhouse/deckhouse/pull/17080)
 - **[vertical-pod-autoscaler]** update vpa module to 1.5.1 [#16814](https://github.com/deckhouse/deckhouse/pull/16814)
    mode auto is deprecated and will be removed in a future API version. Use explicit modes like "Recreate", "Initial", or "InPlaceOrRecreate" instead.

## Fixes


 - **[admission-policy-engine]** Fix multiple CVEs in admission-policy-engine module images (ratify, gatekeeper) by updating dependencies. [#17667](https://github.com/deckhouse/deckhouse/pull/17667)
 - **[admission-policy-engine]** Preserve tri-state semantics for empty arrays and avoid empty objects in OperationPolicy/SecurityPolicy values [#17343](https://github.com/deckhouse/deckhouse/pull/17343)
 - **[admission-policy-engine]** Preserve explicitly empty lists ([]) in SecurityPolicy/OperationPolicy Values to ensure Gatekeeper constraints render and policies apply [#17308](https://github.com/deckhouse/deckhouse/pull/17308)
 - **[candi]** refusal to use nc in bashible [#17451](https://github.com/deckhouse/deckhouse/pull/17451)
 - **[candi]** disable kernel.panic parameter check in kubelet [#17296](https://github.com/deckhouse/deckhouse/pull/17296)
 - **[candi]** Make modify_user in add_node_user bashible step idempotent. [#17111](https://github.com/deckhouse/deckhouse/pull/17111)
 - **[candi]** fallback to dnf package manager from yum install and remove bashbooster func's [#17012](https://github.com/deckhouse/deckhouse/pull/17012)
 - **[candi]** bashible 064 step criDir fallback [#16934](https://github.com/deckhouse/deckhouse/pull/16934)
 - **[candi]** bashible events generateName [#16768](https://github.com/deckhouse/deckhouse/pull/16768)
 - **[candi]** Moved the default values for registry in initConfiguration to dhctl [#16103](https://github.com/deckhouse/deckhouse/pull/16103)
 - **[cloud-provider-dvp]** prevents the CCM from recreating external LoadBalancers during Service deletion [#17446](https://github.com/deckhouse/deckhouse/pull/17446)
 - **[cloud-provider-dynamix]** fixed a queue hang caused by the module components failing to start [#16796](https://github.com/deckhouse/deckhouse/pull/16796)
 - **[cloud-provider-huaweicloud]** fixed a queue hang caused by the module components failing to start [#16796](https://github.com/deckhouse/deckhouse/pull/16796)
 - **[cloud-provider-vcd]** fixed a queue hang caused by the module components failing to start [#16796](https://github.com/deckhouse/deckhouse/pull/16796)
 - **[cloud-provider-yandex]** add fallback to `nat_instance_internal_address_calculated` [#17341](https://github.com/deckhouse/deckhouse/pull/17341)
 - **[cloud-provider-zvirt]** fixed a queue hang caused by the module components failing to start [#16796](https://github.com/deckhouse/deckhouse/pull/16796)
 - **[common]** disable kernel.panic parameter check in kubelet [#17296](https://github.com/deckhouse/deckhouse/pull/17296)
 - **[control-plane-manager]** Extend authz webhook matchConditions to bypass critical control-plane identities and avoid deadlocks [#17644](https://github.com/deckhouse/deckhouse/pull/17644)
    Prevents control-plane components (including CAPI controllers) and Deckhouse service accounts
    from being blocked by the authorization webhook.
    Reduces the risk of cluster deadlocks and improves recoverability when fail-closed authorization is enabled.
 - **[control-plane-manager]** upgrade etcd to 3.6.7. [#17492](https://github.com/deckhouse/deckhouse/pull/17492)
    etcd will restart.
 - **[control-plane-manager]** Switch kube-apiserver to structured authorization config with fail-closed webhook. [#17183](https://github.com/deckhouse/deckhouse/pull/17183)
    Authorization webhook now works in fail-closed mode. If the webhook is unavailable, authorization requests are denied instead of falling back to RBAC.
 - **[deckhouse]** Sync modules with its metadata. [#17244](https://github.com/deckhouse/deckhouse/pull/17244)
 - **[deckhouse]** validate deckhouse-registry secret [#17122](https://github.com/deckhouse/deckhouse/pull/17122)
 - **[deckhouse]** validate deckhouse-registry secret for spaces or newlines. [#16101](https://github.com/deckhouse/deckhouse/pull/16101)
 - **[deckhouse-controller]** Fix release notification time for deckhouse and module releases [#17583](https://github.com/deckhouse/deckhouse/pull/17583)
 - **[deckhouse-controller]** Fixed `--insecure` flag being ignored in registry client operations [#17554](https://github.com/deckhouse/deckhouse/pull/17554)
 - **[deckhouse-controller]** Fix patch releases being skipped on minor updates [#17548](https://github.com/deckhouse/deckhouse/pull/17548)
 - **[deckhouse-controller]** Fix D8ModuleOutdatedByMajorVersion alert persist after update [#17468](https://github.com/deckhouse/deckhouse/pull/17468)
 - **[deckhouse-controller]** Fix incorrect MUP fallback for module releases [#17434](https://github.com/deckhouse/deckhouse/pull/17434)
 - **[deckhouse-controller]** fix corner cases in d8-cluster-configuration webhook [#17342](https://github.com/deckhouse/deckhouse/pull/17342)
 - **[deckhouse-controller]** Fix stale ModuleConfigurationError metrics not being reset when ModuleRelease is deleted or module is disabled [#16940](https://github.com/deckhouse/deckhouse/pull/16940)
 - **[deckhouse-controller]** Rollback nelm version [#16770](https://github.com/deckhouse/deckhouse/pull/16770)
 - **[deckhouse-controller]** Remove track-termination-mode notation [#16612](https://github.com/deckhouse/deckhouse/pull/16612)
 - **[descheduler]** Remove implicit default thresholds from Descheduler CRD and align behavior with upstream [#17488](https://github.com/deckhouse/deckhouse/pull/17488)
    Thresholds and targetThresholds are no longer implicitly defaulted.
    If a resource is not specified in the Descheduler CR, it is treated as 100% and does not participate in eviction logic.
 - **[dhctl]** dhctl panic fix on destructive chages if master ip node is nill in update pipeline [#17351](https://github.com/deckhouse/deckhouse/pull/17351)
 - **[dhctl]** Remove some internal phases from progress bar [#17340](https://github.com/deckhouse/deckhouse/pull/17340)
 - **[dhctl]** Updated tests [#17310](https://github.com/deckhouse/deckhouse/pull/17310)
 - **[dhctl]** dhctl delete object node from cluster in converge. [#17163](https://github.com/deckhouse/deckhouse/pull/17163)
 - **[dhctl]** Change ssh logging. [#17143](https://github.com/deckhouse/deckhouse/pull/17143)
 - **[dhctl]** dvp provider.kubeconfigDataBase64 preflight check [#16945](https://github.com/deckhouse/deckhouse/pull/16945)
 - **[dhctl]** Fix --skip-resources flag behaviour in destroy command. [#16904](https://github.com/deckhouse/deckhouse/pull/16904)
 - **[dhctl]** Many fixes in destroy command and restart destroy command. [#16904](https://github.com/deckhouse/deckhouse/pull/16904)
 - **[dhctl]** Fix dhctl bootstrap-phase abort running after dhctl bootstrap-phasebase-infra. [#16829](https://github.com/deckhouse/deckhouse/pull/16829)
 - **[dhctl]** fix kube token handling [#16735](https://github.com/deckhouse/deckhouse/pull/16735)
 - **[docs]** Fix registry-modules-watcher deleting all documentation when registry returns an error [#16771](https://github.com/deckhouse/deckhouse/pull/16771)
 - **[ingress-nginx]** The real-ip-cidr patches are updated to use correct nginx variables. [#17402](https://github.com/deckhouse/deckhouse/pull/17402)
    All ingress-nginx controllers' pods will be restarted.
 - **[ingress-nginx]** OWASP modesecurity core rule set support is added [#17348](https://github.com/deckhouse/deckhouse/pull/17348)
    Pods of all ingress-nginx controller will be restarted
 - **[ingress-nginx]** Improved configuration validation and documentation. [#17307](https://github.com/deckhouse/deckhouse/pull/17307)
 - **[ingress-nginx]** Added panel GeoIP DB status per controller in VHosts Grafana dashboard. [#17219](https://github.com/deckhouse/deckhouse/pull/17219)
 - **[ingress-nginx]** Accepting X-Forwareded/ProxyProtocol headers from untrusted networks is fixed. [#17060](https://github.com/deckhouse/deckhouse/pull/17060)
    All ingress nginx controller pods will be restarted.
 - **[ingress-nginx]** Correct controller termination has been fixed. [#17041](https://github.com/deckhouse/deckhouse/pull/17041)
    restart controllers
 - **[ingress-nginx]** Architecture-bashed node affinity settings are provided. [#16939](https://github.com/deckhouse/deckhouse/pull/16939)
 - **[ingress-nginx]** Fixed the display of IP addresses in the status of Ingress resources with the LoadBalancer type. [#15892](https://github.com/deckhouse/deckhouse/pull/15892)
 - **[monitoring-kubernetes]** Handle unsupported ValidatingAdmissionPolicy API versions on Kubernetes 1.34 [#17007](https://github.com/deckhouse/deckhouse/pull/17007)
 - **[multitenancy-manager]** Fix multiple CVEs in multitenancy-manager module images by updating dependencies. [#17534](https://github.com/deckhouse/deckhouse/pull/17534)
 - **[node-manager]** Automatically remove kubelet checkpoint file before restart during upgrade to 1.32. [#17403](https://github.com/deckhouse/deckhouse/pull/17403)
    Prevents kubelet startup panic caused by incompatible or corrupted
 - **[node-manager]** Clean up Memory Manager state during graceful node shutdown [#17331](https://github.com/deckhouse/deckhouse/pull/17331)
 - **[node-manager]** adjust StaticMachineTemplate webhook to allow first change of labelSelector [#17276](https://github.com/deckhouse/deckhouse/pull/17276)
 - **[node-manager]** bashible-apiserver retry on start failed [#17249](https://github.com/deckhouse/deckhouse/pull/17249)
 - **[node-manager]** fix capi_crds_cabundle_injection [#17193](https://github.com/deckhouse/deckhouse/pull/17193)
 - **[node-manager]** annotate draining node when deleting [#17189](https://github.com/deckhouse/deckhouse/pull/17189)
 - **[node-manager]** optional prom-rule enable [#17112](https://github.com/deckhouse/deckhouse/pull/17112)
 - **[node-manager]** use node ip to get ApiServer for CAPI on bootstrap. [#17076](https://github.com/deckhouse/deckhouse/pull/17076)
 - **[node-manager]** Add middleware to bashible-apiserver to log bashible resource requests and responses. [#17019](https://github.com/deckhouse/deckhouse/pull/17019)
 - **[node-manager]** Fix conditions calc for static NodeGroup. [#16811](https://github.com/deckhouse/deckhouse/pull/16811)
 - **[node-manager]** CAPS logs noise reduction [#16805](https://github.com/deckhouse/deckhouse/pull/16805)
 - **[node-manager]** Updated go dependencies in the bashible-api-server [#16103](https://github.com/deckhouse/deckhouse/pull/16103)
 - **[prometheus]** Fix rebuild of trickster [#17115](https://github.com/deckhouse/deckhouse/pull/17115)
 - **[registry]** Fixed validation of input image list changes in the registry checker. [#17472](https://github.com/deckhouse/deckhouse/pull/17472)
 - **[registry]** Omitted the auth field in DockerConfig when credentials (username and password) are empty. [#17310](https://github.com/deckhouse/deckhouse/pull/17310)
 - **[registry]** Added bootstrap support [#16103](https://github.com/deckhouse/deckhouse/pull/16103)
 - **[registrypackages]** Upgrade containerd to 1.7.30 and 2.1.6. [#17510](https://github.com/deckhouse/deckhouse/pull/17510)
    Containerd will restart.
 - **[user-authn]** Improve Dex LDAP Kerberos (SPNEGO) logs and error handling. [#17543](https://github.com/deckhouse/deckhouse/pull/17543)
 - **[user-authn]** Fix multiple CVEs in user-authn module images by updating dependencies. [#17518](https://github.com/deckhouse/deckhouse/pull/17518)
 - **[user-authn]** Hide internal error details from users in Dex to prevent information disclosure [#17177](https://github.com/deckhouse/deckhouse/pull/17177)
 - **[user-authz]** Fixed SecurityPolicyException usage, added CR presence check [#17660](https://github.com/deckhouse/deckhouse/pull/17660)
 - **[user-authz]** Make user-authz webhook use node-local kube-apiserver endpoint to avoid ClusterIP connectivity issues [#17580](https://github.com/deckhouse/deckhouse/pull/17580)
    Improves stability of the user-authz authorization webhook in environments where hostNetwork pods cannot reach ClusterIP services.
    Prevents intermittent Kubernetes API errors when kube-apiserver authorization is configured in fail-closed mode.

## Chore


 - **[candi]** Bump patch versions of Kubernetes images. [#16955](https://github.com/deckhouse/deckhouse/pull/16955)
    Kubernetes control-plane components will restart, kubelet will restart
 - **[candi]** Update documentation in candi. [#16933](https://github.com/deckhouse/deckhouse/pull/16933)
 - **[candi]** enrich static nodes with topology labels via /var/lib/node_labels [#16816](https://github.com/deckhouse/deckhouse/pull/16816)
 - **[candi]** bump autoscaler version to 1.32.2 [#16610](https://github.com/deckhouse/deckhouse/pull/16610)
 - **[cloud-provider-aws]** nelm adaptation [#17150](https://github.com/deckhouse/deckhouse/pull/17150)
 - **[cloud-provider-azure]** nelm adaptation [#17150](https://github.com/deckhouse/deckhouse/pull/17150)
 - **[cloud-provider-dvp]** nelm adaptation [#17150](https://github.com/deckhouse/deckhouse/pull/17150)
 - **[cloud-provider-gcp]** nelm adaptation [#17150](https://github.com/deckhouse/deckhouse/pull/17150)
 - **[cloud-provider-openstack]** nelm adaptation [#17150](https://github.com/deckhouse/deckhouse/pull/17150)
 - **[cloud-provider-vsphere]** nelm adaptation [#17150](https://github.com/deckhouse/deckhouse/pull/17150)
 - **[cloud-provider-yandex]** nelm adaptation [#17150](https://github.com/deckhouse/deckhouse/pull/17150)
 - **[cni-cilium]** Added SVACE analyze for modules. [#17514](https://github.com/deckhouse/deckhouse/pull/17514)
 - **[control-plane-manager]** Improving the control-plane-manager etcd re-join logic for losing a member by destructive changes converge [#17347](https://github.com/deckhouse/deckhouse/pull/17347)
 - **[csi-vsphere]** nelm adaptation [#17150](https://github.com/deckhouse/deckhouse/pull/17150)
 - **[dhctl]** add preflight tests [#17261](https://github.com/deckhouse/deckhouse/pull/17261)
 - **[extended-monitoring]** Move events exporter to our repo [#17091](https://github.com/deckhouse/deckhouse/pull/17091)
 - **[node-manager]** bump autoscaler version to 1.34 [#16610](https://github.com/deckhouse/deckhouse/pull/16610)
 - **[node-manager]** bumped capi version 1.10.6 > 1.11.3. [#16153](https://github.com/deckhouse/deckhouse/pull/16153)
 - **[prometheus]** Add new rules for grafana dashboards [#15865](https://github.com/deckhouse/deckhouse/pull/15865)
 - **[service-with-healthchecks]** Added SVACE analyze for modules. [#17514](https://github.com/deckhouse/deckhouse/pull/17514)
 - **[terraform-manager]** Update terraform-manager images build. [#16941](https://github.com/deckhouse/deckhouse/pull/16941)
 - **[user-authz]** Fix golangci-lint findings in multiple modules (user-authn, user-authz, admission-policy-engine, multitenancy-manager). [#17672](https://github.com/deckhouse/deckhouse/pull/17672)
 - **[vertical-pod-autoscaler]** Use InPlaceOrRecreate update mode instead of Auto for Deckhouse-managed VPAs. [#17011](https://github.com/deckhouse/deckhouse/pull/17011)
    Control-plane components and kubelets will restart to pick up the new feature gates
    on clusters where they were not enabled before. VPA may start applying in-place
    updates instead of only eviction-based updates.

