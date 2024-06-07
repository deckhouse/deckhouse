# Changelog v1.61

## Know before update


 - Cluster will upgrade Kubernetes version to `1.27` if the [kubernetesVersion](https://deckhouse.io/documentation/v1/installing/configuration.html#clusterconfiguration-kubernetesversion) parameter is set to `Automatic`.

## Features


 - **[candi]** Add support for Ubuntu 24.04. Support for Ubuntu 24.04 ensures compatibility with the latest OS version, providing updated packages and configurations. [#8540](https://github.com/deckhouse/deckhouse/pull/8540)
 - **[candi]** Added support for Red OS 8.0. [#8530](https://github.com/deckhouse/deckhouse/pull/8530)
    Support for Red OS 8.0 ensures compatibility with the latest OS version, providing updated packages and configurations.
 - **[cloud-provider-yandex]** Add requirements for deprecated zone removal in the next release. [#8590](https://github.com/deckhouse/deckhouse/pull/8590)
 - **[cloud-provider-yandex]** Add `diskType` parameter for [masterNodeGroup](https://deckhouse.io/documentation/latest/modules/030-cloud-provider-yandex/cluster_configuration.html#yandexclusterconfiguration-masternodegroup-instanceclass-disktype) and [nodeGroups](https://deckhouse.io/documentation/latest/modules/030-cloud-provider-yandex/cluster_configuration.html#yandexclusterconfiguration-nodegroups-instanceclass-disktype) in `YandexClusterConfiguration`. [#8384](https://github.com/deckhouse/deckhouse/pull/8384)
 - **[cni-cilium]** Fault-tolerant egress gateway based on a group of nodes. [#8093](https://github.com/deckhouse/deckhouse/pull/8093)
 - **[dhctl]** Preflight check exist embedded containerd. [#8550](https://github.com/deckhouse/deckhouse/pull/8550)
 - **[dhctl]** Preflight check ssh credentials before start instalations [#8409](https://github.com/deckhouse/deckhouse/pull/8409)
 - **[dhctl]** Preflight check registry credentials before start instalations [#8361](https://github.com/deckhouse/deckhouse/pull/8361)
 - **[ingress-nginx]** Add support of Ingress NGINX version 1.10 (it supports Nginx 1.25). [#8327](https://github.com/deckhouse/deckhouse/pull/8327)
 - **[node-manager]** Export availability metrics for node groups. Metrics have the prefix `d8_node_group_`. [#8355](https://github.com/deckhouse/deckhouse/pull/8355)
 - **[user-authn]** Added additional validations for Users and Groups. [#8401](https://github.com/deckhouse/deckhouse/pull/8401)
 - **[user-authn]** Add OIDC support to basic auth proxy. [#7407](https://github.com/deckhouse/deckhouse/pull/7407)

## Fixes


 - **[candi]** Updated local port range to "32768 61000" to avoid conflicts with ports used by other apps. [#8470](https://github.com/deckhouse/deckhouse/pull/8470)
 - **[candi]** Set `LC_NUMERIC` in configure kubelet. [#8383](https://github.com/deckhouse/deckhouse/pull/8383)
 - **[cloud-provider-vcd]** Support catalog in instance class template [#8539](https://github.com/deckhouse/deckhouse/pull/8539)
 - **[node-manager]** Write the SSH private key to a temporary file and delete the file after use. [#8490](https://github.com/deckhouse/deckhouse/pull/8490)
 - **[user-authn]** Replace the `enable` option with the `enabled` in the `publishAPI` field. [#8441](https://github.com/deckhouse/deckhouse/pull/8441)

## Chore


 - **[candi]** Update base images and dev versions. [#8549](https://github.com/deckhouse/deckhouse/pull/8549)
 - **[deckhouse]** Change the default Kubernetes version to `1.27`. [#8154](https://github.com/deckhouse/deckhouse/pull/8154)
    Cluster will upgrade Kubernetes version to `1.27` if the [kubernetesVersion](https://deckhouse.io/documentation/v1/installing/configuration.html#clusterconfiguration-kubernetesversion) parameter is set to `Automatic`.
 - **[deckhouse-controller]** Fix documentation updates for deployed yet overridden module releases. [#8504](https://github.com/deckhouse/deckhouse/pull/8504)
 - **[deckhouse-controller]** Add tests to MPO controller. [#8467](https://github.com/deckhouse/deckhouse/pull/8467)
 - **[istio]** Remove deprecated Istio versions (1.12 and 1.13). [#8452](https://github.com/deckhouse/deckhouse/pull/8452)
 - **[istio]** Added the globalVersion and additionalVersions ModuleConfig options validations. [#8404](https://github.com/deckhouse/deckhouse/pull/8404)
 - **[kube-dns]** Removed deprecated coredns patch and alert (`KubeDnsServiceWithDeprecatedAnnotation`). [#8492](https://github.com/deckhouse/deckhouse/pull/8492)

