# Changelog v1.54

## Know before update


 - The [configOverrides](https://deckhouse.io/documentation/v1.54/installing/configuration.html#initconfiguration-deckhouse-configoverrides) parameter of the `InitConfiguration` resource has been deprecated. Use corresponding `ModuleConfig` resources when bootstrapping a new cluster. Read [the documentation](https://deckhouse.io/documentation/latest/installing/#preparing-the-configuration) for additional information.

## Features


 - **[admission-policy-engine]** Add Java vulnerability scan capability to `trivy-provider`. [#6139](https://github.com/deckhouse/deckhouse/pull/6139)
    `trivy-provider` will restart.
 - **[chrony]** Chrony image is based on distroless image. [#6240](https://github.com/deckhouse/deckhouse/pull/6240)
 - **[deckhouse]** Change `deckhouse-controller` user to `deckhouse`. [#5841](https://github.com/deckhouse/deckhouse/pull/5841)
 - **[deckhouse-controller]** Use ModuleConfig as the primary source of configuration. Don't use ConfigMap `deckhouse` anymore. [#6061](https://github.com/deckhouse/deckhouse/pull/6061)
 - **[dhctl]** `dhctl` now supports uploading mirrored Deckhouse images to custom repo paths. [#6467](https://github.com/deckhouse/deckhouse/pull/6467)
 - **[dhctl]** `dhctl` will compute stribog 256 bit hash for downloaded registry copy. [#6409](https://github.com/deckhouse/deckhouse/pull/6409)
 - **[dhctl]** Implemented copying of Deckhouse images to third-party registries for air-gapped installation. [#6257](https://github.com/deckhouse/deckhouse/pull/6257)
 - **[dhctl]** Use ModuleConfig to override the default configuration instead of the `configOverrides` section of the `InitConfiguration` resource. [#6061](https://github.com/deckhouse/deckhouse/pull/6061)
    The [configOverrides](https://deckhouse.io/documentation/v1.54/installing/configuration.html#initconfiguration-deckhouse-configoverrides) parameter of the `InitConfiguration` resource has been deprecated. Use corresponding `ModuleConfig` resources when bootstrapping a new cluster. Read [the documentation](https://deckhouse.io/documentation/latest/installing/#preparing-the-configuration) for additional information.
 - **[external-module-manager]** Add support for module pull from insecure (HTTP) registry. [#6340](https://github.com/deckhouse/deckhouse/pull/6340)
 - **[ingress-nginx]** Use chrooted image for controller version `1.9`. Add `enable-annotation-validation` feature for version `1.9`. [#6370](https://github.com/deckhouse/deckhouse/pull/6370)
 - **[ingress-nginx]** Add v1.9.3 Ingress Nginx controller version. [#6312](https://github.com/deckhouse/deckhouse/pull/6312)
    In case of switching to '1.9' controller version, relevant Ingress nginx's pods will be recreated.
 - **[linstor]** Add a custom script for eviction of LINSTOR resources from a node. [#6457](https://github.com/deckhouse/deckhouse/pull/6457)
 - **[local-path-provisioner]** Image is based on distroless image. [#6194](https://github.com/deckhouse/deckhouse/pull/6194)
 - **[log-shipper]** Add an option to encode messages to CEF format (often accepted by SIEM systems, such as KUMA (Kaspersky Unified Monitoring and Analysis Platform). [#6406](https://github.com/deckhouse/deckhouse/pull/6406)
 - **[monitoring-ping]** Image is based on distroless image. Use static Python. [#6204](https://github.com/deckhouse/deckhouse/pull/6204)
 - **[prometheus]** Ability to set a custom logo for the Grafana dashboard. [#6268](https://github.com/deckhouse/deckhouse/pull/6268)

## Fixes


 - **[candi]** Do not use cloud network setup scripts for static NodeGroups. [#6464](https://github.com/deckhouse/deckhouse/pull/6464)
 - **[candi]** Fix big time drift on nodes. [#6297](https://github.com/deckhouse/deckhouse/pull/6297)
    All chrony pods will restart.
 - **[common]** Fix CVE issues in the `kube-rbac-proxy` image. [#6316](https://github.com/deckhouse/deckhouse/pull/6316)
    The pods that are behind the `kube-rbac-proxy` will restart.
 - **[dashboard]** Fix apiVersion for CronJobs to display with the dashboard module. [#5799](https://github.com/deckhouse/deckhouse/pull/5799)
 - **[dhctl]** Fix `edit provider-cluster-configuration` command to not remove `discovery-data.json` file from `kube-system/d8-provider-cluster-configuration` Secret. [#6486](https://github.com/deckhouse/deckhouse/pull/6486)
 - **[dhctl]** Improved the seeding and usage of rand. [#5094](https://github.com/deckhouse/deckhouse/pull/5094)
    Higher quality of insecure randomness, slightly better performance.
 - **[extended-monitoring]** Change the node search command for a DaemonSet in the `KubernetesDaemonSetReplicasUnavailable` alert. [#6068](https://github.com/deckhouse/deckhouse/pull/6068)
 - **[external-module-manager]** Fix deckhouse ModuleSource recreation on startup. [#6448](https://github.com/deckhouse/deckhouse/pull/6448)
 - **[external-module-manager]** Add support for hardlinks and symlinks to the module. [#6330](https://github.com/deckhouse/deckhouse/pull/6330)
 - **[ingress-nginx]** Fix CVE issues in the `protobuf-exporter` image. [#6327](https://github.com/deckhouse/deckhouse/pull/6327)
 - **[ingress-nginx]** Fix CVE issues in the `nginx-exporter` image. [#6325](https://github.com/deckhouse/deckhouse/pull/6325)
 - **[ingress-nginx]** Fix CVE issues in the `kruise-state-metrics` image. [#6321](https://github.com/deckhouse/deckhouse/pull/6321)
 - **[ingress-nginx]** Fix CVE issues in the `kruise` image. [#6320](https://github.com/deckhouse/deckhouse/pull/6320)
 - **[ingress-nginx]** Change the node search command for a DaemonSet in the `NginxIngressDaemonSetReplicasUnavailable` alert. [#6068](https://github.com/deckhouse/deckhouse/pull/6068)
 - **[local-path-provisioner]** Fix CVE issues in the `local-path-provisioner` image. [#6346](https://github.com/deckhouse/deckhouse/pull/6346)
 - **[log-shipper]** Remove buffer locks on startup. [#6322](https://github.com/deckhouse/deckhouse/pull/6322)
 - **[loki]** Fix CVE issues in the `loki` image. Bump Loki version to `2.7.7`. [#6375](https://github.com/deckhouse/deckhouse/pull/6375)
 - **[metallb]** Fix error with preserving controller internal state after reboot. [#6418](https://github.com/deckhouse/deckhouse/pull/6418)
    Metallb pods will restart.
 - **[monitoring-kubernetes]** Fix CVE issues in the `kube-state-metrics` image. [#6336](https://github.com/deckhouse/deckhouse/pull/6336)
 - **[multitenancy-manager]** Non-valid `Project` or `ProjectType` resources don't block the main queue. [#6049](https://github.com/deckhouse/deckhouse/pull/6049)
 - **[node-manager]** Fix `CVE-2021-4238` and  `GHSA-m425-mq94-257g` in `bashible-apiserver`. [#6348](https://github.com/deckhouse/deckhouse/pull/6348)
 - **[operator-prometheus]** Fix RBAC for updating alertmanager status. [#6466](https://github.com/deckhouse/deckhouse/pull/6466)
 - **[pod-reloader]** Add a forgotten `nodeSelector`. [#6338](https://github.com/deckhouse/deckhouse/pull/6338)
 - **[prometheus]** Fix Prometheus image size. [#6434](https://github.com/deckhouse/deckhouse/pull/6434)
 - **[prometheus]** Fix HIGH CVE issues in the `alertmanager` image. [#6294](https://github.com/deckhouse/deckhouse/pull/6294)
    Check that the alerts come after the update.
 - **[prometheus]** Fix HIGH CVE issues in the `trickster` image. [#6281](https://github.com/deckhouse/deckhouse/pull/6281)
    Check that Prometheus metrics come after the update.
 - **[user-authn]** Provide userID field for correct JWT generation. [#6484](https://github.com/deckhouse/deckhouse/pull/6484)

## Chore


 - **[candi]** Force bashible start after node reboot. [#6380](https://github.com/deckhouse/deckhouse/pull/6380)
 - **[candi]** Bump patch versions of Kubernetes images: `v1.25.15`, `v1.26.10`, `v1.27.7`, `v1.28.3`. [#6293](https://github.com/deckhouse/deckhouse/pull/6293)
    Kubernetes control plane components will restart, kubelet will restart.
 - **[deckhouse]** Send `clusterUUID` when checking for Deckhouse release. [#6412](https://github.com/deckhouse/deckhouse/pull/6412)
 - **[docs]** Add a guide for mirroring the Deckhouse registry using the `dhctl mirror` tool. [#6339](https://github.com/deckhouse/deckhouse/pull/6339)
 - **[go_lib]** Bump `addon-operator` to avoid race panics. [#6505](https://github.com/deckhouse/deckhouse/pull/6505)
 - **[virtualization]** Add a validating webhook to prevent the virtualization module from being enabled. [#6356](https://github.com/deckhouse/deckhouse/pull/6356)
    The `virtualization` module cannot be enabled, but it will continue to work if it was already enabled before the update (the current version of the module is deprecated, and a new version will be published soon).

