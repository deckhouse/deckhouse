# Changelog v1.71

## [MALFORMED]


 - #13095 missing section, missing summary, missing type, unknown section ""
 - #13433 unknown section "static-routing-manager"
 - #13692 missing section, missing summary, missing type, unknown section ""

## Know before update


 - Snapshot-controller module will be restarted while Deckhouse updating.
 - The minimum supported version of Kubernetes is now 1.29. All control plane components will restart.
 - The new version of Cilium requires that the Linux kernel on the nodes be version 5.8 or later. If any of the cluster nodes have a kernel version lower than 5.8, the D8 upgrade will be blocked. Also, pods of `cilium` will be restarted and some features may be added or changed.

## Features


 - **[admission-policy-engine]** Add SecurityPolicy with ability to check images, signed with cosign. For SE+ [#13699](https://github.com/deckhouse/deckhouse/pull/13699)
 - **[candi]** Added support for Kubernetes 1.33 and discontinued support for Kubernetes 1.28. [#13357](https://github.com/deckhouse/deckhouse/pull/13357)
    The minimum supported version of Kubernetes is now 1.29. All control plane components will restart.
 - **[cni-cilium]** Added a traffic encryption mode using WireGuard (`pod-to-pod` and `node-to-node`). [#13749](https://github.com/deckhouse/deckhouse/pull/13749)
 - **[cni-cilium]** Cni-cilium is updated to consider Virtualization Nesting Level when discovering tunnel-port value. [#9996](https://github.com/deckhouse/deckhouse/pull/9996)
 - **[deckhouse-controller]** add readiness probe hook support [#13748](https://github.com/deckhouse/deckhouse/pull/13748)
 - **[deckhouse-controller]** add LTS release channel [#13546](https://github.com/deckhouse/deckhouse/pull/13546)
 - **[dhctl]** add detailed phase/sub-phase progress reporting with JSONL file output or RPC updates from dhctl-server [#13412](https://github.com/deckhouse/deckhouse/pull/13412)
 - **[node-manager]** Add capiEmergencyBrake setting to node-manager ModuleConfig, what can disable CAPI if set to true. [#13614](https://github.com/deckhouse/deckhouse/pull/13614)
 - **[node-manager]** Add version v1alpha2 to SSHCredential, with field sudoPasswordEncoded. [#13346](https://github.com/deckhouse/deckhouse/pull/13346)
 - **[node-manager]** Add systemd shutdown inhibitors to delay system shutdown until Pods with the specific label are gone from the Node. [#12241](https://github.com/deckhouse/deckhouse/pull/12241)
 - **[upmeter]** add automatic detection of frozen or growing deckhouse queue in upmeter-agent [#13179](https://github.com/deckhouse/deckhouse/pull/13179)

## Fixes


 - **[candi]** fix double preview version in candi/version_map [#13670](https://github.com/deckhouse/deckhouse/pull/13670)
 - **[candi]** containerd auditd rules [#13560](https://github.com/deckhouse/deckhouse/pull/13560)
 - **[candi]** added ignoring user configuration files for bashible scripts [#13559](https://github.com/deckhouse/deckhouse/pull/13559)
 - **[cilium-hubble]** Added probes for `kube-rbac-proxy`. [#13877](https://github.com/deckhouse/deckhouse/pull/13877)
 - **[cloud-provider-dynamix]** Fix ssh public key absence on CloudEphemeral nodes [#13907](https://github.com/deckhouse/deckhouse/pull/13907)
 - **[cni-cilium]** Fix build in closed environments [#14094](https://github.com/deckhouse/deckhouse/pull/14094)
 - **[cni-cilium]** Added probes for `kube-rbac-proxy`. [#13877](https://github.com/deckhouse/deckhouse/pull/13877)
 - **[cni-cilium]** fixed bug in cilium 1.17 operator priority filter [#13734](https://github.com/deckhouse/deckhouse/pull/13734)
 - **[control-plane-manager]** Use last_over_time to fetch the last available etcd DB size metric if it's missing. [#13682](https://github.com/deckhouse/deckhouse/pull/13682)
 - **[deckhouse-controller]** add handling required module empty version for module dependency [#14157](https://github.com/deckhouse/deckhouse/pull/14157)
 - **[deckhouse-controller]** Prevent module configuration errors from blocking the entire Deckhouse queue [#13730](https://github.com/deckhouse/deckhouse/pull/13730)
 - **[dhctl]** Add Local Registry Configuration Check to Installation Process [#13645](https://github.com/deckhouse/deckhouse/pull/13645)
 - **[extended-monitoring]** Fix CVEs vulnerabilities x509-certificate-exporter [#13804](https://github.com/deckhouse/deckhouse/pull/13804)
 - **[extended-monitoring]** Fix CVEs vulnerabilities image-availability-exporter [#13802](https://github.com/deckhouse/deckhouse/pull/13802)
 - **[extended-monitoring]** Fix CVEs vulnerabilities events-exporter [#13801](https://github.com/deckhouse/deckhouse/pull/13801)
 - **[extended-monitoring]** Fix CVEs vulnerabilities extended-monitoring-exporter [#13798](https://github.com/deckhouse/deckhouse/pull/13798)
 - **[istio]** Added probes for `kube-rbac-proxy`. [#13877](https://github.com/deckhouse/deckhouse/pull/13877)
 - **[kube-dns]** Added probes for `kube-rbac-proxy`. [#13877](https://github.com/deckhouse/deckhouse/pull/13877)
 - **[kube-proxy]** Added probes for `kube-rbac-proxy`. [#13877](https://github.com/deckhouse/deckhouse/pull/13877)
 - **[loki]** Fix CVEs vulnerabilities loki [#13796](https://github.com/deckhouse/deckhouse/pull/13796)
 - **[metallb]** BGP grafane dashboard is deployed only when BGP balancing is enabled [#13478](https://github.com/deckhouse/deckhouse/pull/13478)
 - **[node-local-dns]** Added probes for `kube-rbac-proxy`. [#13877](https://github.com/deckhouse/deckhouse/pull/13877)
 - **[node-manager]** Add support scaling from zero to CAPI node groups [#13744](https://github.com/deckhouse/deckhouse/pull/13744)
 - **[openvpn]** Added probes for `kube-rbac-proxy`. [#13877](https://github.com/deckhouse/deckhouse/pull/13877)
 - **[operator-prometheus]** Fix CVEs vulnerabilities operator-prometheus [#13792](https://github.com/deckhouse/deckhouse/pull/13792)
 - **[operator-trivy]** added startup probe to trivy-server [#13731](https://github.com/deckhouse/deckhouse/pull/13731)
 - **[pod-reloader]** added probes for `kube-rbac-proxy` in pod-reloader components. [#13874](https://github.com/deckhouse/deckhouse/pull/13874)
 - **[prometheus]** Fix CVEs vulnerabilities grafana [#13947](https://github.com/deckhouse/deckhouse/pull/13947)
 - **[prometheus]** Fix CVEs vulnerabilities prometheus [#13751](https://github.com/deckhouse/deckhouse/pull/13751)
 - **[prometheus]** Fix CVEs vulnerabilities aggregatio-proxy [#13746](https://github.com/deckhouse/deckhouse/pull/13746)
 - **[prometheus]** Fix CVEs vulnerabilities trickster [#13745](https://github.com/deckhouse/deckhouse/pull/13745)
 - **[prometheus]** Fix CVEs vulnerabilities promxy [#13743](https://github.com/deckhouse/deckhouse/pull/13743)
 - **[prometheus]** Fix CVEs vulnerabilities memcached-exporter [#13742](https://github.com/deckhouse/deckhouse/pull/13742)
 - **[prometheus]** Fix CVEs vulnerabilities  alerts receiver [#13740](https://github.com/deckhouse/deckhouse/pull/13740)
 - **[prometheus]** Fix CVEs vulnerabilities alertmanager [#13739](https://github.com/deckhouse/deckhouse/pull/13739)
 - **[prometheus-metrics-adapter]** Fix CVEs vulnerabilities prometheus-metrics-adapter [#13794](https://github.com/deckhouse/deckhouse/pull/13794)
 - **[runtime-audit-engine]** falco build fixes for CSE [#14160](https://github.com/deckhouse/deckhouse/pull/14160)
 - **[service-with-healthchecks]** Added probes for `kube-rbac-proxy`. [#13877](https://github.com/deckhouse/deckhouse/pull/13877)
 - **[service-with-healthchecks]** fixed handling of Pods without IP addresses and corrected initial readiness threshold evaluation. [#12390](https://github.com/deckhouse/deckhouse/pull/12390)
 - **[user-authn]** The logic of label transfer and annotation to secret has been changed for DexClient [#14055](https://github.com/deckhouse/deckhouse/pull/14055)
 - **[user-authz]** fix user-authz hook, rolebinding empty subject namespace [#13756](https://github.com/deckhouse/deckhouse/pull/13756)
    low

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
 - **[deckhouse]** Removed `snapshot-controller` module (`snapshot-controller` external module will be used instead automatically). [#13613](https://github.com/deckhouse/deckhouse/pull/13613)
    Snapshot-controller module will be restarted while Deckhouse updating.
 - **[istio]** The .enableHTTP10 and .proxyConfig fields have been moved to the .dataPlane section in the ModuleConfig. [#13435](https://github.com/deckhouse/deckhouse/pull/13435)
 - **[user-authz]** bump golang.org/x/net to v0.40.0 [#13672](https://github.com/deckhouse/deckhouse/pull/13672)
 - **[user-authz]** add CRD to dict [#13622](https://github.com/deckhouse/deckhouse/pull/13622)

