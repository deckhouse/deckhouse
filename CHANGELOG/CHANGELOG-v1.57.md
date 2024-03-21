# Changelog v1.57

## Know before update


 - All containers that use `spotify/scratch` image will be restarted (almost all Deckhouse containers).
 - Deckhouse will not upgrade if the istio version in the cluster is lower than `1.16`.

## Features


 - **[candi]** Add Deckhouse Kubernetes Platform Basic Edition (BE). [#7260](https://github.com/deckhouse/deckhouse/pull/7260)
 - **[candi]** Add the ability to install packages from images inside an external module. [#7254](https://github.com/deckhouse/deckhouse/pull/7254)
 - **[candi]** Set curl connect timeout to 10s and explicitly set overall timeout to 300s. [#7059](https://github.com/deckhouse/deckhouse/pull/7059)
 - **[ceph-csi]** ceph-csi module is based on a distroless image. [#6724](https://github.com/deckhouse/deckhouse/pull/6724)
 - **[cloud-provider-yandex]** Add alert about deprecated NAT Instance zone. [#6736](https://github.com/deckhouse/deckhouse/pull/6736)
 - **[common]** shell-operator image is based on a distroless image. [#7047](https://github.com/deckhouse/deckhouse/pull/7047)
 - **[control-plane-manager]** Kubernetes version 1.24 support will be removed in the next Deckhouse release (1.58). [#7268](https://github.com/deckhouse/deckhouse/pull/7268)
 - **[delivery]** Redis image is based on a distroless image. [#6224](https://github.com/deckhouse/deckhouse/pull/6224)
 - **[dhctl]** Retry failed pull/push operations during the mirroring. [#7080](https://github.com/deckhouse/deckhouse/pull/7080)
 - **[dhctl]** Added a flag that forces mirror to accept non-trusted registry certificates. [#7068](https://github.com/deckhouse/deckhouse/pull/7068)
 - **[documentation]** Improved stability of the documentation site. [#6873](https://github.com/deckhouse/deckhouse/pull/6873)
 - **[extended-monitoring]** `extended-monitoring-exporter` image is based on a distroless image. [#7164](https://github.com/deckhouse/deckhouse/pull/7164)
 - **[extended-monitoring]** Group `CronJobFailed` alerts. [#6715](https://github.com/deckhouse/deckhouse/pull/6715)
 - **[external-module-manager]** Always create default MUP for Deckhouse [#7185](https://github.com/deckhouse/deckhouse/pull/7185)
 - **[monitoring-kubernetes]** Add `NodeFilesystemIsRO` alert. [#6744](https://github.com/deckhouse/deckhouse/pull/6744)
 - **[monitoring-kubernetes]** **(Reverted in #7269)** `ebpf_exporter` image is based on a distroless image. Bump version to `v2.3.0`. [#6241](https://github.com/deckhouse/deckhouse/pull/6241)
 - **[network-gateway]** network-gateway module is based on a distroless image. [#6968](https://github.com/deckhouse/deckhouse/pull/6968)
 - **[node-manager]** Allow specifying the `CloudStatic` type in the `staticInstance` field of a NodeGroup. [#7178](https://github.com/deckhouse/deckhouse/pull/7178)
 - **[operator-trivy]** Disable non-remote image source for remote scans. [#7016](https://github.com/deckhouse/deckhouse/pull/7016)
    trivy-operator pod will be recreated.
 - **[prometheus]** Add the ability to specify a CA certificate in `PrometheusRemoteWrite` CR. [#6933](https://github.com/deckhouse/deckhouse/pull/6933)
 - **[prometheus-pushgateway]** Pushgateway image is based on a distroless image. Bump version to `v1.6.2`. [#7058](https://github.com/deckhouse/deckhouse/pull/7058)
 - **[runtime-audit-engine]** Module images are based on a distroless image. [#7035](https://github.com/deckhouse/deckhouse/pull/7035)
 - **[upmeter]** database retention [#7153](https://github.com/deckhouse/deckhouse/pull/7153)
    The upmeter database will only store data for the last 548 days.

## Fixes


 - **[candi]** Fix bash word splitting. [#7410](https://github.com/deckhouse/deckhouse/pull/7410)
 - **[candi]** Deckhouse Kubernetes Platform BE improvements. [#7338](https://github.com/deckhouse/deckhouse/pull/7338)
 - **[candi]** Add validation pattern for the `imagesRepo` parameter. [#7169](https://github.com/deckhouse/deckhouse/pull/7169)
 - **[candi]** Raise the priority for NodeUser step. [#7140](https://github.com/deckhouse/deckhouse/pull/7140)
 - **[candi]** Decrease shutdownGracePeriod for YandexCloud. [#6897](https://github.com/deckhouse/deckhouse/pull/6897)
 - **[candi]** Fixes for compliance with CIS Benchmarks. [#6647](https://github.com/deckhouse/deckhouse/pull/6647)
 - **[candi]** Wait for a node to be added to the cluster before annotating the node. [#6443](https://github.com/deckhouse/deckhouse/pull/6443)
 - **[common]** Fixed vulnerabilities in csi livenessprobe and node-driver-registrar: CVE-2022-41723, CVE-2023-39325, GHSA-m425-mq94-257g [#6956](https://github.com/deckhouse/deckhouse/pull/6956)
    csi-controller pod will restart.
 - **[deckhouse]** Сhange the way the `deckhouse` pod readiness is determined during the minor version update. [#7866](https://github.com/deckhouse/deckhouse/pull/7866)
 - **[deckhouse]** Keep enabled modules without helm charts after converge. [#7315](https://github.com/deckhouse/deckhouse/pull/7315)
 - **[deckhouse-controller]** fix for `change-registry` helper's handling of registry credentials. [#7095](https://github.com/deckhouse/deckhouse/pull/7095)
 - **[deckhouse-controller]** Fix ModuleConfig validation for configs with empty settings. [#7064](https://github.com/deckhouse/deckhouse/pull/7064)
 - **[descheduler]** Set the number of replicas to 0 if we have only one node. [#5221](https://github.com/deckhouse/deckhouse/pull/5221)
 - **[dhctl]** Change the order in which resources are created. Service accounts will be created before secrets. [#7470](https://github.com/deckhouse/deckhouse/pull/7470)
 - **[dhctl]** Mirroring will now include Trivy vulnerability database image. [#7359](https://github.com/deckhouse/deckhouse/pull/7359)
 - **[dhctl]** Skip converge base infra if user does not want converge base infra [#7313](https://github.com/deckhouse/deckhouse/pull/7313)
 - **[dhctl]** Fix skipping preflight check about registry-through-proxy. [#7135](https://github.com/deckhouse/deckhouse/pull/7135)
 - **[dhctl]** Fix ModuleConfig update error: 'Invalid value: 0x0: must be specified for an update' [#7048](https://github.com/deckhouse/deckhouse/pull/7048)
 - **[external-module-manager]** Fix outdated module versions in multi-master environment. [#7234](https://github.com/deckhouse/deckhouse/pull/7234)
 - **[istio]** Improved checking for currently running deprecated versions of Istio in the cluster. [#7028](https://github.com/deckhouse/deckhouse/pull/7028)
 - **[istio]** After disabling the module, clean up any orphaned Istio components. [#6906](https://github.com/deckhouse/deckhouse/pull/6906)
 - **[metallb]** Change VPA `updateMode` to `Initial`. [#7432](https://github.com/deckhouse/deckhouse/pull/7432)
 - **[metallb]** Add `livenessProbe` and `readinessProbe` in metallb speaker spec. [#7382](https://github.com/deckhouse/deckhouse/pull/7382)
    The `metallb-speaker` pods will restart.
 - **[monitoring-kubernetes]** Revert https://github.com/deckhouse/deckhouse/pull/7272 [#7411](https://github.com/deckhouse/deckhouse/pull/7411)
 - **[monitoring-kubernetes]** Revert https://github.com/deckhouse/deckhouse/pull/6241. [#7269](https://github.com/deckhouse/deckhouse/pull/7269)
 - **[monitoring-kubernetes]** **(Reverted in #7411)** Add control of minimal Linux kernel version >= `5.8.0` for `ebpf_exporter` and a corresponding alert. [#7272](https://github.com/deckhouse/deckhouse/pull/7272)
 - **[monitoring-kubernetes]** Fix generation of `metrics kube_persistentvolume_is_local` recording rule. [#6755](https://github.com/deckhouse/deckhouse/pull/6755)
 - **[monitoring-kubernetes]** Bump `node-exporter` to `v1.7.0`. Fix crashes of `node-exporter`. [#6730](https://github.com/deckhouse/deckhouse/pull/6730)
 - **[monitoring-kubernetes]** Fix AppArmor rule in `kubelet-eviction-thresholds-exporter`. [#6711](https://github.com/deckhouse/deckhouse/pull/6711)
 - **[network-gateway]** Fix distroless build. [#7250](https://github.com/deckhouse/deckhouse/pull/7250)
 - **[network-policy-engine]** Module images are based on a distroless image. [#6460](https://github.com/deckhouse/deckhouse/pull/6460)
 - **[node-manager]** Fix panic when the vSphere driver creates a disk. [#7465](https://github.com/deckhouse/deckhouse/pull/7465)
 - **[node-manager]** Add RBAC rules for kube-rbac-proxy in capi-controller-manager. [#6854](https://github.com/deckhouse/deckhouse/pull/6854)
 - **[operator-trivy]** Fix `node-collector` image. [#7329](https://github.com/deckhouse/deckhouse/pull/7329)
 - **[operator-trivy]** CIS compliance checks are now available immediately after activating the module. [#6951](https://github.com/deckhouse/deckhouse/pull/6951)
 - **[prometheus]** Fix alerts-receiver reconcile loop issue. [#7287](https://github.com/deckhouse/deckhouse/pull/7287)
    Alerts-receiver pod will be recreated.
 - **[terraform-manager]** Rename plugin `terraform-provider-gcp` to `terraform-provider-google` in `terraform-state-exporter`. [#7156](https://github.com/deckhouse/deckhouse/pull/7156)

## Chore


 - **[candi]** Fix the bashible message about node annotation. [#7452](https://github.com/deckhouse/deckhouse/pull/7452)
 - **[candi]** Bump patch versions of Kubernetes images: `v1.26.13`, `v1.27.10`, `v1.28.6` [#7262](https://github.com/deckhouse/deckhouse/pull/7262)
    Kubernetes control-plane components will restart, kubelet will restart.
 - **[candi]** Update `cni-plugins` to version `1.4.0`. [#7078](https://github.com/deckhouse/deckhouse/pull/7078)
    cni-plugins will restart.
 - **[candi]** Change base_scratch from spotify/scratch to base_images/scratch. [#6748](https://github.com/deckhouse/deckhouse/pull/6748)
    All containers that use `spotify/scratch` image will be restarted (almost all Deckhouse containers).
 - **[cilium-hubble]** Add the [ingressClass](https://deckhouse.io/documentation/latest/modules/500-cilium-hubble/configuration.html#parameters-ingressclass) parameter to the module configuration. [#7007](https://github.com/deckhouse/deckhouse/pull/7007)
 - **[cni-cilium]** Add user-authz RBACs for `ciliumnetworkpolicies`. [#6813](https://github.com/deckhouse/deckhouse/pull/6813)
 - **[deckhouse-controller]** Add deckhouse-service initialization check. [#7163](https://github.com/deckhouse/deckhouse/pull/7163)
 - **[external-module-manager]** Prevent releases with versions less than current deployed version from deploying. [#7297](https://github.com/deckhouse/deckhouse/pull/7297)
 - **[external-module-manager]** Provide a registry scheme in a module OpenAPI. [#7263](https://github.com/deckhouse/deckhouse/pull/7263)
 - **[istio]** Add a minimum version of istio to the Deckhouse update requirements. [#7119](https://github.com/deckhouse/deckhouse/pull/7119)
    Deckhouse will not upgrade if the istio version in the cluster is lower than `1.16`.
 - **[istio]** Improve `hack_iop_reconciling` hook to prevent `istio-operator` stucking. [#7043](https://github.com/deckhouse/deckhouse/pull/7043)
 - **[istio]** Generate only requested mutating and validating webhooks. [#7037](https://github.com/deckhouse/deckhouse/pull/7037)
 - **[istio]** Add the [ingressClass](https://deckhouse.io/documentation/latest/modules/110-istio/configuration.html#parameters-ingressclass) parameter to the module configuration. [#7007](https://github.com/deckhouse/deckhouse/pull/7007)
 - **[keepalived]** keepalived is now based on a distroless image. [#6962](https://github.com/deckhouse/deckhouse/pull/6962)
    keepalived pods will restart.
 - **[monitoring-kubernetes]** Move `helm` module to `monitoring-kubernetes` module. [#6726](https://github.com/deckhouse/deckhouse/pull/6726)
 - **[prometheus]** Fix concurrent map access error. [#7261](https://github.com/deckhouse/deckhouse/pull/7261)
    Internal alerts-receiver will restart.
 - **[prometheus]** Set `.spec.externalURL` in the alermanager manifest when a public domain is specified. [#7042](https://github.com/deckhouse/deckhouse/pull/7042)
 - **[user-authn]** Don't recreate the CA certificate if the `publishAPI.https.mode` parameter changes. [#6927](https://github.com/deckhouse/deckhouse/pull/6927)

