# Changelog v1.35

## Know before update


 - All linstor components will be moved from master to system nodes.
 - Ingress controllers will restart.
 - LB Ingress controllers will restart.
 - Prometheus Pods will be restarted.
 - Some system Pods will be restarted: `kube-dns`, `chrony`, Pods of cni-* modules and cloud-provider-* modules.
 - Webhook handler will restart. During the handler restart, Deckhouse controller could generate a few error messages when it will not be able to access the webhook. It should be resolved in the next 15 seconds.

## Features


 - **[control-plane-manager]** Add an alert about the deprecated Kubernetes version. [#2251](https://github.com/deckhouse/deckhouse/pull/2251)
 - **[deckhouse]** Show release status message in a releases list view. [#2029](https://github.com/deckhouse/deckhouse/pull/2029)
 - **[deckhouse]** Added the ability to control disruptive releases manually. [#2025](https://github.com/deckhouse/deckhouse/pull/2025)
 - **[extended-monitoring]** Added events logging to stdout in `events_exporter`. [#2203](https://github.com/deckhouse/deckhouse/pull/2203)
 - **[ingress-nginx]** Adds the ability to exclude ingress metrics via adding the label `ingress.deckhouse.io/discard-metrics: "true"` to a namespace or an Ingress resource.  Ingress controllers will restart once to enable this feature handling. [#2206](https://github.com/deckhouse/deckhouse/pull/2206)
    Ingress controllers will restart.
 - **[ingress-nginx]** Validate Ingress controllers compatibility with Kubernetes version. [#2183](https://github.com/deckhouse/deckhouse/pull/2183)
 - **[istio]** Exclude d8-related namespaces from istiod discovery. [#2188](https://github.com/deckhouse/deckhouse/pull/2188)
    Deckhouse services won't be accessible from applications (except `d8-user-authn` and `d8-ingress-nginx`).
 - **[istio]** Data plane versions monitoring refactoring. [#2181](https://github.com/deckhouse/deckhouse/pull/2181)
 - **[linstor]** Added local spatch-as-a-service to generate and cache DRBD compatibility patches. [#2056](https://github.com/deckhouse/deckhouse/pull/2056)
    This change reduces the size of the kernel-module-injector container by removing spatch dependencies and introduces centralized server for DRBD compatibility patches which makes possible to build DRBD without spatch in isolated environments.
 - **[linstor]** Add linstor-affinity-controller. [#2056](https://github.com/deckhouse/deckhouse/pull/2056)
    New `linstor-affinity-controller` allows to keep nodeAffinity rules updated for PVs with provisioned with `allowRemoteVolumeAccess=false`.
 - **[linstor]** Added the ability to specify a master passphrase. [#2054](https://github.com/deckhouse/deckhouse/pull/2054)
    A master password enables some features like backup shipping and volume encryption using LUKS.

## Fixes


 - **[candi]** Fixed `cloudNATAddresses` discovery when bootstrapping cluster in GCP with the `standard` layout. [#2157](https://github.com/deckhouse/deckhouse/pull/2157)
 - **[candi]** Tolerate CA `DeletionCandidateOfClusterAutoscaler` taint for some system Pods. [#2125](https://github.com/deckhouse/deckhouse/pull/2125)
    Some system Pods will be restarted: `kube-dns`, `chrony`, Pods of cni-* modules and cloud-provider-* modules.
 - **[cloud-provider-yandex]** Fix defaults for `diskType` and `platformID`. [#2179](https://github.com/deckhouse/deckhouse/pull/2179)
 - **[deckhouse]** Ignore evicted and shutdown Pods on the Deckhouse update process as they may block the update. [#2266](https://github.com/deckhouse/deckhouse/pull/2266)
 - **[deckhouse]** Fix webhook handler TLS certificate expiration time. [#2146](https://github.com/deckhouse/deckhouse/pull/2146)
    Webhook handler will restart. During the handler restart, Deckhouse controller could generate a few error messages when it will not be able to access the webhook. It should be resolved in the next 15 seconds.
 - **[dhctl]** Fixed `config render bashible-bundle` command and added `config render master-bootstrap-scripts` command. [#2212](https://github.com/deckhouse/deckhouse/pull/2212)
 - **[dhctl]** Fixed output `Request failed. Probably pod was restarted during installation` multiple times during the bootstrap cluster. [#2167](https://github.com/deckhouse/deckhouse/pull/2167)
 - **[flant-integration]** Fix expression for `D8PrometheusMadisonErrorSendingAlertsToBackend`. [#2298](https://github.com/deckhouse/deckhouse/pull/2298)
 - **[go_lib]** Fixed `copy_custom_certificate` value priority. [#2299](https://github.com/deckhouse/deckhouse/pull/2299)
 - **[ingress-nginx]** Fix pbmetrics collector. [#2318](https://github.com/deckhouse/deckhouse/pull/2318)
    Ingress Nginx controller will restart.
 - **[ingress-nginx]** Add publish-service for LB controllers to update Ingress status correctly. [#2276](https://github.com/deckhouse/deckhouse/pull/2276)
    LB Ingress controllers will restart.
 - **[istio]** Fixed `D8IstioDataPlaneVersionMismatch` alert. [#2370](https://github.com/deckhouse/deckhouse/pull/2370)
 - **[istio]** Don't exclude d8-namespaces from istiod discovery. [#2315](https://github.com/deckhouse/deckhouse/pull/2315)
 - **[istio]** Don't export unready `ingressgateway` nodes via `metadata-exporter` for multiclusters and federations. [#2055](https://github.com/deckhouse/deckhouse/pull/2055)
 - **[kube-dns]** Set `prefer_udp` option in `forward` plugin. [#2413](https://github.com/deckhouse/deckhouse/pull/2413)
 - **[linstor]** Fix `linstor-node` label and `podAntiAffinity` in HA mode. [#2408](https://github.com/deckhouse/deckhouse/pull/2408)
 - **[namespace-configurator]** React to module values changes. [#2277](https://github.com/deckhouse/deckhouse/pull/2277)
 - **[prometheus]** Fix Grafana dashboard provisioned — avoid missing all dashboards on update. [#2384](https://github.com/deckhouse/deckhouse/pull/2384)
 - **[prometheus]** Changes Grafana version in `patches/build_go.patch.tpl` automatically from docker arguments. [#2214](https://github.com/deckhouse/deckhouse/pull/2214)
 - **[prometheus]** Do not restart Trickster if Prometheus is unavailable. [#1972](https://github.com/deckhouse/deckhouse/pull/1972)
 - **[prometheus-metrics-adapter]** Use scrape interval x2 as a resync interval to fix missing metrics flapping and added more logs. [#1970](https://github.com/deckhouse/deckhouse/pull/1970)
 - **[upmeter]** Fixed bug when cleaning old upmeter probe garbage resulting in errors stucks Deckhouse main queue. [#2264](https://github.com/deckhouse/deckhouse/pull/2264)
 - **[user-authn]** Fixed LDAP `insecureNoSSL` support. [#2065](https://github.com/deckhouse/deckhouse/pull/2065)

## Chore


 - **[ceph-csi]** Upgrade CSI images [#2056](https://github.com/deckhouse/deckhouse/pull/2056)
    Upgrade CSI components images:
    - provisioner v3.2.1
    - attacher v3.5.0
    - resizer v1.5.0
    - registrar v2.5.1
    - snapshotter v6.0.1
    - livenessprobe v2.7.0
 - **[cloud-provider-aws]** Upgrade CSI images [#2056](https://github.com/deckhouse/deckhouse/pull/2056)
    Upgrade CSI components images:
    - provisioner v3.2.1
    - attacher v3.5.0
    - resizer v1.5.0
    - registrar v2.5.1
    - snapshotter v6.0.1
    - livenessprobe v2.7.0
 - **[cloud-provider-azure]** Upgrade CSI images [#2056](https://github.com/deckhouse/deckhouse/pull/2056)
    Upgrade CSI components images:
    - provisioner v3.2.1
    - attacher v3.5.0
    - resizer v1.5.0
    - registrar v2.5.1
    - snapshotter v6.0.1
    - livenessprobe v2.7.0
 - **[cloud-provider-azure]** Rewrite hooks on Go. [#1799](https://github.com/deckhouse/deckhouse/pull/1799)
 - **[cloud-provider-gcp]** Upgrade CSI images [#2056](https://github.com/deckhouse/deckhouse/pull/2056)
    Upgrade CSI components images:
    - provisioner v3.2.1
    - attacher v3.5.0
    - resizer v1.5.0
    - registrar v2.5.1
    - snapshotter v6.0.1
    - livenessprobe v2.7.0
 - **[cloud-provider-openstack]** Upgrade CSI images [#2056](https://github.com/deckhouse/deckhouse/pull/2056)
    Upgrade CSI components images:
    - provisioner v3.2.1
    - attacher v3.5.0
    - resizer v1.5.0
    - registrar v2.5.1
    - snapshotter v6.0.1
    - livenessprobe v2.7.0
 - **[cloud-provider-vsphere]** Upgrade CSI images [#2056](https://github.com/deckhouse/deckhouse/pull/2056)
    Upgrade CSI components images:
    - provisioner v3.2.1
    - attacher v3.5.0
    - resizer v1.5.0
    - registrar v2.5.1
    - snapshotter v6.0.1
    - livenessprobe v2.7.0
 - **[cloud-provider-yandex]** Upgrade CSI images [#2056](https://github.com/deckhouse/deckhouse/pull/2056)
    Upgrade CSI components images:
    - provisioner v3.2.1
    - attacher v3.5.0
    - resizer v1.5.0
    - registrar v2.5.1
    - snapshotter v6.0.1
    - livenessprobe v2.7.0
 - **[control-plane-manager]** Renamed `D8EtcdCannotDecreaseQuotaBackendBytes` alerts and fixed description. [#2140](https://github.com/deckhouse/deckhouse/pull/2140)
 - **[deckhouse]** Added the ability to resume a suspended release. [#1964](https://github.com/deckhouse/deckhouse/pull/1964)
 - **[deckhouse-controller]** Fixed `vimrc.local`. [#2197](https://github.com/deckhouse/deckhouse/pull/2197)
 - **[linstor]** Move linstor components to system nodes. [#2056](https://github.com/deckhouse/deckhouse/pull/2056)
    All linstor components will be moved from master to system nodes.
 - **[linstor]** Upgrade LINSTOR to v1.19.1 [#2056](https://github.com/deckhouse/deckhouse/pull/2056)
    Upgrade LINSTOR components:
    - DRBD v9.1.8
    - drbd-utils v9.21.4
    - drbd-reacotr v0.8.0
    - linstor-csi v0.20.0
    - linstor-scheduler-extender 0.2.1
    - linstor-server v1.19.1
    - linstor-client v1.14.0
    - linstor-api v1.14.0
    - piraeus-ha-controller v1.1.0
    - piraeus-operator v1.9.1
 - **[linstor]** Include quorum options into linstor storageClasses [#2056](https://github.com/deckhouse/deckhouse/pull/2056)
    Update linstor storageClasses to include recommended quorum options.
 - **[linstor]** Upgrade CSI images [#2056](https://github.com/deckhouse/deckhouse/pull/2056)
    Upgrade CSI components images:
    - provisioner v3.2.1
    - attacher v3.5.0
    - resizer v1.5.0
    - registrar v2.5.1
    - snapshotter v6.0.1
    - livenessprobe v2.7.0
 - **[monitoring-kubernetes]** Fixed units for network graphs. [#2189](https://github.com/deckhouse/deckhouse/pull/2189)
 - **[node-manager]** Increases verbosity to the `bashible-apiserver` logs. [#2150](https://github.com/deckhouse/deckhouse/pull/2150)
    `bashible-apiserver` will restart.
 - **[prometheus]** Update Prometheus to the latest LTS version (v2.37.0). [#2034](https://github.com/deckhouse/deckhouse/pull/2034)
    Prometheus Pods will be restarted.
 - **[prometheus]** Update Prometheus to v2.36.2 (decreases memory consumption). [#2006](https://github.com/deckhouse/deckhouse/pull/2006)
    Prometheus Pods will be restarted.
 - **[prometheus]** Prometheus doc VPA example fix. [#1720](https://github.com/deckhouse/deckhouse/pull/1720)

