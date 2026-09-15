# Changelog v1.73.15

## [MALFORMED]


 - #18950 unknown section "Fix dhctl cloud tests"
 - #19751 missing summary
 - #19751 unknown section "nodeManager"
 - #19751 unknown section "registryPackagesProxy cve fixes"
 - #20137 unknown section "<kebab-case of a module name> | <1st level dir in the repo>"
 - #20375 unknown section "<kebab-case of a module name> | <1st level dir in the repo>"

## Know before update


 - Cilium agents will be restarted during the update.
 - The `local-path-provisioner` Pod is restarted during the update. Custom edits to the `local-path-config` ConfigMap that set unsafe HelperPod fields (privileged, capabilities, host namespaces, initContainers, custom volumes/volumeMounts, container probes/lifecycle, sysctls, etc.) will be rejected by the provisioner at startup. Default Deckhouse installations are unaffected.
 - The `local-path-provisioner` Pod is restarted during the update. PV provisioning/teardown briefly pauses while the new Pod becomes Ready; existing volumes are not affected.
 - The `local-path-provisioner` Pod is restarted during the update. PV provisioning/teardown briefly pauses while the new Pod becomes Ready; existing volumes are not affected. After the update the provisioner refuses to create a HelperPod whose template (loaded from the `local-path-config` ConfigMap) declares privileged containers, hostPath/custom volumes, host namespaces, added Linux capabilities or other security-sensitive fields, so any pre-existing manual override of `helperPod.yaml` that uses one of these fields must be removed before the upgrade.
 - The cilium-hubble components (hubble-ui, hubble-relay) will restart after the update.
 - The cni-cilium components (cilium agent, operator) will restart after the update.
 - The coredns and kube-dns components will restart after the update.
 - The egress-gateway-agent component will restart after the update.
 - The metallb components (controller, speaker, l2lb) will restart after the update.
 - The node-local-dns DaemonSet pods will restart after the update.
 - The node-local-dns components will restart after the update.
 - The service-with-healthchecks components (controller, agent) will restart after the update.
 - This update triggers a rolling update of the flannel pods.
 - This update triggers a rolling update of the kube-proxy pods.
 - This update triggers a rolling update of the network-policy-engine pods.
 - Unsafe custom HelperPod settings in the `local-path-config` ConfigMap are no longer accepted. Default DKP installations are unaffected.
 - When using containerdV2, the performance of istio-cni breaks when mounting internal paths.

## Features


 - **[candi]** Bump deckhouse-cli version up to v0.29.28 [#19228](https://github.com/deckhouse/deckhouse/pull/19228)
 - **[candi]** Bump deckhouse-cli version up to v0.29.29 [#19230](https://github.com/deckhouse/deckhouse/pull/19230)
 - **[istio]** Allow custom ports in metadataEndpoint URLs for IstioFederation and IstioMulticluster CRDs. [#19323](https://github.com/deckhouse/deckhouse/pull/19323)
 - **[node-manager]** Added a release requirement that can block updates while nodes are still running containerd v1.x. [#22025](https://github.com/deckhouse/deckhouse/pull/22025)
 - **[node-manager]** Backported per-GPU custom MIG configurations via `customConfigs` with `partedConfig: custom` support. [#18999](https://github.com/deckhouse/deckhouse/pull/18999)
 - **[operator-trivy]** Add CIS Benchmark reporting on/off and verbosity toggles [#22720](https://github.com/deckhouse/deckhouse/pull/22720)
 - **[user-authn]** Add UserOperation CR (Lock, Unlock, ResetPassword, Reset2FA) for local users and Lock/Unlock for external password-connector accounts. [#19664](https://github.com/deckhouse/deckhouse/pull/19664)

## Fixes


 - **[admission-policy-engine]** Fix cve for ratify [#18927](https://github.com/deckhouse/deckhouse/pull/18927)
 - **[admission-policy-engine]** cve fixes for ratify [#19274](https://github.com/deckhouse/deckhouse/pull/19274)
 - **[candi]** Add cve patches for admission-policy-engine, cert-manager [#18566](https://github.com/deckhouse/deckhouse/pull/18566)
 - **[candi]** Fix cve for admission-policy-engine, cert-manager [#19881](https://github.com/deckhouse/deckhouse/pull/19881)
 - **[candi]** Fix cve for admission-policy-engine, cert-manager, kube-rbac-proxy [#18790](https://github.com/deckhouse/deckhouse/pull/18790)
 - **[candi]** cve fix for user-authn module [#18679](https://github.com/deckhouse/deckhouse/pull/18679)
 - **[candi]** fix cve node-manager and opentofu. [#19947](https://github.com/deckhouse/deckhouse/pull/19947)
 - **[candi]** fix vex for admission-policy-engine and operator-trivy modules [#20004](https://github.com/deckhouse/deckhouse/pull/20004)
 - **[candi]** increase runtimeRequestTimeout for kubelet [#19511](https://github.com/deckhouse/deckhouse/pull/19511)
 - **[cilium-hubble]** Fixed CVE-2026-29181 in hubble-ui-backend  by bumping OpenTelemetry Go to v1.41.0 [#20263](https://github.com/deckhouse/deckhouse/pull/20263)
 - **[cilium-hubble]** Fixed CVE-2026-33186 in the hubble-ui image. [#18721](https://github.com/deckhouse/deckhouse/pull/18721)
 - **[cilium-hubble]** Fixed CVE-2026-41520 in hubble-ui-backend [#20362](https://github.com/deckhouse/deckhouse/pull/20362)
 - **[cilium-hubble]** Upgrade hubble-ui backend dependencies (cilium v1.17.16, Go 1.25.0) and switch build base image to fix known CVEs. [#21646](https://github.com/deckhouse/deckhouse/pull/21646)
    The cilium-hubble components (hubble-ui, hubble-relay) will restart after the update.
 - **[cloud-provider-dvp]** Stopped preferring FQDN to hostname in cloud-init configurations. [#21534](https://github.com/deckhouse/deckhouse/pull/21534)
 - **[cloud-provider-dvp]** Update Go dependencies and version to 1.25.8 [#21679](https://github.com/deckhouse/deckhouse/pull/21679)
 - **[cloud-provider-dvp]** fix cve [#19057](https://github.com/deckhouse/deckhouse/pull/19057)
 - **[cloud-provider-dvp]** fixe cve [#18599](https://github.com/deckhouse/deckhouse/pull/18599)
 - **[cloud-provider-dvp]** make DVP cloud-init provisioning secret creation idempotent to avoid `secret already exists` reconciliation failures [#20168](https://github.com/deckhouse/deckhouse/pull/20168)
 - **[cloud-provider-dvp]** update cloud-controller-manager dependencies and VEX metadata to address current CVE findings [#19272](https://github.com/deckhouse/deckhouse/pull/19272)
 - **[cni-cilium]** Bump Go dependencies and backport upstream cilium security patches to fix known CVEs. [#21646](https://github.com/deckhouse/deckhouse/pull/21646)
    The cni-cilium components (cilium agent, operator) will restart after the update.
 - **[cni-cilium]** Bump Go dependencies in the egress-gateway-agent image to fix known CVEs. [#21614](https://github.com/deckhouse/deckhouse/pull/21614)
    The egress-gateway-agent component will restart after the update.
 - **[cni-cilium]** Fixed CVE-2026-33186, CVE-2026-27142, and CVE-2026-27139 by updating grpc dependency and Go version, and resolved build compatibility issues. [#18616](https://github.com/deckhouse/deckhouse/pull/18616)
 - **[cni-cilium]** Fixed CVE-2026-41520 for cilium-bugtool util [#20269](https://github.com/deckhouse/deckhouse/pull/20269)
 - **[cni-cilium]** Fixed CVE-2026-41520 for the cilium-bugtool util. [#20067](https://github.com/deckhouse/deckhouse/pull/20067)
 - **[cni-cilium]** Updated go-jose dependency to v4.1.4 to fix CVE-2026-34986. [#19013](https://github.com/deckhouse/deckhouse/pull/19013)
    Cilium agents will be restarted during the update.
 - **[cni-flannel]** Fixed CVE-2026-33186 by updating google.golang.org/grpc in flanneld. [#19107](https://github.com/deckhouse/deckhouse/pull/19107)
    This update triggers a rolling update of the flannel pods.
 - **[cni-simple-bridge]** Refactored python image source and pip exclusion. [#19148](https://github.com/deckhouse/deckhouse/pull/19148)
 - **[common]** Fixed CVE-2026-24051 in the CoreDNS image. [#18615](https://github.com/deckhouse/deckhouse/pull/18615)
 - **[common]** Fixed CVE-2026-33186 in the CoreDNS image. [#18724](https://github.com/deckhouse/deckhouse/pull/18724)
    CoreDNS pods will undergo a rolling restart.
 - **[common]** Fixed CVE-2026-40898 in CoreDNS by updating the quic-go dependency. [#20769](https://github.com/deckhouse/deckhouse/pull/20769)
 - **[common]** Go deps are updated. [#21414](https://github.com/deckhouse/deckhouse/pull/21414)
    All kube-rbac-proxy pods will be restarted.
 - **[common]** Removed Python completely from the debug-container image as it is no longer needed, resolving corresponding CVEs, and silenced false positives for etcd binaries via VEX. [#18845](https://github.com/deckhouse/deckhouse/pull/18845)
 - **[common]** fix cve's in docker-registry docker_auth image. [#19360](https://github.com/deckhouse/deckhouse/pull/19360)
 - **[common]** fixed CVE-2026-29181 in the CoreDNS [#20261](https://github.com/deckhouse/deckhouse/pull/20261)
 - **[control-plane-manager]** Added VEX entries for control-plane and registrypackages components; bumped golang.org/x/net, x/sys, and x/crypto in Kubernetes 1.29/1.30/1.31 gomod patches. [#21458](https://github.com/deckhouse/deckhouse/pull/21458)
 - **[control-plane-manager]** Allow change labels and annotations for secret d8-secret-encryption-key [#20150](https://github.com/deckhouse/deckhouse/pull/20150)
 - **[deckhouse-controller]** A module that conditionally depends on another is no longer disabled when an incompatible version of that dependency is enabled; the enable is rejected instead. [#20341](https://github.com/deckhouse/deckhouse/pull/20341)
 - **[deckhouse]** Add VEX entries for CVEs. [#19671](https://github.com/deckhouse/deckhouse/pull/19671)
 - **[deckhouse]** Fix CVEs and update VEX entries. [#19233](https://github.com/deckhouse/deckhouse/pull/19233)
 - **[deckhouse]** Fix CVEs in docs-builder and add cryptography VEX. [#19343](https://github.com/deckhouse/deckhouse/pull/19343)
 - **[deckhouse]** Fix CVEs in webhook-handler and cleanup VEX files. [#19033](https://github.com/deckhouse/deckhouse/pull/19033)
 - **[deckhouse]** Remove notified=false annotation reset from runReleaseDeploy in the module release controller. [#19178](https://github.com/deckhouse/deckhouse/pull/19178)
 - **[dhctl]** Static master cleanup no longer reports success when the cleanup script times out; NodeUser update no longer fails on missing resourceVersion [#20581](https://github.com/deckhouse/deckhouse/pull/20581)
 - **[dhctl]** Wait for stronghold cluster sync before node deletion [#19794](https://github.com/deckhouse/deckhouse/pull/19794)
 - **[dhctl]** dhctl cve fix [#19359](https://github.com/deckhouse/deckhouse/pull/19359)
 - **[dhctl]** dhctl vex upd [#18894](https://github.com/deckhouse/deckhouse/pull/18894)
 - **[dhctl]** mitigate CVE-2026-33186 [#18610](https://github.com/deckhouse/deckhouse/pull/18610)
 - **[extended-monitoring]** always use the default CA bundle in the image-availability-exporter [#20404](https://github.com/deckhouse/deckhouse/pull/20404)
 - **[ingress-nginx]** CVE fix backported. [#21536](https://github.com/deckhouse/deckhouse/pull/21536)
 - **[ingress-nginx]** CVE-2025-15566 is backported. [#19220](https://github.com/deckhouse/deckhouse/pull/19220)
    All pods of Ingress-NGINX controller will be restarted.
 - **[ingress-nginx]** CVEs are fixed and VEX files were added. [#21463](https://github.com/deckhouse/deckhouse/pull/21463)
    All ingress-nginx pods will be restarted.
 - **[ingress-nginx]** CVEs are fixed. [#21517](https://github.com/deckhouse/deckhouse/pull/21517)
 - **[ingress-nginx]** Fixes are backported to 1.73. [#18960](https://github.com/deckhouse/deckhouse/pull/18960)
    All Ingress-NGINX controller pods will be restarted.
 - **[ingress-nginx]** Go deps are updated. [#21414](https://github.com/deckhouse/deckhouse/pull/21414)
    All ingress-nginx pods will be restarted.
 - **[ingress-nginx]** Nginx is updated to 1.30.3. [#20785](https://github.com/deckhouse/deckhouse/pull/20785)
    All ingress-nginx pods will be restarted.
 - **[ingress-nginx]** Nginx is updated to 1.30.4. [#21663](https://github.com/deckhouse/deckhouse/pull/21663)
    All ingress-nginx controller pods will be restarted.
 - **[ingress-nginx]** Nginx is updated up to 1.30.1. [#19848](https://github.com/deckhouse/deckhouse/pull/19848)
    All Ingress-nginx controller pods will be restarted.
 - **[ingress-nginx]** Nginx was updated to 1.30.2. [#20172](https://github.com/deckhouse/deckhouse/pull/20172)
    All Ingress-nginx controller pods will be restarted.
 - **[ingress-nginx]** The regression in nginx 1.30.4 is fixed. [#21727](https://github.com/deckhouse/deckhouse/pull/21727)
    All Ingress-nginx controller pods will be restarted.
 - **[istio]** CNI-node readonly root filesystem enable [#19342](https://github.com/deckhouse/deckhouse/pull/19342)
 - **[istio]** CNI-node readonly root filesystem enable fix [#19690](https://github.com/deckhouse/deckhouse/pull/19690)
    When using containerdV2, the performance of istio-cni breaks when mounting internal paths.
 - **[istio]** July cve fix [#21426](https://github.com/deckhouse/deckhouse/pull/21426)
 - **[istio]** added iptables wrapper in cni-v1x21x6 [#18955](https://github.com/deckhouse/deckhouse/pull/18955)
    istio-cni-nodes will be restarted
 - **[istio]** fixed CVE in istio module v1.19.7 [#18811](https://github.com/deckhouse/deckhouse/pull/18811)
    istio module pods will be restarted
 - **[istio]** fixed CVE in istio module v1.21.6 [#18688](https://github.com/deckhouse/deckhouse/pull/18688)
    istio module pods will be restarted
 - **[istio]** fixed CVE-2026-34986 [#18967](https://github.com/deckhouse/deckhouse/pull/18967)
    istio module pods will be restarted
 - **[istio]** fixed CVE-2026-39882, CVE-2026-39883 and CVE-2026-35206 [#19096](https://github.com/deckhouse/deckhouse/pull/19096)
    istio module pods will be restarted
 - **[istio]** fixed CVEs in module images [#19358](https://github.com/deckhouse/deckhouse/pull/19358)
    module pods will be restarted
 - **[istio]** fixed CVEs in module images [#20008](https://github.com/deckhouse/deckhouse/pull/20008)
 - **[istio]** fixed CVEs in module v1.25.2 images [#18807](https://github.com/deckhouse/deckhouse/pull/18807)
    istio module pods will be restarted
 - **[keepalived]** Excluded vulnerable pip-25.3 from keepalived final image to fix CVE-2026-1703 [#19152](https://github.com/deckhouse/deckhouse/pull/19152)
 - **[kube-dns]** Bump Go dependencies in the sts-pods-hosts-appender-webhook and coredns images to fix known CVEs. [#21633](https://github.com/deckhouse/deckhouse/pull/21633)
    The coredns and kube-dns components will restart after the update.
 - **[kube-proxy]** Fixed CVE-2026-33186 and CVE-2026-24051 in kube-proxy dependencies. [#19120](https://github.com/deckhouse/deckhouse/pull/19120)
    This update triggers a rolling update of the kube-proxy pods.
 - **[local-path-provisioner]** Add wildcard tolerations to the helper pod template so PVC provisioning works on tainted nodes after local-path-provisioner v0.0.32+. [#19449](https://github.com/deckhouse/deckhouse/pull/19449)
 - **[local-path-provisioner]** Backport HelperPod template validation to `local-path-provisioner` v0.0.34 to fix CVE-2026-44543 (HelperPod Template Injection, GHSA-7fxv-8wr2-mfc4, CVSS 8.7 High). [#20329](https://github.com/deckhouse/deckhouse/pull/20329)
    The `local-path-provisioner` Pod is restarted during the update. PV provisioning/teardown briefly pauses while the new Pod becomes Ready; existing volumes are not affected. After the update the provisioner refuses to create a HelperPod whose template (loaded from the `local-path-config` ConfigMap) declares privileged containers, hostPath/custom volumes, host namespaces, added Linux capabilities or other security-sensitive fields, so any pre-existing manual override of `helperPod.yaml` that uses one of these fields must be removed before the upgrade.
 - **[local-path-provisioner]** Bump `local-path-provisioner` to `v0.0.34` to fix CVE-2025-62878 (path traversal via `StorageClass.parameters.pathPattern`, CVSS 10.0). [#19354](https://github.com/deckhouse/deckhouse/pull/19354)
    The `local-path-provisioner` Pod is restarted during the update. PV provisioning/teardown briefly pauses while the new Pod becomes Ready; existing volumes are not affected.
 - **[local-path-provisioner]** Update local-path-provisioner to v0.0.36 to align release-1.73 with the upstream fix for CVE-2026-44543 (HelperPod template injection, CVSS 8.7) instead of carrying a separate backport patch. [#20455](https://github.com/deckhouse/deckhouse/pull/20455)
    The `local-path-provisioner` Pod is restarted during the update. Custom edits to the `local-path-config` ConfigMap that set unsafe HelperPod fields (privileged, capabilities, host namespaces, initContainers, custom volumes/volumeMounts, container probes/lifecycle, sysctls, etc.) will be rejected by the provisioner at startup. Default Deckhouse installations are unaffected.
 - **[local-path-provisioner]** Update local-path-provisioner to v0.0.36 to pick up the upstream fix for CVE-2026-44543 (HelperPod template injection, CVSS 8.7). [#20449](https://github.com/deckhouse/deckhouse/pull/20449)
    Unsafe custom HelperPod settings in the `local-path-config` ConfigMap are no longer accepted. Default DKP installations are unaffected.
 - **[metallb]** Bump Go dependencies in the metallb and l2lb images to fix known CVEs. [#21590](https://github.com/deckhouse/deckhouse/pull/21590)
    The metallb components (controller, speaker, l2lb) will restart after the update.
 - **[network-gateway]** Fixed werf import syntax for compatibility with older werf versions. [#19275](https://github.com/deckhouse/deckhouse/pull/19275)
 - **[network-gateway]** Updated dnsmasq to v2.92-alt2 to address multiple security vulnerabilities (CVE-2026-*) [#19936](https://github.com/deckhouse/deckhouse/pull/19936)
 - **[network-gateway]** Updated python image source and mitigated pip CVE-2026-1703 [#19149](https://github.com/deckhouse/deckhouse/pull/19149)
 - **[network-policy-engine]** Fixed CVE-2026-34040, CVE-2026-33997, and CVE-2026-33186 in network-policy-engine dependencies. [#19109](https://github.com/deckhouse/deckhouse/pull/19109)
    This update triggers a rolling update of the network-policy-engine pods.
 - **[node-local-dns]** Bump Go dependencies in the coredns helper image to fix known CVEs. [#21890](https://github.com/deckhouse/deckhouse/pull/21890)
    The node-local-dns DaemonSet pods will restart after the update.
 - **[node-local-dns]** Bump Go dependencies in the safe-updater and stale-dns-connections-cleaner images to fix known CVEs. [#21633](https://github.com/deckhouse/deckhouse/pull/21633)
    The node-local-dns components will restart after the update.
 - **[node-local-dns]** Return stale-dns-connections-cleaner [#18755](https://github.com/deckhouse/deckhouse/pull/18755)
    An additional service daemonset will be added.
 - **[node-manager]** Fixed TLS vulnerabilities for capi-controller-manager [#20137](https://github.com/deckhouse/deckhouse/pull/20137)
 - **[node-manager]** Issue the CAPI webhook certificate so strict TLS validators accept it (distinct CA Subject, `server auth` EKU); legacy certificates are re-issued, restarting `capi-controller-manager` [#20349](https://github.com/deckhouse/deckhouse/pull/20349)
    The `capi-controller-manager` pod will be restarted to pick up the re-issued webhook certificate
 - **[node-manager]** Mitigate multiple CVEs [#18867](https://github.com/deckhouse/deckhouse/pull/18867)
 - **[node-manager]** fixed cve in caps [#18892](https://github.com/deckhouse/deckhouse/pull/18892)
 - **[node-manager]** mitigate CVE-2026-33186 [#18649](https://github.com/deckhouse/deckhouse/pull/18649)
 - **[operator-trivy]** Backported CVE patches from external module incarnation [#18879](https://github.com/deckhouse/deckhouse/pull/18879)
 - **[prometheus]** Graceful aggregating-proxy rollout [#21335](https://github.com/deckhouse/deckhouse/pull/21335)
 - **[registrypackages]** Added `which` to RPP. [#19069](https://github.com/deckhouse/deckhouse/pull/19069)
 - **[registrypackages]** Added vex with CVE-2026-33186 for kubeadm and kubelet. [#18783](https://github.com/deckhouse/deckhouse/pull/18783)
 - **[registrypackages]** Added vex with CVE-2026-33186. [#18700](https://github.com/deckhouse/deckhouse/pull/18700)
 - **[registrypackages]** Fix last changes in go.mod patch [#19880](https://github.com/deckhouse/deckhouse/pull/19880)
 - **[registrypackages]** Rebuild kubernetes-cni with updated Go dependencies to fix CVEs. [#22069](https://github.com/deckhouse/deckhouse/pull/22069)
 - **[registrypackages]** Updated registrypackages/docker-registry image Go dependencies to fix Go CVEs. [#20375](https://github.com/deckhouse/deckhouse/pull/20375)
 - **[service-with-healthchecks]** Bump Go dependencies in the service-with-healthchecks image to fix known CVEs. [#21594](https://github.com/deckhouse/deckhouse/pull/21594)
    The service-with-healthchecks components (controller, agent) will restart after the update.
 - **[terraform-manager]** Fixed critical CVEs in dependencies across all cloud providers [#18599](https://github.com/deckhouse/deckhouse/pull/18599)
 - **[terraform-manager]** Implemented automatic VEX file merging from dhctl into terraform-manager images [#18892](https://github.com/deckhouse/deckhouse/pull/18892)
 - **[terraform-manager]** bump moby/spdystream to v0.5.1 in terraform-manager-dvp image to fix CVE [#19722](https://github.com/deckhouse/deckhouse/pull/19722)
 - **[terraform-manager]** fixed cve in terraform manager [#18892](https://github.com/deckhouse/deckhouse/pull/18892)
 - **[terraform-manager]** fixed cve in terraform manager and update version in terraform-manager [#18800](https://github.com/deckhouse/deckhouse/pull/18800)
 - **[upmeter]** Add proper securityContext to the upmeter probe to meet the restricted security profile constraints. [#18743](https://github.com/deckhouse/deckhouse/pull/18743)

## Chore


 - **[admission-policy-engine]** Updated VEX entries for trivy-provider [#18951](https://github.com/deckhouse/deckhouse/pull/18951)
 - **[cilium-hubble]** Added vex with CVE-2026-33726 for hubble [#18922](https://github.com/deckhouse/deckhouse/pull/18922)
 - **[cilium-hubble]** Fixed vex file [#19016](https://github.com/deckhouse/deckhouse/pull/19016)
 - **[cni-cilium]** Added vex with CVE-2026-33726 for hubble [#18922](https://github.com/deckhouse/deckhouse/pull/18922)
 - **[cni-cilium]** Fixed vex file [#19016](https://github.com/deckhouse/deckhouse/pull/19016)
 - **[cni-cilium]** Refactor build to use pre-packaged dependencies from envoyproxy_deps repository instead of downloading from GitHub at build time [#18941](https://github.com/deckhouse/deckhouse/pull/18941)
    Cilium agents will be restarted.
 - **[common]** Remove `etcdctl` and `etcdutl` from the debug container image. [#21993](https://github.com/deckhouse/deckhouse/pull/21993)
 - **[docs]** Upgrade Hugo to v0.162.1 with API migration. [#20253](https://github.com/deckhouse/deckhouse/pull/20253)
 - **[ingress-nginx]** Shrinking 1.10 image is backported. [#19518](https://github.com/deckhouse/deckhouse/pull/19518)
    All pods of the 1.10 Ingress-NGINX controller will be restarted.
 - **[istio]** Fixed CVE-2026-46680 in operator 1.25 [#20209](https://github.com/deckhouse/deckhouse/pull/20209)
 - **[istio]** fixes for dmt lint↓ [#21489](https://github.com/deckhouse/deckhouse/pull/21489)
 - **[istio]** set readOnlyRootFilesystem to true in istio-init container [#21577](https://github.com/deckhouse/deckhouse/pull/21577)
 - **[operator-trivy]** Fix CVE [#19226](https://github.com/deckhouse/deckhouse/pull/19226)
 - **[operator-trivy]** Fixed CVE's [#19908](https://github.com/deckhouse/deckhouse/pull/19908)
 - **[operator-trivy]** Use updated trivy patches [#19340](https://github.com/deckhouse/deckhouse/pull/19340)
 - **[registry]** Update dependencies to fix CVEs [#18621](https://github.com/deckhouse/deckhouse/pull/18621)
 - **[registrypackages]** Add vex for etcdl/etcdutil, crictl, containerd. [#18784](https://github.com/deckhouse/deckhouse/pull/18784)
 - **[registrypackages]** Fixed CVEs through vex files [#19014](https://github.com/deckhouse/deckhouse/pull/19014)
 - **[registrypackages]** Update containerd V1, V2, runc and several dependencies in cri-tools to address multiple CVEs. [#21452](https://github.com/deckhouse/deckhouse/pull/21452)
