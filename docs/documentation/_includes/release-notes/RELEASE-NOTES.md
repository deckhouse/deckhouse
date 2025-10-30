## Version 1.73

### Important

- This release includes several important security improvements. Multiple known vulnerabilities have been fixed, including one in the user-authn module (CVE-2025-22868) that could potentially allow bypassing authentication checks. It is recommended that you schedule this update. See the Security section for details.

- The `dashboard` module will be removed in DKP version 1.75. Use the [Deckhouse web UI](https://deckhouse.io/products/kubernetes-platform/documentation/v1.73/user/web/ui.html) instead (requires the [`console`](https://deckhouse.io/modules/console/stable/) module to be enabled).

- The built-in runtime-audit-engine module is now loaded from an external source (ModuleSource deckhouse).

- All DKP components will be restarted during the update.

### Major changes

- Added support for changing the registry settings (via the [`mode`](https://deckhouse.io/modules/deckhouse/v1.73/configuration.html#parameters-registry-mode) parameter) in `Unmanaged` mode, similar to `Direct` mode. A new parameter [`checkMode`](https://deckhouse.io/modules/deckhouse/v1.73/configuration.html#parameters-registry-direct-checkmode) has been added to control registry validation behavior:
  - `Default`: Verifies the availability of all system module images and the deckhouse-controller images.
  - `Relax`: Verifies only the deckhouse-controller images.

- Updated requirements for the `email` field of the [User](https://deckhouse.io/modules/user-authn/v1.73/cr.html#user) object: the email address must now be in lowercase. Existing users will not be affected.

- Dex updated to **v2.44.0**. It now allows authentication through available identity providers if one of them is down, and supports authentication via identity providers through a proxy.

- The [User](https://deckhouse.io/modules/user-authn/v1.73/cr.html#user) object status now displays the reason for user [`lockout`](https://deckhouse.io/modules/user-authn/v1.73/configuration.html#parameters-passwordpolicy-lockout) (controlled by the lockout parameter).

- Added the [`additionalDisks`](https://deckhouse.io/modules/cloud-provider-dvp/v1.73/cluster_configuration.html#dvpclusterconfiguration-masternodegroup-instanceclass-additionaldisks) parameter for the Deckhouse Virtualization Platform integration provider, allowing creation and attachment of additional disks to VMs in a NodeGroup (`size` and StorageClass must be specified). This simplifies data distribution across multiple disks.

- Added support for [`additionalMetadata`](https://deckhouse.io/modules/cloud-provider-vcd/v1.73/cr.html#vcdinstanceclass-v1-spec-additionalmetadata) in objects (networks, VMs, disks) for the VMware Cloud Director integration provider. Metadata is merged with existing data, with `additionalMetadata` values taking precedence. Changing `additionalMetadata` triggers recreation of CloudEphemeral node groups that use the affected VCDInstanceClass.

- For the VMware vSphere integration provider, you can now specify an SPBM storage policy ID (via the [`storagePolicyID`](https://deckhouse.io/modules/cloud-provider-vsphere/v1.73/cluster_configuration.html#vsphereclusterconfiguration-storagepolicyid) parameter) and configure automatic creation of a StorageClass for each available SPBM policy. You can now explicitly select a policy for master and worker nodes and use the corresponding storage classes.

- Added alerts to help plan module deprecation or migration:
  - [ModuleIsDeprecated](https://deckhouse.io/products/kubernetes-platform/documentation/v1.73//reference/alerts.html#monitoring-deckhouse-moduleisdeprecated): Notifies when a module is deprecated and nearing end of support.
  - [D8ModuleOutdatedByMajorVersion](https://deckhouse.io/products/kubernetes-platform/documentation/v1.73//reference/alerts.html#monitoring-deckhouse-d8moduleoutdatedbymajorversion): Notifies when a module is behind by one or more major versions.

- Added [GeoIPDownloadErrorDetected](https://deckhouse.io/products/kubernetes-platform/documentation/v1.73/reference/alerts.html#ingress-nginx-geoipdownloaderrordetected) alert to notify about MaxMind GeoIP database download issues.

- The [update notification workflow](https://deckhouse.io/modules/deckhouse/v1.73/usage.html#deckhouse-update-notifications) has changed — a release is applied only after the notification is successfully delivered to the configured webhook. If delivery fails, the update is paused until the webhook is restored.

- Reorganized in-cluster documentation. All module documentation (including connected ones) is now located under the [Modules section]((https://deckhouse.io/modules/)). Search has been updated.

- For NGINX Ingress Controller v1.10, added the option to enable the profiler (via the [`nginxProfilingEnabled`](https://deckhouse.ru/modules/ingress-nginx/v1.73/cr.html#ingressnginxcontroller-v1-spec-nginxprofilingenabled) parameter). Enabling the profiler increases resource consumption but may be useful for debugging controller issues.

- Added support for custom HTTP authentication headers (via the [`headers`](https://deckhouse.io/modules/upmeter/v1.73/cr.html#upmeterremotewrite-v1-spec-config-headers) parameter of UpmeterRemoteWrite) when sending SLA monitoring metrics via Prometheus Remote Write protocol.

- Optimized DKP core module loading, reducing startup time in clusters with many modules.

- Audit logs now show which OIDC provider issued the authentication token

- Deckhouse CLI (`d8`) updated to v0.20.7:
  - Added the [`d8 status`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.73/cli/d8/reference/#d8-status) command, which provides a quick cluster summary (nodes, releases, Deckhouse pods, alerts, registry, Deckhouse settings, CNI, queue state).
  - Added the [`d8 k debug`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.73/cli/d8/reference/#d8-k-debug) command, which runs the DKP built-in debug container (image can be overridden via `--image`). This simplifies interactive pod debugging.
  - Added the `--watch` flag to the [`d8 system queue list`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.73/cli/d8/reference/#d8-system-queue-list) command to track queue state changes.

### Security

- Improved baseline container security. Updated security profiles for the following modules to restrict privileges and access rights to the minimum required: `cni-cilium`, `cni-flannel`, `cni-simple-bridge`, `ingress-nginx`, `istio`, `keepalived`, `kube-dns`, `kube-proxy`, `node-local-dns`, `network-gateway`, `network-policy-engine`, `open-vpn`.

- Added the [`allowRbacWildcards`](https://deckhouse.io/modules/admission-policy-engine/v1.73/cr.html#securitypolicy-v1alpha1-spec-policies-allowrbacwildcards) flag to the SecurityPolicy, controlling whether wildcards are allowed in Role and RoleBinding definitions (set to `true` by default). Security policies can now also restrict interactive connections to pods (`CONNECT` for `pods/exec` and `pods/attach`) within namespaces.

- Added support for preventing creation of pods with specific tolerations from a list ([`pods.disallowedTolerations`](https://deckhouse.io/modules/admission-policy-engine/v1.73/cr.html#operationpolicy-v1alpha1-spec-policies-disallowedtolerations) parameter in the operational policy). This helps prevent user workloads from running on nodes reserved for special tasks.

- Enhanced security in NGINX Ingress Controller v1.12 (distroless image, vulnerability fixes, and other improvements).

- Fixed known vulnerabilities in the following modules: `operator-trivy`, `registry`, `user-authn`, `cloud-provider-dvp`, `multitenancy-manager`, `admission-policy-engine`, `ingress-nginx`, `alertmanager`, `metallb`, `istio`, `node-local-dns`, `kube-apiserver`.

### Network

- For the VMware Cloud Director integration provider, added support for [LoadBalancer](https://deckhouse.io/modules/cloud-provider-vcd/v1.73/environment.html#using-the-loadbalancer) backed by VMware NSX Advanced Load Balancer (ALB/Avi) when using `NSX-T`. Requires the Load Balancer feature to be enabled on the Edge Gateway. If LoadBalancer is enabled after cluster creation, DKP automatically detects and applies the change within an hour. For open ports, DKP creates corresponding `Pool + Virtual Service` pairs. If there is a firewall, add allow rules for the LoadBalancer’s external IP address and relevant ports.

### Component version updates

The following DKP components have been updated:

- `Kubernetes Control Plane`: 1.31.13, 1.32.9, 1.33.5
- `Deckhouse CLI`: 0.20.7
- `shell-operator`: 1.9.3
- `OpenTofu`: 1.9.4
- `CoreDNS`: 1.13.1
- `cloud-provider-dynamix`:
  - `cloud-data-discoverer`: 0.7.0
- `cloud-provider-huaweicloud`:
  - `cloud-data-discoverer`: 0.7.0
- `cloud-provider-yandex`:
  - `yandex-csi-plugin`: 0.15.0
- `node-manager`:
  - `nvidia-device-plugin`: 0.17.4
  - `nvidia-mig-manager`: 0.12.3
- `admission-policy-engine`:
  - `gatekeeper`: 3.18.3
- `registry`:
  - `docker-auth`: 1.14.0
- `user-authn`:
  - `dex`: 2.44.0
- `prometheus-pushgateway`: 1.11.1
- `monitoring-kubernetes`:
  - `ebpf-exporter`: 2.5.1
  - `kube-state-metrics`: 2.17.0
  - `node-exporter`: 1.9.1
- `loki`: 2.9.15

## Version 1.72

### Important

- All DKP components will be restarted during the update.
- To use [experimental modules](https://deckhouse.io/products/kubernetes-platform/documentation/v1.72/architecture/module-development/versioning/#module-lifecycle) in the cluster, you now need to explicitly enable the [allowExperimentalModules](https://deckhouse.io/modules/deckhouse/configuration.html#parameters-allowexperimentalmodules) parameter. By default, experimental modules are disabled. Modules that were enabled before the update will not be automatically disabled. However, if an experimental module enabled prior to the update is manually disabled during the update process, you will need to grant permission to use experimental modules again in order to re-enable it.
- If there are WireGuard interfaces on the cluster nodes, you must update the Linux kernel to version 6.8 or higher.

### Major changes

- Added a new [registry module](https://deckhouse.io/modules/registry/) and the ability to adjust container registry parameters without restarting all DKP components. [Two modes](https://deckhouse.io/modules/deckhouse/configuration.html#parameters-registry-mode) for working with the container registry are now available in DKP: `Unmanaged` (the approach used in previous versions) and `Direct` (a new mode). In `Direct` mode, DKP creates a virtual container registry address in the cluster that all DKP components use. Changing the container registry address (for example, switching to a different registry or changing the DKP edition in the cluster) in this mode does not trigger a forced restart of all DKP components.

- Added support for recursive DNS servers (configured via the [recursiveSettings](https://deckhouse.io/modules/cert-manager/configuration.html#parameters-recursivesettings) section of the `cert-manager` module). They are used to verify the existence of a DNS record before starting the ACME DNS-01 domain ownership validation process. This is useful if the same domain is used both publicly and within the cluster, or if the domain has dedicated authoritative DNS servers.

- Introduced separation of modules into critical and functional using the `critical` flag in `module.yaml`. Critical modules are started first. Functional modules are started after the bootstrap process is complete. Their tasks run in parallel and do not block the queue in case of failure. This speeds up cluster installation and improves fault tolerance when starting modules.

- You can now enable logging of all DNS queries (the [enableLogs](https://deckhouse.io/modules/node-local-dns/configuration.html#parameters-enablelogs) parameter of the `node-local-dns` module).

- In the `cloud-provider-vcd` module, a new [WithNAT layout](https://deckhouse.io/modules/cloud-provider-vcd/layouts.html#withnat) has been added for cluster deployment. It automatically configures NAT and, if necessary, firewall rules for accessing nodes through a bastion host. It also supports both `NSX-T` and `NSX-V`. This makes it possible to deploy a cluster in VMware Cloud Director without pre-configuring the environment (unlike the `Standard` layout).

### Security

- Added the fields `user-authn.deckhouse.io/name` and `user-authn.deckhouse.io/preferred_username` to [Kubernetes audit log events](https://deckhouse.io/modules/control-plane-manager/#auditing). These fields display user claims from the OIDC provider, improving authentication monitoring and troubleshooting.

- Kubernetes versions 1.30–1.33 have been updated to the latest patch releases.

- For the AWS provider, added the ability to disable the creation of default security groups (the [disableDefaultSecurityGroup](https://deckhouse.io/modules/cloud-provider-aws/cluster_configuration.html#awsclusterconfiguration-disabledefaultsecuritygroup) parameter). When disabled, security groups must be created manually and explicitly specified in AWSClusterConfiguration, AWSInstanceClass, and NodeGroup. This new feature provides greater control over security settings.

- Added support for password policies for local users (configured in the [passwordPolicy](https://deckhouse.io/modules/user-authn/configuration.html#parameters-passwordpolicy) section). You can now enforce a minimum password complexity, set password expiration, require password rotation, prevent reuse of old passwords, and lock accounts after a specified number of failed login attempts. These changes allow administrators to centrally enforce password requirements and improve cluster security.

### Component version updates

The following DKP components have been updated:

- Kubernetes control plane: 1.30.14, 1.31.11, 1.32.7, 1.33.3
- `cloud-provider-huaweicloud cloud-data-discoverer`: v0.6.0
- `node-manager capi-controller-manager`: 1.10.4

## Version 1.71

### Important

- Prometheus has been replaced with Deckhouse Prom++. If you want to keep using Prometheus, disable the `prompp` module manually before upgrading DKP by running the command `d8 system module disable prompp`.

- Support for Kubernetes 1.33 has been added, while support for Kubernetes 1.28 has been discontinued. In future DKP releases, support for Kubernetes 1.29 will be removed. The default Kubernetes version (used when the [`kubernetesVersion`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.71/installing/configuration.html#clusterconfiguration-kubernetesversion) parameter is set to `Automatic`) has been changed to [1.31](https://deckhouse.io/products/kubernetes-platform/documentation/v1.71/supported_versions.html#kubernetes).

- Upgrading the cluster to Kubernetes 1.31 requires a sequential update of all nodes, with each node drained. You can control how node updates requiring workload disruptions are applied using the [`disruptions`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.71/modules/node-manager/cr.html#nodegroup-v1-spec-disruptions) parameter section.

- The built-in `snapshot-controller` and `static-routing-manager` modules will now be replaced with their external counterparts of the same name, sourced via ModuleSource deckhouse.

- The new version of Cilium requires nodes to run Linux kernel version 5.8 or newer. If any node in the cluster has a kernel older than 5.8, the Deckhouse Kubernetes Platform upgrade will be blocked. Cilium Pods will be restarted.

- All DKP components will be restarted during the update.

### Major changes

- You can now enforce two-factor authentication for static users. This is configured via the [`staticUsers2FA`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.71/modules/user-authn/configuration.html#parameters-staticusers2fa) parameter section of the `user-authn` module.

- Added support for GPUs on nodes. Three GPU resource sharing modes are now available: Exclusive (no sharing), TimeSlicing (time-based sharing), and MIG (a single GPU split into multiple instances). The NodeGroup [spec.gpu](https://deckhouse.io/products/kubernetes-platform/documentation/v1.71/modules/node-manager/cr.html#nodegroup-v1-spec-gpu) parameter section is used to configure the GPU resource sharing mode. Using a GPU on a node requires installing the NVIDIA Container Toolkit and the GPU driver.

- When enabling a module (with `d8 system module enable`) or editing a ModuleConfig resource, a warning is now displayed if multiple module sources are found. In such a case, explicitly specify the module source using the [source](https://deckhouse.io/products/kubernetes-platform/documentation/v1.71/cr.html#moduleconfig-v1alpha1-spec-source) parameter in the module’s configuration.

- Improved error handling for module configuration. Module-related errors no longer block DKP operations. Instead, they are now displayed in the status fields of Module and ModuleRelease objects.

- Improved virtualization support:
  - Added a provider for integration with [Deckhouse Virtualization Platform (DVP)](https://deckhouse.io/products/kubernetes-platform/documentation/v1.71/modules/cloud-provider-dvp/), enabling deployment of DKP clusters on top of DVP.
  - Added support for nested virtualization on nodes in the `cni-cilium` module.

- The `node-manager` module now includes several enhancements for improved node reliability and manageability:
  - You can now prevent a node from restarting if it still hosts critical Pods (labeled with `pod.deckhouse.io/inhibit-node-shutdown`). This can be necessary for workloads with stateful components, such as long-running data migrations.
  - Introduced API version `v1alpha2` for the [SSHCredential](https://deckhouse.io/products/kubernetes-platform/documentation/v1.71/modules/node-manager/cr.html#sshcredentials) resource, where the [`sudoPasswordEncoded`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.71/modules/node-manager/cr.html#sshcredentials-v1alpha2-spec-sudopasswordencoded) parameter allows specifying the `sudo` password in Base64 format.
  - The [`capiEmergencyBrake`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.71/modules/node-manager/configuration.html#parameters-capiemergencybrake) parameter allows you to disable Cluster API (CAPI) in emergency scenarios, preventing potentially destructive changes. Its behavior is similar to the existing [`mcmEmergencyBrake`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.71/modules/node-manager/configuration.html#parameters-mcmemergencybrake) setting.

- Added a pre-installation check to verify connectivity to the DKP container image registry.

- Improved the log file rotation mechanism when using short-term log storage (via the `loki` module). Added the [`LokiInsufficientDiskForRetention`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.71/alerts.html#loki-lokiinsufficientdiskforretention) alert to warn about insufficient disk space for log retention.

- The documentation now includes a [reference for the Deckhouse CLI](https://deckhouse.io/products/kubernetes-platform/documentation/v1.71/deckhouse-cli/reference/) (`d8` utility) commands and parameters.

- When using CEF encoding for collecting logs from [Apache Kafka](https://deckhouse.io/products/kubernetes-platform/documentation/v1.71/modules/log-shipper/cr.html#clusterlogdestination-v1alpha1-spec-kafka-encoding-cef) or [socket](https://deckhouse.io/products/kubernetes-platform/documentation/v1.71/modules/log-shipper/cr.html#clusterlogdestination-v1alpha1-spec-socket-encoding-cef) sources, you can now configure auxiliary CEF fields such as Device Product, Device Vendor, and Device ID.

- The [`passwordHash`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.71/modules/node-manager/cr.html#nodeuser-v1-spec-passwordhash) field in the NodeUser resource is no longer required. This allows you to create users without passwords — for example, in clusters that use external authentication systems (such as PAM or LDAP).

- Added support for CRI Containerd v2 with CgroupsV2. The new version introduces a different configuration format and includes a mechanism to migrate between Containerd v1 and v2. You can change the CRI type used on nodes via the [`cri.type`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.71/modules/node-manager/cr.html#nodegroup-v1-spec-cri-type) parameter and configure it using [`cri.containerdV2`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.71/modules/node-manager/cr.html#nodegroup-v1-spec-cri-containerdv2).

### Security

- [Container image signature verification](https://deckhouse.io/products/kubernetes-platform/documentation/v1.71/modules/admission-policy-engine/cr.html#securitypolicy-v1alpha1-spec-policies-verifyimagesignatures) is now available in DKP SE+. This feature is now supported in DKP SE+ and EE.

- The `log-shipper`, `deckhouse-controller`, and `Istio` (version 1.21) modules have been migrated to distroless builds. This improves security and ensures a more transparent and controlled build process.

- New audit rules have been added to track interactions with containerd. The following are now monitored: access to the `/run/containerd/containerd.sock` socket, modifications to the `/etc/containerd` and `/var/lib/containerd` directories and the `/opt/deckhouse/bin/containerd` file.

- Known vulnerabilities have been fixed in the following modules: `loki`, `extended-monitoring`, `operator-prometheus`, `prometheus`, `prometheus-metrics-adapter`, `user-authn`, and `cloud-provider-zvirt`.

### Network

- Added support for Istio version 1.25.2, which uses the Sail operator instead of the deprecated Istio Operator. Also added support for Kiali version 2.7, without Ambient Mesh support. Istio version 1.19 is now considered deprecated.

- Added support for encrypting traffic between nodes and Pods using the WireGuard protocol (via the [`encryption.mode`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.71/modules/cni-cilium/configuration.html#parameters-encryption-mode) parameter).

- Fixed the logic for determining service readiness in the [ServiceWithHealthcheck](https://deckhouse.io/products/kubernetes-platform/documentation/v1.71/modules/service-with-healthchecks/cr.html#servicewithhealthchecks) resource. Previously, Pods without an IP address (for example, in `Pending` state) could be mistakenly included in the load balancing list.

- Added support for the least-conn load balancing algorithm. This algorithm directs traffic to the service backend with the fewest active connections, improving performance for connection-heavy applications (such as WebSocket services). To use this algorithm, enable the [`extraLoadBalancerAlgorithmsEnabled`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.71/modules/cni-cilium/configuration.html#parameters-extraloadbalanceralgorithmsenabled) parameter in the `cni-cilium` module settings and use the `service.cilium.io/lb-algorithm` annotation on the service and set it to a supported value: random, maglev, or least-conn.

- Fixed an issue in Cilium 1.17 `cilium-operator` where IP addresses were not reused after a `CiliumEndpoint` was deleted. The issue was caused by improper cleanup of priority filters, which could lead to IP pool exhaustion in large clusters.

- Refined the [list of ports used for networking](https://deckhouse.io/products/kubernetes-platform/documentation/v1.71/network_security_setup.html):
  - Added and updated:
    - `4287/UDP`: WireGuard port used for CNI Cilium traffic encryption.
    - `4295-4297/UDP`: Used by the `cni-cilium` module for VXLAN encapsulation of inter-pod traffic in multiple nested virtualization — when DKP with the `virtualization` module enabled is deployed inside virtual machines that are also created in DKP with the `virtualization` module enabled.
    - `4298/UDP`: Used by the `cni-cilium` module for VXLAN encapsulation of traffic between pods if the cluster was deployed on DKP version starting from **1.71** (for clusters deployed on DKP versions up to **1.71**, see the note for ports `4299/UDP`, `8469/UDP`, and `8472/UDP`).
    - `4299/UDP`: Port **for clusters deployed on DKP versions 1.64–1.70.** Used by the `cni-cilium` module for VXLAN encapsulation of traffic between pods. Updating DKP to newer versions will not change the port used unless the `virtualization` module is enabled.
    - `8469/UDP`: Port **for clusters deployed on DKP version 1.63 and below with the `virtualization` module enabled prior to DKP version 1.63.** Used by the `cni-cilium` module for VXLAN encapsulation of traffic between pods. Updating DKP to newer versions will not change the occupied port
    - `8472/UDP`: Port **for clusters deployed on DKP version 1.63 and below.** Used by the `cni-cilium` module for VXLAN encapsulation of traffic between pods. Updating DKP to newer versions will not change the occupied port if the `virtualization` module is not enabled. **Note** that in such clusters, enabling the `virtualization` module on DKP before version 1.70 changes the port:
      - Enabling the `virtualization` module on DKP version 1.63 and below will change it to `8469/UDP` and will not change with subsequent DKP updates
      - Enabling the `virtualization` module on DKP starting from version 1.64 will change it to `4298/UDP` and will not change with subsequent DKP updates
  - Removed:
    - `49152`, `49153/TCP`: Previously used for live migration of virtual machines (in the `virtualization` module). Migration now occurs over the Pod network.

### Component version updates

The following DKP components have been updated:

- `cilium`: 1.17.4
- `golang.org/x/net`: v0.40.0
- `etcd`: v3.6.1
- `terraform-provider-azure`: 3.117.1
- `Deckhouse CLI`: 0.13.2
- `Falco`: 0.41.1
- `falco-ctl`: 0.11.2
- `gcpaudit`: v0.6.0
- `Grafana`: 10.4.19
- `Vertical pod autoscaler`: 1.4.1
- `dhctl-kube-client`: v1.3.1
- `cloud-provider-dynamix dynamix-common`: v0.5.0
- `cloud-provider-dynamix capd-controller-manager`: v0.5.0
- `cloud-provider-dynamix cloud-controller-manager`: v0.4.0
- `cloud-provider-dynamix cloud-data-discoverer`: v0.6.0
- `cloud-provider-huaweicloud huaweicloud-common`: v0.5.0
- `cloud-provider-huaweicloud caphc-controller-manager`: v0.3.0
- `cloud-provider-huaweicloud cloud-data-discoverer`: v0.5.0
- `registry-packages-containerdv2`: 2.1.3
- `registry-packages-containerdv2-runc`: 1.3.0
- `cilium`: 1.17.4
- `cilium envoy-bazel`: 6.5.0
- `cilium cni-plugins`: 1.7.1
- `cilium protoc`: 30.2
- `cilium grpc-go`: 1.5.1
- `cilium protobuf-go`: 1.36.6
- `cilium protoc-gen-go-json`: 1.5.0
- `cilium gops`: 0.3.27
- `cilium llvm`: 18.1.8
- `cilium llvm-build-cache`: llvmorg-18.1.8-alt-p11-gcc11-v2-180225
- `User-authn basic-auth-proxy go`: 1.23.0
- `Prometheus alerts-reciever go`: 1.23.0
- `Prometheus memcached_exporter`: 0.15.3
- `Prometheus mimir`: 2.14.3
- `Prometheus promxy`: 0.0.93
- `Extended-monitoring k8s-image-availability-exporter`: 0.13.0
- `Extended-monitoring x509-certificate-exporter`: 3.19.1
- `Cilium-hubble hubble-ui`: 0.13.2
- `Cilium-hubble hubble-ui-frontend-assets`: 0.13.2

## Version 1.70

### Important

- The `ceph-csi` module has been removed. Use the `csi-ceph` module instead. Deckhouse will not be updated as long as `ceph-csi` is enabled in the cluster. For `csi-ceph` migration instructions, refer to the [module documentation](https://deckhouse.io/products/kubernetes-platform/modules/csi-ceph/stable/).

- Version 1.12 of the NGINX Ingress Controller has been added. The default controller version has been changed to 1.10. All Ingress controllers that do not have an explicitly specified version (via the [`controllerVersion`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.70/modules/ingress-nginx/cr.html#ingressnginxcontroller-v1-spec-controllerversion) parameter in the IngressNginxController resource or the [`defaultControllerVersion`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.70/modules/ingress-nginx/configuration.html#parameters-defaultcontrollerversion) parameter in the `ingress-nginx` module) will be restarted.

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

- The `deckhouse-controller collect-debug-info` command now also collects [debug information](https://deckhouse.io/products/kubernetes-platform/documentation/v1.70/modules/deckhouse/faq.html#how-to-collect-debug-info) for `Istio`, including:
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

- It is now possible to use stronger or more modern encryption algorithms (such as `RSA-3072`, `RSA-4096`, or `ECDSA-P256`) for control plane cluster certificates instead of the default `RSA-2048`. You can use the [`encryptionAlgorithm`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.70/installing/configuration.html#clusterconfiguration-encryptionalgorithm) parameter in the ClusterConfiguration resource to configure this.

- The `descheduler` module can now be configured to evict pods that are using local storage. Use the [`evictLocalStoragePods`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.70/modules/descheduler/cr.html#descheduler-v1alpha2-spec-evictlocalstoragepods) parameter in the module configuration to adjust this.

- You can now adjust the logging level of the Ingress controller using the [`controllerLogLevel`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.70/modules/ingress-nginx/cr.html#ingressnginxcontroller-v1-spec-controllerloglevel) parameter in the IngressNginxController resource. The default log level is `Info`. Controlling the logging level can help prevent log collector overload during Ingress controller restarts.

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

- The DexAuthenticator resource now has a [`highAvailability`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.71/modules/user-authn/cr.html#dexauthenticator-v1-spec-highavailability) parameter
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
