# Changelog v1.46

## Know before update


 - An alert will be generated for each instance of an object with a deprecated `extended-monitoring.flant.com` annotation. **Pay attention** that you must change deprecated annotations to `extended-monitoring.deckhouse.io` label ASAP.
 - Control plane components and kubelet will restart.
 - If you deploy the `deckhouse-web` moduleConfig via a CI/CD process, then you have to replace it with the `documentation` moduleConfig (run `kubectl get mc documentation -o yaml` to get its content).
 - Ingress controller Pods will restart.
 - Linux Kernel >= 5.8 is required to run the `runtime-audit-engine` module.
 - OpenVPN will be restarted and connections will be terminated.
 - Removed write permissions on `namespace`, `limitrange`, `resourcequota`, `role` and `clusterrole` resources for the `Editor`, `Admin`, and `ClusterEditor` access levels. Read the [issue](https://github.com/deckhouse/deckhouse/pull/4494) description If you want to return the permissions.

## Features


 - **[admission-policy-engine]** Add support for `PrometheusRule`. [#4407](https://github.com/deckhouse/deckhouse/pull/4407)
 - **[candi]** Add deckhouse release requirements about docker presence. [#4816](https://github.com/deckhouse/deckhouse/pull/4816)
    It will be impossible to update Deckhouse until docker is replaced with containerd.
 - **[ceph-csi]** Add support for `PrometheusRule`. [#4407](https://github.com/deckhouse/deckhouse/pull/4407)
 - **[cert-manager]** Add support for `PrometheusRule`. [#4407](https://github.com/deckhouse/deckhouse/pull/4407)
 - **[chrony]** Add support for `PrometheusRule`. [#4407](https://github.com/deckhouse/deckhouse/pull/4407)
 - **[cloud-provider-aws]** Add support for `PrometheusRule`. [#4407](https://github.com/deckhouse/deckhouse/pull/4407)
 - **[cloud-provider-aws]** Add cloud data discoverer service which get information about available instance types for node groups. [#4218](https://github.com/deckhouse/deckhouse/pull/4218)
 - **[cloud-provider-azure]** Add support for `PrometheusRule`. [#4407](https://github.com/deckhouse/deckhouse/pull/4407)
 - **[cloud-provider-azure]** Add cloud data discoverer service which get information about available instance types for node groups. [#4213](https://github.com/deckhouse/deckhouse/pull/4213)
 - **[cloud-provider-gcp]** Add support for `PrometheusRule`. [#4407](https://github.com/deckhouse/deckhouse/pull/4407)
 - **[cloud-provider-gcp]** Add cloud data discoverer service which get information about available instance types for node groups. [#4221](https://github.com/deckhouse/deckhouse/pull/4221)
 - **[cloud-provider-openstack]** Add support for `PrometheusRule`. [#4407](https://github.com/deckhouse/deckhouse/pull/4407)
 - **[cloud-provider-openstack]** Add cloud data discoverer service which gets information about available instance types for node groups. [#4187](https://github.com/deckhouse/deckhouse/pull/4187)
 - **[cloud-provider-vsphere]** Add support for `PrometheusRule`. [#4407](https://github.com/deckhouse/deckhouse/pull/4407)
 - **[cloud-provider-yandex]** Add support for `PrometheusRule`. [#4407](https://github.com/deckhouse/deckhouse/pull/4407)
 - **[cni-cilium]** Add support for `PrometheusRule`. [#4407](https://github.com/deckhouse/deckhouse/pull/4407)
 - **[cni-cilium]** Enable external access to ClusterIP services. [#4302](https://github.com/deckhouse/deckhouse/pull/4302)
    Cilium Pods will be restarted.
 - **[cni-flannel]** Add support for `PrometheusRule`. [#4407](https://github.com/deckhouse/deckhouse/pull/4407)
 - **[cni-simple-bridge]** Add support for `PrometheusRule`. [#4407](https://github.com/deckhouse/deckhouse/pull/4407)
 - **[containerized-data-importer]** Add support for `PrometheusRule`. [#4407](https://github.com/deckhouse/deckhouse/pull/4407)
 - **[dashboard]** Add support for `PrometheusRule`. [#4407](https://github.com/deckhouse/deckhouse/pull/4407)
 - **[deckhouse]** Automatically set GOMAXPROCS according to container limits. [#4595](https://github.com/deckhouse/deckhouse/pull/4595)
 - **[deckhouse]** Add support for `PrometheusRule`. [#4407](https://github.com/deckhouse/deckhouse/pull/4407)
 - **[delivery]** Add support for `PrometheusRule`. [#4407](https://github.com/deckhouse/deckhouse/pull/4407)
 - **[descheduler]** Add support for `PrometheusRule`. [#4407](https://github.com/deckhouse/deckhouse/pull/4407)
 - **[dhctl]** Introduces dependency verification on bootstrap. [#4647](https://github.com/deckhouse/deckhouse/pull/4647)
 - **[flant-integration]** Add support for `PrometheusRule`. [#4407](https://github.com/deckhouse/deckhouse/pull/4407)
 - **[flant-integration]** Add deckhouse controller resource consumption metrics. [#4352](https://github.com/deckhouse/deckhouse/pull/4352)
 - **[ingress-nginx]** Add support for `PrometheusRule`. [#4407](https://github.com/deckhouse/deckhouse/pull/4407)
 - **[istio]** Add support for `PrometheusRule`. [#4407](https://github.com/deckhouse/deckhouse/pull/4407)
 - **[keepalived]** Add support for `PrometheusRule`. [#4407](https://github.com/deckhouse/deckhouse/pull/4407)
 - **[linstor]** Added params for enabled SELinux support. [#4849](https://github.com/deckhouse/deckhouse/pull/4849)
    linstor satellite Pods will be restarted.
 - **[linstor]** Add support for `PrometheusRule`. [#4407](https://github.com/deckhouse/deckhouse/pull/4407)
 - **[local-path-provisioner]** Add support for `PrometheusRule`. [#4407](https://github.com/deckhouse/deckhouse/pull/4407)
 - **[log-shipper]** Add support for `PrometheusRule`. [#4407](https://github.com/deckhouse/deckhouse/pull/4407)
 - **[metallb]** Add support for `PrometheusRule`. [#4407](https://github.com/deckhouse/deckhouse/pull/4407)
 - **[network-gateway]** Add support for `PrometheusRule`. [#4407](https://github.com/deckhouse/deckhouse/pull/4407)
 - **[node-manager]** Automatic capacity nodegroup discovery [#4607](https://github.com/deckhouse/deckhouse/pull/4607)
 - **[node-manager]** Create an event bound to a Node object if node drain was failed during the bashible update. [#4558](https://github.com/deckhouse/deckhouse/pull/4558)
 - **[node-manager]** Remove the `adopt.sh` script and modify the documentation. [#4496](https://github.com/deckhouse/deckhouse/pull/4496)
 - **[node-manager]** Add instance resource. [#4417](https://github.com/deckhouse/deckhouse/pull/4417)
 - **[node-manager]** Add support for `PrometheusRule`. [#4407](https://github.com/deckhouse/deckhouse/pull/4407)
 - **[node-manager]** Add annotation `update.node.deckhouse.io/draining=user` for starting node drain process [#4310](https://github.com/deckhouse/deckhouse/pull/4310)
 - **[okmeter]** Add support for `PrometheusRule`. [#4407](https://github.com/deckhouse/deckhouse/pull/4407)
 - **[openvpn]** Add support for `PrometheusRule`. [#4407](https://github.com/deckhouse/deckhouse/pull/4407)
 - **[operator-prometheus]** Add support for `PrometheusRule`. [#4407](https://github.com/deckhouse/deckhouse/pull/4407)
 - **[operator-trivy]** Add support for `PrometheusRule`. [#4407](https://github.com/deckhouse/deckhouse/pull/4407)
 - **[operator-trivy]** Added CIS Benchmark reports and dashboard. [#3995](https://github.com/deckhouse/deckhouse/pull/3995)
 - **[operator-trivy]** Added the `NodeRestriction` admission plugin and turned on the `RotateKubeletServerCertificate` feature flag via the feature gate. [#3995](https://github.com/deckhouse/deckhouse/pull/3995)
    Control plane components and kubelet will restart.
 - **[pod-reloader]** Add support for `PrometheusRule`. [#4407](https://github.com/deckhouse/deckhouse/pull/4407)
 - **[prometheus]** Display Prometheus alerts as a custom resource in a cluster.
    - To get alerts: `kubectl get clusteralerts`
    - To view an alert: `kubectl get clusteralerts <ALERT_NAME> -o yaml` [#4614](https://github.com/deckhouse/deckhouse/pull/4614)
 - **[prometheus]** Add support for `PrometheusRule`. [#4407](https://github.com/deckhouse/deckhouse/pull/4407)
 - **[prometheus]** Added local alerts receiver, which publishes alerts as events. [#4382](https://github.com/deckhouse/deckhouse/pull/4382)
 - **[runtime-audit-engine]** Migrate to using the modern eBPF probe. [#4552](https://github.com/deckhouse/deckhouse/pull/4552)
    Linux Kernel >= 5.8 is required to run the `runtime-audit-engine` module.
 - **[runtime-audit-engine]** Add support for `PrometheusRule`. [#4407](https://github.com/deckhouse/deckhouse/pull/4407)
 - **[snapshot-controller]** Add support for `PrometheusRule`. [#4407](https://github.com/deckhouse/deckhouse/pull/4407)
 - **[upmeter]** Add support for `PrometheusRule`. [#4407](https://github.com/deckhouse/deckhouse/pull/4407)
 - **[user-authn]** Added validating webhook to check the uniqueness of `userID` and `email` in User object. [#4561](https://github.com/deckhouse/deckhouse/pull/4561)
 - **[user-authn]** Add support for `PrometheusRule`. [#4407](https://github.com/deckhouse/deckhouse/pull/4407)
 - **[user-authz]** Add the new `AuthorizationRule` CR for namespaced control access. [#4494](https://github.com/deckhouse/deckhouse/pull/4494)
 - **[user-authz]** Add support for `PrometheusRule`. [#4407](https://github.com/deckhouse/deckhouse/pull/4407)
 - **[virtualization]** Add support for `PrometheusRule`. [#4407](https://github.com/deckhouse/deckhouse/pull/4407)

## Fixes


 - **[candi]** Force deletion of the `/usr/local/bin/crictl` directory. [#4742](https://github.com/deckhouse/deckhouse/pull/4742)
 - **[candi]** Update bashible network bootstrap in AWS cloud to use IMDSv2 for obtaining instance metadata. [#4632](https://github.com/deckhouse/deckhouse/pull/4632)
 - **[candi]** Events created by bashible get connected to the relevant node objects. [#4623](https://github.com/deckhouse/deckhouse/pull/4623)
 - **[cloud-data-crd]** The `cluster-autoscaler-crd module has been renamed to the `cloud-data-crd` module. [#4497](https://github.com/deckhouse/deckhouse/pull/4497)
 - **[cni-flannel]** flannel's entrypoint now correctly passes arguments to the flannel itself. [#4837](https://github.com/deckhouse/deckhouse/pull/4837)
 - **[cni-flannel]** Fix cleanup flannel used IPs on migration from docker to containerd. [#4306](https://github.com/deckhouse/deckhouse/pull/4306)
 - **[common]** Prevent usage of vulnerable TLS ciphers in `kube-rbac-proxy`. [#4825](https://github.com/deckhouse/deckhouse/pull/4825)
    Ingress controller Pods will restart.
 - **[control-plane-manager]** Fix errors in control-plane-manager converge and preflight checks. [#4822](https://github.com/deckhouse/deckhouse/pull/4822)
    control-plane-manager will restart.
 - **[deckhouse]** Add `prometheus.deckhouse.io/rules-watcher-enabled` on the `d8-system` namespace. [#4752](https://github.com/deckhouse/deckhouse/pull/4752)
 - **[deckhouse]** Remove Deckhouse release naming transformation. [#4568](https://github.com/deckhouse/deckhouse/pull/4568)
 - **[deckhouse]** Change liveness probe for `webhook-handler` to prevent EOF log spamming. [#4562](https://github.com/deckhouse/deckhouse/pull/4562)
    The `webhook-handler` Pod will restart.
 - **[deckhouse-controller]** Reverted "mergo" library update. Fixed an issue where Deckhouse might panic on concurrent map access. [#4872](https://github.com/deckhouse/deckhouse/pull/4872)
 - **[docs]** Update the description of the global `storageClass` parameter. [#4424](https://github.com/deckhouse/deckhouse/pull/4424)
 - **[documentation]** Add migration for the `documentation` module (former name - `deckhouse-web`). [#4982](https://github.com/deckhouse/deckhouse/pull/4982)
    If you deploy the `deckhouse-web` moduleConfig via a CI/CD process, then you have to replace it with the `documentation` moduleConfig (run `kubectl get mc documentation -o yaml` to get its content).
 - **[extended-monitoring]** Send one `ExtendedMonitoringDeprecatatedAnnotation` alert per cluster. [#4829](https://github.com/deckhouse/deckhouse/pull/4829)
 - **[global-hooks]** Fix cluster DNS address discovery. [#4521](https://github.com/deckhouse/deckhouse/pull/4521)
 - **[global-hooks]** Fix the Kubernetes version hook for `DigitalOcean`. [#4473](https://github.com/deckhouse/deckhouse/pull/4473)
 - **[ingress-nginx]** Update the Kruise controller manager before updating Ingress Nginx so that an updated Kruise controller manager takes care of Ingress nginx demonsets. [#5103](https://github.com/deckhouse/deckhouse/pull/5103)
 - **[ingress-nginx]** Fix `proxy-failover-iptables` panicking and `iptables` rules duplicating. [#4959](https://github.com/deckhouse/deckhouse/pull/4959)
 - **[ingress-nginx]** Increase `minReadySeconds` for all inlets. [#4919](https://github.com/deckhouse/deckhouse/pull/4919)
 - **[ingress-nginx]** Fixed incorrect indentation of resources block in `kube-rbac-proxy` container of `kruise-controller-manager` deployment. [#4738](https://github.com/deckhouse/deckhouse/pull/4738)
 - **[linstor]** Add disabling the `lvmetad` service to `NodeGroupConfiguration` for the `linstor` module. [#4885](https://github.com/deckhouse/deckhouse/pull/4885)
 - **[linstor]** Support Debian 11. [#4724](https://github.com/deckhouse/deckhouse/pull/4724)
 - **[linstor]** Enable `WaitForFirstConsumer`. [#4681](https://github.com/deckhouse/deckhouse/pull/4681)
    - all auto-generated linstor storageclasses will be recreated with WaitForFirstConsumer option.
    - all existing Persistent Volumes do not require any update or modifications.
 - **[linstor]** Disable the `auto-resync-after` option. [#4501](https://github.com/deckhouse/deckhouse/pull/4501)
 - **[log-shipper]** Add host label and the doc about labels. [#4383](https://github.com/deckhouse/deckhouse/pull/4383)
 - **[metallb]** Fix MetalLB speaker tolerations. [#4435](https://github.com/deckhouse/deckhouse/pull/4435)
 - **[monitoring-kubernetes]** Resolve symbolic links before getting file system statistics in `kubelet-eviction-thresholds-exporter`. [#4923](https://github.com/deckhouse/deckhouse/pull/4923)
 - **[monitoring-kubernetes]** Fixed path to hostPath in thresholds-exporter. [#4736](https://github.com/deckhouse/deckhouse/pull/4736)
 - **[monitoring-kubernetes]** Remove duplicates of memory graphs on namespace dashboard [#4701](https://github.com/deckhouse/deckhouse/pull/4701)
 - **[node-local-dns]** Added logs if changed state iptables. [#4613](https://github.com/deckhouse/deckhouse/pull/4613)
 - **[node-manager]** Prevent usage of vulnerable TLS ciphers in `bashible-apiserver`. [#4827](https://github.com/deckhouse/deckhouse/pull/4827)
 - **[node-manager]** Fix draining hook queue flooding. [#4770](https://github.com/deckhouse/deckhouse/pull/4770)
 - **[node-manager]** Fix bashible-apiserver altlinux docker containerd version (otherwise, bashible-apiserver will not work). [#4553](https://github.com/deckhouse/deckhouse/pull/4553)
 - **[node-manager]** Fix the error node group condition. [#4367](https://github.com/deckhouse/deckhouse/pull/4367)
 - **[openvpn]** Fix updating user list in HA mode. [#4506](https://github.com/deckhouse/deckhouse/pull/4506)
    OpenVPN will be restarted and connections will be terminated.
 - **[operator-prometheus]** Added secret-field-selector in args. [#4619](https://github.com/deckhouse/deckhouse/pull/4619)
 - **[operator-trivy]** Add support for kubernetes.io/dockercfg secrets in imagePullSecrets pods field for scan jobs. [#4469](https://github.com/deckhouse/deckhouse/pull/4469)
 - **[operator-trivy]** Fixed k8s file permissions. [#3995](https://github.com/deckhouse/deckhouse/pull/3995)
 - **[prometheus]** Fixed `d8_prometheus_fs` metrics. [#4805](https://github.com/deckhouse/deckhouse/pull/4805)
 - **[prometheus]** Fixed creation of multiple CustomAlertmanager resources. [#4402](https://github.com/deckhouse/deckhouse/pull/4402)
 - **[prometheus]** Update Prometheus to `2.43.0` (bug and security fixes, performance improvements). [#4269](https://github.com/deckhouse/deckhouse/pull/4269)
 - **[snapshot-controller]** Added a list of csi drivers that support snapshots to the documentation [#4765](https://github.com/deckhouse/deckhouse/pull/4765)
 - **[user-authn]** Use a static background image for Dex login screen. [#4696](https://github.com/deckhouse/deckhouse/pull/4696)
 - **[user-authz]** Removed some rules for the `Editor`, `Admin`, and `ClusterEditor` access levels. [#4494](https://github.com/deckhouse/deckhouse/pull/4494)
    Removed write permissions on `namespace`, `limitrange`, `resourcequota`, `role` and `clusterrole` resources for the `Editor`, `Admin`, and `ClusterEditor` access levels. Read the [issue](https://github.com/deckhouse/deckhouse/pull/4494) description If you want to return the permissions.
 - **[virtualization]** Fixed docs path. [#4575](https://github.com/deckhouse/deckhouse/pull/4575)

## Chore


 - **[admission-policy-engine]** Admission-policy-engine security dashboard in Grafana has been updated so that OPA Violations table doesn't show multiple entries for the same violations. [#4651](https://github.com/deckhouse/deckhouse/pull/4651)
 - **[candi]** Deliver Kubernetes node components as binaries. [#4458](https://github.com/deckhouse/deckhouse/pull/4458)
    All the Kubernetes control plane and node components will restart.
 - **[candi]** Control plane manager image refactoring in Go. [#4237](https://github.com/deckhouse/deckhouse/pull/4237)
    Control plane manager will restart.
 - **[ceph-csi]** update all CSI images to latest versions [#4600](https://github.com/deckhouse/deckhouse/pull/4600)
 - **[ceph-csi]** The `volumeBindingMode` changed from the `default` to `WaitForFirstConsumer`. [#3974](https://github.com/deckhouse/deckhouse/pull/3974)
 - **[cloud-provider-aws]** update all CSI images to latest versions [#4600](https://github.com/deckhouse/deckhouse/pull/4600)
 - **[cloud-provider-aws]** Change the default etcd disk type from `gp2` to `gp3`, and disk size from `150GB` to `20GB`. [#4560](https://github.com/deckhouse/deckhouse/pull/4560)
 - **[cloud-provider-azure]** update all CSI images to latest versions [#4600](https://github.com/deckhouse/deckhouse/pull/4600)
 - **[cloud-provider-gcp]** update all CSI images to latest versions [#4600](https://github.com/deckhouse/deckhouse/pull/4600)
 - **[cloud-provider-openstack]** update all CSI images to latest versions [#4600](https://github.com/deckhouse/deckhouse/pull/4600)
 - **[cloud-provider-vsphere]** update all CSI images to latest versions [#4600](https://github.com/deckhouse/deckhouse/pull/4600)
 - **[cloud-provider-yandex]** update all CSI images to latest versions [#4600](https://github.com/deckhouse/deckhouse/pull/4600)
 - **[deckhouse-controller]** Bump addon-operator version. [#4425](https://github.com/deckhouse/deckhouse/pull/4425)
 - **[documentation]** Rename the `deckhouse-web` module to the `documentation` module. [#4636](https://github.com/deckhouse/deckhouse/pull/4636)
 - **[documentation]** Renames Deckhouse documentation web interface link in Grafana from "docs" to "documentation". [#4604](https://github.com/deckhouse/deckhouse/pull/4604)
 - **[extended-monitoring]** Starting the migration process from the extended-monitoring annotations to labels. [#4356](https://github.com/deckhouse/deckhouse/pull/4356)
    An alert will be generated for each instance of an object with a deprecated `extended-monitoring.flant.com` annotation. **Pay attention** that you must change deprecated annotations to `extended-monitoring.deckhouse.io` label ASAP.
 - **[flant-integration]** Shell-operator image was updated. [#4562](https://github.com/deckhouse/deckhouse/pull/4562)
    The `flant-pricing` Pod will restart.
 - **[istio]** Shell-operator image was updated. [#4562](https://github.com/deckhouse/deckhouse/pull/4562)
    The `metadata-exporter` Pod will restart.
 - **[linstor]** update LINSTOR to 1.22.1 [#4600](https://github.com/deckhouse/deckhouse/pull/4600)
 - **[linstor]** update all CSI images to latest versions [#4600](https://github.com/deckhouse/deckhouse/pull/4600)
 - **[linstor]** Update DRBD 9.2.3 [#4600](https://github.com/deckhouse/deckhouse/pull/4600)
 - **[linstor]** Adjust timers and timeouts for more stability in non-stable networks. [#4463](https://github.com/deckhouse/deckhouse/pull/4463)
 - **[node-manager]** Add `altlinux` to the default values list of the [`allowedBundles`](https://deckhouse.io/documentation/latest/modules/040-node-manager/configuration.html#parameters-allowedbundles) parameter (EE edition). [#4466](https://github.com/deckhouse/deckhouse/pull/4466)
 - **[operator-trivy]** Update operator-trivy version to `v0.13.1` and trivy version to `v0.40.0`. [#4465](https://github.com/deckhouse/deckhouse/pull/4465)
 - **[prometheus]** Bump Prometheus to `2.44.0`. [#4684](https://github.com/deckhouse/deckhouse/pull/4684)
 - **[prometheus]** Shell-operator image was updated. [#4562](https://github.com/deckhouse/deckhouse/pull/4562)
    Grafana will restart.
 - **[prometheus]** Excluded some modules from the list of enabled modules on the grafana home dashboard and sorted the list alphabetically. [#4384](https://github.com/deckhouse/deckhouse/pull/4384)
 - **[runtime-audit-engine]** Shell-operator image was updated. [#4562](https://github.com/deckhouse/deckhouse/pull/4562)
    The `runtime-audit-engine` Pod will restart.

