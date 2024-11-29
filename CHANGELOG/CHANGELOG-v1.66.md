# Changelog v1.66

## Know before update


 - All control plane components will restart.
 - The minimum supported Kubernetes version is 1.27.

## Features


 - **[admission-policy-engine]** Set MutatingWebhookConfiguration reinvocationPolicy to `IfNeeded` to enable the use of webhook with other mutating webhooks [#10611](https://github.com/deckhouse/deckhouse/pull/10611)
 - **[candi]** Add Kubernetes 1.31 support. [#9772](https://github.com/deckhouse/deckhouse/pull/9772)
    All control plane components will restart.
 - **[candi]** Remove support Kubernetes 1.26. [#9772](https://github.com/deckhouse/deckhouse/pull/9772)
    The minimum supported Kubernetes version is 1.27.
 - **[candi]** Added support of new cloud provider - Dynamix. [#9009](https://github.com/deckhouse/deckhouse/pull/9009)
 - **[control-plane-manager]** Update etcd version. [#9772](https://github.com/deckhouse/deckhouse/pull/9772)
 - **[dhctl]** Improve panic handling. Fixed line breaks in logs. [#10473](https://github.com/deckhouse/deckhouse/pull/10473)
 - **[dhctl]** Print cloud objects which will be destroyed when dhctl destroys a cluster. [#10181](https://github.com/deckhouse/deckhouse/pull/10181)
 - **[dhctl]** Remove validation rules to enable master nodegroup auto converge. [#10052](https://github.com/deckhouse/deckhouse/pull/10052)
 - **[dhctl]** Add parallel bootstrap `cloudpermanent` nodes to dhctl. [#10015](https://github.com/deckhouse/deckhouse/pull/10015)
 - **[global-hooks]** Add the `global.defaultClusterStorageClass` setting. [#9591](https://github.com/deckhouse/deckhouse/pull/9591)
    cloud-provider's `storageClass.default` parameter was deprecated (not used anymore) and replaced with `global.defaultClusterStorageClass`
 - **[ingress-nginx]** Add worker_max_connections, worker_processes and worker_rlimit_nofile metrics. [#10154](https://github.com/deckhouse/deckhouse/pull/10154)
    ingress-nginx controllers' pods will be recreated.
 - **[metallb]** Added extended pre-upgrade compatibility check for metallb configuration. [#10477](https://github.com/deckhouse/deckhouse/pull/10477)
 - **[multitenancy-manager]** Add high availability mode. [#10630](https://github.com/deckhouse/deckhouse/pull/10630)
 - **[multitenancy-manager]** Move the multitenancy-manager module to CE. [#10505](https://github.com/deckhouse/deckhouse/pull/10505)
 - **[operator-prometheus]** Fixed `backup.deckhouse.io/cluster-config` value. [#10570](https://github.com/deckhouse/deckhouse/pull/10570)
 - **[operator-prometheus]** Added `backup.deckhouse.io/cluster-config` label to relevant operator CRDs. [#10298](https://github.com/deckhouse/deckhouse/pull/10298)
 - **[operator-trivy]** Add extra fields to vulnerability reports. [#10460](https://github.com/deckhouse/deckhouse/pull/10460)
 - **[prometheus]** Added `longtermPodAntiAffinity` options to module. [#10324](https://github.com/deckhouse/deckhouse/pull/10324)
 - **[prometheus]** Added `backup.deckhouse.io/cluster-config` label to relevant module CRDs. [#10297](https://github.com/deckhouse/deckhouse/pull/10297)
 - **[registrypackages]** Update crictl version. [#9772](https://github.com/deckhouse/deckhouse/pull/9772)
 - **[user-authn]** Add ability to set multiple domains for DexAuthenticator. [#10452](https://github.com/deckhouse/deckhouse/pull/10452)

## Fixes


 - **[candi]** Fix LC_MESSAGES unknown locale. [#10440](https://github.com/deckhouse/deckhouse/pull/10440)
 - **[candi]** Change permissions for containerd dir. [#10133](https://github.com/deckhouse/deckhouse/pull/10133)
 - **[control-plane-manager]** Label `heritage: deckhouse` in namespace kube-system. [#10224](https://github.com/deckhouse/deckhouse/pull/10224)
 - **[dhctl]** Add human readable error on dhctl converge except [#10589](https://github.com/deckhouse/deckhouse/pull/10589)
 - **[docs]** Update docs about module creation, fix bugs [#10476](https://github.com/deckhouse/deckhouse/pull/10476)
 - **[monitoring-kubernetes]** Minor `Nodes` dashboard improvements. [#10339](https://github.com/deckhouse/deckhouse/pull/10339)
 - **[multitenancy-manager]** Enable multitenancy-manager by default in default and managed bundles. [#10652](https://github.com/deckhouse/deckhouse/pull/10652)
 - **[node-manager]** Add instruction on how to add static node to cluster. [#10655](https://github.com/deckhouse/deckhouse/pull/10655)
 - **[node-manager]** Fix handling of machine creation errors in the `machine-controller-manager`(`vsphere` driver). [#10225](https://github.com/deckhouse/deckhouse/pull/10225)
 - **[user-authn]** Numbers in dex groups. [#10211](https://github.com/deckhouse/deckhouse/pull/10211)

## Chore


 - **[candi]** Update D8 CLI to 0.4.0 [#10571](https://github.com/deckhouse/deckhouse/pull/10571)
 - **[deckhouse-controller]** Fix image flattening procedure on downloading an image. [#10474](https://github.com/deckhouse/deckhouse/pull/10474)
 - **[istio]** Got rid of self-made IP address allocation for public service's ServiceEntries in Federation. [#10218](https://github.com/deckhouse/deckhouse/pull/10218)
 - **[monitoring-kubernetes]** Minor "Node" dashboard improvements. [#10328](https://github.com/deckhouse/deckhouse/pull/10328)
 - **[node-manager]** Scale down for nodes running advanced daemonsets. [#10366](https://github.com/deckhouse/deckhouse/pull/10366)

