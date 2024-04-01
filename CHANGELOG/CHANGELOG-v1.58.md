# Changelog v1.58

## Know before update


 - All Deckhouse containers will restart.
 - Ingress controller will restart.

## Features


 - **[candi]** Allow to customize Yandex Cloud NAT instance resources. [#7309](https://github.com/deckhouse/deckhouse/pull/7309)
 - **[candi]** Add Kubernetes 1.29 support. [#7247](https://github.com/deckhouse/deckhouse/pull/7247)
    All control plane components will restart.
 - **[candi]** Add support for the new cloud provider — VMware Cloud Director. [#6527](https://github.com/deckhouse/deckhouse/pull/6527)
 - **[control-plane-manager]** Option to change service account tokens issuer. [#7892](https://github.com/deckhouse/deckhouse/pull/7892)
 - **[dhctl]** Generate `DeckhouseRelease` manifests for mirrored releases. [#7697](https://github.com/deckhouse/deckhouse/pull/7697)
 - **[documentation]** Module documentation is available in the cluster. [#6449](https://github.com/deckhouse/deckhouse/pull/6449)
 - **[extended-monitoring]** Support custom container registry CA, registry credentials and insecure (HTTP) registries in the image-availability-exporter. Change ModuleConfig settings. [#7354](https://github.com/deckhouse/deckhouse/pull/7354)
 - **[flant-integration]** flant-pricing based on ALT Linux image, grafana-agent and madison-proxy images based on a distroless image. [#6957](https://github.com/deckhouse/deckhouse/pull/6957)
 - **[ingress-nginx]** The controller image is now based on ALT Linux. [#7002](https://github.com/deckhouse/deckhouse/pull/7002)
 - **[kube-dns]** Added a parameter that allows you to change the upstream transport protocol (tcp/udp). [#7541](https://github.com/deckhouse/deckhouse/pull/7541)
 - **[metallb]** MetalLB dashboard for Grafana [#7459](https://github.com/deckhouse/deckhouse/pull/7459)
 - **[multitenancy-manager]** Prevent manual modification of Project resources. [#7480](https://github.com/deckhouse/deckhouse/pull/7480)
 - **[multitenancy-manager]** Remove all namespace from the `Project`, except the project one. [#7443](https://github.com/deckhouse/deckhouse/pull/7443)
 - **[openvpn]** Images are based on a distroless image. [#6245](https://github.com/deckhouse/deckhouse/pull/6245)
 - **[upmeter]** Add the ability to configure some TLS parameters in `UpmeterRemoteWrite` CR. [#7495](https://github.com/deckhouse/deckhouse/pull/7495)

## Fixes


 - **[admission-policy-engine]** Fix audit policy generation error. [#7406](https://github.com/deckhouse/deckhouse/pull/7406)
 - **[candi]** Fix regex validation pattern of the `additionalRolePolicies` field. [#7696](https://github.com/deckhouse/deckhouse/pull/7696)
 - **[candi]** Add the `tzdata` package to the bootstrap script on AltLinux `10.0`. [#7403](https://github.com/deckhouse/deckhouse/pull/7403)
 - **[candi]** Update `containerd` version to `1.7.13`, `runc` version to `1.1.12`.
    Fix error with two sequental containerd restarts on version change. Set LimitNOFILE=1048576. [#7390](https://github.com/deckhouse/deckhouse/pull/7390)
 - **[candi]** Packet forwarding for IPv4 is enabled via sysctl-tuner. [#7278](https://github.com/deckhouse/deckhouse/pull/7278)
 - **[candi]** Fix setting the default value for the `kubelet.resourceReservation` parameter in `NodeGroup`. [#7100](https://github.com/deckhouse/deckhouse/pull/7100)
 - **[ceph-csi]** Avoid listening on all addresses and listen on the host IP address. [#7524](https://github.com/deckhouse/deckhouse/pull/7524)
 - **[cert-manager]** Bind `kube-rbac-proxy` sidecar container to the pod IP address. [#7521](https://github.com/deckhouse/deckhouse/pull/7521)
 - **[chrony]** Avoid listening on all addresses and listen on the host IP address. [#7519](https://github.com/deckhouse/deckhouse/pull/7519)
 - **[chrony]** Fix the incorrect path in the `NTPDaemonOnNodeDoesNotSynchronizeTime` alert. [#7507](https://github.com/deckhouse/deckhouse/pull/7507)
 - **[cloud-provider-vcd]** Change the default network mode for new VMs. [#7875](https://github.com/deckhouse/deckhouse/pull/7875)
 - **[cloud-provider-vcd]** Add validation for a virtual application name. [#7817](https://github.com/deckhouse/deckhouse/pull/7817)
 - **[cni-cilium]** Run `safe_agent_updater` pods in the `hostNetwork` mode and use `kubernetes-api-proxy`. [#7760](https://github.com/deckhouse/deckhouse/pull/7760)
    `cilium-agent` pods will probably restart and L7 policies will flap.
 - **[cni-cilium]** Improve `safe-agent-updater`. [#7576](https://github.com/deckhouse/deckhouse/pull/7576)
    Cilium-agent pods may be reloaded.
 - **[cni-cilium]** Adding safe-agent-updater. [#7333](https://github.com/deckhouse/deckhouse/pull/7333)
    Cilium-agent pods will restart.
 - **[common]** Fix k8s patches for 1.28 and 1.29. [#7537](https://github.com/deckhouse/deckhouse/pull/7537)
 - **[control-plane-manager]** Fix error when `d8-cluster-configuration` secret is missing. [#7861](https://github.com/deckhouse/deckhouse/pull/7861)
 - **[control-plane-manager]** Fixed `D8KubernetesVersionIsDeprecated` alert. [#7768](https://github.com/deckhouse/deckhouse/pull/7768)
 - **[control-plane-manager]** Fix race reading between the deckhouse pod status and the `minUsedControlPlaneKubernetesVersion` variable. [#7637](https://github.com/deckhouse/deckhouse/pull/7637)
    Prevents the Deckhouse version update error from being skipped.
 - **[control-plane-manager]** Fix audit policy generation error. [#7406](https://github.com/deckhouse/deckhouse/pull/7406)
 - **[control-plane-manager]** Automatic Kubernetes version update will be aborted by an error if any resource in the cluster does not support the new Kubernetes version. [#7401](https://github.com/deckhouse/deckhouse/pull/7401)
 - **[control-plane-manager]** Fix serviceaccounts generation in `basicAuditPolicy`. [#7342](https://github.com/deckhouse/deckhouse/pull/7342)
 - **[control-plane-manager]** Remove `/healthz` HTTP endpoint from the `kubernetes-api-proxy-reloader`. [#7311](https://github.com/deckhouse/deckhouse/pull/7311)
 - **[dashboard]** Bind `kube-rbac-proxy` sidecar container to the pod IP address. [#7521](https://github.com/deckhouse/deckhouse/pull/7521)
 - **[deckhouse]** Run `DeckhouseRelease` requirements checks only for enabled modules. [#7925](https://github.com/deckhouse/deckhouse/pull/7925)
 - **[deckhouse]** Сhange the way the `deckhouse` pod readiness is determined during the minor version update. [#7867](https://github.com/deckhouse/deckhouse/pull/7867)
 - **[deckhouse]** Bind `kube-rbac-proxy` sidecar container to the pod IP address. [#7521](https://github.com/deckhouse/deckhouse/pull/7521)
 - **[deckhouse]** Fix release apply on the cluster bootstrap. [#7303](https://github.com/deckhouse/deckhouse/pull/7303)
 - **[deckhouse-controller]** Repeat queries to get CRD and apply conversion strategies. [#7827](https://github.com/deckhouse/deckhouse/pull/7827)
    Prevents a critical error from occurring in the cluster when starting or turning on modules.
 - **[deckhouse-controller]** Add CA certificates to the standard `/etc/ssl/` path. [#7625](https://github.com/deckhouse/deckhouse/pull/7625)
 - **[dhctl]** Prevent deadlock when re-bootstrap cluster. [#7753](https://github.com/deckhouse/deckhouse/pull/7753)
 - **[dhctl]** Avoid mirroring versions in `dhctl mirror` that were not yet released. [#7716](https://github.com/deckhouse/deckhouse/pull/7716)
 - **[dhctl]** Set version field for the `install-data` ConfigMap in the `dhctl bootstrap-phase install-deckhouse` command. [#7493](https://github.com/deckhouse/deckhouse/pull/7493)
 - **[dhctl]** Ignore a master node SSH fingerprint. [#7360](https://github.com/deckhouse/deckhouse/pull/7360)
 - **[docs]** Istio and `admission-policy-engine` restrictions clarifications. [#7398](https://github.com/deckhouse/deckhouse/pull/7398)
 - **[docs]** Add support for Astra Linux `1.7.5`. [#7396](https://github.com/deckhouse/deckhouse/pull/7396)
 - **[extended-monitoring]** Bind `kube-rbac-proxy` sidecar container to the pod IP address. [#7521](https://github.com/deckhouse/deckhouse/pull/7521)
 - **[flant-integration]** Bind `kube-rbac-proxy` sidecar container to the pod IP address. [#7521](https://github.com/deckhouse/deckhouse/pull/7521)
 - **[flow-schema]** Change apiVersion for `FlowSchema` and `PriorityLevelConfiguration` to the minimum available. [#7750](https://github.com/deckhouse/deckhouse/pull/7750)
 - **[helm_lib]** Running check-kernel-version init-container as deckhouse user [#7518](https://github.com/deckhouse/deckhouse/pull/7518)
    All related Pods will be restarted — cilium-agent, node-local-dns, openvpn.
 - **[ingress-nginx]** Fix MaxMind DB download for controller `1.9`. [#7944](https://github.com/deckhouse/deckhouse/pull/7944)
    Ingress-nginx 1.9 controller will restart.
 - **[ingress-nginx]** Add missed libraries to the Ingress controller v1.6 image. [#7764](https://github.com/deckhouse/deckhouse/pull/7764)
    Ingress controller v1.6 will restart.
 - **[ingress-nginx]** Use `go get` instead of `go get -u` to image immutability. [#7726](https://github.com/deckhouse/deckhouse/pull/7726)
 - **[ingress-nginx]** Fix missed libs. [#7717](https://github.com/deckhouse/deckhouse/pull/7717)
    Ingress controller will restart.
 - **[ingress-nginx]** Add libraries to the final image. [#7666](https://github.com/deckhouse/deckhouse/pull/7666)
    Ingress nginx controller will restart.
 - **[ingress-nginx]** Fix `/tmp` access rights for controller v1.6. [#7498](https://github.com/deckhouse/deckhouse/pull/7498)
 - **[istio]** Got rid of `istioMinimalVersion` requirement in `release.yaml` to fix the upgrading issue. [#7815](https://github.com/deckhouse/deckhouse/pull/7815)
 - **[istio]** Fix audit policy generation error. [#7406](https://github.com/deckhouse/deckhouse/pull/7406)
 - **[kube-dns]** Use `go get` instead of `go get -u` to image immutability. [#7726](https://github.com/deckhouse/deckhouse/pull/7726)
 - **[kube-dns]** Bind `kube-rbac-proxy` sidecar container to the pod IP address. [#7521](https://github.com/deckhouse/deckhouse/pull/7521)
 - **[kube-dns]** Increased sts-pods-hosts-appender-webhook wait timeout [#7389](https://github.com/deckhouse/deckhouse/pull/7389)
 - **[local-path-provisioner]** Use `go get` instead of `go get -u` to image immutability. [#7726](https://github.com/deckhouse/deckhouse/pull/7726)
 - **[log-shipper]** Add missing ca-certs to prevent errors with HTTPS connections. [#7686](https://github.com/deckhouse/deckhouse/pull/7686)
 - **[loki]** Use `go get` instead of `go get -u` to image immutability. [#7726](https://github.com/deckhouse/deckhouse/pull/7726)
 - **[metallb]** Bind `kube-rbac-proxy` sidecar container to the pod IP address. [#7521](https://github.com/deckhouse/deckhouse/pull/7521)
 - **[monitoring-custom]** Add the `reserved_domain_nodes` metrics. [#7361](https://github.com/deckhouse/deckhouse/pull/7361)
 - **[monitoring-kubernetes]** Use `go get` instead of `go get -u` to image immutability. [#7726](https://github.com/deckhouse/deckhouse/pull/7726)
 - **[monitoring-kubernetes]** Fix `UnsupportedContainerRuntimeVersion` alert to support the newest containerd versions (`1.7.*`). [#7622](https://github.com/deckhouse/deckhouse/pull/7622)
 - **[monitoring-kubernetes]** Bind `kube-rbac-proxy` sidecar container to the pod IP address. [#7521](https://github.com/deckhouse/deckhouse/pull/7521)
 - **[monitoring-kubernetes-control-plane]** Bind `kube-rbac-proxy` sidecar container to the pod IP address. [#7521](https://github.com/deckhouse/deckhouse/pull/7521)
 - **[multitenancy-manager]** Fix Project rendering in some cases for embedded templates. [#7876](https://github.com/deckhouse/deckhouse/pull/7876)
 - **[multitenancy-manager]** All Pods of a project for which the value "dedicatedNodeName" is defined must run only on the Node with the corresponding taint key. [#6864](https://github.com/deckhouse/deckhouse/pull/6864)
 - **[multitenancy-manager]** Add default project templates:
    - the **default** — to match most cases
    - the **secure** — for advanced secured projects. [#6633](https://github.com/deckhouse/deckhouse/pull/6633)
 - **[multitenancy-manager]** Renames in multitenancy applied and functionality preserved. [#6544](https://github.com/deckhouse/deckhouse/pull/6544)
 - **[network-policy-engine]** Add /run/xtables.lock mount. [#7554](https://github.com/deckhouse/deckhouse/pull/7554)
 - **[node-local-dns]** Fix node local dns readiness probes [#7553](https://github.com/deckhouse/deckhouse/pull/7553)
 - **[node-local-dns]** Bind `kube-rbac-proxy` sidecar container to the pod IP address. [#7521](https://github.com/deckhouse/deckhouse/pull/7521)
 - **[node-manager]** Add permission for _cluster API_ resources for a cluster. [#7893](https://github.com/deckhouse/deckhouse/pull/7893)
 - **[node-manager]** Disable MCM for unsupported cloud providers. [#7817](https://github.com/deckhouse/deckhouse/pull/7817)
 - **[node-manager]** kubelet resource reservation fixes. [#7788](https://github.com/deckhouse/deckhouse/pull/7788)
 - **[node-manager]** Fixed kubelet resource reservation for Static nodes [#7724](https://github.com/deckhouse/deckhouse/pull/7724)
 - **[node-manager]** Set providerID only on Static nodes (fix CloudStatic nodes bootstrap). [#7486](https://github.com/deckhouse/deckhouse/pull/7486)
 - **[node-manager]** Prevent node (with `CloudPermanent` or `Static` type) deletion by autoscaler. [#7339](https://github.com/deckhouse/deckhouse/pull/7339)
 - **[node-manager]** Forbid to change `NodeGroup` if it contains unknown zone. [#7248](https://github.com/deckhouse/deckhouse/pull/7248)
 - **[openvpn]** Add missing ca-certs to prevent errors with HTTPS connections. [#7686](https://github.com/deckhouse/deckhouse/pull/7686)
 - **[openvpn]** Bind `kube-rbac-proxy` sidecar container to the pod IP address. [#7521](https://github.com/deckhouse/deckhouse/pull/7521)
 - **[operator-prometheus]** Use `go get` instead of `go get -u` to image immutability. [#7726](https://github.com/deckhouse/deckhouse/pull/7726)
 - **[operator-prometheus]** Bind `kube-rbac-proxy` sidecar container to the pod IP address. [#7521](https://github.com/deckhouse/deckhouse/pull/7521)
 - **[operator-trivy]** Update trivy ConfigMap template. [#7780](https://github.com/deckhouse/deckhouse/pull/7780)
 - **[operator-trivy]** Use `go get` instead of `go get -u` to image immutability. [#7726](https://github.com/deckhouse/deckhouse/pull/7726)
 - **[operator-trivy]** Bind `kube-rbac-proxy` sidecar container to the pod IP address. [#7521](https://github.com/deckhouse/deckhouse/pull/7521)
 - **[prometheus]** Use `go get` instead of `go get -u` to image immutability. [#7726](https://github.com/deckhouse/deckhouse/pull/7726)
 - **[prometheus]** Fix validating webhook build for promtool work. [#7636](https://github.com/deckhouse/deckhouse/pull/7636)
 - **[prometheus]** Fix Prometheus build to return sample limit patch. [#7636](https://github.com/deckhouse/deckhouse/pull/7636)
 - **[prometheus]** Bind `kube-rbac-proxy` sidecar container to the pod IP address. [#7521](https://github.com/deckhouse/deckhouse/pull/7521)
 - **[prometheus]** Update Prometheus version from `v2.44.0` to `v2.45.2`. [#7195](https://github.com/deckhouse/deckhouse/pull/7195)
 - **[prometheus-metrics-adapter]** Use `go get` instead of `go get -u` to image immutability. [#7726](https://github.com/deckhouse/deckhouse/pull/7726)
 - **[runtime-audit-engine]** Remove the `trusted_sa` macros in Falco rules. [#7241](https://github.com/deckhouse/deckhouse/pull/7241)
 - **[shell_lib]** Fix an error with password generation in shell hooks. [#7548](https://github.com/deckhouse/deckhouse/pull/7548)
 - **[terraform-manager]** Bind `kube-rbac-proxy` sidecar container to the pod IP address. [#7521](https://github.com/deckhouse/deckhouse/pull/7521)
 - **[upmeter]** Bind `kube-rbac-proxy` sidecar container to the pod IP address. [#7521](https://github.com/deckhouse/deckhouse/pull/7521)
 - **[user-authn]** Use `go get` instead of `go get -u` to image immutability. [#7726](https://github.com/deckhouse/deckhouse/pull/7726)
 - **[user-authz]** Fix audit policy generation error. [#7406](https://github.com/deckhouse/deckhouse/pull/7406)

## Chore


 - **[candi]** Bump patch versions of Kubernetes images: `v1.26.14`, `v1.27.11`, `v1.28.7`, `v1.29.2` [#7566](https://github.com/deckhouse/deckhouse/pull/7566)
    Kubernetes control-plane components will restart, kubelet will restart.
 - **[candi]** Set CA permissions more restrictive. [#7367](https://github.com/deckhouse/deckhouse/pull/7367)
 - **[candi]** Use prebuilt dev images. [#7101](https://github.com/deckhouse/deckhouse/pull/7101)
    All Deckhouse containers will restart.
 - **[candi]** Validate `StaticInstance` address is unique. [#6823](https://github.com/deckhouse/deckhouse/pull/6823)
 - **[cilium-hubble]** Build images from sources. [#7305](https://github.com/deckhouse/deckhouse/pull/7305)
    cilium-hubble pods will restart.
 - **[cilium-hubble]** cilium-hubble is now based on distroless images. [#7174](https://github.com/deckhouse/deckhouse/pull/7174)
    cilium-hubble pods will restart.
 - **[cni-cilium]** Remove deprecated annotation from the `safe-agent-updater`. [#7529](https://github.com/deckhouse/deckhouse/pull/7529)
    The `safe-agent-updater` pods will restart.
 - **[cni-cilium]** Remove deprecated agent annotation. [#7461](https://github.com/deckhouse/deckhouse/pull/7461)
    All cilium-agent pods will restart. If you use L7-policies, they will be temporarily non-functional.
 - **[cni-cilium]** cilium-operator is now based on a distroless image. [#7196](https://github.com/deckhouse/deckhouse/pull/7196)
    cilium-operator pods will restart.
 - **[deckhouse-controller]** Update module values if the corresponding moduleSource was updated. [#7096](https://github.com/deckhouse/deckhouse/pull/7096)
 - **[external-module-manager]** Restore modules from ModulePullOverride objects. [#7266](https://github.com/deckhouse/deckhouse/pull/7266)
 - **[ingress-nginx]** Add missing tests. [#7392](https://github.com/deckhouse/deckhouse/pull/7392)
 - **[istio]** Added a pre-upgrade compatibility check for the coming Kubernetes version. [#7712](https://github.com/deckhouse/deckhouse/pull/7712)
 - **[istio]** Bump istio version to `1.19.7`. [#7584](https://github.com/deckhouse/deckhouse/pull/7584)
    The Istio control plane will restart. User workloads will not restart automaticaly, you will have to restart them eventually.
 - **[istio]** Add the ability to use more than 2 replicas of `istiod` pods and autoscaling. [#7421](https://github.com/deckhouse/deckhouse/pull/7421)
 - **[local-path-provisioner]** Bump to version `0.0.26`. [#7283](https://github.com/deckhouse/deckhouse/pull/7283)
 - **[multitenancy-manager]** Don't render the ProjectType resource in the documentation as it has been deprecated. [#7595](https://github.com/deckhouse/deckhouse/pull/7595)
 - **[network-policy-engine]** Deny module setup if the `cni-cilium` module is enabled. [#7687](https://github.com/deckhouse/deckhouse/pull/7687)

