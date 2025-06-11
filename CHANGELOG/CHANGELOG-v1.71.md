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
 - **[cni-cilium]** Added a traffic encryption mode using WireGuard (`pod-to-pod` and `node-to-node`). [#13749](https://github.com/deckhouse/deckhouse/pull/13749)
 - **[cni-cilium]** Cni-cilium is updated to consider Virtualization Nesting Level when discovering tunnel-port value. [#9996](https://github.com/deckhouse/deckhouse/pull/9996)
 - **[deckhouse-controller]** add readiness probe hook support [#13748](https://github.com/deckhouse/deckhouse/pull/13748)
 - **[deckhouse-controller]** add LTS release channel [#13546](https://github.com/deckhouse/deckhouse/pull/13546)
 - **[dhctl]** add detailed phase/sub-phase progress reporting with JSONL file output or RPC updates from dhctl-server [#13412](https://github.com/deckhouse/deckhouse/pull/13412)
 - **[node-manager]** Add capiEmergencyBrake setting to node-manager ModuleConfig, what can disable CAPI if set to true. [#13614](https://github.com/deckhouse/deckhouse/pull/13614)
 - **[node-manager]** Add version v1alpha2 to SSHCredential, with field sudoPasswordEncoded. [#13346](https://github.com/deckhouse/deckhouse/pull/13346)
 - **[upmeter]** add automatic detection of frozen or growing deckhouse queue in upmeter-agent [#13179](https://github.com/deckhouse/deckhouse/pull/13179)

## Fixes


 - **[candi]** fix double preview version in candi/version_map [#13670](https://github.com/deckhouse/deckhouse/pull/13670)
 - **[candi]** containerd auditd rules [#13560](https://github.com/deckhouse/deckhouse/pull/13560)
 - **[candi]** added ignoring user configuration files for bashible scripts [#13559](https://github.com/deckhouse/deckhouse/pull/13559)
 - **[cilium-hubble]** Added probes for `kube-rbac-proxy`. [#13877](https://github.com/deckhouse/deckhouse/pull/13877)
 - **[cloud-provider-dynamix]** Fix ssh public key absence on CloudEphemeral nodes [#13907](https://github.com/deckhouse/deckhouse/pull/13907)
 - **[cni-cilium]** Added probes for `kube-rbac-proxy`. [#13877](https://github.com/deckhouse/deckhouse/pull/13877)
 - **[cni-cilium]** fixed bug in cilium 1.17 operator priority filter [#13734](https://github.com/deckhouse/deckhouse/pull/13734)
 - **[control-plane-manager]** Use last_over_time to fetch the last available etcd DB size metric if it's missing. [#13682](https://github.com/deckhouse/deckhouse/pull/13682)
 - **[dhctl]** Add Local Registry Configuration Check to Installation Process [#13645](https://github.com/deckhouse/deckhouse/pull/13645)
 - **[istio]** Added probes for `kube-rbac-proxy`. [#13877](https://github.com/deckhouse/deckhouse/pull/13877)
 - **[kube-dns]** Added probes for `kube-rbac-proxy`. [#13877](https://github.com/deckhouse/deckhouse/pull/13877)
 - **[kube-proxy]** Added probes for `kube-rbac-proxy`. [#13877](https://github.com/deckhouse/deckhouse/pull/13877)
 - **[metallb]** BGP grafane dashboard is deployed only when BGP balancing is enabled [#13478](https://github.com/deckhouse/deckhouse/pull/13478)
 - **[node-local-dns]** Added probes for `kube-rbac-proxy`. [#13877](https://github.com/deckhouse/deckhouse/pull/13877)
 - **[node-manager]** Add support scaling from zero to CAPI node groups [#13744](https://github.com/deckhouse/deckhouse/pull/13744)
 - **[openvpn]** Added probes for `kube-rbac-proxy`. [#13877](https://github.com/deckhouse/deckhouse/pull/13877)
 - **[operator-trivy]** added startup probe to trivy-server [#13731](https://github.com/deckhouse/deckhouse/pull/13731)
 - **[service-with-healthchecks]** Added probes for `kube-rbac-proxy`. [#13877](https://github.com/deckhouse/deckhouse/pull/13877)

## Chore


 - **[cilium-hubble]** Upgrade Cilium to 1.17.4. [#12199](https://github.com/deckhouse/deckhouse/pull/12199)
    Pods of `cilium` will be restarted and some features may be added or changed.
 - **[cloud-provider-aws]** Add `stage` to every cloud provider module [#13908](https://github.com/deckhouse/deckhouse/pull/13908)
 - **[cloud-provider-azure]** Add `stage` to every cloud provider module [#13908](https://github.com/deckhouse/deckhouse/pull/13908)
 - **[cloud-provider-dvp]** Add `stage` to every cloud provider module [#13908](https://github.com/deckhouse/deckhouse/pull/13908)
 - **[cloud-provider-dynamix]** Add `stage` to every cloud provider module [#13908](https://github.com/deckhouse/deckhouse/pull/13908)
 - **[cloud-provider-dynamix]** Fixed vulnerabilities and some bugs for cloud-provider-zvirt and cloud-provider-dynamix [#13562](https://github.com/deckhouse/deckhouse/pull/13562)
 - **[cloud-provider-gcp]** Add `stage` to every cloud provider module [#13908](https://github.com/deckhouse/deckhouse/pull/13908)
 - **[cloud-provider-huaweicloud]** Add `stage` to every cloud provider module [#13908](https://github.com/deckhouse/deckhouse/pull/13908)
 - **[cloud-provider-openstack]** Add `stage` to every cloud provider module [#13908](https://github.com/deckhouse/deckhouse/pull/13908)
 - **[cloud-provider-vcd]** Add `stage` to every cloud provider module [#13908](https://github.com/deckhouse/deckhouse/pull/13908)
 - **[cloud-provider-vsphere]** Add `stage` to every cloud provider module [#13908](https://github.com/deckhouse/deckhouse/pull/13908)
 - **[cloud-provider-yandex]** Add `stage` to every cloud provider module [#13908](https://github.com/deckhouse/deckhouse/pull/13908)
 - **[cloud-provider-zvirt]** Add `stage` to every cloud provider module [#13908](https://github.com/deckhouse/deckhouse/pull/13908)
 - **[cloud-provider-zvirt]** Fixed vulnerabilities and some bugs for cloud-provider-zvirt and cloud-provider-dynamix [#13562](https://github.com/deckhouse/deckhouse/pull/13562)
 - **[cni-cilium]** Upgrade Cilium to 1.17.4. [#12199](https://github.com/deckhouse/deckhouse/pull/12199)
    The new version of Cilium requires that the Linux kernel on the nodes be version 5.8 or later. If any of the cluster nodes have a kernel version lower than 5.8, the D8 upgrade will be blocked. Also, pods of `cilium` will be restarted and some features may be added or changed.
 - **[deckhouse]** Liveness and Readiness probes for kube-rbac-proxy [#13957](https://github.com/deckhouse/deckhouse/pull/13957)
 - **[deckhouse]** Liveness and Readiness probes for kube-rbac-proxy [#13696](https://github.com/deckhouse/deckhouse/pull/13696)
 - **[istio]** The .enableHTTP10 and .proxyConfig fields have been moved to the .dataPlane section in the ModuleConfig. [#13435](https://github.com/deckhouse/deckhouse/pull/13435)
 - **[user-authz]** bump golang.org/x/net to v0.40.0 [#13672](https://github.com/deckhouse/deckhouse/pull/13672)
 - **[user-authz]** add CRD to dict [#13622](https://github.com/deckhouse/deckhouse/pull/13622)

