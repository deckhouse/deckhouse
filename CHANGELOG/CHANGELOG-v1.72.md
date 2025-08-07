# Changelog v1.72

## [MALFORMED]


 - #14804 unknown section "cloud-provider-vkcloud"

## Know before update


 - kube-apiserver will be restarted to migrate to the AuthenticationConfiguration configuration file

## Features


 - **[cert-manager]** Add recursive settings. [#14669](https://github.com/deckhouse/deckhouse/pull/14669)
 - **[cloud-provider-huaweicloud]** discovery logic so Cluster Autoscaler can create nodes starts with zero replicas [#14835](https://github.com/deckhouse/deckhouse/pull/14835)
 - **[cloud-provider-vsphere]** make mainNetwork optional in Vsphere InstanceClass [#14372](https://github.com/deckhouse/deckhouse/pull/14372)
 - **[cloud-provider-zvirt]** zvirt cloud provider to cse. [#14683](https://github.com/deckhouse/deckhouse/pull/14683)
 - **[control-plane-manager]** Migration to AuthenticationConfiguration config file [#14788](https://github.com/deckhouse/deckhouse/pull/14788)
    kube-apiserver will be restarted to migrate to the AuthenticationConfiguration configuration file
 - **[deckhouse]** Add d8 config validation webhook [#14726](https://github.com/deckhouse/deckhouse/pull/14726)
 - **[deckhouse]** Add experimental flag for modules [#14630](https://github.com/deckhouse/deckhouse/pull/14630)
 - **[deckhouse]** added moduleConfig properties for registry [#14327](https://github.com/deckhouse/deckhouse/pull/14327)
 - **[deckhouse]** Add the edition available and enabled extenders. [#14310](https://github.com/deckhouse/deckhouse/pull/14310)
 - **[deckhouse]** Separate queues for critical and functional modules. [#13906](https://github.com/deckhouse/deckhouse/pull/13906)
 - **[deckhouse-controller]** implement major version update restrictions [#14684](https://github.com/deckhouse/deckhouse/pull/14684)
 - **[deckhouse-controller]** Implement metrics collector library. [#14472](https://github.com/deckhouse/deckhouse/pull/14472)
 - **[deckhouse-controller]** Disabling a module will delete its Pending ModuleReleases [#14463](https://github.com/deckhouse/deckhouse/pull/14463)
 - **[deckhouse-controller]** Add a validating webhook for DeckhouseRelease to prevent approval if requirements are not met [#14365](https://github.com/deckhouse/deckhouse/pull/14365)
 - **[deckhouse-controller]** Deckhouse deployment upgrades now use Patch instead of Update [#14311](https://github.com/deckhouse/deckhouse/pull/14311)
 - **[dhctl]** Add password authentication support to dhctl. [#13240](https://github.com/deckhouse/deckhouse/pull/13240)
 - **[docs]** added the registry module docs [#14327](https://github.com/deckhouse/deckhouse/pull/14327)
 - **[node-local-dns]** The `enableLogs' option has been added to the ModuleConfig, which logs all DNS queries when enabled. [#14672](https://github.com/deckhouse/deckhouse/pull/14672)
 - **[node-manager]** updated go.mod dependencies [#14327](https://github.com/deckhouse/deckhouse/pull/14327)
 - **[registry]** added the registry module [#14327](https://github.com/deckhouse/deckhouse/pull/14327)

## Fixes


 - **[candi]** Fix deletion of NodeUsers. [#13240](https://github.com/deckhouse/deckhouse/pull/13240)
 - **[candi]** Fix default AWS subnets tags for LB controller autodiscovery [#10138](https://github.com/deckhouse/deckhouse/pull/10138)
 - **[cloud-provider-aws]** fix incorrect template id for aws e2e cluster [#14605](https://github.com/deckhouse/deckhouse/pull/14605)
 - **[cloud-provider-dvp]** fix logic of work with disks and coreFraction validation [#14284](https://github.com/deckhouse/deckhouse/pull/14284)
 - **[cloud-provider-vsphere]** refactor datastore discovery, RFC 1123 storage class name normalization [#14519](https://github.com/deckhouse/deckhouse/pull/14519)
 - **[cloud-provider-vsphere]** fix main network escaping for names with special symbols [#14328](https://github.com/deckhouse/deckhouse/pull/14328)
 - **[cloud-provider-zvirt]** use virtio instead of virtio-scsi [#13984](https://github.com/deckhouse/deckhouse/pull/13984)
 - **[cni-cilium]** enabled vlan-bpf-bypass feature to fix extra vlan interfaces issues [#14606](https://github.com/deckhouse/deckhouse/pull/14606)
 - **[control-plane-manager]** promote etcd member if needed [#14661](https://github.com/deckhouse/deckhouse/pull/14661)
 - **[control-plane-manager]** ignore HTTPS_PROXY settings in ETCD client [#14504](https://github.com/deckhouse/deckhouse/pull/14504)
 - **[deckhouse]** Fixed a helm issue with patching arrays in deckhouse deployment. [#14599](https://github.com/deckhouse/deckhouse/pull/14599)
 - **[deckhouse-controller]** Recursive CEL validation for all OpenAPI schema types, including scalars, arrays, and maps (additionalProperties). [#14428](https://github.com/deckhouse/deckhouse/pull/14428)
 - **[docs]** Added documentation for the new registry configuration in Containerd [#14790](https://github.com/deckhouse/deckhouse/pull/14790)
 - **[ingress-nginx]** Ingress validation re-enabled. [#14368](https://github.com/deckhouse/deckhouse/pull/14368)
 - **[istio]** Added api-proxy support for short-lived ServiceAccount tokens. [#14137](https://github.com/deckhouse/deckhouse/pull/14137)
 - **[metallb]** Fixed IP pool exhaustion on LoadBalancer deletion. [#14315](https://github.com/deckhouse/deckhouse/pull/14315)
 - **[node-manager]** fix calculation of memory for standby holder [#14522](https://github.com/deckhouse/deckhouse/pull/14522)
 - **[node-manager]** correct processing of the NodeUser [#14151](https://github.com/deckhouse/deckhouse/pull/14151)
 - **[registry]** Enhance registry configuration in OpenAPI specs [#14509](https://github.com/deckhouse/deckhouse/pull/14509)
 - **[user-authz]** Don't allow disabling multitenancy option if there are CARs using multitenancy in the cluster [#14695](https://github.com/deckhouse/deckhouse/pull/14695)

## Chore


 - **[deckhouse]** Make keepalived and network-policy-engine modules functional [#14521](https://github.com/deckhouse/deckhouse/pull/14521)
 - **[dhctl]** Add native ssh client support to dhctl. [#13240](https://github.com/deckhouse/deckhouse/pull/13240)
 - **[ingress-nginx]** Added a hook that add a finalizer on the IngressNginxController. [#13595](https://github.com/deckhouse/deckhouse/pull/13595)
 - **[node-local-dns]** Caching SERVFAIL responses is disabled. [#14836](https://github.com/deckhouse/deckhouse/pull/14836)
 - **[node-local-dns]** Updated the maximum and minimum TTL values for the success and denial parameters in the core dns cache settings. [#14345](https://github.com/deckhouse/deckhouse/pull/14345)

