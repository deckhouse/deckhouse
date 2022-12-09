# Changelog v1.41

## Know before update


 - **The new configuration mechanism.**
    The `deckhouse` ConfigMap is forbidden to edit; use `ModuleConfig` object to change module configuration.
 - The deprecated `auth.password` option is removed from the `cilium-hubble` module.
 - The deprecated `auth.password` option is removed from the `dashboard` module.
 - The deprecated `auth.password` option is removed from the `deckhouse-web` module
 - The deprecated `auth.password` option is removed from the `istio` module.
 - The deprecated `auth.password` option is removed from the `openvpn` module.
 - The deprecated `auth.password` option is removed from the `prometheus` module.
 - The deprecated `auth.status.password` and `auth.webui.password` options are removed from the `upmeter` module.

## Features


 - **[cert-manager]** Ability to disable the `letsencrypt` and `letsencrypt-staging` ClusterIssuers creation. [#3042](https://github.com/deckhouse/deckhouse/pull/3042)
 - **[cilium-hubble]** The `auth.password` option is deprecated. Consider using the `user-authn` module. [#1729](https://github.com/deckhouse/deckhouse/pull/1729)
    The deprecated `auth.password` option is removed from the `cilium-hubble` module.
 - **[dashboard]** The `auth.password` option is deprecated. Consider using the `user-authn` module. [#1729](https://github.com/deckhouse/deckhouse/pull/1729)
    The deprecated `auth.password` option is removed from the `dashboard` module.
 - **[deckhouse-controller]** Use ModuleConfig objects to configure deckhouse modules. [#1729](https://github.com/deckhouse/deckhouse/pull/1729)
    **The new configuration mechanism.**
    The `deckhouse` ConfigMap is forbidden to edit; use `ModuleConfig` object to change module configuration.
 - **[deckhouse-web]** The `auth.password` option is deprecated. Consider using the `user-authn` module. [#1729](https://github.com/deckhouse/deckhouse/pull/1729)
    The deprecated `auth.password` option is removed from the `deckhouse-web` module
 - **[flant-integration]** Add scrape telemetry metrics (with prefix d8_telemetry) from deckhouse pod via new service [#2896](https://github.com/deckhouse/deckhouse/pull/2896)
 - **[istio]** The `auth.password` option is deprecated. Consider using the `user-authn` module. [#1729](https://github.com/deckhouse/deckhouse/pull/1729)
    The deprecated `auth.password` option is removed from the `istio` module.
 - **[node-manager]** Add an alert about missing `control-plane` taints on the `master` node group. [#3057](https://github.com/deckhouse/deckhouse/pull/3057)
 - **[openvpn]** The `auth.password` option is deprecated. Consider using the `user-authn` module. [#1729](https://github.com/deckhouse/deckhouse/pull/1729)
    The deprecated `auth.password` option is removed from the `openvpn` module.
 - **[prometheus]** The `auth.password` option is deprecated. Consider using the `user-authn` module. [#1729](https://github.com/deckhouse/deckhouse/pull/1729)
    The deprecated `auth.password` option is removed from the `prometheus` module.
 - **[upmeter]** Added probe uptime in public status API to use in e2e tests. [#2991](https://github.com/deckhouse/deckhouse/pull/2991)
 - **[upmeter]** The `auth.status.password` and `auth.webui.password` options are deprecated. Consider using the `user-authn` module. [#1729](https://github.com/deckhouse/deckhouse/pull/1729)
    The deprecated `auth.status.password` and `auth.webui.password` options are removed from the `upmeter` module.

## Fixes


 - **[admission-policy-engine]** Watch only desired (constrainted) resources by validation webhook. [#3027](https://github.com/deckhouse/deckhouse/pull/3027)
 - **[cloud-provider-vsphere]** Bump CSI driver to `v2.5.4`. [#3089](https://github.com/deckhouse/deckhouse/pull/3089)
 - **[cloud-provider-yandex]** Removed the Standard layout from the documentation, as it doesn't work. [#3108](https://github.com/deckhouse/deckhouse/pull/3108)
 - **[cloud-provider-yandex]** In case of wget and curl utility usage inside pods, proxy (and proxy ignore) will work. [#3031](https://github.com/deckhouse/deckhouse/pull/3031)
    The `cloud-provider-yandex` module will be restarted if a proxy is enabled in the cluster.
 - **[deckhouse-config]** Disable deckhouse-config webhook for uninitialized cluster. [#3257](https://github.com/deckhouse/deckhouse/pull/3257)
 - **[deckhouse-config]** Apply defaults before spec.settings validation. [#3206](https://github.com/deckhouse/deckhouse/pull/3206)
 - **[ingress-nginx]** Fix auth TLS certificates bug which leads to absent certificates on the Ingress controller bootstrap. [#3259](https://github.com/deckhouse/deckhouse/pull/3259)
 - **[ingress-nginx]** Fix manual pods rollout for `HostPort` inlet. [#3207](https://github.com/deckhouse/deckhouse/pull/3207)
 - **[istio]** Fixed istio control-plane alerts: `D8IstioActualVersionIsNotInstalled`, `D8IstioDesiredVersionIsNotInstalled`. [#3024](https://github.com/deckhouse/deckhouse/pull/3024)
 - **[linstor]** In case of wget and curl utility usage inside pods, proxy (and proxy ignore) will work. [#3031](https://github.com/deckhouse/deckhouse/pull/3031)
    The `linstor` module will be restarted if a proxy is enabled in the cluster.
 - **[metallb]** Add validation for `addressPools` name. [#3110](https://github.com/deckhouse/deckhouse/pull/3110)
 - **[namespace-configurator]** Apply configuration only for namespaces matched the filter in this configuration. [#3273](https://github.com/deckhouse/deckhouse/pull/3273)
 - **[node-manager]** Fix the description in the `NodeGroupMasterTaintIsAbsent` alert. [#3248](https://github.com/deckhouse/deckhouse/pull/3248)
 - **[node-manager]** Fix node-group template generation when `minPerZone==0` and capacity is not set. [#3222](https://github.com/deckhouse/deckhouse/pull/3222)
 - **[node-manager]** Fix script name generation and the bashible-apiserver [#3156](https://github.com/deckhouse/deckhouse/pull/3156)
 - **[node-manager]** Calculate resource requests for a stanby-holder Pod as a percentage of a node's capacity. [#2959](https://github.com/deckhouse/deckhouse/pull/2959)
 - **[prometheus]** Setting up `failureThreshold` of `startupProbes` for the main and longterm Prometheus objects from 60 to 300. [#3064](https://github.com/deckhouse/deckhouse/pull/3064)
    The `prometheus` module will be restarted.
 - **[prometheus]** Update Grafana Home dashboard. [#3015](https://github.com/deckhouse/deckhouse/pull/3015)
 - **[snapshot-controller]** In case of wget and curl utility usage inside pods, proxy (and proxy ignore) will work. [#3031](https://github.com/deckhouse/deckhouse/pull/3031)
    The `snapshot-controller` module will be restarted if a proxy is enabled in the cluster.
 - **[user-authn]** Read CA for OIDC provider from encoded PEM string. [#3249](https://github.com/deckhouse/deckhouse/pull/3249)

## Chore


 - **[basic-auth]** Improved error message on unexpected number of fields in the credentials secret [#3039](https://github.com/deckhouse/deckhouse/pull/3039)
 - **[cloud-provider-yandex]** Remove duplicated keys from YAML in test. [#3094](https://github.com/deckhouse/deckhouse/pull/3094)
 - **[deckhouse-config]** Fix the build of the `deckhouse-config` webhook image. [#3111](https://github.com/deckhouse/deckhouse/pull/3111)
 - **[extended-monitoring]** Pass `HTTP_PROXY`, `HTTPS_PROXY` and `NO_PROXY` environment variables into the `image-availability-exporter`. [#3011](https://github.com/deckhouse/deckhouse/pull/3011)
    The `image-availability-exporter` will be restarted if a proxy is enabled in the cluster.

