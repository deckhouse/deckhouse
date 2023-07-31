# Changelog v1.49

## Know before update


 - Docker CRI is no longer supported. The upgrade will not start if Docker CRI is used.
 - New NodeGroups will have a `systemReserved` field set to a value proportional to the Node capacity. You can disable it via `.spec.kubelet.resourceReservation.mode` field set to `Off`.

## Features


 - **[candi]** Checking the server hostname for compliance with Kubernetes and Deckhouse requirements. [#5259](https://github.com/deckhouse/deckhouse/pull/5259)
 - **[candi]** Remove Docker CRI support. [#4960](https://github.com/deckhouse/deckhouse/pull/4960)
    Docker CRI is no longer supported. The upgrade will not start if Docker CRI is used.
 - **[cert-manager]** Added max concurrent challenges parameter for `cert-manager-controller`. [#4821](https://github.com/deckhouse/deckhouse/pull/4821)
 - **[control-plane-manager]** Add feature-gate CustomResourceValidationExpressions. [#5293](https://github.com/deckhouse/deckhouse/pull/5293)
 - **[deckhouse]** Add release requirement to prevent release from approving if there are nodes with docker in the cluster. [#5329](https://github.com/deckhouse/deckhouse/pull/5329)
 - **[dhctl]** Checking the availability of the `localhost` domain. [#5331](https://github.com/deckhouse/deckhouse/pull/5331)
 - **[dhctl]** Checking availability of ports `6443`, `2379`, `2380` on the server before bootstrap. [#5195](https://github.com/deckhouse/deckhouse/pull/5195)
 - **[dhctl]** Checking the availability of creating the SSH tunnel before bootstrap. [#5101](https://github.com/deckhouse/deckhouse/pull/5101)
 - **[external-module-manager]** Cleanup `ExternalModuleReleases` for deleted external modules. [#5260](https://github.com/deckhouse/deckhouse/pull/5260)
 - **[external-module-manager]** Ability to specify module tags and weight in the `module.yaml` file. [#5186](https://github.com/deckhouse/deckhouse/pull/5186)
 - **[log-shipper]** Render logs timestamps in the local timezone of a node.
    For example, **2019-10-12T07:20:50.52Z** will become **2019-10-12T09:20:50.52+02:00** for the Europe/Berlin timezone. [#4722](https://github.com/deckhouse/deckhouse/pull/4722)
 - **[multitenancy-manager]** Add the new `multitenancy-manager` module. [#4533](https://github.com/deckhouse/deckhouse/pull/4533)
 - **[node-manager]** Provide a resource reservation options to NodeGroup configuration. [#4842](https://github.com/deckhouse/deckhouse/pull/4842)
    New NodeGroups will have a `systemReserved` field set to a value proportional to the Node capacity. You can disable it via `.spec.kubelet.resourceReservation.mode` field set to `Off`.

## Fixes


 - **[admission-policy-engine]** Fix `checkContainerDuplicates` policy. Allow env with the same name in the different containers [#5214](https://github.com/deckhouse/deckhouse/pull/5214)
 - **[candi]** Removed double sudo call for reboot command. [#5272](https://github.com/deckhouse/deckhouse/pull/5272)
 - **[cloud-provider-openstack]** Remove 'minLength: 1' requirement from mainNetwork and defaultImageName properties in OpenAPI specification. [#5381](https://github.com/deckhouse/deckhouse/pull/5381)
 - **[deckhouse-controller]** Fix CA retaining after change-registry command. [#5307](https://github.com/deckhouse/deckhouse/pull/5307)
 - **[deckhouse-controller]** Fix change-registry CA handling for connecting to registry. [#5282](https://github.com/deckhouse/deckhouse/pull/5282)
 - **[linstor]** Update `piraeus-operator` CRDs to be compatible with the version `1.10.5`. [#5325](https://github.com/deckhouse/deckhouse/pull/5325)
 - **[linstor]** Update piraeus-operator and linstor-csi. Bump k8s api package versions. [#5301](https://github.com/deckhouse/deckhouse/pull/5301)
 - **[log-shipper]** Alert if replicas are not available. [#5311](https://github.com/deckhouse/deckhouse/pull/5311)
 - **[monitoring-kubernetes]** Select all controllers by default on the Namespace dashboard in Grafana. [#5224](https://github.com/deckhouse/deckhouse/pull/5224)
 - **[runtime-audit-engine]** Alert if replicas are not available. [#5311](https://github.com/deckhouse/deckhouse/pull/5311)
 - **[secret-copier]** Fix the creation of a secret in terminating or errored namespace. [#5295](https://github.com/deckhouse/deckhouse/pull/5295)

## Chore


 - **[candi]** Bump patch versions of Kubernetes images: `v1.24.16`, `v1.25.12`, `v1.26.7`, `v1.27.4`. [#5333](https://github.com/deckhouse/deckhouse/pull/5333)
    Kubernetes control-plane components will restart, kubelet will restart.
 - **[log-shipper]** Bump vector 0.31 [#4722](https://github.com/deckhouse/deckhouse/pull/4722)
 - **[runtime-audit-engine]** Reduce size of the `rules-reloader` container. [#5322](https://github.com/deckhouse/deckhouse/pull/5322)
    `runtime-audit-engine` Pods will be restarted.
 - **[runtime-audit-engine]** Update Falco to `0.35.1` to fix an issue with multiple active event sources. [#5289](https://github.com/deckhouse/deckhouse/pull/5289)

