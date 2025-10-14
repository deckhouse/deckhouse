# Changelog v1.71

## [MALFORMED]


 - #13874 unknown section "pod-reloader"
 - #14160 unknown section "runtime-audit-engine"
 - #14673 unknown section "runtime-audit-engine"

## Know before update


 - All Prometheuses pods will be restarted
 - If wireguard interface is present on nodes, then cilium-agent upgrade will stuck. Upgrading the linux kernel to 6.8 is required.
 - Prometheus is replaced by the Deckhouse Prom++ by default in all editions of the Deckhouse Kubernetes Platform
 - Snapshot-controller module will be restarted while Deckhouse updating.
 - The minimum supported version of Kubernetes is now 1.29. All control plane components will restart.
 - The new version of Cilium requires that the Linux kernel on the nodes be version 5.8 or later. If any of the cluster nodes have a kernel version lower than 5.8, the D8 upgrade will be blocked. Also, pods of `cilium` will be restarted and some features may be added or changed.
 - The static-routing-manager module is migrated to provisioning via the deckhouse module source (downloading from the registry).

## Features


 - **[admission-policy-engine]** Added SecurityPolicy with ability to check images, signed with cosign (for SE+). [#13699](https://github.com/deckhouse/deckhouse/pull/13699)
 - **[candi]** Add rosa 12.6. [#14631](https://github.com/deckhouse/deckhouse/pull/14631)
 - **[candi]** Add bashible step to check for upgrade k8s to 1.31 and ask for approval. [#14390](https://github.com/deckhouse/deckhouse/pull/14390)
    Upgrade process on the node will be stopped  until it's not approved.
 - **[candi]** contained configuration && new registry bashible context model [#14147](https://github.com/deckhouse/deckhouse/pull/14147)
 - **[candi]** Added support for Kubernetes 1.33 and discontinued support for Kubernetes 1.28. [#13357](https://github.com/deckhouse/deckhouse/pull/13357)
    The minimum supported version of Kubernetes is now 1.29. All control plane components will restart.
 - **[candi]** Added support for containerd V2. [#12674](https://github.com/deckhouse/deckhouse/pull/12674)
 - **[cni-cilium]** Added optional least-conn load-balancing algorithm for Services. [#13867](https://github.com/deckhouse/deckhouse/pull/13867)
 - **[cni-cilium]** Added a traffic encryption mode using WireGuard (`pod-to-pod` and `node-to-node`). [#13749](https://github.com/deckhouse/deckhouse/pull/13749)
 - **[cni-cilium]** Cni-cilium is updated to consider Virtualization Nesting Level when discovering tunnel-port value. [#9996](https://github.com/deckhouse/deckhouse/pull/9996)
 - **[deckhouse-controller]** add validation for module source changes [#14821](https://github.com/deckhouse/deckhouse/pull/14821)
 - **[deckhouse-controller]** Added user notify when module config has conflict. [#14296](https://github.com/deckhouse/deckhouse/pull/14296)
 - **[deckhouse-controller]** Optimized ModuleRelease update flow. [#14144](https://github.com/deckhouse/deckhouse/pull/14144)
 - **[deckhouse-controller]** Added readiness probe hook support. [#13748](https://github.com/deckhouse/deckhouse/pull/13748)
 - **[deckhouse-controller]** Added LTS release channel. [#13546](https://github.com/deckhouse/deckhouse/pull/13546)
 - **[deckhouse-tools]** yq [#14147](https://github.com/deckhouse/deckhouse/pull/14147)
 - **[dhctl]** new registry bashible context model [#14147](https://github.com/deckhouse/deckhouse/pull/14147)
 - **[dhctl]** Added detailed phase/sub-phase progress reporting with JSONL file output or RPC updates from dhctl-server. [#13412](https://github.com/deckhouse/deckhouse/pull/13412)
 - **[docs]** Added descriptions of the problem of using Cilium and Elastic together [#14830](https://github.com/deckhouse/deckhouse/pull/14830)
    low
 - **[docs]** Added documentation for integrating with the DVP cloud provider. [#13380](https://github.com/deckhouse/deckhouse/pull/13380)
 - **[istio]** Images for 1.21 were refactored to achieve distroless. [#14228](https://github.com/deckhouse/deckhouse/pull/14228)
 - **[istio]** Added Istio version `1.25` (1.25.2). Ambient mesh isn't supported yet. [#12356](https://github.com/deckhouse/deckhouse/pull/12356)
 - **[node-manager]** Implement monitoring of GPU nodes. [#14227](https://github.com/deckhouse/deckhouse/pull/14227)
 - **[node-manager]** new registry bashible context model && registry-bashible-config secret [#14147](https://github.com/deckhouse/deckhouse/pull/14147)
 - **[node-manager]** Added capiEmergencyBrake setting to node-manager ModuleConfig, what can disable CAPI if set to true. [#13614](https://github.com/deckhouse/deckhouse/pull/13614)
 - **[node-manager]** Added version v1alpha2 to SSHCredential, with field sudoPasswordEncoded. [#13346](https://github.com/deckhouse/deckhouse/pull/13346)
 - **[node-manager]** Added systemd shutdown inhibitors to delay system shutdown until Pods with the specific label are gone from the Node. [#12241](https://github.com/deckhouse/deckhouse/pull/12241)
 - **[prometheus]** Deckhouse Prom++ is now the default metrics-collecting software in the Deckhouse Kubernetes Platform [#14330](https://github.com/deckhouse/deckhouse/pull/14330)
    Prometheus is replaced by the Deckhouse Prom++ by default in all editions of the Deckhouse Kubernetes Platform
 - **[upmeter]** Added automatic detection of frozen or growing deckhouse queue in upmeter-agent. [#13179](https://github.com/deckhouse/deckhouse/pull/13179)
 - **[user-authn]** Add allowedGroup in dexProvider OIDC [#14570](https://github.com/deckhouse/deckhouse/pull/14570)

## Fixes


 - **[candi]** Fix arg `encryption-provider-config ` for kubeadm configuration. [#15521](https://github.com/deckhouse/deckhouse/pull/15521)
 - **[candi]** Disable immutable flag on erofs files in cleanup node stage. [#15520](https://github.com/deckhouse/deckhouse/pull/15520)
 - **[candi]** containerd migration fix [#14622](https://github.com/deckhouse/deckhouse/pull/14622)
 - **[candi]** Resolved the issue with downloading packages for external modules using ctr for bb-rp-fetch. [#14236](https://github.com/deckhouse/deckhouse/pull/14236)
 - **[candi]** Fixed double preview version in candi/version_map. [#13670](https://github.com/deckhouse/deckhouse/pull/13670)
 - **[candi]** Added audit rules for containerd. [#13560](https://github.com/deckhouse/deckhouse/pull/13560)
 - **[candi]** Removed influence of root user settings on execution of bashible scripts. [#13559](https://github.com/deckhouse/deckhouse/pull/13559)
 - **[cilium-hubble]** Added probes for `kube-rbac-proxy`. [#13877](https://github.com/deckhouse/deckhouse/pull/13877)
 - **[cloud-provider-aws]** Fix root_block_device converge. [#14764](https://github.com/deckhouse/deckhouse/pull/14764)
 - **[cloud-provider-dynamix]** Fixed `sshPublicKey` absence on CloudEphemeral nodes. [#13907](https://github.com/deckhouse/deckhouse/pull/13907)
 - **[cloud-provider-huaweicloud]** Fixed documentation and examples for huaweicloud provider. [#14225](https://github.com/deckhouse/deckhouse/pull/14225)
 - **[cloud-provider-vsphere]** ignore ept_rvi_mode and hv_mode [#14882](https://github.com/deckhouse/deckhouse/pull/14882)
 - **[cloud-provider-vsphere]** Added /tmp emptyDir for csi-node-legacy. [#14208](https://github.com/deckhouse/deckhouse/pull/14208)
 - **[cni-cilium]** Fixed the infinite loop in the "cilium migration" bashible step and improved synchronization between bashible and safe-agent-updater. [#15262](https://github.com/deckhouse/deckhouse/pull/15262)
 - **[cni-cilium]** Add a compatibility check for the Cilium version and the kernel version, if WireGuard is installed on the node [#15155](https://github.com/deckhouse/deckhouse/pull/15155)
    If wireguard interface is present on nodes, then cilium-agent upgrade will stuck. Upgrading the linux kernel to 6.8 is required.
 - **[cni-cilium]** Added a migration mechanism, which was implemented through the node group disruptive updates with approval. [#14977](https://github.com/deckhouse/deckhouse/pull/14977)
 - **[cni-cilium]** fixed invalid annotation name for lb-algorithm in docs [#14947](https://github.com/deckhouse/deckhouse/pull/14947)
 - **[cni-cilium]** fixed hostPort workability with extraLoadBalancerAlgorithmsEnabled. [#14766](https://github.com/deckhouse/deckhouse/pull/14766)
 - **[cni-cilium]** Fixed least-conn logs when feature is disabled [#14572](https://github.com/deckhouse/deckhouse/pull/14572)
 - **[cni-cilium]** Fix cilium least-conn lb algorithm bugs [#14356](https://github.com/deckhouse/deckhouse/pull/14356)
 - **[cni-cilium]** EgressGateway controller optimized for large setups with lot's of EgressGateways. [#14288](https://github.com/deckhouse/deckhouse/pull/14288)
 - **[cni-cilium]** Fixed CiliumLocalRedirectPolicy working if bpf-lb-algorithm-annotation parameter is enabled. [#14179](https://github.com/deckhouse/deckhouse/pull/14179)
 - **[cni-cilium]** Fixed build in private environments. [#14094](https://github.com/deckhouse/deckhouse/pull/14094)
 - **[cni-cilium]** Added probes for `kube-rbac-proxy`. [#13877](https://github.com/deckhouse/deckhouse/pull/13877)
 - **[cni-cilium]** fFxed bug in cilium 1.17 operator priority filter. [#13734](https://github.com/deckhouse/deckhouse/pull/13734)
 - **[control-plane-manager]** correct indentation of the 'certSANs' in v1beta4 template. [#14554](https://github.com/deckhouse/deckhouse/pull/14554)
 - **[control-plane-manager]** Used last_over_time to fetch the last available etcd DB size metric if it's missing. [#13682](https://github.com/deckhouse/deckhouse/pull/13682)
 - **[deckhouse-controller]** Introduced a new mechanism for handling module readiness probes in Deckhouse. [#14226](https://github.com/deckhouse/deckhouse/pull/14226)
 - **[deckhouse-controller]** Added handling required module empty version for module dependency. [#14157](https://github.com/deckhouse/deckhouse/pull/14157)
 - **[deckhouse-controller]** Prevented module configuration errors from blocking the entire Deckhouse queue. [#13730](https://github.com/deckhouse/deckhouse/pull/13730)
 - **[dhctl]** Fix false-positive staticinstance ip duplication preflight checks fails. [#14163](https://github.com/deckhouse/deckhouse/pull/14163)
 - **[dhctl]** Added local registry configuration check to installation process. [#13645](https://github.com/deckhouse/deckhouse/pull/13645)
 - **[docs]** Added steps that patch secret and prevented the image pull fail. [#15166](https://github.com/deckhouse/deckhouse/pull/15166)
 - **[docs]** Add containerv2 additional registry examples [#15100](https://github.com/deckhouse/deckhouse/pull/15100)
 - **[docs]** Fix D8KubernetesStaleTokensDetected alert description. [#14913](https://github.com/deckhouse/deckhouse/pull/14913)
 - **[docs]** Correction of KeyCloak documentation in d8-user-authn [#14755](https://github.com/deckhouse/deckhouse/pull/14755)
    Users will know how to configure KeyCloak and dexProvider to get user rights in dashboard and console.
 - **[extended-monitoring]** Fixed CVEs vulnerabilities x509-certificate-exporter. [#13804](https://github.com/deckhouse/deckhouse/pull/13804)
 - **[extended-monitoring]** Fixed CVEs vulnerabilities image-availability-exporter. [#13802](https://github.com/deckhouse/deckhouse/pull/13802)
 - **[extended-monitoring]** Fixed CVEs vulnerabilities events-exporter. [#13801](https://github.com/deckhouse/deckhouse/pull/13801)
 - **[extended-monitoring]** Fixed CVEs vulnerabilities extended-monitoring-exporter. [#13798](https://github.com/deckhouse/deckhouse/pull/13798)
 - **[istio]** Added probes for `kube-rbac-proxy`. [#13877](https://github.com/deckhouse/deckhouse/pull/13877)
 - **[kube-dns]** Added probes for `kube-rbac-proxy`. [#13877](https://github.com/deckhouse/deckhouse/pull/13877)
 - **[kube-proxy]** Added probes for `kube-rbac-proxy`. [#13877](https://github.com/deckhouse/deckhouse/pull/13877)
 - **[loki]** Refactor file size evaluator using st_blocks in disk-based retention. [#14112](https://github.com/deckhouse/deckhouse/pull/14112)
    Avoid force-expiration checker reaching size threshold too lately.
 - **[loki]** Fixed CVEs vulnerabilities loki. [#13796](https://github.com/deckhouse/deckhouse/pull/13796)
 - **[metallb]** Fixed overwriting of Service `status` field by module components. [#14664](https://github.com/deckhouse/deckhouse/pull/14664)
 - **[metallb]** avoid double prefixes in the dashboard names. [#14608](https://github.com/deckhouse/deckhouse/pull/14608)
    default
 - **[metallb]** Fixed import logic of MetalLB dashboards. [#13478](https://github.com/deckhouse/deckhouse/pull/13478)
 - **[monitoring-ping]** Reducing PROCESSOR time consumption by changing the method of waiting for incoming packets. [#14502](https://github.com/deckhouse/deckhouse/pull/14502)
 - **[node-local-dns]** Added probes for `kube-rbac-proxy`. [#13877](https://github.com/deckhouse/deckhouse/pull/13877)
 - **[node-manager]** containerd migration fix [#14622](https://github.com/deckhouse/deckhouse/pull/14622)
 - **[node-manager]** Changed draining logic in update_approval [#14646](https://github.com/deckhouse/deckhouse/pull/14646)
 - **[node-manager]** fixed render nvidia-mig-manager [#14560](https://github.com/deckhouse/deckhouse/pull/14560)
 - **[node-manager]** Added support scaling from zero to CAPI node groups. [#13744](https://github.com/deckhouse/deckhouse/pull/13744)
 - **[openvpn]** Resolved false-positive OpenVPNServerCertificateExpired alert triggered when certificate Secret lacks expected label. [#14440](https://github.com/deckhouse/deckhouse/pull/14440)
 - **[openvpn]** Added probes for `kube-rbac-proxy`. [#13877](https://github.com/deckhouse/deckhouse/pull/13877)
 - **[operator-prometheus]** Fixed CVEs vulnerabilities operator-prometheus. [#13792](https://github.com/deckhouse/deckhouse/pull/13792)
 - **[operator-trivy]** Added startup probe to trivy-server. [#13731](https://github.com/deckhouse/deckhouse/pull/13731)
 - **[prometheus]** fix securityContext indentation in the Prometheus main and longterm resources [#15116](https://github.com/deckhouse/deckhouse/pull/15116)
    main and longterm Prometheuses will be rollout-restarted
 - **[prometheus]** use the "deckhouse" ModuleSource as the default for the prompp ModuleConfig [#14612](https://github.com/deckhouse/deckhouse/pull/14612)
    if there are several ModuleSources with the prompp module available, the "deckhouse" ModuleSource will be used.
 - **[prometheus]** Fixed CVEs vulnerabilities mimir. [#14287](https://github.com/deckhouse/deckhouse/pull/14287)
 - **[prometheus]** Fixed CVEs vulnerabilities grafana. [#13947](https://github.com/deckhouse/deckhouse/pull/13947)
 - **[prometheus]** Fixed CVEs vulnerabilities prometheus. [#13751](https://github.com/deckhouse/deckhouse/pull/13751)
 - **[prometheus]** Fixed CVEs vulnerabilities aggregatio-proxy. [#13746](https://github.com/deckhouse/deckhouse/pull/13746)
 - **[prometheus]** Fixed CVEs vulnerabilities trickster. [#13745](https://github.com/deckhouse/deckhouse/pull/13745)
 - **[prometheus]** Fixed CVEs vulnerabilities promxy. [#13743](https://github.com/deckhouse/deckhouse/pull/13743)
 - **[prometheus]** Fixed CVEs vulnerabilities memcached-exporter. [#13742](https://github.com/deckhouse/deckhouse/pull/13742)
 - **[prometheus]** Fixed CVEs vulnerabilities  alerts receiver. [#13740](https://github.com/deckhouse/deckhouse/pull/13740)
 - **[prometheus]** Fixed CVEs vulnerabilities alertmanager. [#13739](https://github.com/deckhouse/deckhouse/pull/13739)
 - **[prometheus-metrics-adapter]** Fixed CVEs vulnerabilities prometheus-metrics-adapter. [#13794](https://github.com/deckhouse/deckhouse/pull/13794)
 - **[service-with-healthchecks]** Added probes for `kube-rbac-proxy`. [#13877](https://github.com/deckhouse/deckhouse/pull/13877)
 - **[service-with-healthchecks]** Fixed handling of pods without IP addresses and corrected initial readiness threshold evaluation. [#12390](https://github.com/deckhouse/deckhouse/pull/12390)
 - **[user-authn]** fix dex oidc connector insecureSkipVerify and rootCAData options [#14524](https://github.com/deckhouse/deckhouse/pull/14524)
 - **[user-authn]** Add TOTP support for static users (can be enabled through the module config). [#14366](https://github.com/deckhouse/deckhouse/pull/14366)
 - **[user-authn]** The logic of label transfer and annotation to secret has been changed for DexClient. [#14055](https://github.com/deckhouse/deckhouse/pull/14055)
 - **[user-authn]** Fixed Dex CVE vulnerabilities. [#13309](https://github.com/deckhouse/deckhouse/pull/13309)
 - **[user-authz]** Fixed user-authz hook, rolebinding empty subject namespace. [#13756](https://github.com/deckhouse/deckhouse/pull/13756)
    low

## Chore


 - **[cilium-hubble]** Upgraded Cilium to 1.17.4. [#12199](https://github.com/deckhouse/deckhouse/pull/12199)
    Pods of `cilium` will be restarted and some features may be added or changed.
 - **[cloud-provider-aws]** Added `stage` to every cloud provider module. [#13908](https://github.com/deckhouse/deckhouse/pull/13908)
 - **[cloud-provider-azure]** Added `stage` to every cloud provider module. [#13908](https://github.com/deckhouse/deckhouse/pull/13908)
 - **[cloud-provider-dvp]** Added `stage` to every cloud provider module. [#13908](https://github.com/deckhouse/deckhouse/pull/13908)
 - **[cloud-provider-dynamix]** Added `stage` to every cloud provider module. [#13908](https://github.com/deckhouse/deckhouse/pull/13908)
 - **[cloud-provider-dynamix]** Fixed vulnerabilities and some bugs for cloud-provider-zvirt and cloud-provider-dynamix. [#13562](https://github.com/deckhouse/deckhouse/pull/13562)
 - **[cloud-provider-gcp]** Added `stage` to every cloud provider module. [#13908](https://github.com/deckhouse/deckhouse/pull/13908)
 - **[cloud-provider-huaweicloud]** Added `stage` to every cloud provider module. [#13908](https://github.com/deckhouse/deckhouse/pull/13908)
 - **[cloud-provider-openstack]** Added `stage` to every cloud provider module. [#13908](https://github.com/deckhouse/deckhouse/pull/13908)
 - **[cloud-provider-vcd]** Added `stage` to every cloud provider module. [#13908](https://github.com/deckhouse/deckhouse/pull/13908)
 - **[cloud-provider-vsphere]** Added `stage` to every cloud provider module. [#13908](https://github.com/deckhouse/deckhouse/pull/13908)
 - **[cloud-provider-yandex]** Added `stage` to every cloud provider module. [#13908](https://github.com/deckhouse/deckhouse/pull/13908)
 - **[cloud-provider-zvirt]** Added `stage` to every cloud provider module. [#13908](https://github.com/deckhouse/deckhouse/pull/13908)
 - **[cloud-provider-zvirt]** Fixed vulnerabilities and some bugs for cloud-provider-zvirt and cloud-provider-dynamix. [#13562](https://github.com/deckhouse/deckhouse/pull/13562)
 - **[cni-cilium]** Upgraded Cilium to 1.17.4. [#12199](https://github.com/deckhouse/deckhouse/pull/12199)
    The new version of Cilium requires that the Linux kernel on the nodes be version 5.8 or later. If any of the cluster nodes have a kernel version lower than 5.8, the D8 upgrade will be blocked. Also, pods of `cilium` will be restarted and some features may be added or changed.
 - **[control-plane-manager]** Set max storage logs depth to 30 days in the documentation. [#14088](https://github.com/deckhouse/deckhouse/pull/14088)
 - **[control-plane-manager]** Updated etcd  to v3.6.1. [#14018](https://github.com/deckhouse/deckhouse/pull/14018)
 - **[deckhouse]** Changed requests and limits for the pod, based on the medium usage. Prevent node OOM in the corner cases. [#14189](https://github.com/deckhouse/deckhouse/pull/14189)
 - **[deckhouse]** Changed Liveness and Readiness probes for kube-rbac-proxy. [#13957](https://github.com/deckhouse/deckhouse/pull/13957)
 - **[deckhouse]** Changed Liveness and Readiness probes for kube-rbac-proxy. [#13696](https://github.com/deckhouse/deckhouse/pull/13696)
 - **[deckhouse]** Removed `snapshot-controller` module (`snapshot-controller` external module will be used instead automatically). [#13613](https://github.com/deckhouse/deckhouse/pull/13613)
    Snapshot-controller module will be restarted while Deckhouse updating.
 - **[deckhouse-controller]** The static-routing-manager module is deleted from the list of embedded modules in favor of downloadable module. [#13433](https://github.com/deckhouse/deckhouse/pull/13433)
    The static-routing-manager module is migrated to provisioning via the deckhouse module source (downloading from the registry).
 - **[docs]** Add containerv2 additional registry examples. [#15081](https://github.com/deckhouse/deckhouse/pull/15081)
 - **[docs]** Added reference for Deckhouse CLI (d8) to the documentation. [#14309](https://github.com/deckhouse/deckhouse/pull/14309)
 - **[docs]** Actualize used port list. [#14271](https://github.com/deckhouse/deckhouse/pull/14271)
 - **[ingress-nginx]** A patch has been added that adds additional logs when downloading GeoIP databases. [#14326](https://github.com/deckhouse/deckhouse/pull/14326)
    ingress-nginx Controllers will be restarted, which could cause traffic interruption.
 - **[istio]** The .enableHTTP10 and .proxyConfig fields have been moved to the .dataPlane section in the ModuleConfig. [#13435](https://github.com/deckhouse/deckhouse/pull/13435)
 - **[log-shipper]** Added extralabels to log and CEF device info into CRD. [#13997](https://github.com/deckhouse/deckhouse/pull/13997)
 - **[monitoring-ping]** Garbage collecting legacy metrics from node-exporter was moved to init-container. [#13542](https://github.com/deckhouse/deckhouse/pull/13542)
 - **[node-manager]** usage Instance in update_approval [#14754](https://github.com/deckhouse/deckhouse/pull/14754)
 - **[node-manager]** Set status value to false for GracefulShutdownPostpone condition on start. [#14636](https://github.com/deckhouse/deckhouse/pull/14636)
 - **[node-manager]** Add profiling for bashible-apiserver [#14465](https://github.com/deckhouse/deckhouse/pull/14465)
 - **[node-manager]** Allowed creating NodeUser CR without passwordHash field. [#13623](https://github.com/deckhouse/deckhouse/pull/13623)
 - **[prometheus]** Made Deckhouse Prom++ available in all editions Deckhouse. [#14223](https://github.com/deckhouse/deckhouse/pull/14223)
    All Prometheuses pods will be restarted
 - **[user-authz]** Bumped golang.org/x/net to v0.40.0. [#13672](https://github.com/deckhouse/deckhouse/pull/13672)
 - **[user-authz]** Added CRD to dict. [#13622](https://github.com/deckhouse/deckhouse/pull/13622)

