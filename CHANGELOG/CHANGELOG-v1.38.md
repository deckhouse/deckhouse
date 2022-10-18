# Changelog v1.38

## Features


 - **[admission-policy-engine]** New module `admission-policy-engine` with realized [Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/). 
    Security profiles could be activated by setting a label `security.deckhouse.io/pod-policy: restricted` or `security.deckhouse.io/pod-policy: baseline` to the desired Namespace. 
    Added Grafana dashboard `Application/Admission Policy Engine` and the `PodSecurityStandardsViolation` alert. [#2425](https://github.com/deckhouse/deckhouse/pull/2425)
 - **[openvpn]** Added the ability to use bitmasks in `pushToClientRoutes`. Fixed the feature of assigning static addresses for VPN clients. [#2366](https://github.com/deckhouse/deckhouse/pull/2366)

## Fixes


 - **[cilium-hubble]** No more saving generated password in ConfigMap, add command to reveal generated password from internal values. [#2133](https://github.com/deckhouse/deckhouse/pull/2133)
 - **[global-hooks]** Validating for the `publicDomainTemplate` global parameter. [#2415](https://github.com/deckhouse/deckhouse/pull/2415)
 - **[global-hooks]** Refactoring. [#2133](https://github.com/deckhouse/deckhouse/pull/2133)
 - **[ingress-nginx]** Fixed Kubernetes / Ingress Nginx Controllers Grafana dashboard. [#2597](https://github.com/deckhouse/deckhouse/pull/2597)
 - **[istio]** Do not save calculated `globalVersion` (restore it from Service on every startup). Use the common hook in the `generate_passwords` hook. [#2133](https://github.com/deckhouse/deckhouse/pull/2133)
 - **[user-authn]** Do not use `connectorData` field of refresh token objects to refresh tokens. [#2685](https://github.com/deckhouse/deckhouse/pull/2685)

## Chore


 - **[istio]** Removed the `tlsMode` parameter and all the resources dependent on it. [#2684](https://github.com/deckhouse/deckhouse/pull/2684)
 - **[istio]** CPU limit for `istio-proxy` sidecars has been removed. [#2579](https://github.com/deckhouse/deckhouse/pull/2579)
 - **[log-shipper]** Bump Vector to `0.24.2`. [#2725](https://github.com/deckhouse/deckhouse/pull/2725)
 - **[log-shipper]** Bump vector to `0.24.1`. [#2613](https://github.com/deckhouse/deckhouse/pull/2613)
 - **[node-manager]** Rename the `D8EarlyOOMPodIsNotReady` alert to the `EarlyOOMPodIsNotReady` alert. [#2590](https://github.com/deckhouse/deckhouse/pull/2590)
 - **[prometheus]** Removed the automatic disk expansion feature. [#1743](https://github.com/deckhouse/deckhouse/pull/1743)

