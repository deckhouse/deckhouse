# Changelog v1.71

## [MALFORMED]


 - #13095 missing section, missing summary, missing type, unknown section ""
 - #13433 unknown section "static-routing-manager"
 - #13692 missing section, missing summary, missing type, unknown section ""

## Know before update


 - The minimum supported version of Kubernetes is now 1.29. All control plane components will restart.
 - The new version of Cilium requires that the Linux kernel on the nodes be version 5.8 or later. If any of the cluster nodes have a kernel version lower than 5.8, the D8 upgrade will be blocked. Also, pods of `cilium` will be restarted and some features may be added or changed.

## Features


 - **[candi]** Added support for Kubernetes 1.33 and discontinued support for Kubernetes 1.28. [#13357](https://github.com/deckhouse/deckhouse/pull/13357)
    The minimum supported version of Kubernetes is now 1.29. All control plane components will restart.
 - **[deckhouse-controller]** add LTS release channel [#13546](https://github.com/deckhouse/deckhouse/pull/13546)
 - **[node-manager]** Add capiEmergencyBrake setting to node-manager ModuleConfig, what can disable CAPI if set to true. [#13614](https://github.com/deckhouse/deckhouse/pull/13614)
 - **[upmeter]** add automatic detection of frozen or growing deckhouse queue in upmeter-agent [#13179](https://github.com/deckhouse/deckhouse/pull/13179)

## Fixes


 - **[candi]** fix double preview version in candi/version_map [#13670](https://github.com/deckhouse/deckhouse/pull/13670)

## Chore


 - **[cilium-hubble]** Upgrade Cilium to 1.17.4. [#12199](https://github.com/deckhouse/deckhouse/pull/12199)
    Pods of `cilium` will be restarted and some features may be added or changed.
 - **[cni-cilium]** Upgrade Cilium to 1.17.4. [#12199](https://github.com/deckhouse/deckhouse/pull/12199)
    The new version of Cilium requires that the Linux kernel on the nodes be version 5.8 or later. If any of the cluster nodes have a kernel version lower than 5.8, the D8 upgrade will be blocked. Also, pods of `cilium` will be restarted and some features may be added or changed.
 - **[user-authz]** bump golang.org/x/net to v0.40.0 [#13672](https://github.com/deckhouse/deckhouse/pull/13672)
 - **[user-authz]** add CRD to dict [#13622](https://github.com/deckhouse/deckhouse/pull/13622)

