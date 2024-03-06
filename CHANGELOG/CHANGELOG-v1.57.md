# Changelog v1.57

## Know before update


 - All containers that use `spotify/scratch` image will be restarted (almost all Deckhouse containers).
 - Deckhouse will not upgrade if `linstor` module is enabled.
 - Deckhouse will not upgrade if the istio version in the cluster is lower than `1.16`.
 - The `linstor` module is deprecated. Please switch to [sds-drbd](https://deckhouse.io/modules/sds-drbd/stable/) module ASAP. The `linstor` module cannot be enabled but will continue to work if it was already enabled before.

## Features


 - **[candi]** Set curl connect timeout to 10s and explicitly set overall timeout to 300s. [#7059](https://github.com/deckhouse/deckhouse/pull/7059)
 - **[ceph-csi]** ceph-csi module is based on a distroless image. [#6724](https://github.com/deckhouse/deckhouse/pull/6724)
 - **[cloud-provider-yandex]** Add alert about deprecated NAT Instance zone. [#6736](https://github.com/deckhouse/deckhouse/pull/6736)
 - **[common]** shell-operator image is based on a distroless image. [#7047](https://github.com/deckhouse/deckhouse/pull/7047)
 - **[delivery]** Redis image is based on a distroless image. [#6224](https://github.com/deckhouse/deckhouse/pull/6224)
 - **[dhctl]** Retry failed pull/push operations during the mirroring. [#7080](https://github.com/deckhouse/deckhouse/pull/7080)
 - **[dhctl]** Added a flag that forces mirror to accept non-trusted registry certificates. [#7068](https://github.com/deckhouse/deckhouse/pull/7068)
 - **[documentation]** Improved stability of the documentation site. [#6873](https://github.com/deckhouse/deckhouse/pull/6873)
 - **[extended-monitoring]** `extended-monitoring-exporter` image is based on a distroless image. [#7164](https://github.com/deckhouse/deckhouse/pull/7164)
 - **[extended-monitoring]** Group `CronJobFailed` alerts. [#6715](https://github.com/deckhouse/deckhouse/pull/6715)
 - **[external-module-manager]** Always create default MUP for Deckhouse [#7185](https://github.com/deckhouse/deckhouse/pull/7185)
 - **[monitoring-kubernetes]** Add `NodeFilesystemIsRO` alert. [#6744](https://github.com/deckhouse/deckhouse/pull/6744)
 - **[monitoring-kubernetes]** `ebpf_exporter` image is based on a distroless image. Bump version to `v2.3.0`. [#6241](https://github.com/deckhouse/deckhouse/pull/6241)
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


 - **[candi]** Raise the priority for NodeUser step. [#7140](https://github.com/deckhouse/deckhouse/pull/7140)
 - **[candi]** Decrease shutdownGracePeriod for YandexCloud. [#6897](https://github.com/deckhouse/deckhouse/pull/6897)
 - **[candi]** Fixes for compliance with CIS Benchmarks. [#6647](https://github.com/deckhouse/deckhouse/pull/6647)
 - **[candi]** Wait for a node to be added to the cluster before annotating the node. [#6443](https://github.com/deckhouse/deckhouse/pull/6443)
 - **[common]** Fixed vulnerabilities in csi livenessprobe and node-driver-registrar: CVE-2022-41723, CVE-2023-39325, GHSA-m425-mq94-257g [#6956](https://github.com/deckhouse/deckhouse/pull/6956)
    csi-controller pod will restart.
 - **[deckhouse-controller]** fix for `change-registry` helper's handling of registry credentials. [#7095](https://github.com/deckhouse/deckhouse/pull/7095)
 - **[deckhouse-controller]** Fix ModuleConfig validation for configs with empty settings. [#7064](https://github.com/deckhouse/deckhouse/pull/7064)
 - **[dhctl]** Fix skipping preflight check about registry-through-proxy. [#7135](https://github.com/deckhouse/deckhouse/pull/7135)
 - **[dhctl]** Fix ModuleConfig update error: 'Invalid value: 0x0: must be specified for an update' [#7048](https://github.com/deckhouse/deckhouse/pull/7048)
 - **[external-module-manager]** Fix outdated module versions in multi-master environment. [#7234](https://github.com/deckhouse/deckhouse/pull/7234)
 - **[istio]** Improved checking for currently running deprecated versions of Istio in the cluster. [#7028](https://github.com/deckhouse/deckhouse/pull/7028)
 - **[istio]** After disabling the module, clean up any orphaned Istio components. [#6906](https://github.com/deckhouse/deckhouse/pull/6906)
 - **[monitoring-kubernetes]** Fix generation of `metrics kube_persistentvolume_is_local` recording rule. [#6755](https://github.com/deckhouse/deckhouse/pull/6755)
 - **[monitoring-kubernetes]** Bump `node-exporter` to `v1.7.0`. Fix crashes of `node-exporter`. [#6730](https://github.com/deckhouse/deckhouse/pull/6730)
 - **[monitoring-kubernetes]** Fix AppArmor rule in `kubelet-eviction-thresholds-exporter`. [#6711](https://github.com/deckhouse/deckhouse/pull/6711)
 - **[network-gateway]** Fix distroless build. [#7250](https://github.com/deckhouse/deckhouse/pull/7250)
 - **[network-policy-engine]** Module images are based on a distroless image. [#6460](https://github.com/deckhouse/deckhouse/pull/6460)
 - **[node-manager]** Add RBAC rules for kube-rbac-proxy in capi-controller-manager. [#6854](https://github.com/deckhouse/deckhouse/pull/6854)
 - **[operator-trivy]** CIS compliance checks are now available immediately after activating the module. [#6951](https://github.com/deckhouse/deckhouse/pull/6951)
 - **[terraform-manager]** Rename plugin `terraform-provider-gcp` to `terraform-provider-google` in `terraform-state-exporter`. [#7156](https://github.com/deckhouse/deckhouse/pull/7156)

## Chore


 - **[candi]** Update `cni-plugins` to version `1.4.0`. [#7078](https://github.com/deckhouse/deckhouse/pull/7078)
    cni-plugins will restart.
 - **[candi]** Change base_scratch from spotify/scratch to base_images/scratch. [#6748](https://github.com/deckhouse/deckhouse/pull/6748)
    All containers that use `spotify/scratch` image will be restarted (almost all Deckhouse containers).
 - **[cilium-hubble]** Add the [ingressClass](https://deckhouse.io/documentation/latest/modules/500-cilium-hubble/configuration.html#parameters-ingressclass) parameter to the module configuration. [#7007](https://github.com/deckhouse/deckhouse/pull/7007)
 - **[cni-cilium]** Add user-authz RBACs for `ciliumnetworkpolicies`. [#6813](https://github.com/deckhouse/deckhouse/pull/6813)
 - **[deckhouse-controller]** Add deckhouse-service initialization check. [#7163](https://github.com/deckhouse/deckhouse/pull/7163)
 - **[istio]** Add a minimum version of istio to the Deckhouse update requirements. [#7119](https://github.com/deckhouse/deckhouse/pull/7119)
    Deckhouse will not upgrade if the istio version in the cluster is lower than `1.16`.
 - **[istio]** Improve `hack_iop_reconciling` hook to prevent `istio-operator` stucking. [#7043](https://github.com/deckhouse/deckhouse/pull/7043)
 - **[istio]** Generate only requested mutating and validating webhooks. [#7037](https://github.com/deckhouse/deckhouse/pull/7037)
 - **[istio]** Add the [ingressClass](https://deckhouse.io/documentation/latest/modules/110-istio/configuration.html#parameters-ingressclass) parameter to the module configuration. [#7007](https://github.com/deckhouse/deckhouse/pull/7007)
 - **[keepalived]** keepalived is now based on a distroless image. [#6962](https://github.com/deckhouse/deckhouse/pull/6962)
    keepalived pods will restart.
 - **[linstor]** Disable Deckhouse update while `legacy` linstor module is enabled. [#7088](https://github.com/deckhouse/deckhouse/pull/7088)
    Deckhouse will not upgrade if `linstor` module is enabled.
 - **[linstor]** Add a validating webhook to prevent the linstor module from being enabled. [#7086](https://github.com/deckhouse/deckhouse/pull/7086)
    The `linstor` module is deprecated. Please switch to [sds-drbd](https://deckhouse.io/modules/sds-drbd/stable/) module ASAP. The `linstor` module cannot be enabled but will continue to work if it was already enabled before.
 - **[monitoring-kubernetes]** Move `helm` module to `monitoring-kubernetes` module. [#6726](https://github.com/deckhouse/deckhouse/pull/6726)
 - **[prometheus]** Set `.spec.externalURL` in the alermanager manifest when a public domain is specified. [#7042](https://github.com/deckhouse/deckhouse/pull/7042)
 - **[user-authn]** Don't recreate the CA certificate if the `publishAPI.https.mode` parameter changes. [#6927](https://github.com/deckhouse/deckhouse/pull/6927)

