---
title: "Control plane components"
permalink: en/virtualization-platform/documentation/admin/platform-management/control-plane-settings/control-plane-components.html
---

## Control plane components

Control plane components are responsible for basic cluster operations (e.g. scheduling) and also handle cluster events (e.g. starting a new pod when the number of replicas in the Deployment configuration does not match the number of replicas running).

Control plane components can be run on any machine in the cluster. However, to simplify cluster setup and maintenance, all control plane components are run on dedicated nodes where user containers are not allowed to run. When components are run on a single node, this is a `single-master` configuration. Running components on multiple nodes puts the control plane in `multi-master` high availability mode.

### kube-apiserver

API server is a Kubernetes control plane component that exposes the Kubernetes API to clients.

The primary implementation of the Kubernetes API server is kube-apiserver. kube-apiserver can be horizontally scaled, i.e. deployed across multiple instances. You can run multiple instances of kube-apiserver and load balance traffic between these instances on different nodes.

The `control-plane-manager` module makes it easy to configure kube-apiserver to enable [audit](audit.html) mode.

### etcd

A distributed and highly reliable key-value store that is used as the primary storage for all cluster data in Kubernetes.

Like kube-apiserver, it supports running in multiple instances to provide high availability, forming an etcd cluster.

### kube-scheduler

A component that monitors pods without an affinity node and selects the node on which they should be launched.

When scheduling pod deployments on nodes, many factors are taken into account, including resource requirements, constraints related to hardware or software policies, affinity and anti-affinity of nodes/pods, data location, deadlines.

In cases where the general algorithm is not enough (for example, it is necessary to take into account the storageClass of the connected PVCs), the scheduler algorithm can be extended using plugins.

More details about the scheduler's algorithm and connecting plugins are described in the [Scheduler](scheduler.html) section.

### kube-controller manager

A component that runs the embedded resource controller processes. Each controller is a separate process, but for simplicity, all controllers are compiled into a single binary and run in a single process.

These controllers include:

- Node Controller: Notifies and responds to node failures.
- Replication Controller: Maintains the correct number of pods for each replication controller object in the system.
- Endpoints Controller: Populates the Endpoints object, i.e., binds Services to pods.
- Account & Token Controllers: Creates default accounts and API access tokens for new namespaces.

<!-- TODO is there anything to add about configuration, is it even required from an admin? -->

### cloud-controller manager

A component that launches controllers that interact with major cloud providers.
Not covered in the DVP documentation.

<!-- TODO do you need some link about cloud-controller-manager, for example to some module? -->

## Control plane component management

The listed control plane components are managed using the `control-plane-manager` module, which runs on all master nodes of the cluster (nodes with the `node-role.kubernetes.io/control-plane: ""` label).

The `control-plane-manager` module functions:

- **Manage certificates** required for the components to operate, including renewal, release when changing the configuration, etc. Allows you to automatically maintain a secure control plane configuration and quickly add additional names (SAN) to organize secure access to the Kubernetes API.
- **Configure components**. Automatically creates the necessary configurations and manifests of control plane components.
- **Upgrade or downgrade components**. Maintains the same versions of components in the cluster.
- **Manage the configuration of the etcd cluster and its nodes**. Scales etcd by the number of master nodes, migrates from a single-master cluster to a multi-master cluster and vice versa.
- **Configuring kubeconfig**. Ensures that kubectl always has an up-to-date configuration. Generates, extends, updates kubeconfig with cluster-admin rights and creates a symlink for the root user so that kubeconfig is used by default.
- **Extending the scheduler** by connecting external plugins via webhooks. Managed by the [KubeSchedulerWebhookConfiguration](../../../../reference/cr/kubeschedulerwebhookconfiguration.html) resource. Allows you to use more complex logic when solving load planning problems in a cluster. For example:
- placing data storage organization application pods closer to the data itself,
- prioritizing nodes depending on their state (network load, storage subsystem state, etc.),
- dividing nodes into zones, etc.
- **Backup copies of settings** are saved in the `/etc/kubernetes/deckhouse/backup` directory.
-

### Certificate Management

The `control-plane-manager` module manages the lifecycle of control plane SSL certificates:

- Root certificates for `kube-apiserver` and `etcd`. They are stored in the `d8-pki` secret of the `kube-system` namespace:
  - Kubernetes root CA (`ca.crt` and `ca.key`);
  - etcd root CA (`etcd/ca.crt` and `etcd/ca.key`);
  - RSA certificate and key for signing Service Account (`sa.pub` and `sa.key`);
  - Root CA for extension API servers (`front-proxy-ca.key` and `front-proxy-ca.crt`).
- Client and server certificates for connecting control plane components to each other. Certificates are issued, renewed, and reissued if something changes (e.g. the SAN list). The following certificates are stored only on nodes:
  - API server server certificate (`apiserver.crt` and `apiserver.key`);
  - client certificate for connecting `kube-apiserver` to `kubelet` (`apiserver-kubelet-client.crt` and `apiserver-kubelet-client.key`);
  - client certificate for connecting `kube-apiserver` to `etcd` (`apiserver-etcd-client.crt` and `apiserver-etcd-client.key`);
  - client certificate for connecting `kube-apiserver` to extension API servers (`front-proxy-client.crt` and `front-proxy-client.key`);
  - server certificate for `etcd` (`etcd/server.crt` and `etcd/server.key`);
  - client certificate for connecting `etcd` to other cluster members (`etcd/peer.crt` and `etcd/peer.key`);
  - client certificate for connecting `kubelet` to `etcd` for healthchecks (`etcd/healthcheck-client.crt` and `etcd/healthcheck-client.key`).

An additional SAN list can be added to certificates, which allows you to quickly and easily create additional "entry points" to the Kubernetes API.

When changing certificates, the corresponding kubeconfig configuration is automatically updated.

### Scaling components

This module configures the operation of control plane components in both `single-master` and `multi-master` configurations.

In a `single-master` configuration:

- `kube-apiserver` only uses the instance of `etcd` that is hosted on the same node;
- A proxy server is configured on the node that responds to localhost, and `kube-apiserver` responds to the master node's IP address.

In a `multi-master` configuration, control plane components are automatically deployed in high availability mode:

- `kube-apiserver` is configured to work with all instances of `etcd`.
- An additional proxy server is configured on each master node that responds to localhost. The proxy server by default accesses the local instance of `kube-apiserver`, but if it is unavailable, it sequentially queries other instances of `kube-apiserver`.

### Master node scaling

Control plane nodes are scaled automatically using the `node-role.kubernetes.io/control-plane=""` label:

- Setting the `node-role.kubernetes.io/control-plane=""` label on a node results in the deployment of the `control-plane` components on it, connecting a new `etcd` node to the etcd cluster, and regenerating the necessary certificates and configuration files.
- Removing the `node-role.kubernetes.io/control-plane=""` label from a node results in the removal of all `control-plane` components, regeneration of the necessary configuration files and certificates, and the correct exclusion of the node from the etcd cluster.

**Attention.** When scaling nodes from 2 to 1, [manual actions](/products/virtualization-platform/documentation/admin/platform-management/control-plane-settings/etcd.html#rebuild-class-etcd) with `etcd` are required.

Other operations with master nodes are discussed in the section [working with master nodes](/products/virtualization-platform/documentation/admin/platform-management/control-plane-settings/masters.html).

### Versioning

Updating the **patch version** of control plane components (i.e. within a minor version, e.g. from `1.29.13` to `1.29.14`) happens automatically when you upgrade the Deckhouse version. You cannot manage patch version upgrades.

Updating the **minor version** of control plane components (e.g. from `1.29.*` to `1.31.*`) can be managed using the [kubernetesVersion](/products/virtualization-platform/reference/cr/clusterconfiguration.html#clusterconfiguration-kubernetesversion) parameter, which allows you to select the automatic upgrade mode (`Automatic`) or specify the desired minor version. The version that is used by default (`kubernetesVersion: Automatic`) and the list of supported Kubernetes versions can be found in the [documentation](/products/kubernetes-platform/documentation/v1.66/supported_versions.html).

The control plane upgrade is performed safely for both `multi-master` and `single-master` configurations. During the upgrade, the API server may be briefly unavailable. The upgrade does not affect the operation of applications in the cluster and can be performed without allocating a maintenance window.

If the version specified for the upgrade (the [kubernetesVersion](/products/virtualization-platform/reference/cr/clusterconfiguration.html#clusterconfiguration-kubernetesversion) parameter) does not match the current version of the control plane in the cluster, a smart strategy for changing component versions is launched:

- General notes:
  - Upgrades in different NodeGroups are performed in parallel. Within each NodeGroup, nodes are upgraded sequentially, one at a time.
- When upgrading:
  - The upgrade occurs in **sequential stages**, one minor version at a time: 1.29 -> 1.30, 1.30 -> 1.31, 1.31 -> 1.32.
  - At each stage, the control plane version is first updated, then the kubelet on the cluster nodes is updated.
- When downgrading:
  - A successful downgrade is guaranteed only one version down from the highest minor control plane version ever used in the cluster.
  - The kubelet on the cluster nodes is downgraded first, then the control plane components are downgraded.

### Audit

To diagnose API operations, for example, in case of unexpected behavior of control plane components, Kubernetes provides a mode for logging API operations. This mode can be configured by creating [Audit Policy](https://kubernetes.io/docs/tasks/debug/debug-cluster/audit/#audit-policy) rules, and the result of the audit will be the log file `/var/log/kube-audit/audit.log` with all the operations of interest. For more details, see the [Auditing](https://kubernetes.io/docs/tasks/debug/debug-cluster/audit/) section of the Kubernetes documentation.

By default, Deckhouse clusters have basic audit policies:

- logging resource creation, deletion, and modification operations;
<!-- TODO: what resources are meant here? We should clarify. -->
- logging actions performed on behalf of service accounts from system namespaces: `kube-system`, `d8-*`;
- logging actions performed with resources in system namespaces: `kube-system`, `d8-*`.

You can disable basic policy logging by setting the [basicAuditPolicyEnabled](../../../../reference/mc.html#control-plane-manager-parameters-apiserver-basicauditpolicyenabled) flag to `false`.

Configuring audit policies is discussed in detail in the [Audit](audit.hmtl) section.

### Platform API interfaces description

The platform API provides an OpenAPI specification, which can be obtained via the `/openapi/v2` endpoint. To do this, run the command:

```bash
d8 k get --raw /openapi/v2 > swagger.json
```

This file can be used to view the documentation locally using tools such as Swagger UI or Redoc.
