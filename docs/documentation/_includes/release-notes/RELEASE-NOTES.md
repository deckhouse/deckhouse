## Version 1.70

### Important

- The `ceph-csi` module has been removed. Use the `csi-ceph` module instead. Deckhouse will not be updated as long as `ceph-csi` is enabled in the cluster. For `csi-ceph` migration instructions, refer to the [module documentation](https://deckhouse.io/products/kubernetes-platform/modules/csi-ceph/stable/).

- Version 1.12 of the NGINX Ingress Controller has been added. The default controller version has been changed to 1.10. All Ingress controllers that do not have an explicitly specified version (via the [controllerVersion](https://deckhouse.io/products/kubernetes-platform/documentation/v1.70/modules/ingress-nginx/cr.html#ingressnginxcontroller-v1-spec-controllerversion) parameter in the IngressNginxController resource or the [defaultControllerVersion](https://deckhouse.io/products/kubernetes-platform/documentation/v1.70/modules/ingress-nginx/configuration.html#parameters-defaultcontrollerversion) parameter in the `ingress-nginx` module) will be restarted.

- The `falco_events` metric (from the `runtime-audit-engine` module) has been removed. The `falco_events` metric was considered deprecated since DKP 1.68. Use the [`falcosecurity_falcosidekick_falco_events_total`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.70/modules/runtime-audit-engine/faq.html#how-to-create-an-alert) metric instead. Dashboards and alerts based on the `falco_events` metric may stop working.

- All DKP components will be restarted during the update.

### Major changes

- In the `Auto` [update mode](https://deckhouse.io/products/kubernetes-platform/documentation/v1.70/modules/deckhouse/configuration.html#parameters-update-mode), patch version updates (for example, from `v1.70.1` to `v1.70.2`) are now applied taking into account the update windows, if they are set. Previously, in this update mode, only minor version updates (for example, from `v1.69.x` to `v1.70.x`) were applied with consideration to update windows, while patch version updates were applied as they appeared on a release channel.
- A node can now be rebooted if the corresponding Node object has the `update.node.deckhouse.io/reboot` annotation set.
- When cleaning up a static node, any local users created by Deckhouse Kubernetes Platform are now also removed.
- Added synchronization monitoring for Istio in multi-cluster configurations. A new alert [`D8IstioRemoteClusterNotSynced`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.70/alerts.html#istio-d8istioremoteclusternotsynced) has been introduced and triggers in the following cases:
  - The remote cluster is offline.
  - The remote API endpoint is not reachable.
  - The remote `ServiceAccount` token is invalid or expired.
  - There is a TLS or certificate issue between the clusters.

- The `deckhouse-controller collect-debug-info` command now also collects [debug information](https://deckhouse.io/products/kubernetes-platform/documentation/v1.70/modules/deckhouse/faq.html#how-to-collect-debug-info) for Istio, including:
  - Resources in the `d8-istio` namespace.
  - CRDs from the `istio.io` and `gateway.networking.k8s.io` groups.
  - `Istio` logs.
  - `Sidecar` logs of a single randomly selected user application.

- A new monitoring dashboard has been added to display OpenVPN certificate status. Upon expiration, server certificates will now be reissued, and client certificates will be removed. The following alerts have been added: \
  - [`OpenVPNClientCertificateExpired`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.70/alerts.html#openvpn-openvpnclientcertificateexpired): Warns about expired client certificates.
  - [`OpenVPNServerCACertificateExpired`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.70/alerts.html#openvpn-openvpnservercacertificateexpired): Warns about an expired OpenVPN CA certificate.
  - [`OpenVPNServerCACertificateExpiringSoon`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.70/alerts.html#openvpn-openvpnservercacertificateexpiringsoon) and [`OpenVPNServerCACertificateExpiringInAWeek`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.70/alerts.html#openvpn-openvpnservercacertificateexpiringinaweek): Warn when an OpenVPN CA certificate is expiring in less than 30 or 7 days, respectively.
  - [`OpenVPNServerCertificateExpired`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.70/alerts.html#openvpn-openvpnservercertificateexpired): Warns about an expired OpenVPN server certificate.
  - [`OpenVPNServerCertificateExpiringSoon`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.70/alerts.html#openvpn-openvpnservercertificateexpiringsoon) and [`OpenVPNServerCertificateExpiringInAWeek`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.70/alerts.html#openvpn-openvpnservercertificateexpiringinaweek): Warn when an OpenVPN server certificate is expiring in less than 30 or 7 days, respectively.

- Monitoring dashboards have been renamed and updated:
  - "L2LoadBalancer" renamed to "MetalLB L2"; pool and column filtering added.
  - "Metallb" renamed to "MetalLB BGP"; pool and column filtering added. The ARP request panel has been removed.
  - "L2LoadBalancer / Pools" renamed to "MetalLB / Pools".

- The `upmeter` module’s PVC size has been increased to accommodate data retention for 13 months. In some cases, the previous PVC size was insufficient.

- The [ModuleSource](https://deckhouse.io/products/kubernetes-platform/documentation/v1.70/cr.html#modulesource) resource status now includes information about module versions in the source.

- The [Module](https://deckhouse.io/products/kubernetes-platform/documentation/v1.70/cr.html#module) resource status now includes information about the module’s lifecycle stage. A module can move through the following stages in its lifecycle: Experimental, Preview, General Availability, and Deprecated. For details on module lifecycle stages and how to evaluate its stability, refer to the [corresponding section in the documentation](https://deckhouse.io/products/kubernetes-platform/documentation/v1.70/module-development/versioning/#how-do-i-figure-out-how-stable-a-module-is).

- It is now possible to use stronger or more modern encryption algorithms (such as `RSA-3072`, `RSA-4096`, or `ECDSA-P256`) for control plane cluster certificates instead of the default `RSA-2048`. You can use the [encryptionAlgorithm](https://deckhouse.io/products/kubernetes-platform/documentation/v1.70/installing/configuration.html#clusterconfiguration-encryptionalgorithm) parameter in the ClusterConfiguration resource to configure this.

- The `descheduler` module can now be configured to evict pods that are using local storage. Use the [evictLocalStoragePods](https://deckhouse.io/products/kubernetes-platform/documentation/v1.70/modules/descheduler/cr.html#descheduler-v1alpha2-spec-evictlocalstoragepods) parameter in the module configuration to adjust this.

- You can now adjust the logging level of the Ingress controller using the [controllerLogLevel](https://deckhouse.io/products/kubernetes-platform/documentation/v1.70/modules/ingress-nginx/cr.html#ingressnginxcontroller-v1-spec-controllerloglevel) parameter in the IngressNginxController resource. The default log level is `Info`. Controlling the logging level can help prevent log collector overload during Ingress controller restarts.

### Security

- The severity level of alerts indicating security policy violations has been raised from 7 to 3.

- The configuration for `Yandex Cloud`, `Zvirt`, and `Dynamix` providers now uses `OpenTofu` instead of `Terraform`. This enables easier provider updates, such as applying fixes for known vulnerabilities (CVEs).

- CVE vulnerabilities have been fixed in the following modules: `chrony`, `descheduler`, `dhctl`, `node-manager`, `registry-packages-proxy`, `falco`, `cni-cilium`, and `vertical-pod-autoscaler`.

### Component version updates

The following DKP components have been updated:

- `containerd`: 1.7.27
- `runc`: 1.2.5
- `go`: 1.24.2, 1.23.8
- `golang.org/x/net`: v0.38.0
- `mcm`: v0.36.0-flant.23
- `ingress-nginx`: 1.12.1
- `terraform-provider-aws`: 5.83.1
- `Deckhouse CLI`: 0.12.1
- `etcd`: v3.5.21

## Version 1.69

### Important

- Support for Kubernetes 1.32 has been added, while support for Kubernetes 1.27 has been discontinued.
  The default Kubernetes version has been changed to [1.30](https://deckhouse.io/products/kubernetes-platform/documentation/v1.69/supported_versions.html#kubernetes).
  In future DKP releases, support for Kubernetes 1.28 will be removed.

- All DKP components will be restarted during the update.

### Major changes

- The `ceph-csi` module is now deprecated.
  Plan to migrate to the [`csi-ceph`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.69/reference/mc/csi-ceph/) module instead.
  For details, refer to the [Ceph documentation](https://deckhouse.io/products/kubernetes-platform/documentation/v1.69/storage/admin/external/ceph.html).

- You can now grant access to Deckhouse web interfaces using user names via the `auth.allowedUserEmails` field.
  Access restriction is configured together with the `auth.allowedUserGroups` parameter
  in configuration of the following modules with web interfaces: `cilium-hubble`, `dashboard`, `deckhouse-tools`,
  `documentation`, `istio`, `openvpn`, `prometheus`, and `upmeter` ([example for `prometheus`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.69/modules/prometheus/configuration.html#parameters-auth-alloweduseremails)).

- A new dashboard **Cilium Nodes Connectivity Status & Latency** has been added to Grafana in the `cni-cilium` module.
  It helps monitor node network connectivity issues.
  The dashboard displays a connectivity matrix similar to the `cilium-health status` command,
  using metrics that are already available in Prometheus.

- A new [`D8KubernetesStaleTokensDetected`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.69/alerts.html#control-plane-manager-d8kubernetesstaletokensdetected) alert has been added in the `control-plane-manager` module
  that is triggered when stale service account tokens are detected in the cluster.

- You can now create a Project from an existing namespace and adopt existing objects into it.
  To do this, annotate the namespace and its resources with `projects.deckhouse.io/adopt`.
  This lets you switch to using Projects without recreating cluster resources.

- A `Terminating` status has been added to ModuleSource and ModuleRelease resources.
  The new status will be displayed when an attempt to delete one of them fails.

- The installer container now automatically configures cluster access after a successful bootstrap.
  A `kubeconfig` file is generated in `~/.kube/config`, and a local TCP proxy is set up through an SSH tunnel.
  This allows you to use kubectl locally right away without manually connecting to the control-plane node via SSH.

- Changes to Kubernetes resources in multi-cluster and federation setups are now tracked directly via Kubernetes API.
  This enables faster synchronization between clusters and eliminates the use of outdated certificates.
  In addition, mounting of ConfigMap and Secret resources into Pods has been removed
  to eliminate family system compromise risks.

- A new [dynamicforward](https://github.com/coredns/coredns/pull/7105) plugin has been added to CoreDNS, improving DNS query processing in the cluster.
  It integrates with `node-local-dns`, continuously monitors `kube-dns` endpoints,
  and automatically updates the list of DNS forwarders.
  If the control-plane node is unavailable,
  DNS queries are still forwarded to available endpoints, improving cluster stability.

- A new log rotation approach has been introduced in the `loki` module.
  Now, old logs are automatically removed when disk usage exceeds a threshold:
  either 95% of PVC size or PVC size minus the size required to store two minutes of log data
  at the configured ingestion rate ([`ingestionRateMB`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.69/modules/loki/configuration.html#parameters-lokiconfig-ingestionratemb)).
  The [`retentionPeriodHours`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.69/modules/loki/configuration.html#parameters-retentionperiodhours) parameter no longer controls the data retention and is used for monitoring alerts only.
  If `loki` begins removing old logs before the set period is reached,
  a [`LokiRetentionPerionViolation`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.69/alerts.html#loki-lokiretentionperionviolation) alert will be triggered,
  informing the user that they must reduce the value of `retentionPeriodHours` or increase the PVC size.

- A new [`nodeDrainTimeoutSecond`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.69/modules/node-manager/cr.html#nodegroup-v1-spec-nodedraintimeoutsecond) parameter lets you set the maximum timeout
  when attempting to drain a node (in seconds) for each NodeGroup resource.
  Previously, you could only use the default value (10 minutes)
  or reduce it to 5 minutes using the `quickShutdown` parameter, which is now deprecated.

- The `openvpn` module now includes a [`defaultClientCertExpirationDays`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.69/modules/openvpn/configuration.html#parameters-clientcertexpirationdays) parameter,
  allowing you to define the lifetime of client certificates.

### Security

Known vulnerabilities have been addressed in the following modules:
`ingress-nginx`, `istio`, `prometheus`, and `local-path-provisioner`.

### Component version updates

The following DKP components have been updated:

- `cert-manager`: 1.17.1
- `dashboard`: 1.6.1
- `dex`: 2.42.0
- `go-vcloud-director`: 2.26.1
- Grafana: 10.4.15
- Kubernetes control plane: 1.29.14, 1.30.1, 1.31.6, 1.32.2
- `kube-state-metrics` (`monitoring-kubernetes`): 2.15.0
- `local-path-provisioner`: 0.0.31
- `machine-controller-manager`: v0.36.0-flant.19
- `pod-reloader`: 1.2.1
- `prometheus`: 2.55.1
- Terraform providers:
  - OpenStack: 1.54.1
  - vCD: 3.14.1

## Version 1.68

### Important

- After the update,
  the UID will change for all Grafana data sources created using the GrafanaAdditionalDatasource resource.
  If a data source was referenced by UID, that reference will no longer be valid.

### Major changes

- A new parameter, [`iamNodeRole`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.68/modules/cloud-provider-aws/cluster_configuration.html#awsclusterconfiguration-iamnoderole),
  has been introduced for the AWS provider.
  It lets you specify the name of the IAM role to bind to all AWS instances of cluster nodes.
  This can come in handy if you need to grant additional permissions (for example, access to ECR, etc.).

- Creating nodes of the [CloudPermanent type](https://deckhouse.io/products/kubernetes-platform/documentation/v1.68/modules/node-manager/cr.html#nodegroup-v1-spec-nodetype)
  now takes less time.
  Now, CloudPermanent nodes are created in parallel.
  Previously, they were created in parallel only within a single group.

- Monitoring changes:
  - Support for monitoring certificates in secrets of the `Opaque` type has been added.
  - Support for monitoring images in Amazon ECR has been added.
  - A bug that could cause partial loss of metrics when Prometheus instances were restarted has been fixed.

- When using a multi-cluster Istio configuration or federation,
  you can now explicitly specify the list of addresses used for inter-cluster requests.
  Previously, these addresses were determined automatically;
  however, in some configurations, they could not be resolved.

- The DexAuthenticator resource now has a [`highAvailability`](https://deckhouse.io/products/kubernetes-platform/documentation/latest/modules/user-authn/cr.html#dexauthenticator-v1-spec-highavailability) parameter
  that controls high availability mode.
  In high availability mode, multiple replicas of the authenticator are launched.
  Previously, high availability mode of all authenticators was determined by a [global parameter](https://deckhouse.io/products/kubernetes-platform/documentation/v1.68/deckhouse-configure-global.html#parameters-highavailability)
  or by the `user-authn` module.
  All authenticators deployed by DKP now inherit the high availability mode of the corresponding module.

- Node labels can now be added, removed, or modified
  using files stored on the node in the `/var/lib/node_labels` directory and its subdirectories.
  The full set of applied labels is stored in the `node.deckhouse.io/last-applied-local-labels` annotation.

- Support for the [Huawei Cloud provider](https://deckhouse.io/products/kubernetes-platform/documentation/v1.68/modules/cloud-provider-huaweicloud/) has been added.

- The new [`keepDeletedFilesOpenedFor`](https://deckhouse.io/products/kubernetes-platform/documentation/latest/modules/log-shipper/cr.html#clusterloggingconfig-v1alpha2-spec-kubernetespods-keepdeletedfilesopenedfor) parameter
  in the `log-shipper` module allows you to configure the period to keep the deleted log files open.
  This way, you can continue reading logs from deleted pods for some time if log storage is temporarily unavailable.

- TLS encryption for log collectors (Elasticsearch, Vector, Loki, Splunk, Logstash, Socket, Kafka)
  can now be configured using secrets, rather than by storing certificates in the ClusterLogDestination resources.
  The secret must reside in the `d8-log-shipper` namespace and have the `log-shipper.deckhouse.io/watch-secret: true` label.

- In the [project](https://deckhouse.io/products/kubernetes-platform/documentation/v1.68/modules/multitenancy-manager/cr.html#project) status under the `resources` section,
  you can now see which project resources have been installed.
  Those resources are marked with `installed: true`.

- A new parameter, `--tf-resource-management-timeout`, has been added to the installer.
  It controls the resource creation timeout in cloud environments.
  By default, the timeout is set to 10 minutes.
  This parameter applies only to the following clouds: AWS, Azure, GCP, OpenStack.

### Security

Known vulnerabilities have been addressed in the following modules:

- `admission-policy-engine`
- `chrony`
- `cloud-provider-azure`
- `cloud-provider-gcp`
- `cloud-provider-openstack`
- `cloud-provider-yandex`
- `cloud-provider-zvirt`
- `cni-cilium`
- `control-plane-manager`
- `extended-monitoring`
- `descheduler`
- `documentation`
- `ingress-nginx`
- `istio`
- `loki`
- `metallb`
- `monitoring-kubernetes`
- `monitoring-ping`
- `node-manager`
- `operator-trivy`
- `pod-reloader`
- `prometheus`
- `prometheus-metrics-adapter`
- `registrypackages`
- `runtime-audit-engine`
- `terraform-manager`
- `user-authn`
- `vertical-pod-autoscaler`
- `static-routing-manager`

### Component version updates

The following DKP components have been updated:

- Kubernetes Control Plane: 1.29.14, 1.30.10, 1.31.6
- `aws-node-termination-handler`: 1.22.1
- `capcd-controller-manager`: 1.3.2
- `cert-manager`: 1.16.2
- `chrony`: 4.6.1
- `cni-flannel`: 0.26.2
- `docker_auth`: 1.13.0
- `flannel-cni`: 1.6.0-flannel1
- `gatekeeper`: 3.18.1
- `jq`: 1.7.1
- `kubernetes-cni`: 1.6.2
- `kube-state-metrics`: 2.14.0
- `vector` (`log-shipper`): 0.44.0
- `prometheus`: 2.55.1
- `snapshot-controller`: 8.2.0
- `yq4`: 3.45.1

### Mandatory component restart

The following components will be restarted after updating DKP to 1.68:

- Kubernetes Control Plane
- Ingress controller
- Prometheus, Grafana
- `admission-policy-engine`
- `chrony`
- `cloud-provider-azure`
- `cloud-provider-gcp`
- `cloud-provider-openstack`
- `cloud-provider-yandex`
- `cloud-provider-zvirt`
- `cni-cilium`
- `control-plane-manager`
- `descheduler`
- `documentation`
- `extended-monitoring`
- `ingress-nginx`
- `istio`
- `kube-state-metrics`
- `log-shipper`
- `loki`
- `metallb`
- `monitoring-kubernetes`
- `monitoring-ping`
- `node-manager`
- `openvpn`
- `operator-trivy`
- `prometheus`
- `prometheus-metrics-adapter`
- `pod-reloader`
- `registrypackages`
- `runtime-audit-engine`
- `service-with-healthchecks`
- `static-routing-manager`
- `terraform-manager`
- `user-authn`
- `vertical-pod-autoscaler`
