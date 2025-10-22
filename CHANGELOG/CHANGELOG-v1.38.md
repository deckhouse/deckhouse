# Changelog v1.38

## Features


 - **[admission-policy-engine]** New module `admission-policy-engine` with realized [Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/). 
    Security profiles could be activated by setting a label `security.deckhouse.io/pod-policy: restricted` or `security.deckhouse.io/pod-policy: baseline` to the desired Namespace. 
    Added Grafana dashboard `Application/Admission Policy Engine` and the `PodSecurityStandardsViolation` alert. [#2425](https://github.com/deckhouse/deckhouse/pull/2425)
 - **[openvpn]** Added the ability to use bitmasks in `pushToClientRoutes`. Fixed the feature of assigning static addresses for VPN clients. [#2366](https://github.com/deckhouse/deckhouse/pull/2366)

## Fixes


 - **[candi]** Remove `master` taint if `control-plane` taint was removed in a single node installation. [#2816](https://github.com/deckhouse/deckhouse/pull/2816)
 - **[cilium-hubble]** No more saving generated password in ConfigMap, add command to reveal generated password from internal values. [#2133](https://github.com/deckhouse/deckhouse/pull/2133)
 - **[cloud-provider-yandex]** Disable the `csi-snapshotter` module. [#2823](https://github.com/deckhouse/deckhouse/pull/2823)
 - **[deckhouse-controller]** Reworked flow that checks release requirements. [#2785](https://github.com/deckhouse/deckhouse/pull/2785)
 - **[docs]** Updated search indexing for the ability to search by OpenAPI spec parameters. [#2822](https://github.com/deckhouse/deckhouse/pull/2822)
 - **[global-hooks]** Validating for the `publicDomainTemplate` global parameter. [#2415](https://github.com/deckhouse/deckhouse/pull/2415)
 - **[global-hooks]** Refactoring. [#2133](https://github.com/deckhouse/deckhouse/pull/2133)
 - **[ingress-nginx]** Increase Ingress validation webhook timeout. [#2818](https://github.com/deckhouse/deckhouse/pull/2818)
 - **[ingress-nginx]** Fixed Kubernetes / Ingress Nginx Controllers Grafana dashboard. [#2597](https://github.com/deckhouse/deckhouse/pull/2597)
 - **[istio]** Exclude the `d8-upmeter` namespace from the istio discovery process. [#2858](https://github.com/deckhouse/deckhouse/pull/2858)
 - **[istio]** Do not save calculated `globalVersion` (restore it from Service on every startup). Use the common hook in the `generate_passwords` hook. [#2133](https://github.com/deckhouse/deckhouse/pull/2133)
 - **[kube-dns]** Added "prefer_udp" to stub zones. [#2774](https://github.com/deckhouse/deckhouse/pull/2774)
 - **[log-shipper]** Loki fix extra labels. [#2852](https://github.com/deckhouse/deckhouse/pull/2852)
 - **[node-manager]** Avoid "node-role.kubernetes.io/master" taint removal when it is explicitly set in the master NG. [#2837](https://github.com/deckhouse/deckhouse/pull/2837)
 - **[prometheus]** Fixed calculation of PVC size and retention size. [#2918](https://github.com/deckhouse/deckhouse/pull/2918)
 - **[prometheus]** Fixed disk retention size calculation for small disks. [#2825](https://github.com/deckhouse/deckhouse/pull/2825)
 - **[user-authn]** Do not use `connectorData` field of refresh token objects to refresh tokens. [#2685](https://github.com/deckhouse/deckhouse/pull/2685)

## Chore


 - **[istio]** Refreshed documentation on the use of Istio. [#1732](https://github.com/deckhouse/deckhouse/pull/1732)
 - **[istio]** Removed the `tlsMode` parameter and all the resources dependent on it. [#2684](https://github.com/deckhouse/deckhouse/pull/2684)
 - **[istio]** CPU limit for `istio-proxy` sidecars has been removed. [#2579](https://github.com/deckhouse/deckhouse/pull/2579)
 - **[log-shipper]** Bump Vector to `0.24.2`. [#2725](https://github.com/deckhouse/deckhouse/pull/2725)
 - **[log-shipper]** Bump vector to `0.24.1`. [#2613](https://github.com/deckhouse/deckhouse/pull/2613)
 - **[node-manager]** Rename the `D8EarlyOOMPodIsNotReady` alert to the `EarlyOOMPodIsNotReady` alert. [#2590](https://github.com/deckhouse/deckhouse/pull/2590)
 - **[prometheus]** Removed the automatic disk expansion feature. [#1743](https://github.com/deckhouse/deckhouse/pull/1743)

