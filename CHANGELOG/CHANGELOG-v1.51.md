# Changelog v1.51

## Know before update


 - All Ingress nginx controllers' pods will be recreated consecutively. Ingress nginx pods will stop to reply on :8080/healthz in favour of :80/healthz. Some LB's health checks might need readjusting.
 - Deploy the `deckhouse` modules source. It will make it possible to enable modules developed by the Deckhouse team but distributed separately.
    
    The most awaited module that can be enabled now is the `deckhouse-admin` module — a convenient web-interface to administer deckhouse clusters.
 - Linstor controller and piraeus operator will restart.
 - The [instruction](https://github.com/deckhouse/deckhouse/blob/f0ccf1b0d472455ca05ff0748e5ba6c634967a7f/modules/002-deckhouse/docs/internal/IMAGE_COPIER.md) for copying images no longer work. Use `d8-pull/d8-push` or `dhctl mirror` with `deckhouse-controller helper change-registry`.

## Features


 - **[admission-policy-engine]** Images are based on a distroless image. [#5522](https://github.com/deckhouse/deckhouse/pull/5522)
 - **[cloud-provider-azure]** Add `serviceEndpoints` parameter in a provider cluster configuration. [#5659](https://github.com/deckhouse/deckhouse/pull/5659)
 - **[cloud-provider-yandex]** Add storage class for the new disk type — `network-ssd-io-m3`. [#5684](https://github.com/deckhouse/deckhouse/pull/5684)
 - **[deckhouse]** Remove images copier from deckhouse module. [#5713](https://github.com/deckhouse/deckhouse/pull/5713)
    The [instruction](https://github.com/deckhouse/deckhouse/blob/f0ccf1b0d472455ca05ff0748e5ba6c634967a7f/modules/002-deckhouse/docs/internal/IMAGE_COPIER.md) for copying images no longer work. Use `d8-pull/d8-push` or `dhctl mirror` with `deckhouse-controller helper change-registry`.
 - **[deckhouse]** Set ready and converging status for module configs. [#5641](https://github.com/deckhouse/deckhouse/pull/5641)
 - **[deckhouse]** Start debug HTTP API server next to socket server. [#5570](https://github.com/deckhouse/deckhouse/pull/5570)
 - **[deckhouse]** Move ModuleConfig handling to the `deckhouse` module. Delete the module `deckhouse-config`. [#5566](https://github.com/deckhouse/deckhouse/pull/5566)
 - **[extended-monitoring]** Images are based on a distroless image (part 2). [#5547](https://github.com/deckhouse/deckhouse/pull/5547)
 - **[extended-monitoring]** Images are based on a distroless image. [#5451](https://github.com/deckhouse/deckhouse/pull/5451)
 - **[external-module-manager]** Deploy the `deckhouse` modules source. It will make it possible to enable modules developed by the Deckhouse team but distributed separately.
    
    The most awaited module that can be enabled now is the `deckhouse-admin` module — a convenient web-interface to administer deckhouse clusters. [#5710](https://github.com/deckhouse/deckhouse/pull/5710)
    Deploy the `deckhouse` modules source. It will make it possible to enable modules developed by the Deckhouse team but distributed separately.
    
    The most awaited module that can be enabled now is the `deckhouse-admin` module — a convenient web-interface to administer deckhouse clusters.
 - **[external-module-manager]** Rename custom resources ExternalModule* -> Module* [#5599](https://github.com/deckhouse/deckhouse/pull/5599)
 - **[external-module-manager]** Support custom CA for `ExternalModuleSource`. [#5498](https://github.com/deckhouse/deckhouse/pull/5498)
 - **[linstor]** Update linstor controller version to `1.24.1`. Update DRBD version to `9.2.5`. [#5469](https://github.com/deckhouse/deckhouse/pull/5469)
    Linstor controller and piraeus operator will restart.
 - **[log-shipper]** Images are based on a distroless image. [#5523](https://github.com/deckhouse/deckhouse/pull/5523)
 - **[loki]** Images are based on a distroless image. [#5391](https://github.com/deckhouse/deckhouse/pull/5391)
 - **[monitoring-kubernetes]** Images are based on a distroless image. [#5378](https://github.com/deckhouse/deckhouse/pull/5378)
 - **[operator-trivy]** Use customized trivy binary. [#5399](https://github.com/deckhouse/deckhouse/pull/5399)
 - **[prometheus]** Images are based now on a distroless image. [#5783](https://github.com/deckhouse/deckhouse/pull/5783)
    alertmanager, prometheus, grafana, trickster pods will be restarted.
 - **[prometheus]** Images are based now on a distroless image. [#5402](https://github.com/deckhouse/deckhouse/pull/5402)
    alertmanager, prometheus, grafana, trickster pods will be restarted.
 - **[prometheus-metrics-adapter]** All images are now based on a distroless image. [#5519](https://github.com/deckhouse/deckhouse/pull/5519)
 - **[runtime-audit-engine]** Base `falcosidekick` image on a distroless image. [#5623](https://github.com/deckhouse/deckhouse/pull/5623)
    `runtime-audit-engine` pod will restart.
 - **[user-authz]** A new parameter `.spec.namespaceSelector` was added to Cluster Authorization Rule spec. The `.spec.limitNamespaces` parameter was marked as deprecated. [#4666](https://github.com/deckhouse/deckhouse/pull/4666)

## Fixes


 - **[candi]** AWS node's `root_block_device` is now marked with tags from `AWSClusterConfiguration`. [#5650](https://github.com/deckhouse/deckhouse/pull/5650)
 - **[candi]** Fix migration of cluster from one edition to another. [#5603](https://github.com/deckhouse/deckhouse/pull/5603)
 - **[candi]** Changed the command output for `yum versionlock delete` if it is dnf. [#5492](https://github.com/deckhouse/deckhouse/pull/5492)
 - **[cloud-provider-openstack]** Fix `ignoreVolumeMicroversion` parameter appliance for Kubernetes version > 1.24. [#5778](https://github.com/deckhouse/deckhouse/pull/5778)
 - **[deckhouse]** Made it possible to configure `minimalNotificationTime` without configuring the notification webhook. [#5491](https://github.com/deckhouse/deckhouse/pull/5491)
 - **[deckhouse]** Automatically update Deckhouse minor versions step by step. [#5453](https://github.com/deckhouse/deckhouse/pull/5453)
 - **[deckhouse-controller]** Skip defaulting an array without items schema to prevent panic [#5711](https://github.com/deckhouse/deckhouse/pull/5711)
 - **[deckhouse-controller]** Add Prometeus logs to the debugging information. [#5616](https://github.com/deckhouse/deckhouse/pull/5616)
 - **[deckhouse-controller]** Improve the readability of raw debugging information. [#5499](https://github.com/deckhouse/deckhouse/pull/5499)
 - **[external-module-manager]** Add the check to prevent nil pointer dereference to the modules migration hook. [#5793](https://github.com/deckhouse/deckhouse/pull/5793)
 - **[external-module-manager]** Fix race condition while handling source on Deckhouse startup. [#5598](https://github.com/deckhouse/deckhouse/pull/5598)
 - **[flant-integration]** Take values from the `clusterConfiguration` parameter instead of the global parameter. [#5681](https://github.com/deckhouse/deckhouse/pull/5681)
 - **[flant-integration]** Change `sum` to `avg` in `controller_metrics` hook and refactor utils. [#5517](https://github.com/deckhouse/deckhouse/pull/5517)
 - **[linstor]** Fix rare issues with building the DRBD module when changing the kernels. [#5758](https://github.com/deckhouse/deckhouse/pull/5758)
 - **[linstor]** Revert the commit that updated the versions of Linstor and DRBD (revert https://github.com/deckhouse/deckhouse/pull/5469 ). [#5755](https://github.com/deckhouse/deckhouse/pull/5755)
 - **[monitoring-kubernetes]** Revert `node-exporter` `kube-rbac-proxy` liveness probe. [#5642](https://github.com/deckhouse/deckhouse/pull/5642)
 - **[operator-trivy]** Fix constant creation and deletion of node-collector pods. [#5688](https://github.com/deckhouse/deckhouse/pull/5688)
 - **[operator-trivy]** Fix handling empty list in operator trivy deployment in `OPERATOR_TARGET_NAMESPACES` env (set `default` value). [#5662](https://github.com/deckhouse/deckhouse/pull/5662)
 - **[prometheus]** Fix external auth handling for alertmanager. [#5706](https://github.com/deckhouse/deckhouse/pull/5706)
 - **[runtime-audit-engine]** Add read-only root for the `falco` container. [#5664](https://github.com/deckhouse/deckhouse/pull/5664)
    `runtime-audit-engine` pods should be restarted.

## Chore


 - **[candi]** Bump patch versions of Kubernetes images: `v1.24.17`, `v1.25.13`, `v1.26.8`, `v1.27.5`. [#5666](https://github.com/deckhouse/deckhouse/pull/5666)
    Kubernetes control plane components will restart, kubelet will restart.
 - **[ingress-nginx]** Fix `ingress-nginx` healthz handler replying with 200 unconditionally. [#5613](https://github.com/deckhouse/deckhouse/pull/5613)
    All Ingress nginx controllers' pods will be recreated consecutively. Ingress nginx pods will stop to reply on :8080/healthz in favour of :80/healthz. Some LB's health checks might need readjusting.
 - **[runtime-audit-engine]** Move rules-reloader build to werf. [#5694](https://github.com/deckhouse/deckhouse/pull/5694)

