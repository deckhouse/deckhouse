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
 - **[cni-cilium]** Cni-cilium is updated to consider Virtualization Nesting Level when discovering tunnel-port value. [#9996](https://github.com/deckhouse/deckhouse/pull/9996)
 - **[deckhouse-controller]** add LTS release channel [#13546](https://github.com/deckhouse/deckhouse/pull/13546)
 - **[dhctl]** add detailed phase/sub-phase progress reporting with JSONL file output or RPC updates from dhctl-server [#13412](https://github.com/deckhouse/deckhouse/pull/13412)
 - **[node-manager]** Add capiEmergencyBrake setting to node-manager ModuleConfig, what can disable CAPI if set to true. [#13614](https://github.com/deckhouse/deckhouse/pull/13614)
 - **[node-manager]** Add version v1alpha2 to SSHCredential, with field sudoPasswordEncoded. [#13346](https://github.com/deckhouse/deckhouse/pull/13346)
 - **[upmeter]** add automatic detection of frozen or growing deckhouse queue in upmeter-agent [#13179](https://github.com/deckhouse/deckhouse/pull/13179)

## Fixes


 - **[candi]** fix double preview version in candi/version_map [#13670](https://github.com/deckhouse/deckhouse/pull/13670)
 - **[candi]** containerd auditd rules [#13560](https://github.com/deckhouse/deckhouse/pull/13560)
 - **[candi]** added ignoring user configuration files for bashible scripts [#13559](https://github.com/deckhouse/deckhouse/pull/13559)
 - **[dhctl]** Add Local Registry Configuration Check to Installation Process [#13645](https://github.com/deckhouse/deckhouse/pull/13645)
 - **[metallb]** BGP grafane dashboard is deployed only when BGP balancing is enabled [#13478](https://github.com/deckhouse/deckhouse/pull/13478)
 - **[node-manager]** Add support scaling from zero to CAPI node groups [#13744](https://github.com/deckhouse/deckhouse/pull/13744)

## Chore


 - **[cilium-hubble]** Upgrade Cilium to 1.17.4. [#12199](https://github.com/deckhouse/deckhouse/pull/12199)
    Pods of `cilium` will be restarted and some features may be added or changed.
 - **[cni-cilium]** Upgrade Cilium to 1.17.4. [#12199](https://github.com/deckhouse/deckhouse/pull/12199)
    The new version of Cilium requires that the Linux kernel on the nodes be version 5.8 or later. If any of the cluster nodes have a kernel version lower than 5.8, the D8 upgrade will be blocked. Also, pods of `cilium` will be restarted and some features may be added or changed.
 - **[user-authz]** bump golang.org/x/net to v0.40.0 [#13672](https://github.com/deckhouse/deckhouse/pull/13672)
 - **[user-authz]** add CRD to dict [#13622](https://github.com/deckhouse/deckhouse/pull/13622)

