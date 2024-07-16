# Changelog v1.62

## Know before update


 - Ingress controller v1.10 will restart.
 - The following system pods will restart:
    * node-local-dns,
    * cloud-provider-*,
    * runtime-audit-engine,
    * metallb,
    * cilium-agent,
    * kube-proxy,
    * registry-packages-proxy,
    * bashible-apiserver,
    * capi-controller-manager,
    * machine-controller-manager,
    * network-policy-engine,
    * ingress-nginx with HostPortWithFailover inlet,
    * runtime-audit-engine.
    Note that you will need to change the access policies on the firewalls before upgrading the cluster.
 - The minimum supported Kubernetes version is 1.26.
 - Yandex Cloud `ru-central1-c` zone has been removed from terraform. If you still use `ru-central1-c` zone in Yandex Cloud, you need to manually run `dhctl converge` command to remove subnets from the cloud.
 - kubelet and kube-proxy will restart.

## Features


 - **[candi]** Added support for Rosa Server OS. [#8666](https://github.com/deckhouse/deckhouse/pull/8666)
 - **[candi]** Build image for standalone run of Deckhouse installer. [#8613](https://github.com/deckhouse/deckhouse/pull/8613)
 - **[candi]** Added function to create user and group with specified IDs, logging warnings if they already exist or IDs are taken. [#8595](https://github.com/deckhouse/deckhouse/pull/8595)
    Ensures consistent user and group creation across different environments.
 - **[candi]** Add Kubernetes 1.30 support. [#8525](https://github.com/deckhouse/deckhouse/pull/8525)
    All control plane components will restart.
 - **[candi]** Remove support Kubernetes 1.25. [#8525](https://github.com/deckhouse/deckhouse/pull/8525)
    The minimum supported Kubernetes version is 1.26.
 - **[candi]** Remove deprecated `ru-central1-c` zone from terraform. [#8442](https://github.com/deckhouse/deckhouse/pull/8442)
    Yandex Cloud `ru-central1-c` zone has been removed from terraform. If you still use `ru-central1-c` zone in Yandex Cloud, you need to manually run `dhctl converge` command to remove subnets from the cloud.
 - **[candi]** Use statically linked binaries for most common package-dependencies of cluster components. [#8241](https://github.com/deckhouse/deckhouse/pull/8241)
    kubelet and kube-proxy will restart.
 - **[cloud-provider-openstack]** Add support for the [ConfigDrive](https://deckhouse.io/documentation/v1.62/modules/030-cloud-provider-openstack/cluster_configuration.html#openstackclusterconfiguration-nodegroups-instanceclass-configdrive) parameter in mcm and `SimpleWithInternalNetwork` layout. [#8733](https://github.com/deckhouse/deckhouse/pull/8733)
 - **[cloud-provider-vsphere]** Update CSI version. [#8525](https://github.com/deckhouse/deckhouse/pull/8525)
 - **[cloud-provider-vsphere]** Disable root reservation for PVC. [#8525](https://github.com/deckhouse/deckhouse/pull/8525)
 - **[cloud-provider-yandex]** Update CSI driver to disable root reservation. [#8761](https://github.com/deckhouse/deckhouse/pull/8761)
 - **[control-plane-manager]** Specify `service-account-jwks-uri` flag in kubernetes-apiserver if a custom issuer is provided. [#8712](https://github.com/deckhouse/deckhouse/pull/8712)
 - **[deckhouse]** Restrict creating system namespaces. [#8696](https://github.com/deckhouse/deckhouse/pull/8696)
 - **[deckhouse]** Set `clusterDomain` from `global.clusterConfiguration.clusterDomain`. [#8671](https://github.com/deckhouse/deckhouse/pull/8671)
 - **[deckhouse-controller]** Add a convenient way of resolving which _deckhouse_ pod is a current leader. [#8720](https://github.com/deckhouse/deckhouse/pull/8720)
 - **[deckhouse-controller]** Hide some sensitive data in debug output. [#8612](https://github.com/deckhouse/deckhouse/pull/8612)
 - **[dhctl]** dhctl will now check if required python modules are installed on the node before bootstrapping. [#8867](https://github.com/deckhouse/deckhouse/pull/8867)
 - **[dhctl]** Add new commander/detach operation, add commander-uuid option for all commander operations. [#8746](https://github.com/deckhouse/deckhouse/pull/8746)
 - **[dhctl]** Build image for standalone run of Deckhouse installer. [#8613](https://github.com/deckhouse/deckhouse/pull/8613)
 - **[ingress-nginx]** Bump nginx to `v1.25.5` in Ingress controller `v1.10`. [#8675](https://github.com/deckhouse/deckhouse/pull/8675)
    Ingress controller v1.10 will restart.
 - **[log-shipper]** Add an ability to send logs via socket (TCP/UDP). 
    Messages can be formatted as text, JSON, CEF, or Syslog. [#8870](https://github.com/deckhouse/deckhouse/pull/8870)
 - **[node-manager]** Exclude machines from balancing after drain-delay. [#8617](https://github.com/deckhouse/deckhouse/pull/8617)
 - **[node-manager]** Build image for standalone run of Deckhouse installer. [#8613](https://github.com/deckhouse/deckhouse/pull/8613)
 - **[prometheus]** System-independent Probes. [#8771](https://github.com/deckhouse/deckhouse/pull/8771)
 - **[runtime-audit-engine]** Add VPA settings. [#8703](https://github.com/deckhouse/deckhouse/pull/8703)
 - **[terraform-manager]** Build image for standalone run of Deckhouse installer. [#8613](https://github.com/deckhouse/deckhouse/pull/8613)
 - **[user-authn]** Update dex to `v2.40.0`. [#8686](https://github.com/deckhouse/deckhouse/pull/8686)

## Fixes


 - **[candi]** Fix home directory permission for NodeUser. [#9030](https://github.com/deckhouse/deckhouse/pull/9030)
    default
 - **[candi]** Fix AWS identity for a EBS device. [#8951](https://github.com/deckhouse/deckhouse/pull/8951)
    low
 - **[candi]** Fix regex pattern for `httpProxy` to allow using reserved characters. [#8794](https://github.com/deckhouse/deckhouse/pull/8794)
 - **[candi]** Updated  `bootstrap-network` script to determine the primary network interface from the `50-cloud-init.yaml` file, with fallback to regex matching if MAC address is missing. [#8755](https://github.com/deckhouse/deckhouse/pull/8755)
    Ensures correct network interface identification and configuration in all scenarios.
 - **[candi]** Clean up units created by registry packages install scripts. [#8701](https://github.com/deckhouse/deckhouse/pull/8701)
 - **[candi]** Disable `systemd-gpt-auto-generator`, which automatically detects swap partition in GPT and activates it. [#8680](https://github.com/deckhouse/deckhouse/pull/8680)
 - **[candi]** Fix patch application for all Kubernetes versions. [#8525](https://github.com/deckhouse/deckhouse/pull/8525)
    Components that use _kube-rbac-proxy_ will restart.
 - **[cloud-provider-openstack]** Add the `--tls-cipher-suites` option to the cloud-controller-manager deployment. [#8820](https://github.com/deckhouse/deckhouse/pull/8820)
 - **[cloud-provider-vsphere]** Fix bootstrap to existing folder. [#8478](https://github.com/deckhouse/deckhouse/pull/8478)
 - **[cloud-provider-yandex]** Change default root disk size for master and cloud permanent nodes to 50 GB [#8421](https://github.com/deckhouse/deckhouse/pull/8421)
 - **[cni-cilium]** Add `CiliumAgentMetricNotFound` Prometheus alert. [#8751](https://github.com/deckhouse/deckhouse/pull/8751)
 - **[deckhouse]** Restore ability to edit global ModuleConfig in cases with disabled kube-dns module. [#8932](https://github.com/deckhouse/deckhouse/pull/8932)
    default
 - **[deckhouse]** Clarify `ValidatingAdmissionPolicy` for objects with label `heritage: deckhouse`. [#8819](https://github.com/deckhouse/deckhouse/pull/8819)
 - **[deckhouse]** registry-packages-proxy revision. [#8796](https://github.com/deckhouse/deckhouse/pull/8796)
 - **[deckhouse]** Fix ValidatingAdmissionPolicy for objects with the label `heritage: deckhouse`. [#8778](https://github.com/deckhouse/deckhouse/pull/8778)
 - **[dhctl]** Fix a preflight check for python breaking without `python` symlink installed. [#8890](https://github.com/deckhouse/deckhouse/pull/8890)
 - **[dhctl]** Set right path for terrafrom plugins. [#8831](https://github.com/deckhouse/deckhouse/pull/8831)
 - **[dhctl]** Fixed bootstrap on systems with umask `027/077`. [#8622](https://github.com/deckhouse/deckhouse/pull/8622)
 - **[dhctl]** Fix incorrect error handling. [#8506](https://github.com/deckhouse/deckhouse/pull/8506)
 - **[docs]** The outdated `--resources` flag has been replaced by `--config` in the Getting Started. Update kind-d8.sh to use newer KIND and Kubectl versions. [#8775](https://github.com/deckhouse/deckhouse/pull/8775)
 - **[go_lib]** Fix working registry packages proxy with insecure registries(HTTP). [#8891](https://github.com/deckhouse/deckhouse/pull/8891)
 - **[log-shipper]** Delete lock files before sending a signal to vector to update the configuration. [#8730](https://github.com/deckhouse/deckhouse/pull/8730)
 - **[monitoring-kubernetes]** Fix node exporter NTP address. [#9016](https://github.com/deckhouse/deckhouse/pull/9016)
    node-exporter will restart.
 - **[monitoring-kubernetes]** Fix false-positive results in precomputed metric `oom_kills:normalized`. [#8592](https://github.com/deckhouse/deckhouse/pull/8592)
 - **[multitenancy-manager]** Replace special characters in a AuthorizationRule `metadata.name`. [#8665](https://github.com/deckhouse/deckhouse/pull/8665)
 - **[operator-trivy]** Fix service URL to work in env where HTTP_PROXY/HTTPS_PROXY is set. [#8958](https://github.com/deckhouse/deckhouse/pull/8958)
 - **[operator-trivy]** Set `node.collector.imagePullSecret` to `deckhouse-registry`. [#8679](https://github.com/deckhouse/deckhouse/pull/8679)
 - **[prometheus]** Fix missing _kube-rbac-proxy_ CA in aggregating proxy deployment. [#8789](https://github.com/deckhouse/deckhouse/pull/8789)
 - **[prometheus]** Fix copying of Grafana v10 custom certificate. [#8749](https://github.com/deckhouse/deckhouse/pull/8749)
 - **[prometheus]** Expose Grafana v10 metrics. [#8723](https://github.com/deckhouse/deckhouse/pull/8723)
 - **[prometheus]** Update documentation. Remove the patch for Grafana 10. [#8580](https://github.com/deckhouse/deckhouse/pull/8580)

## Chore


 - **[candi]** Bump patch versions of Kubernetes images: `v1.30.2` [#8873](https://github.com/deckhouse/deckhouse/pull/8873)
    Kubernetes v1.30 control-plane components will restart, kubelet will restart.
 - **[candi]** Bump patch versions of Kubernetes images: `v1.27.14`, `v1.28.11`, `v1.29.6` [#8719](https://github.com/deckhouse/deckhouse/pull/8719)
    Kubernetes control-plane components will restart, kubelet will restart.
 - **[candi]** Moved most ports that are listened to on nodes to the range 4200-4299. [#8598](https://github.com/deckhouse/deckhouse/pull/8598)
    The following system pods will restart:
    * node-local-dns,
    * cloud-provider-*,
    * runtime-audit-engine,
    * metallb,
    * cilium-agent,
    * kube-proxy,
    * registry-packages-proxy,
    * bashible-apiserver,
    * capi-controller-manager,
    * machine-controller-manager,
    * network-policy-engine,
    * ingress-nginx with HostPortWithFailover inlet,
    * runtime-audit-engine.
    Note that you will need to change the access policies on the firewalls before upgrading the cluster.
 - **[cni-cilium]** Add alert for orphan EgressGatewayPolicy. [#8912](https://github.com/deckhouse/deckhouse/pull/8912)
 - **[deckhouse]** Fix overwriting embedded modules' images tags. [#8722](https://github.com/deckhouse/deckhouse/pull/8722)
 - **[docs]** Add documentation on module development. [#7779](https://github.com/deckhouse/deckhouse/pull/7779)
 - **[ingress-nginx]** Adjust `D8NginxIngressKruiseControllerPodIsRestartingTooOften` alert's threshold. [#8966](https://github.com/deckhouse/deckhouse/pull/8966)
 - **[ingress-nginx]** Make deprecated GeoIP hook less intrusive. [#8822](https://github.com/deckhouse/deckhouse/pull/8822)
 - **[ingress-nginx]** Add GeoIP deprecated version alert. [#8744](https://github.com/deckhouse/deckhouse/pull/8744)
 - **[local-path-provisioner]** Remove wildcard from module RBAC. [#8900](https://github.com/deckhouse/deckhouse/pull/8900)
 - **[operator-trivy]** Update Java-db image manifest. [#8676](https://github.com/deckhouse/deckhouse/pull/8676)
 - **[registrypackages]** Bump [Deckhouse CLI](https://deckhouse.io/documentation/v1.62/deckhouse-cli/) version 0.2.1. [#8981](https://github.com/deckhouse/deckhouse/pull/8981)
 - **[registrypackages]** Bump Deckhnouse CLI to `0.2.0`. Refactor the Deckhouse CLI installation page. [#8907](https://github.com/deckhouse/deckhouse/pull/8907)
 - **[snapshot-controller]** Switch _snapshot-controller_ module to distroless. [#8769](https://github.com/deckhouse/deckhouse/pull/8769)

