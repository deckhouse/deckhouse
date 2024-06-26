# Changelog v1.61

## Know before update


 - Cluster will upgrade Kubernetes version to `1.27` if the [kubernetesVersion](https://deckhouse.io/documentation/v1/installing/configuration.html#clusterconfiguration-kubernetesversion) parameter is set to `Automatic`.
 - Ingress controller will restart.
 - `dhctl mirror` is deprecated. All mirroring-related functionality has been moved into the Deckhouse CLI and will be completely removed from dhctl in Deckhouse Kubernetes Platform v1.64.

## Features


 - **[candi]** Add support for Ubuntu 24.04. Support for Ubuntu 24.04 ensures compatibility with the latest OS version, providing updated packages and configurations. [#8540](https://github.com/deckhouse/deckhouse/pull/8540)
 - **[candi]** Added support for Red OS 8.0. [#8530](https://github.com/deckhouse/deckhouse/pull/8530)
    Support for Red OS 8.0 ensures compatibility with the latest OS version, providing updated packages and configurations.
 - **[cloud-provider-yandex]** Add requirements for deprecated zone removal in the next release. [#8590](https://github.com/deckhouse/deckhouse/pull/8590)
 - **[cloud-provider-yandex]** Add `diskType` parameter for [masterNodeGroup](https://deckhouse.io/documentation/latest/modules/030-cloud-provider-yandex/cluster_configuration.html#yandexclusterconfiguration-masternodegroup-instanceclass-disktype) and [nodeGroups](https://deckhouse.io/documentation/latest/modules/030-cloud-provider-yandex/cluster_configuration.html#yandexclusterconfiguration-nodegroups-instanceclass-disktype) in `YandexClusterConfiguration`. [#8384](https://github.com/deckhouse/deckhouse/pull/8384)
 - **[cni-cilium]** Fault-tolerant egress gateway based on a group of nodes. [#8093](https://github.com/deckhouse/deckhouse/pull/8093)
 - **[control-plane-manager]** Adding a new module static-routing-manager. [#7952](https://github.com/deckhouse/deckhouse/pull/7952)
 - **[deckhouse-controller]** Adding a new module static-routing-manager. [#7952](https://github.com/deckhouse/deckhouse/pull/7952)
 - **[dhctl]** Add new commander/detach operation, add commander-uuid option for all commander operations [#8607](https://github.com/deckhouse/deckhouse/pull/8607)
 - **[dhctl]** Add cluster config validation gRPC services. [#8606](https://github.com/deckhouse/deckhouse/pull/8606)
 - **[dhctl]** Preflight check exist embedded containerd. [#8550](https://github.com/deckhouse/deckhouse/pull/8550)
 - **[dhctl]** Preflight check ssh credentials before start instalations [#8409](https://github.com/deckhouse/deckhouse/pull/8409)
 - **[dhctl]** Preflight check registry credentials before start instalations [#8361](https://github.com/deckhouse/deckhouse/pull/8361)
 - **[docs]** Adding a new module static-routing-manager. [#7952](https://github.com/deckhouse/deckhouse/pull/7952)
 - **[ingress-nginx]** Add support of Ingress NGINX version 1.10 (it supports Nginx 1.25). [#8327](https://github.com/deckhouse/deckhouse/pull/8327)
 - **[node-manager]** Re-enable ClusterHasOrphanedDisks alert for Yandex Cloud. [#8718](https://github.com/deckhouse/deckhouse/pull/8718)
 - **[node-manager]** Export availability metrics for node groups. Metrics have the prefix `d8_node_group_`. [#8355](https://github.com/deckhouse/deckhouse/pull/8355)
 - **[static-routing-manager]** Adding a new module static-routing-manager. [#7952](https://github.com/deckhouse/deckhouse/pull/7952)
 - **[user-authn]** Added additional validations for Users and Groups. [#8401](https://github.com/deckhouse/deckhouse/pull/8401)
 - **[user-authn]** Add OIDC support to basic auth proxy. [#7407](https://github.com/deckhouse/deckhouse/pull/7407)

## Fixes


 - **[candi]** Updated local port range to "32768 61000" to avoid conflicts with ports used by other apps. [#8470](https://github.com/deckhouse/deckhouse/pull/8470)
 - **[candi]** Set `LC_NUMERIC` in configure kubelet. [#8383](https://github.com/deckhouse/deckhouse/pull/8383)
 - **[cloud-provider-vcd]** Support catalog in instance class template [#8539](https://github.com/deckhouse/deckhouse/pull/8539)
 - **[cloud-provider-yandex]** Fix typo in label key in discoverer. [#8792](https://github.com/deckhouse/deckhouse/pull/8792)
 - **[dhctl]** Fix working with registries on non-standard ports. [#8809](https://github.com/deckhouse/deckhouse/pull/8809)
 - **[dhctl]** Fix registry path calculation. [#8802](https://github.com/deckhouse/deckhouse/pull/8802)
 - **[dhctl]** Fix bootstrap cluster. [#8787](https://github.com/deckhouse/deckhouse/pull/8787)
 - **[ingress-nginx]** Fix pod descheduling from a not-ready node. [#8801](https://github.com/deckhouse/deckhouse/pull/8801)
    Kruise controller's pods will be recreated.
 - **[ingress-nginx]** Fix usage custom logs formats without our fields. [#8621](https://github.com/deckhouse/deckhouse/pull/8621)
    Ingress controller will restart.
 - **[ingress-nginx]** Fix HostPortWithProxyProtocol inlet. [#8742](https://github.com/deckhouse/deckhouse/pull/8742)
 - **[ingress-nginx]** Add missing `severity_level` for NginxIngressConfigTestFailed rule. [#8661](https://github.com/deckhouse/deckhouse/pull/8661)
 - **[istio]** Fixed Istio module release requirements checker. [#8678](https://github.com/deckhouse/deckhouse/pull/8678)
 - **[monitoring-kubernetes-control-plane]** Add missing datasource variable to deprecated-apis dashboard [#8689](https://github.com/deckhouse/deckhouse/pull/8689)
 - **[node-manager]** Fix RBAC permissions and startup schedule cleanup of NodeUser creation errors. [#8803](https://github.com/deckhouse/deckhouse/pull/8803)
 - **[node-manager]** Write the SSH private key to a temporary file and delete the file after use. [#8490](https://github.com/deckhouse/deckhouse/pull/8490)
 - **[registry-packages-proxy]** Fix working with registries on non-standard ports. [#8809](https://github.com/deckhouse/deckhouse/pull/8809)
    registry-packages-proxy will restart.
 - **[registry-packages-proxy]** Fix registry path calculation. [#8802](https://github.com/deckhouse/deckhouse/pull/8802)
    registry-packages-proxy will restart.
 - **[user-authn]** Fix crowd basic auth proxy migration. [#8704](https://github.com/deckhouse/deckhouse/pull/8704)
 - **[user-authn]** Replace the `enable` option with the `enabled` in the `publishAPI` field. [#8441](https://github.com/deckhouse/deckhouse/pull/8441)

## Chore


 - **[candi]** Update base images and dev versions. [#8549](https://github.com/deckhouse/deckhouse/pull/8549)
 - **[deckhouse]** Invoke modules' requirements checks only for enabled modules. [#8688](https://github.com/deckhouse/deckhouse/pull/8688)
 - **[deckhouse]** Change the default Kubernetes version to `1.27`. [#8154](https://github.com/deckhouse/deckhouse/pull/8154)
    Cluster will upgrade Kubernetes version to `1.27` if the [kubernetesVersion](https://deckhouse.io/documentation/v1/installing/configuration.html#clusterconfiguration-kubernetesversion) parameter is set to `Automatic`.
 - **[deckhouse-controller]** Fix documentation updates for deployed yet overridden module releases. [#8504](https://github.com/deckhouse/deckhouse/pull/8504)
 - **[deckhouse-controller]** Add tests to MPO controller. [#8467](https://github.com/deckhouse/deckhouse/pull/8467)
 - **[dhctl]** `dhctl mirror` is deprecated in favour of Deckhouse CLI. [#8682](https://github.com/deckhouse/deckhouse/pull/8682)
    `dhctl mirror` is deprecated. All mirroring-related functionality has been moved into the Deckhouse CLI and will be completely removed from dhctl in Deckhouse Kubernetes Platform v1.64.
 - **[istio]** Remove deprecated Istio versions (1.12 and 1.13). [#8452](https://github.com/deckhouse/deckhouse/pull/8452)
 - **[istio]** Added the globalVersion and additionalVersions ModuleConfig options validations. [#8404](https://github.com/deckhouse/deckhouse/pull/8404)
 - **[kube-dns]** Removed deprecated coredns patch and alert (`KubeDnsServiceWithDeprecatedAnnotation`). [#8492](https://github.com/deckhouse/deckhouse/pull/8492)

