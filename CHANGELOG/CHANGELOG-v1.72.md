# Changelog v1.72

## [MALFORMED]


 - #14509 unknown section "registry"

## Features


 - **[cert-manager]** Add recursive settings. [#14669](https://github.com/deckhouse/deckhouse/pull/14669)
 - **[deckhouse]** Add the edition available and enabled extenders. [#14310](https://github.com/deckhouse/deckhouse/pull/14310)
 - **[deckhouse]** Separate queues for critical and functional modules. [#13906](https://github.com/deckhouse/deckhouse/pull/13906)
 - **[deckhouse-controller]** Implement metrics collector library. [#14472](https://github.com/deckhouse/deckhouse/pull/14472)
 - **[deckhouse-controller]** Disabling a module will delete its Pending ModuleReleases [#14463](https://github.com/deckhouse/deckhouse/pull/14463)
 - **[deckhouse-controller]** Deckhouse deployment upgrades now use Patch instead of Update [#14311](https://github.com/deckhouse/deckhouse/pull/14311)

## Fixes


 - **[candi]** Fix default AWS subnets tags for LB controller autodiscovery [#10138](https://github.com/deckhouse/deckhouse/pull/10138)
 - **[cloud-provider-aws]** fix incorrect template id for aws e2e cluster [#14605](https://github.com/deckhouse/deckhouse/pull/14605)
 - **[cloud-provider-vsphere]** fix main network escaping for names with special symbols [#14328](https://github.com/deckhouse/deckhouse/pull/14328)
 - **[cni-cilium]** enabled vlan-bpf-bypass feature to fix extra vlan interfaces issues [#14606](https://github.com/deckhouse/deckhouse/pull/14606)
 - **[deckhouse]** Fixed a helm issue with patching arrays in deckhouse deployment. [#14599](https://github.com/deckhouse/deckhouse/pull/14599)
 - **[deckhouse-controller]** Recursive CEL validation for all OpenAPI schema types, including scalars, arrays, and maps (additionalProperties). [#14428](https://github.com/deckhouse/deckhouse/pull/14428)
 - **[ingress-nginx]** Ingress validation re-enabled. [#14368](https://github.com/deckhouse/deckhouse/pull/14368)
 - **[istio]** Added api-proxy support for short-lived ServiceAccount tokens. [#14137](https://github.com/deckhouse/deckhouse/pull/14137)
 - **[metallb]** Fixed IP pool exhaustion on LoadBalancer deletion. [#14315](https://github.com/deckhouse/deckhouse/pull/14315)

## Chore


 - **[deckhouse]** Make keepalived and network-policy-engine modules functional [#14521](https://github.com/deckhouse/deckhouse/pull/14521)
 - **[ingress-nginx]** Added a hook that add a finalizer on the IngressNginxController. [#13595](https://github.com/deckhouse/deckhouse/pull/13595)
 - **[node-local-dns]** Updated the maximum and minimum TTL values for the success and denial parameters in the core dns cache settings. [#14345](https://github.com/deckhouse/deckhouse/pull/14345)

