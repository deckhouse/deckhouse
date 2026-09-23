# Changelog v1.76.12

## Know before update


 - The `vpa-recommender` pod is restarted and recommendations are recalculated with the new granularity.
    Memory recommendations will generally become lower (less over-provisioning), CPU recommendations become
    multiples of 10m. VPA objects with `updateMode: Auto`/`Recreate` may evict and recreate pods to apply
    the updated requests.
 - ValidatingAdmissionPolicy reserved-public-hosts-* is removed. Ingresses that were denied because they matched publicDomainTemplate are allowed again. settings.reservedPublicHosts in ModuleConfig deckhouse remains accepted and is ignored. Enforcement returns in a later release.

## Fixes


 - **[deckhouse-controller]** Fixed a bug when PackageRepository always used a HTTPS scheme. [#22564](https://github.com/deckhouse/deckhouse/pull/22564)
 - **[deckhouse]** Fixed goroutine and memory leak in upmeter-agent caused by per-request HTTP clients leaving idle keep-alive connections to Prometheus open forever. [#22480](https://github.com/deckhouse/deckhouse/pull/22480)
 - **[deckhouse]** Stop reserving platform public hostnames at admission; keep the ModuleConfig field so existing settings still validate. [#22533](https://github.com/deckhouse/deckhouse/pull/22533)
    ValidatingAdmissionPolicy reserved-public-hosts-* is removed. Ingresses that were denied because they matched publicDomainTemplate are allowed again. settings.reservedPublicHosts in ModuleConfig deckhouse remains accepted and is ignored. Enforcement returns in a later release.
 - **[local-path-provisioner]** Add wildcard tolerations to the helper pod template so PVC provisioning works on tainted nodes after local-path-provisioner v0.0.32+. [#22585](https://github.com/deckhouse/deckhouse/pull/22585)
 - **[user-authn]** Commander releases older than 1.18 can grant Kubernetes API access on DexClient again. [#22523](https://github.com/deckhouse/deckhouse/pull/22523)
 - **[vertical-pod-autoscaler]** Reduce VPA memory recommendation rounding from 64Mi to 16Mi and round CPU recommendations up to 10m. [#22529](https://github.com/deckhouse/deckhouse/pull/22529)
    The `vpa-recommender` pod is restarted and recommendations are recalculated with the new granularity.
    Memory recommendations will generally become lower (less over-provisioning), CPU recommendations become
    multiples of 10m. VPA objects with `updateMode: Auto`/`Recreate` may evict and recreate pods to apply
    the updated requests.
