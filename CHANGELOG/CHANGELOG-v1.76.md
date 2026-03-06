# Changelog v1.76

## Know before update


 - A new resource, `kubernetes_resource_ready_v1`, has been introduced to perform readiness checks for cloud resources. It replaces the previously used `wait` blocks.
    After updating to this version, the OpenTofu plan will include the creation of `kubernetes_resource_ready_v1` resources and the removal of `wait` blocks. To apply these changes, you must run `converge`.
    The converge operation is safe and does not modify existing cloud resources. In a normal case, the plan should only contain resource creation operations (for example, `Plan: N to add`) and should not include `to change` or `to destroy` actions unless other configuration changes are present.
    During migration, readiness checks are automatically skipped for existing resources older than 5 days. In this case, converge may produce a warning such as the following:
    
    Warning: Resource is too old for checking ready. Skip readiness check.
    
    with module.static-node.kubernetes_resource_ready_v1.vm,
    on ../../../terraform-modules/static-node/main.tf line 138, in resource "kubernetes_resource_ready_v1" "vm":
    138: resource "kubernetes_resource_ready_v1" "vm" {
    
    Resource lifetime is 130h2m20.906973419s. Lifetime for skipping is 120h0m0s.
    
    This warning is expected and does not indicate a problem.
 - Changes were introduced in the OpenTofu integration to support the new `kubernetes_resource_ready_v1` resource used by the `cloud-provider-dvp`.
    These changes prevent unnecessary or destructive plan updates that could occur when data sources depend on readiness-check resources. The behavior of other cloud providers is not affected.
    If you encounter unexpected converge plans or cluster bootstrap issues when using OpenTofu-based providers (such as dvp, dynamics, zvirt, or yandex), please report them to Deckhouse Technical Support.
 - During migration to new implementation of apiserver-proxy, it's possible to flapping connections for apiserver usage.
    Added new exposed hostPort: 6480 for healthchecks and upstreams statistics

## Features


 - **[cloud-provider-dvp]** Add readiness check resource to prevent lost OpenTofu state if resource is not ready. [#18212](https://github.com/deckhouse/deckhouse/pull/18212)
    A new resource, `kubernetes_resource_ready_v1`, has been introduced to perform readiness checks for cloud resources. It replaces the previously used `wait` blocks.
    After updating to this version, the OpenTofu plan will include the creation of `kubernetes_resource_ready_v1` resources and the removal of `wait` blocks. To apply these changes, you must run `converge`.
    The converge operation is safe and does not modify existing cloud resources. In a normal case, the plan should only contain resource creation operations (for example, `Plan: N to add`) and should not include `to change` or `to destroy` actions unless other configuration changes are present.
    During migration, readiness checks are automatically skipped for existing resources older than 5 days. In this case, converge may produce a warning such as the following:
    
    Warning: Resource is too old for checking ready. Skip readiness check.
    
    with module.static-node.kubernetes_resource_ready_v1.vm,
    on ../../../terraform-modules/static-node/main.tf line 138, in resource "kubernetes_resource_ready_v1" "vm":
    138: resource "kubernetes_resource_ready_v1" "vm" {
    
    Resource lifetime is 130h2m20.906973419s. Lifetime for skipping is 120h0m0s.
    
    This warning is expected and does not indicate a problem.
 - **[cloud-provider-dvp]** Fail fast on dhctl operations if resources has incorrect status or conditions (like quota exceeded) with some limitations. [#18212](https://github.com/deckhouse/deckhouse/pull/18212)
 - **[cloud-provider-gcp]** add nestedVirtualization and additionalDisks options to GCPInstanceClass [#18023](https://github.com/deckhouse/deckhouse/pull/18023)
 - **[cloud-provider-huaweicloud]** migrate CAPI provider to cluster-api v1beta2 contract [#17989](https://github.com/deckhouse/deckhouse/pull/17989)
 - **[cni-cilium]** new import/export conntrack http endpoints [#17429](https://github.com/deckhouse/deckhouse/pull/17429)
 - **[dhctl]** Add ability for change default opentofu backend core and provider log levels with TF_LOG_CORE and TF_LOG_PROVIDER envs on run dhctl operations. [#18212](https://github.com/deckhouse/deckhouse/pull/18212)
 - **[dhctl]** Add conversions for ModuleConfigs to dhctl. [#17917](https://github.com/deckhouse/deckhouse/pull/17917)
 - **[node-manager]** replace nginx implementation of apiserver-proxy to native go application with discovery [#17619](https://github.com/deckhouse/deckhouse/pull/17619)
    During migration to new implementation of apiserver-proxy, it's possible to flapping connections for apiserver usage.
    Added new exposed hostPort: 6480 for healthchecks and upstreams statistics
 - **[terraform-manager]** Skip depends_on meta-argument changes for data sources. [#18212](https://github.com/deckhouse/deckhouse/pull/18212)
    Changes were introduced in the OpenTofu integration to support the new `kubernetes_resource_ready_v1` resource used by the `cloud-provider-dvp`.
    These changes prevent unnecessary or destructive plan updates that could occur when data sources depend on readiness-check resources. The behavior of other cloud providers is not affected.
    If you encounter unexpected converge plans or cluster bootstrap issues when using OpenTofu-based providers (such as dvp, dynamics, zvirt, or yandex), please report them to Deckhouse Technical Support.
 - **[user-authn]** Add SAML authentication provider support with refresh tokens and Single Logout (SLO) [#18002](https://github.com/deckhouse/deckhouse/pull/18002)

## Fixes


 - **[candi]** fix CVE in cloud-provider-azure [#18067](https://github.com/deckhouse/deckhouse/pull/18067)
 - **[cloud-provider-azure]** fix CVE in cloud-provider-azure [#18067](https://github.com/deckhouse/deckhouse/pull/18067)
 - **[cloud-provider-gcp]** fix CVEs in cloud-provider-gcp [#18095](https://github.com/deckhouse/deckhouse/pull/18095)
 - **[cloud-provider-huaweicloud]** fix CVEs in cloud-provider-huaweicloud [#18096](https://github.com/deckhouse/deckhouse/pull/18096)
 - **[cloud-provider-openstack]** fix CVE in cloud-provider-openstack module [#18099](https://github.com/deckhouse/deckhouse/pull/18099)
 - **[cloud-provider-vcd]** fix CVE in cloud-provider-vcd module [#18113](https://github.com/deckhouse/deckhouse/pull/18113)
 - **[cloud-provider-zvirt]** fix CVE in cloud-provider-zvirt module [#18115](https://github.com/deckhouse/deckhouse/pull/18115)
 - **[deckhouse]** Fix race in ModuleConfig processing at the start. [#18280](https://github.com/deckhouse/deckhouse/pull/18280)
 - **[deckhouse-controller]** Fix problem when creating the config for the global. [#18161](https://github.com/deckhouse/deckhouse/pull/18161)
 - **[node-manager]** fix go lint errors in node-controller [#18187](https://github.com/deckhouse/deckhouse/pull/18187)
 - **[node-manager]** Fix cluster-autoscaler deadlock when machine creation fails with a non-ResourceExhausted error, preventing scale-up to alternative node groups. [#18154](https://github.com/deckhouse/deckhouse/pull/18154)
 - **[node-manager]** Fix capacity parsing logic for DVPInstanceClass and add test case for DVPSpecWorker [#17935](https://github.com/deckhouse/deckhouse/pull/17935)
    Capacity values (CPU/memory) for DVPInstanceClass are now correctly extracted according to spec shape. Nested `virtualMachine` fields are used and memory quantities like `Gi` are properly parsed.
 - **[registry]** Updated auth image Go dependencies to fix Go CVEs. [#18346](https://github.com/deckhouse/deckhouse/pull/18346)
    Registry pods will be restarted.
 - **[registry]** Updated auth image Go dependencies to fix Go CVEs. [#18234](https://github.com/deckhouse/deckhouse/pull/18234)
    Registry pods will be restarted.
 - **[user-authn]** Disable implicit flow due to security concerns. [#18288](https://github.com/deckhouse/deckhouse/pull/18288)

## Chore


 - **[candi]** Change the way to determinate registry packages proxy addresses during node bootstrap. [#17977](https://github.com/deckhouse/deckhouse/pull/17977)
 - **[candi]** add container-selinux package for selinux policies on rhel based distributions. [#17714](https://github.com/deckhouse/deckhouse/pull/17714)
 - **[cilium-hubble]** open source components versions migrated from werf.inc.yaml to oss.yaml [#18117](https://github.com/deckhouse/deckhouse/pull/18117)
 - **[cloud-provider-aws]** module directory localization [#17740](https://github.com/deckhouse/deckhouse/pull/17740)
 - **[cloud-provider-azure]** module direcotry localization [#17749](https://github.com/deckhouse/deckhouse/pull/17749)
 - **[cloud-provider-dvp]** add ownerReferences to VM-related objects (managed by Terraform) [#16777](https://github.com/deckhouse/deckhouse/pull/16777)
 - **[cloud-provider-dynamix]** module directory localization [#17715](https://github.com/deckhouse/deckhouse/pull/17715)
 - **[cloud-provider-gcp]** module directory localization [#17747](https://github.com/deckhouse/deckhouse/pull/17747)
 - **[cloud-provider-huaweicloud]** module directory localization [#17716](https://github.com/deckhouse/deckhouse/pull/17716)
 - **[cloud-provider-openstack]** module directory localization [#17710](https://github.com/deckhouse/deckhouse/pull/17710)
 - **[cloud-provider-vcd]** module directory localization [#17707](https://github.com/deckhouse/deckhouse/pull/17707)
 - **[cloud-provider-vsphere]** module directory localization [#17718](https://github.com/deckhouse/deckhouse/pull/17718)
 - **[cloud-provider-yandex]** module directory localization [#17743](https://github.com/deckhouse/deckhouse/pull/17743)
 - **[cloud-provider-zvirt]** module directory localization [#17717](https://github.com/deckhouse/deckhouse/pull/17717)
 - **[cni-cilium]** open source components versions migrated from werf.inc.yaml to oss.yaml [#18117](https://github.com/deckhouse/deckhouse/pull/18117)
 - **[cni-flannel]** open source components versions migrated from werf.inc.yaml to oss.yaml [#18117](https://github.com/deckhouse/deckhouse/pull/18117)
 - **[deckhouse-controller]** Convert MUP CRD v1alpha1 not served. [#18222](https://github.com/deckhouse/deckhouse/pull/18222)
 - **[deckhouse-controller]** convert MPO CRD v1alpha1 to not served. [#18010](https://github.com/deckhouse/deckhouse/pull/18010)
 - **[ingress-nginx]** open source components versions migrated from werf.inc.yaml to oss.yaml [#18117](https://github.com/deckhouse/deckhouse/pull/18117)
 - **[istio]** open source components versions migrated from werf.inc.yaml to oss.yaml [#18117](https://github.com/deckhouse/deckhouse/pull/18117)
 - **[keepalived]** open source components versions migrated from werf.inc.yaml to oss.yaml [#18117](https://github.com/deckhouse/deckhouse/pull/18117)
 - **[kube-dns]** disabled DMT-lint for ommited oss.yaml [#18117](https://github.com/deckhouse/deckhouse/pull/18117)
 - **[kube-proxy]** fixed tests of discover_api_endpoints.go [#18270](https://github.com/deckhouse/deckhouse/pull/18270)
 - **[metallb]** open source components versions migrated from werf.inc.yaml to oss.yaml [#18117](https://github.com/deckhouse/deckhouse/pull/18117)
 - **[network-policy-engine]** open source components versions migrated from werf.inc.yaml to oss.yaml [#18117](https://github.com/deckhouse/deckhouse/pull/18117)
 - **[node-manager]** update cluster-api version in caps to v1.11.5 [#17936](https://github.com/deckhouse/deckhouse/pull/17936)
 - **[openvpn]** open source components versions migrated from werf.inc.yaml to oss.yaml [#18117](https://github.com/deckhouse/deckhouse/pull/18117)

