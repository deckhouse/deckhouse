# Changelog v1.31

## Know before update


 - All Daemonsets in `d8-*` namespaces are excluded from eviction on a down-scale and kept ready until node removal.
 - Fix deploy of DexAuthenticators to clusters with the OnlyInURI HTTPS mode
 - Ingress controllers of version >=0.33 will be restarted.
 - Kubernetes control-plane components and kubelet will restart for 1.20, 1.21 and 1.22 minor versions.
 - The new module - Linstor. It enables a replicated block storage solution in the cluster using the LINSTOR and the DRBD kernel module.
 - node.kubernetes.io/exclude-from-external-load-balancers label will be deleted from the master node group. It also can not be set manually in the current release.
    Without "node.kubernetes.io/exclude-from-external-load-balancers" label traffic can be directed to control plane nodes.
    In the next release, migration will delete it, and users can add it manually if necessary.

## Features


 - **[cloud-provider-aws]** Update csi images and manifests. [#831](https://github.com/deckhouse/deckhouse/pull/831)
 - **[cloud-provider-azure]** Update csi images and manifests. [#831](https://github.com/deckhouse/deckhouse/pull/831)
 - **[cloud-provider-gcp]** Update csi images and manifests. [#831](https://github.com/deckhouse/deckhouse/pull/831)
 - **[cloud-provider-openstack]** Update csi images and manifests. [#831](https://github.com/deckhouse/deckhouse/pull/831)
 - **[cloud-provider-vsphere]** Update csi images and manifests. [#831](https://github.com/deckhouse/deckhouse/pull/831)
 - **[cloud-provider-vsphere]** Add ability to install Deckhouse in vsphere installations with DRS disabled. [#656](https://github.com/deckhouse/deckhouse/pull/656)
 - **[cloud-provider-yandex]** Update csi images and manifests. [#831](https://github.com/deckhouse/deckhouse/pull/831)
 - **[control-plane-manager]** Added option to stream audit log to stdout. [#949](https://github.com/deckhouse/deckhouse/pull/949)
 - **[control-plane-manager]** Added option to change audit log files location. [#949](https://github.com/deckhouse/deckhouse/pull/949)
 - **[control-plane-manager]** Set Kubernetes version to `Automatic` for clusters where current version is `1.19`. This change applies only in FE/EE release. [#807](https://github.com/deckhouse/deckhouse/pull/807)
 - **[control-plane-manager]** Define default config that spreads Pods between zones with finer granularity than before. [#784](https://github.com/deckhouse/deckhouse/pull/784)
 - **[control-plane-manager]** Allow changing a list of active admission plugins via `controlPlaneManager.apiserver.admissionPlugins` configuration.
    ExtendedResourceToleration and EventRateLimit are always enabled. [#783](https://github.com/deckhouse/deckhouse/pull/783)
 - **[control-plane-manager]** Enabled `TTLAfterFinished` on Kubernetes <1.21.
    Allows to clean up old jobs automatically. 
    https://kubernetes.io/docs/concepts/workloads/controllers/ttlafterfinished/ [#781](https://github.com/deckhouse/deckhouse/pull/781)
 - **[control-plane-manager]** Support bound service account tokens in Kubernetes >=1.21. Support TokenRequest API in all supported Kubernetes versions. [#773](https://github.com/deckhouse/deckhouse/pull/773)
 - **[control-plane-manager]** Allows configuring Kubernetes API LoadBalancer external port via `controlPlaneManager.apiserver.loadBalancer.port` config value. [#765](https://github.com/deckhouse/deckhouse/pull/765)
 - **[deckhouse]** Add node affinity in a `deckhouse` deployment for evicting Pod from converging node. [#353](https://github.com/deckhouse/deckhouse/pull/353)
    Nodes labeled 'dhctl.deckhouse.io/node-for-converge' will be excluded from scheduling `deckhouse` Pod.
 - **[dhctl]** Create additional kube resources according to the order. [#833](https://github.com/deckhouse/deckhouse/pull/833)
 - **[dhctl]** Add unit tests for Terraform runners. [#798](https://github.com/deckhouse/deckhouse/pull/798)
 - **[dhctl]** Add flags to the installation command to deploy Deckhouse without master node selector and tuned connection options. [#716](https://github.com/deckhouse/deckhouse/pull/716)
 - **[dhctl]** Control plane readiness check before control plane node converging. [#353](https://github.com/deckhouse/deckhouse/pull/353)
 - **[extended-monitoring]** Update events_exporter and omit the message field. [#827](https://github.com/deckhouse/deckhouse/pull/827)
 - **[ingress-nginx]** Deny locations with invalid auth URL. [#989](https://github.com/deckhouse/deckhouse/pull/989)
    Ingress controllers of version >=0.33 will be restarted.
 - **[kube-dns]** Add ability to tune cache TTL for stub zones. [#815](https://github.com/deckhouse/deckhouse/pull/815)
 - **[linstor]** Added linstor module [#746](https://github.com/deckhouse/deckhouse/pull/746)
    The new module - Linstor. It enables a replicated block storage solution in the cluster using the LINSTOR and the DRBD kernel module.
 - **[monitoring-kubernetes]** Enable `systemd` collector in the `node-exporter`. [#768](https://github.com/deckhouse/deckhouse/pull/768)
 - **[node-manager]** Add a file with context-building error on failure. [#850](https://github.com/deckhouse/deckhouse/pull/850)
 - **[node-manager]** Upgrade `cluster-autoscaler` to v0.19.0. [#793](https://github.com/deckhouse/deckhouse/pull/793)
    All Daemonsets in `d8-*` namespaces are excluded from eviction on a down-scale and kept ready until node removal.
 - **[node-manager]** Allows changing kubelet log rotation via new NodeGroup parameters: `containerLogMaxSize` and `containerLogMaxFiles`. [#766](https://github.com/deckhouse/deckhouse/pull/766)
 - **[prometheus]** Authenticate using Prometheus service account bearer token. [#718](https://github.com/deckhouse/deckhouse/pull/718)
 - **[user-authn]** Bump Dex to v2.31.0 [#823](https://github.com/deckhouse/deckhouse/pull/823)

## Fixes


 - **[candi]** Renamed the centos-7 bundle to centos. [#1398](https://github.com/deckhouse/deckhouse/pull/1398)
 - **[candi]** Added proper labels and taints on cluster bootstrap to master nodes. [#1287](https://github.com/deckhouse/deckhouse/pull/1287)
 - **[candi]** Update Kubernetes components to the latest patch versions. [#770](https://github.com/deckhouse/deckhouse/pull/770)
    Kubernetes control-plane components and kubelet will restart for 1.20, 1.21 and 1.22 minor versions.
 - **[cloud-provider-aws]** The Standard layout is considered deprecated. [#1292](https://github.com/deckhouse/deckhouse/pull/1292)
 - **[cloud-provider-openstack]** Remove default `podNetworkMode` from the config-values. [#1248](https://github.com/deckhouse/deckhouse/pull/1248)
 - **[cloud-provider-vsphere]** Fix handle of compatibilityFlag in Deckhouse config [#1156](https://github.com/deckhouse/deckhouse/pull/1156)
 - **[common]** The `csi-controller` template requires NAMESPACE environment variable. [#864](https://github.com/deckhouse/deckhouse/pull/864)
 - **[deckhouse]** Remove additional print column applyAfter. [#805](https://github.com/deckhouse/deckhouse/pull/805)
 - **[deckhouse-controller]** Set debug level for snapshot info messages [#1160](https://github.com/deckhouse/deckhouse/pull/1160)
 - **[deckhouse-web]** OpenAPI fix and copy_custom_certificate hook fix — do nothing if the https.mode isn't CustomCertificate, but there is <module>.https.customCertificate.secretName configured. [#755](https://github.com/deckhouse/deckhouse/pull/755)
 - **[dhctl]** Fix potential panic for bashible logs in `dhctl bootstrap` command. [#724](https://github.com/deckhouse/deckhouse/pull/724)
 - **[extended-monitoring]** Start webserver immediately for the extended-monitoring-exporter [#1137](https://github.com/deckhouse/deckhouse/pull/1137)
 - **[ingress-nginx]** Make the VHost Detail dashboard show all locations by default [#1251](https://github.com/deckhouse/deckhouse/pull/1251)
 - **[ingress-nginx]** Fix ingress admission webhook [#1207](https://github.com/deckhouse/deckhouse/pull/1207)
 - **[ingress-nginx]** Proper validating webhook configuration for k8s 1.22+. [#637](https://github.com/deckhouse/deckhouse/pull/637)
 - **[istio]** The `istio.tracing.kiali.jaegerURLForUsers` parameter bugfix. [#1402](https://github.com/deckhouse/deckhouse/pull/1402)
 - **[istio]** AuthURL fix in external_auth.go hook. [#1216](https://github.com/deckhouse/deckhouse/pull/1216)
 - **[istio]** Canary usage doc fix. [#731](https://github.com/deckhouse/deckhouse/pull/731)
 - **[kube-dns]** FAQ clarifications about changing `clusterDomain`, ServiceAccount tokens and Istio. [#686](https://github.com/deckhouse/deckhouse/pull/686)
 - **[linstor]** Add DRBD devices to blacklist on nodes. DRBD devices should not be queried by LVM and multipath commands So we add DRBD devices into blacklist for multipath and configure global_filter in lvm.conf for them. [#1153](https://github.com/deckhouse/deckhouse/pull/1153)
 - **[log-shipper]** Fire the alert only if there are more pods absent than allowed by the DaemonSet status. [#756](https://github.com/deckhouse/deckhouse/pull/756)
 - **[monitoring-applications]** Make dashboards immutable (that weren't already). [#840](https://github.com/deckhouse/deckhouse/pull/840)
 - **[monitoring-kubernetes-control-plane]** Proper kubectl command in alert description. [#741](https://github.com/deckhouse/deckhouse/pull/741)
 - **[node-manager]** Add migration to remove "node.kubernetes.io/exclude-from-external-load-balancers" label from control-plane nodes [#1218](https://github.com/deckhouse/deckhouse/pull/1218)
    node.kubernetes.io/exclude-from-external-load-balancers label will be deleted from the master node group. It also can not be set manually in the current release.
    Without "node.kubernetes.io/exclude-from-external-load-balancers" label traffic can be directed to control plane nodes.
    In the next release, migration will delete it, and users can add it manually if necessary.
 - **[openvpn]** Fixed DexAuthenticator applicationDomain. [#1309](https://github.com/deckhouse/deckhouse/pull/1309)
 - **[prometheus]** Handle path prefix for `CustomAlertmanager`. [#1308](https://github.com/deckhouse/deckhouse/pull/1308)
 - **[prometheus]** Exposing API doc fixes. [#870](https://github.com/deckhouse/deckhouse/pull/870)
 - **[prometheus-metrics-adapter]** Fix custom metrics workability [#1259](https://github.com/deckhouse/deckhouse/pull/1259)
 - **[upmeter]** Make dashboards immutable (that weren't already). [#840](https://github.com/deckhouse/deckhouse/pull/840)
 - **[upmeter]** Rework scheduler with respect of cluster-autoscaler taints. [#793](https://github.com/deckhouse/deckhouse/pull/793)
 - **[user-authn]** Do not set ingress TLS certificate secret name if HTTPS mode is the OnlyInURI [#1128](https://github.com/deckhouse/deckhouse/pull/1128)
    Fix deploy of DexAuthenticators to clusters with the OnlyInURI HTTPS mode
 - **[user-authn]** Fix namespace for DexAuthenticator openvpn adoption [#1112](https://github.com/deckhouse/deckhouse/pull/1112)
 - **[user-authn]** Kubeconfig: hide the "connect to api.%s" button if publish API is not enabled. [#764](https://github.com/deckhouse/deckhouse/pull/764)
 - **[vertical-pod-autoscaler]** If the new calculated `max_allowed` values for Pods are less than 10% of old values, the values are not changed. Hook starts only when Deckhouse Pod becomes ready. [#627](https://github.com/deckhouse/deckhouse/pull/627)

