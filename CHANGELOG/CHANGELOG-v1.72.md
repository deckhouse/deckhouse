# Changelog v1.72

## [MALFORMED]


 - #14509 unknown section "registry"

## Features


 - **[deckhouse]** Separate queues for critical and functional modules. [#13906](https://github.com/deckhouse/deckhouse/pull/13906)
 - **[deckhouse-controller]** Implement metrics collector library. [#14472](https://github.com/deckhouse/deckhouse/pull/14472)
 - **[deckhouse-controller]** Deckhouse deployment upgrades now use Patch instead of Update [#14311](https://github.com/deckhouse/deckhouse/pull/14311)

## Fixes


 - **[candi]** Fix default AWS subnets tags for LB controller autodiscovery [#10138](https://github.com/deckhouse/deckhouse/pull/10138)
 - **[cloud-provider-vsphere]** fix main network escaping for names with special symbols [#14328](https://github.com/deckhouse/deckhouse/pull/14328)
 - **[istio]** Added api-proxy support for short-lived ServiceAccount tokens. [#14137](https://github.com/deckhouse/deckhouse/pull/14137)
 - **[metallb]** Fixed IP pool exhaustion on LoadBalancer deletion. [#14315](https://github.com/deckhouse/deckhouse/pull/14315)

## Chore


 - **[deckhouse]** Make keepalived and network-policy-engine modules functional [#14521](https://github.com/deckhouse/deckhouse/pull/14521)

