# Changelog v1.75

## [MALFORMED]


 - #16328 unknown section "operator-trivy"
 - #17095 missing type

## Know before update


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
 - **[candi]** Add annotation for node by creating converger user. [#16734](https://github.com/deckhouse/deckhouse/pull/16734)
 - **[cloud-provider-dvp]** clarify CSI errors [#16434](https://github.com/deckhouse/deckhouse/pull/16434)
 - **[cni-cilium]** Allow configuring the InPlaceOrRecreate VPA update mode for Cilium components. [#17252](https://github.com/deckhouse/deckhouse/pull/17252)
    Users can explicitly select the InPlaceOrRecreate VPA mode for Cilium pods via ModuleConfig.
    Default behavior remains unchanged.
 - **[control-plane-manager]** the ability to enable and disable some scheduler extensions and set up custom values. [#16892](https://github.com/deckhouse/deckhouse/pull/16892)
 - **[control-plane-manager]** Implement etcd-arbiter mode for HA capability with less resources. This will allow to bootstrap only etcd node without control-plane components. [#16716](https://github.com/deckhouse/deckhouse/pull/16716)
    Will restart all d8 pods on dkp release with this changes.
 - **[deckhouse-controller]** add Application statistics logic [#16809](https://github.com/deckhouse/deckhouse/pull/16809)
 - **[descheduler]** update descheduler to the 1.34 version. 
    Descheduler evicts pods with a larger restart count first it should make workload balancing in the cluster more stable.
    Descheduler respects DRA resources. [#16846](https://github.com/deckhouse/deckhouse/pull/16846)
 - **[dhctl]** allow updating master images [#17295](https://github.com/deckhouse/deckhouse/pull/17295)
 - **[dhctl]** Check if user shell differs from bash. [#16980](https://github.com/deckhouse/deckhouse/pull/16980)
 - **[dhctl]** Wait for converger user will be presented on all master nodes. [#16734](https://github.com/deckhouse/deckhouse/pull/16734)
 - **[dhctl]** Added bootstrap support with the registry module [#16103](https://github.com/deckhouse/deckhouse/pull/16103)
 - **[istio]** Support InPlaceOrRecreate VPA update mode for Istio components. [#17255](https://github.com/deckhouse/deckhouse/pull/17255)
    Users can explicitly configure the InPlaceOrRecreate VPA mode for Istio workloads in ModuleConfig.
    Default VPA mode for Istio has been updated to InPlaceOrRecreate.
 - **[loki]** Change the default VPA update mode for Loki from Auto to InPlaceOrRecreate. [#17254](https://github.com/deckhouse/deckhouse/pull/17254)
    The default VPA mode for Loki components is changed from Auto to InPlaceOrRecreate.
    Loki pods will now prefer in-place resource updates when supported by the cluster,
    falling back to pod recreation only when required.
 - **[node-manager]** Add configurable swap mechanism for Kubernetes pods using new memorySwap NodeGroup field [#16747](https://github.com/deckhouse/deckhouse/pull/16747)
    Enabling swap for a NodeGroup will cause a kubelet restart on all nodes of that group
 - **[prometheus]** improve redirects from Grafana to the Deckhouse UI when Grafana is disabled [#16988](https://github.com/deckhouse/deckhouse/pull/16988)
    no impact
 - **[user-authn]** Added Prometheus alert for Dex AuthRequest ResourceQuota monitoring. [#17263](https://github.com/deckhouse/deckhouse/pull/17263)
 - **[user-authn]** Add refreshTokenAbsoluteLifetime parameter to limit maximum lifetime of refresh tokens [#17114](https://github.com/deckhouse/deckhouse/pull/17114)
 - **[user-authn]** Add `enableBasicAuth` support for LDAP provider. [#17022](https://github.com/deckhouse/deckhouse/pull/17022)
    LDAP provider can now enable Basic Auth for the published Kubernetes API endpoint.
 - **[user-authz]** Allow project Admins access to Roles, RoleBindings and AuthorizationRules on project namespaces [#17090](https://github.com/deckhouse/deckhouse/pull/17090)
 - **[user-authz]** Add BulkSubjectAccessReview API for checking multiple permissions in a single request [#17080](https://github.com/deckhouse/deckhouse/pull/17080)
 - **[vertical-pod-autoscaler]** update vpa module to 1.5.1 [#16814](https://github.com/deckhouse/deckhouse/pull/16814)
    mode auto is deprecated and will be removed in a future API version. Use explicit modes like "Recreate", "Initial", or "InPlaceOrRecreate" instead.

## Fixes


 - **[admission-policy-engine]** Preserve explicitly empty lists ([]) in SecurityPolicy/OperationPolicy Values to ensure Gatekeeper constraints render and policies apply [#17308](https://github.com/deckhouse/deckhouse/pull/17308)
 - **[candi]** Updated the bashible step to include Linux kernel versions that address CVE-2025-37999 [#17300](https://github.com/deckhouse/deckhouse/pull/17300)
 - **[candi]** Make modify_user in add_node_user bashible step idempotent. [#17111](https://github.com/deckhouse/deckhouse/pull/17111)
 - **[candi]** fallback to dnf package manager from yum install and remove bashbooster func's [#17012](https://github.com/deckhouse/deckhouse/pull/17012)
 - **[candi]** bashible 064 step criDir fallback [#16934](https://github.com/deckhouse/deckhouse/pull/16934)
 - **[candi]** bashible events generateName [#16768](https://github.com/deckhouse/deckhouse/pull/16768)
 - **[candi]** Moved the default values for registry in initConfiguration to dhctl [#16103](https://github.com/deckhouse/deckhouse/pull/16103)
 - **[cloud-provider-dynamix]** fixed a queue hang caused by the module components failing to start [#16796](https://github.com/deckhouse/deckhouse/pull/16796)
 - **[cloud-provider-huaweicloud]** fixed a queue hang caused by the module components failing to start [#16796](https://github.com/deckhouse/deckhouse/pull/16796)
 - **[cloud-provider-vcd]** fixed a queue hang caused by the module components failing to start [#16796](https://github.com/deckhouse/deckhouse/pull/16796)
 - **[cloud-provider-zvirt]** fixed a queue hang caused by the module components failing to start [#16796](https://github.com/deckhouse/deckhouse/pull/16796)
 - **[deckhouse]** validate deckhouse-registry secret [#17122](https://github.com/deckhouse/deckhouse/pull/17122)
 - **[deckhouse]** validate deckhouse-registry secret for spaces or newlines. [#16101](https://github.com/deckhouse/deckhouse/pull/16101)
 - **[deckhouse-controller]** Fix stale ModuleConfigurationError metrics not being reset when ModuleRelease is deleted or module is disabled [#16940](https://github.com/deckhouse/deckhouse/pull/16940)
 - **[deckhouse-controller]** Rollback nelm version [#16770](https://github.com/deckhouse/deckhouse/pull/16770)
 - **[deckhouse-controller]** Remove track-termination-mode notation [#16612](https://github.com/deckhouse/deckhouse/pull/16612)
 - **[dhctl]** dhctl delete object node from cluster in converge. [#17163](https://github.com/deckhouse/deckhouse/pull/17163)
 - **[dhctl]** Change ssh logging. [#17143](https://github.com/deckhouse/deckhouse/pull/17143)
 - **[dhctl]** dvp provider.kubeconfigDataBase64 preflight check [#16945](https://github.com/deckhouse/deckhouse/pull/16945)
 - **[dhctl]** Fix --skip-resources flag behaviour in destroy command. [#16904](https://github.com/deckhouse/deckhouse/pull/16904)
 - **[dhctl]** Many fixes in destroy command and restart destroy command. [#16904](https://github.com/deckhouse/deckhouse/pull/16904)
 - **[dhctl]** Fix dhctl bootstrap-phase abort running after dhctl bootstrap-phasebase-infra. [#16829](https://github.com/deckhouse/deckhouse/pull/16829)
 - **[dhctl]** fix kube token handling [#16735](https://github.com/deckhouse/deckhouse/pull/16735)
 - **[docs]** Fix registry-modules-watcher deleting all documentation when registry returns an error [#16771](https://github.com/deckhouse/deckhouse/pull/16771)
 - **[ingress-nginx]** Correct controller termination has been fixed. [#17041](https://github.com/deckhouse/deckhouse/pull/17041)
    restart controllers
 - **[ingress-nginx]** Architecture-bashed node affinity settings are provided. [#16939](https://github.com/deckhouse/deckhouse/pull/16939)
 - **[ingress-nginx]** Fixed the display of IP addresses in the status of Ingress resources with the LoadBalancer type. [#15892](https://github.com/deckhouse/deckhouse/pull/15892)
 - **[monitoring-kubernetes]** Handle unsupported ValidatingAdmissionPolicy API versions on Kubernetes 1.34 [#17007](https://github.com/deckhouse/deckhouse/pull/17007)
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
 - **[registry]** Added bootstrap support [#16103](https://github.com/deckhouse/deckhouse/pull/16103)
 - **[user-authn]** Hide internal error details from users in Dex to prevent information disclosure [#17177](https://github.com/deckhouse/deckhouse/pull/17177)

## Chore


 - **[candi]** Update documentation in candi. [#16933](https://github.com/deckhouse/deckhouse/pull/16933)
 - **[candi]** enrich static nodes with topology labels via /var/lib/node_labels [#16816](https://github.com/deckhouse/deckhouse/pull/16816)
 - **[candi]** bump autoscaler version to 1.32.2 [#16610](https://github.com/deckhouse/deckhouse/pull/16610)
 - **[node-manager]** bump autoscaler version to 1.34 [#16610](https://github.com/deckhouse/deckhouse/pull/16610)
 - **[node-manager]** bumped capi version 1.10.6 > 1.11.3. [#16153](https://github.com/deckhouse/deckhouse/pull/16153)
 - **[prometheus]** Add new rules for grafana dashboards [#15865](https://github.com/deckhouse/deckhouse/pull/15865)
 - **[terraform-manager]** Update terraform-manager images build. [#16941](https://github.com/deckhouse/deckhouse/pull/16941)
 - **[vertical-pod-autoscaler]** Use InPlaceOrRecreate update mode instead of Auto for Deckhouse-managed VPAs. [#17011](https://github.com/deckhouse/deckhouse/pull/17011)
    Control-plane components and kubelets will restart to pick up the new feature gates
    on clusters where they were not enabled before. VPA may start applying in-place
    updates instead of only eviction-based updates.

