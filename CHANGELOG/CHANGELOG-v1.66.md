# Changelog v1.66

## Features


 - **[candi]** Added support of new cloud provider - Dynamix. [#9009](https://github.com/deckhouse/deckhouse/pull/9009)
 - **[dhctl]** Improve panic handling. Fixed line breaks in logs. [#10473](https://github.com/deckhouse/deckhouse/pull/10473)
 - **[dhctl]** Print cloud objects which will be destroyed when dhctl destroy. [#10181](https://github.com/deckhouse/deckhouse/pull/10181)
 - **[dhctl]** Remove validation rules to enable master nodegroup auto converge. [#10052](https://github.com/deckhouse/deckhouse/pull/10052)
 - **[dhctl]** Add parallel bootstrap `cloudpermanent` nodes to dhctl. [#10015](https://github.com/deckhouse/deckhouse/pull/10015)
 - **[global-hooks]** Add the `global.defaultClusterStorageClass` setting. [#9591](https://github.com/deckhouse/deckhouse/pull/9591)
    cloud-provider's `storageClass.default` parameter was deprecated (not used anymore) and replaced with `global.defaultClusterStorageClass`
 - **[ingress-nginx]** Add worker_max_connections, worker_processes and worker_rlimit_nofile metrics. [#10154](https://github.com/deckhouse/deckhouse/pull/10154)
    ingress-nginx controllers' pods will be recreated.
 - **[multitenancy-manager]** Move the multitenancy-manager module to CE. [#10505](https://github.com/deckhouse/deckhouse/pull/10505)
 - **[operator-prometheus]** Added `backup.deckhouse.io/cluster-config` label to relevant operator CRDs. [#10298](https://github.com/deckhouse/deckhouse/pull/10298)
 - **[operator-trivy]** Add extra fields to vulnerability reports. [#10460](https://github.com/deckhouse/deckhouse/pull/10460)
 - **[prometheus]** Added `longtermPodAntiAffinity` options to module. [#10324](https://github.com/deckhouse/deckhouse/pull/10324)
 - **[user-authn]** Add ability to set multiple domains for DexAuthenticator. [#10452](https://github.com/deckhouse/deckhouse/pull/10452)

## Fixes


 - **[candi]** Fix LC_MESSAGES unknown locale. [#10440](https://github.com/deckhouse/deckhouse/pull/10440)
 - **[candi]** Change permissions for containerd dir. [#10133](https://github.com/deckhouse/deckhouse/pull/10133)
 - **[control-plane-manager]** Label `heritage: deckhouse` in namespace kube-system. [#10224](https://github.com/deckhouse/deckhouse/pull/10224)
 - **[monitoring-kubernetes]** Minor 'Nodes' dashboard improvements. [#10339](https://github.com/deckhouse/deckhouse/pull/10339)
 - **[multitenancy-manager]** Fix multitenancy-manager. [#10253](https://github.com/deckhouse/deckhouse/pull/10253)
 - **[node-manager]** Fix handling of machine creation errors in the `machine-controller-manager`(`vsphere` driver). [#10225](https://github.com/deckhouse/deckhouse/pull/10225)
 - **[user-authn]** Numbers in dex groups. [#10211](https://github.com/deckhouse/deckhouse/pull/10211)

## Chore


 - **[deckhouse]** Remove unused jq library. [#10444](https://github.com/deckhouse/deckhouse/pull/10444)
 - **[deckhouse]** Exclude disabledModules requirement from valudation. [#10248](https://github.com/deckhouse/deckhouse/pull/10248)
 - **[deckhouse-controller]** Fix image flattening procedure on downloading an image. [#10474](https://github.com/deckhouse/deckhouse/pull/10474)
 - **[deckhouse-controller]** Use Go 1.23. [#10396](https://github.com/deckhouse/deckhouse/pull/10396)
 - **[istio]** Got rid of self-made IP address allocation for public service's ServiceEntries in Federation. [#10218](https://github.com/deckhouse/deckhouse/pull/10218)
 - **[monitoring-kubernetes]** Minor "Node" dashboard improvements. [#10328](https://github.com/deckhouse/deckhouse/pull/10328)
 - **[node-manager]** Scale down for nodes running advanced daemonsets. [#10366](https://github.com/deckhouse/deckhouse/pull/10366)

